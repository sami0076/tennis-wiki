// Package ongoing keeps this week's provisional results: the source's ongoing
// files, fetched on their own schedule and replaced a file at a time. They
// never reach matches; the weekly load brings a finished event in from the
// season file (ADR-0016).
package ongoing

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sami0076/tennis-wiki/internal/db"
	"github.com/sami0076/tennis-wiki/internal/ingest"
)

// BaseURL is where the maintainer publishes; ADR-0002's amendment.
const BaseURL = "https://stats.tennismylife.org/data"

// File is one ongoing file and the tour it carries.
type File struct {
	Name string
	Tour db.Tour
}

// Files are the tour-level ones. The Challenger file exists too and is left
// out until something shows Challengers week by week.
var Files = []File{
	{Name: "ongoing_tourneys.csv", Tour: db.TourAtp},
	{Name: "wta_ongoing_tourneys.csv", Tour: db.TourWta},
}

// Opener fetches a file unless it matches the validator; ingest.HTTPFetcher is one.
type Opener interface {
	OpenPathIfChanged(ctx context.Context, baseURL, relPath, validator string) (io.ReadCloser, string, error)
}

// maxSkipped is far more bad rows than a week's file has matches.
const maxSkipped = 100

// Result is what one refresh did.
type Result struct {
	Changed bool
	Rows    int
	Skipped int
}

// Refresh fetches one file and, if it moved, replaces its rows in one
// transaction, so a reader sees the old week or the new one and never half.
func Refresh(ctx context.Context, pool *pgxpool.Pool, open Opener, baseURL string, f File) (Result, error) {
	q := db.New(pool)
	var validator string
	if prev, err := q.GetOngoingFile(ctx, f.Name); err == nil && prev.Etag != nil {
		validator = *prev.Etag
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Result{}, err
	}

	body, etag, err := open.OpenPathIfChanged(ctx, baseURL, f.Name, validator)
	if errors.Is(err, ingest.ErrUnchanged) {
		return Result{}, q.SaveOngoingFile(ctx, db.SaveOngoingFileParams{File: f.Name})
	}
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = body.Close() }()

	rows, skipped, err := read(f, body)
	if err != nil {
		return Result{}, err
	}

	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		qtx := db.New(tx)
		if err := qtx.DeleteOngoingMatches(ctx, f.Name); err != nil {
			return err
		}
		for _, row := range rows {
			if err := qtx.InsertOngoingMatch(ctx, row); err != nil {
				return fmt.Errorf("%s %s/%d: %w", f.Name, row.TourneySourceID, row.MatchNum, err)
			}
		}
		var tag *string
		if etag != "" {
			tag = &etag
		}
		return qtx.SaveOngoingFile(ctx, db.SaveOngoingFileParams{
			File: f.Name, Etag: tag, Rows: int32(len(rows)), Changed: true,
		})
	})
	return Result{Changed: true, Rows: len(rows), Skipped: skipped}, err
}

// read parses the file with the season files' own reader, keeping main-draw
// matches at tour level. A row the reader rejects is counted and skipped, as
// ingest does, rather than failing the week.
func read(f File, r io.Reader) ([]db.InsertOngoingMatchParams, int, error) {
	reader, err := ingest.NewReader(ingest.Source{Name: f.Name, Profile: "tml"}, r)
	if err != nil {
		return nil, 0, err
	}
	var out []db.InsertOngoingMatchParams
	skipped := 0
	for {
		m, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return out, skipped, nil
		}
		if err != nil {
			// A failed read repeats its error rather than reaching EOF.
			if skipped++; skipped > maxSkipped {
				return nil, skipped, fmt.Errorf("%s: unreadable: %w", f.Name, err)
			}
			slog.Warn("ongoing: row skipped", "error", err)
			continue
		}
		if m.Tier("tour") != "tour" || m.IsQualifying() || ingest.IsTeamEvent(m.Level) {
			continue
		}
		out = append(out, params(f, m))
	}
}

func params(f File, m ingest.MatchRow) db.InsertOngoingMatchParams {
	p := db.InsertOngoingMatchParams{
		File:            f.Name,
		Tour:            f.Tour,
		TourneySourceID: m.TourneyID,
		TourneyName:     m.TourneyName,
		Level:           m.Level,
		Indoor:          m.Indoor,
		MatchNum:        int32(m.MatchNum),
		Round:           roundCode(m.Round),
		PlayedOn:        m.TourneyDate,
		WinnerSourceID:  m.Winner.SourceID,
		WinnerName:      m.Winner.Name,
		WinnerSeed:      seed(m.Winner.Seed),
		LoserSourceID:   m.Loser.SourceID,
		LoserName:       m.Loser.Name,
		LoserSeed:       seed(m.Loser.Seed),
	}
	if m.Surface != "" {
		s := db.Surface(m.Surface)
		p.Surface = &s
	}
	if m.Score != "" {
		p.Score = &m.Score
	}
	return p
}

// roundCode puts the WTA ongoing file's words into the codes every other file
// uses: "Quarterfinals" there is QF everywhere else.
func roundCode(round string) string {
	switch r := strings.ToLower(strings.TrimSpace(round)); {
	case r == "final" || r == "finals":
		return "F"
	case strings.HasPrefix(r, "semi"):
		return "SF"
	case strings.HasPrefix(r, "quarter"):
		return "QF"
	case strings.HasPrefix(r, "round of "):
		return "R" + strings.TrimPrefix(r, "round of ")
	case strings.HasPrefix(r, "round robin"):
		return "RR"
	}
	return strings.TrimSpace(round)
}

func seed(s *int) *int16 {
	if s == nil {
		return nil
	}
	v := int16(*s)
	return &v
}
