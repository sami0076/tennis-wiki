package charting

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sami0076/tennis-wiki/internal/ingest"
)

// Store is the persistence the loader needs, as an interface so the loader
// can run against a fake.
type Store interface {
	// Candidates returns one tour's matches whose event started in the range.
	Candidates(ctx context.Context, tour ingest.Tour, from, to time.Time) ([]Candidate, error)
	// Write attaches a charted match and its figures to the row it resolved
	// to, replacing whatever an earlier run attached under either key.
	Write(ctx context.Context, source string, m Match, r Resolution, stats []Stat) (int, error)
	// RecordUnresolved keeps the charted matches that found no row, by reason.
	RecordUnresolved(ctx context.Context, source, kind string, counts map[string]int) error
}

// PGStore is Store on Postgres.
type PGStore struct {
	pool *pgxpool.Pool
}

// NewStore wraps a pool.
func NewStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

// Candidates loads the rows a charted match might be. Team events are
// included: the project charts Davis Cup ties, and those are rows here too.
func (s *PGStore) Candidates(ctx context.Context, tour ingest.Tour, from, to time.Time) ([]Candidate, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.winner_id, m.loser_id, w.full_name, l.full_name, m.round, m.played_on
		  FROM matches m
		  JOIN tournaments t ON t.id = m.tournament_id
		  JOIN players w ON w.id = m.winner_id
		  JOIN players l ON l.id = m.loser_id
		 WHERE t.tour = $1::tour AND m.played_on BETWEEN $2 AND $3`, string(tour), from, to)
	if err != nil {
		return nil, fmt.Errorf("load candidates: %w", err)
	}
	defer rows.Close()

	var out []Candidate
	for rows.Next() {
		var c Candidate
		if err := rows.Scan(&c.MatchID, &c.WinnerID, &c.LoserID, &c.Winner, &c.Loser, &c.Round, &c.PlayedOn); err != nil {
			return nil, fmt.Errorf("load candidates: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Write is one transaction per charted match: the old attachment under either
// key goes, the new one lands whole or not at all.
func (s *PGStore) Write(ctx context.Context, source string, m Match, r Resolution, stats []Stat) (written int, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			err = errors.Join(err, rbErr)
		}
	}()

	if _, err := tx.Exec(ctx,
		`DELETE FROM charted_matches WHERE charting_id = $1 OR match_id = $2`, m.ID, r.MatchID); err != nil {
		return 0, fmt.Errorf("clear charted match: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO charted_matches (match_id, charting_id, played_on, charted_by, source)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5)`,
		r.MatchID, m.ID, m.PlayedOn, m.ChartedBy, source); err != nil {
		return 0, fmt.Errorf("write charted match %s: %w", m.ID, err)
	}

	for _, st := range stats {
		var playerID int64
		switch st.Player {
		case m.Player1:
			playerID = r.Player1ID
		case m.Player2:
			playerID = r.Player2ID
		default:
			continue // a name the match row does not carry; none in the files so far
		}
		f := st.Figures
		if _, err := tx.Exec(ctx, `
			INSERT INTO charted_stats (match_id, player_id, set_no,
			        serve_points, aces, double_faults, first_in, first_won, second_in, second_won,
			        bp_faced, bp_saved, return_points, return_points_won,
			        winners, winners_fh, winners_bh, unforced, unforced_fh, unforced_bh)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
			ON CONFLICT (match_id, player_id, set_no) DO NOTHING`,
			r.MatchID, playerID, st.Set,
			f.ServePoints, f.Aces, f.DoubleFaults, f.FirstIn, f.FirstWon, f.SecondIn, f.SecondWon,
			f.BPFaced, f.BPSaved, f.ReturnPoints, f.ReturnPointsWon,
			f.Winners, f.WinnersFH, f.WinnersBH, f.Unforced, f.UnforcedFH, f.UnforcedBH); err != nil {
			return 0, fmt.Errorf("write charted stats %s: %w", m.ID, err)
		}
		written++
	}

	// Resolved now, so no longer unresolved.
	if _, err := tx.Exec(ctx,
		`DELETE FROM unresolved_references WHERE source = $1 AND source_id = $2`, source, m.ID); err != nil {
		return 0, fmt.Errorf("clear unresolved: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return written, nil
}

// RecordUnresolved is the ingest store's, on this pool.
func (s *PGStore) RecordUnresolved(ctx context.Context, source, kind string, counts map[string]int) error {
	return ingest.NewStore(s.pool).RecordUnresolved(ctx, source, kind, counts)
}
