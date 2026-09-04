package ingest

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// refColumns resolves a header row by column name, so a mirror that reorders or
// adds columns still reads correctly.
type refColumns map[string]int

func readRefHeader(r *csv.Reader, required ...string) (refColumns, error) {
	head, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	cols := make(refColumns, len(head))
	for i, name := range head {
		cols[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "\ufeff")))] = i
	}
	for _, name := range required {
		if _, ok := cols[name]; !ok {
			return nil, fmt.Errorf("missing required column %q", name)
		}
	}
	return cols, nil
}

func (c refColumns) get(rec []string, name string) string {
	i, ok := c[name]
	if !ok || i >= len(rec) {
		return ""
	}
	return strings.TrimSpace(rec[i])
}

// PlayerReader streams player biography rows.
type PlayerReader struct {
	csv  *csv.Reader
	cols refColumns
	rows int
}

// NewPlayerReader validates the header and prepares to stream.
func NewPlayerReader(r io.Reader) (*PlayerReader, error) {
	cr := csv.NewReader(r)
	cr.ReuseRecord = true
	cr.FieldsPerRecord = -1
	cols, err := readRefHeader(cr, "player_id")
	if err != nil {
		return nil, fmt.Errorf("player table: %w", err)
	}
	return &PlayerReader{csv: cr, cols: cols}, nil
}

// Next returns the next player, or io.EOF.
func (p *PlayerReader) Next() (PlayerBio, error) {
	for {
		rec, err := p.csv.Read()
		if err != nil {
			return PlayerBio{}, err
		}
		p.rows++

		bio := PlayerBio{
			SourceID:   p.cols.get(rec, "player_id"),
			FirstName:  p.cols.get(rec, "name_first"),
			LastName:   p.cols.get(rec, "name_last"),
			Country:    strings.ToUpper(p.cols.get(rec, "ioc")),
			WikidataID: p.cols.get(rec, "wikidata_id"),
		}
		if bio.SourceID == "" {
			continue // a blank line, not a player
		}
		// UNK is the source saying it does not know, which is a NULL, not a
		// country whose code happens to be UNK.
		if bio.Country == "UNK" || len(bio.Country) != 3 {
			bio.Country = ""
		}
		if hand := strings.ToUpper(p.cols.get(rec, "hand")); hand == "R" || hand == "L" || hand == "U" {
			bio.Hand = hand
		}
		if dob, ok := parseCompactDate(p.cols.get(rec, "dob")); ok {
			bio.BirthDate = &dob
		}
		if h, err := strconv.ParseInt(p.cols.get(rec, "height"), 10, 16); err == nil && h > 100 && h < 260 {
			height := int16(h)
			bio.HeightCm = &height
		}
		return bio, nil
	}
}

// Rows is how many records have been read.
func (p *PlayerReader) Rows() int { return p.rows }

// RankingReader streams ranking history.
type RankingReader struct {
	csv  *csv.Reader
	cols refColumns
	rows int
	bad  int
}

// NewRankingReader validates the header and prepares to stream.
func NewRankingReader(r io.Reader) (*RankingReader, error) {
	cr := csv.NewReader(r)
	cr.ReuseRecord = true
	cr.FieldsPerRecord = -1
	cols, err := readRefHeader(cr, "ranking_date", "rank", "player")
	if err != nil {
		return nil, fmt.Errorf("ranking file: %w", err)
	}
	return &RankingReader{csv: cr, cols: cols}, nil
}

// Next returns the next ranking, or io.EOF. Rows with an unusable date or rank
// are skipped and counted rather than guessed at.
func (r *RankingReader) Next() (RankingEntry, error) {
	for {
		rec, err := r.csv.Read()
		if err != nil {
			return RankingEntry{}, err
		}
		r.rows++

		date, ok := parseCompactDate(r.cols.get(rec, "ranking_date"))
		if !ok {
			r.bad++
			continue
		}
		rank, err := strconv.ParseInt(r.cols.get(rec, "rank"), 10, 32)
		if err != nil || rank < 1 {
			r.bad++
			continue
		}
		id := r.cols.get(rec, "player")
		if id == "" {
			r.bad++
			continue
		}

		entry := RankingEntry{Date: date, Rank: int32(rank), SourceID: id}
		// Points are absent for much of the early history, and zero points is a
		// real value, so this stays nullable.
		if p, err := strconv.ParseInt(r.cols.get(rec, "points"), 10, 32); err == nil {
			points := int32(p)
			entry.Points = &points
		}
		return entry, nil
	}
}

// Rows is how many records have been read.
func (r *RankingReader) Rows() int { return r.rows }

// Rejected is how many rows were unusable. Counted rather than dropped: a
// ranking file that is largely unparseable should be visible, not quiet.
func (r *RankingReader) Rejected() int { return r.bad }
