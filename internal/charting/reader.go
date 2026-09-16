package charting

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sami0076/tennis-wiki/internal/ingest"
)

// Counts summarises a read: rows seen equals kept plus rejected.
type Counts struct {
	Seen     int
	Kept     int
	Rejected int
}

var compactDate = regexp.MustCompile(`^\d{8}$`)

// ReadMatches reads a charting-*-matches.csv by header.
//
// The files are charted by hand, and a few rows are short a field or two: a
// team-event tie with the names left out shifts every later column left, and
// one row has the umpire in "Best of". A row whose Date is not eight digits,
// or whose id does not begin with that date, is rejected and counted rather
// than read as a match on some other day. The first clean row for an id wins;
// the Davis Cup tie that appears once broken and once whole keeps the whole.
func ReadMatches(tour ingest.Tour, r io.Reader) ([]Match, Counts, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true

	header, err := cr.Read()
	if err != nil {
		return nil, Counts{}, fmt.Errorf("read header: %w", err)
	}
	col, err := columns(header, "match_id", "Player 1", "Player 2", "Date", "Tournament", "Round", "Charted by")
	if err != nil {
		return nil, Counts{}, err
	}

	var (
		out   []Match
		stats Counts
		seen  = map[string]struct{}{}
	)
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, stats, fmt.Errorf("row %d: %w", stats.Seen+2, err)
		}
		stats.Seen++

		if len(rec) != len(header) {
			stats.Rejected++
			continue
		}
		id := strings.TrimSpace(rec[col["match_id"]])
		date := strings.TrimSpace(rec[col["Date"]])
		if !compactDate.MatchString(date) || !strings.HasPrefix(id, date) {
			stats.Rejected++
			continue
		}
		played, err := time.Parse("20060102", date)
		if err != nil {
			stats.Rejected++
			continue
		}
		if _, dup := seen[id]; dup {
			stats.Rejected++
			continue
		}
		seen[id] = struct{}{}

		out = append(out, Match{
			ID:        id,
			Tour:      tour,
			Player1:   strings.TrimSpace(rec[col["Player 1"]]),
			Player2:   strings.TrimSpace(rec[col["Player 2"]]),
			PlayedOn:  played,
			Event:     strings.TrimSpace(rec[col["Tournament"]]),
			Round:     strings.TrimSpace(rec[col["Round"]]),
			ChartedBy: strings.TrimSpace(rec[col["Charted by"]]),
		})
		stats.Kept++
	}
	return out, stats, nil
}

// figureColumns is the stats-Overview layout, in the file's order after
// match_id, player and set.
var figureColumns = []string{
	"serve_pts", "aces", "dfs", "first_in", "first_won", "second_in", "second_won",
	"bk_pts", "bp_saved", "return_pts", "return_pts_won",
	"winners", "winners_fh", "winners_bh", "unforced", "unforced_fh", "unforced_bh",
}

// ReadStats reads a charting-*-stats-Overview.csv by header. "Total" is set 0.
// A row with a figure that is not a small non-negative integer is rejected;
// none of the 83,670 rows read on 15 September 2026 was.
func ReadStats(r io.Reader) ([]Stat, Counts, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true

	header, err := cr.Read()
	if err != nil {
		return nil, Counts{}, fmt.Errorf("read header: %w", err)
	}
	col, err := columns(header, append([]string{"match_id", "player", "set"}, figureColumns...)...)
	if err != nil {
		return nil, Counts{}, err
	}

	var (
		out   []Stat
		stats Counts
	)
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, stats, fmt.Errorf("row %d: %w", stats.Seen+2, err)
		}
		stats.Seen++
		if len(rec) != len(header) {
			stats.Rejected++
			continue
		}

		set := 0
		if s := strings.TrimSpace(rec[col["set"]]); s != "Total" {
			n, err := strconv.Atoi(s)
			if err != nil || n < 1 || n > 5 {
				stats.Rejected++
				continue
			}
			set = n
		}

		var f [17]int16
		ok := true
		for i, name := range figureColumns {
			n, err := strconv.Atoi(strings.TrimSpace(rec[col[name]]))
			if err != nil || n < 0 || n > 1000 {
				ok = false
				break
			}
			f[i] = int16(n)
		}
		if !ok {
			stats.Rejected++
			continue
		}

		out = append(out, Stat{
			MatchID: strings.TrimSpace(rec[col["match_id"]]),
			Player:  strings.TrimSpace(rec[col["player"]]),
			Set:     set,
			Figures: Figures{
				ServePoints: f[0], Aces: f[1], DoubleFaults: f[2], FirstIn: f[3], FirstWon: f[4],
				SecondIn: f[5], SecondWon: f[6], BPFaced: f[7], BPSaved: f[8],
				ReturnPoints: f[9], ReturnPointsWon: f[10],
				Winners: f[11], WinnersFH: f[12], WinnersBH: f[13],
				Unforced: f[14], UnforcedFH: f[15], UnforcedBH: f[16],
			},
		})
		stats.Kept++
	}
	return out, stats, nil
}

// columns maps the wanted header names to their positions, and says which is
// missing rather than reading the wrong column by position.
func columns(header []string, want ...string) (map[string]int, error) {
	pos := make(map[string]int, len(header))
	for i, h := range header {
		pos[strings.TrimSpace(strings.TrimPrefix(h, "\ufeff"))] = i
	}
	out := make(map[string]int, len(want))
	for _, w := range want {
		i, ok := pos[w]
		if !ok {
			return nil, fmt.Errorf("column %q not in header", w)
		}
		out[w] = i
	}
	return out, nil
}
