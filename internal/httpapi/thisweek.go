package httpapi

import (
	"net/http"
	"sort"
	"time"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// ThisWeek is the events being played now, from results so far. Provisional:
// the source's ongoing files, fetched hourly and kept apart from matches until
// the weekly load brings a finished event in (ADR-0016). Not live scores.
type ThisWeek struct {
	// CheckedAt is when the files were last asked for, ChangedAt when they
	// last moved. Both null before the first fetch.
	CheckedAt *string     `json:"checked_at"`
	ChangedAt *string     `json:"changed_at"`
	Events    []WeekEvent `json:"events"`
}

// WeekEvent is one event and how far it has got.
type WeekEvent struct {
	Tour    string  `json:"tour"`
	Name    string  `json:"name"`
	Level   string  `json:"level"`
	Surface *string `json:"surface"`
	// Round is the furthest round with a result in; "F" means it is decided.
	Round      string       `json:"round"`
	Matches    int          `json:"matches"`
	LastPlayed string       `json:"last_played"`
	Champion   *WeekPlayer  `json:"champion"`
	Latest     []WeekResult `json:"latest"`
}

// WeekResult is one finished match.
type WeekResult struct {
	// Date is the source's: the day played in the ATP file, the week's start
	// in the WTA file.
	Date   string     `json:"date"`
	Round  string     `json:"round"`
	Winner WeekPlayer `json:"winner"`
	Loser  WeekPlayer `json:"loser"`
	Score  *string    `json:"score"`
}

// WeekPlayer links to the player where the database knows them.
type WeekPlayer struct {
	Name string  `json:"name"`
	Slug *string `json:"slug"`
	Seed *int16  `json:"seed"`
}

const (
	weekLatest = 4
	// An event the source has not dropped a week after its last result is
	// over, whatever the file still says.
	weekStale = 7 * 24 * time.Hour
)

// weekRoundRank ranks how deep a round is; a round robin sits before the knockout
// rounds that follow it.
var weekRoundRank = map[string]int{
	"R128": 1, "R64": 2, "R32": 3, "R16": 4, "RR": 5, "QF": 6, "SF": 7, "F": 8,
}

// levelOrder puts the biggest events first.
var levelOrder = map[string]int{"G": 0, "F": 1, "M": 2, "1000": 2, "PM": 2, "500": 3, "P": 3}

func (a *API) handleThisWeek(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := a.Queries.ListOngoingMatches(ctx)
	if err != nil {
		Internal(w, r, err)
		return
	}
	files, err := a.Queries.ListOngoingFiles(ctx)
	if err != nil {
		Internal(w, r, err)
		return
	}
	out := buildThisWeek(rows, time.Now())
	for _, f := range files {
		out.CheckedAt = laterStamp(out.CheckedAt, f.CheckedAt.Time)
		out.ChangedAt = laterStamp(out.ChangedAt, f.ChangedAt.Time)
	}
	writeJSON(w, r, http.StatusOK, out)
}

func laterStamp(current *string, t time.Time) *string {
	s := t.UTC().Format(time.RFC3339)
	if current == nil || s > *current {
		return &s
	}
	return current
}

func weekPlayer(name string, slug string, seed *int16) WeekPlayer {
	p := WeekPlayer{Name: name, Seed: seed}
	if slug != "" {
		p.Slug = &slug
	}
	return p
}

func buildThisWeek(rows []db.ListOngoingMatchesRow, now time.Time) ThisWeek {
	type key struct{ tour, id string }
	events := map[key]*WeekEvent{}
	results := map[key][]db.ListOngoingMatchesRow{}
	var order []key

	for _, row := range rows {
		k := key{string(row.Tour), row.TourneySourceID}
		if _, ok := events[k]; !ok {
			e := &WeekEvent{Tour: string(row.Tour), Name: row.TourneyName, Level: row.Level}
			if row.Surface != nil {
				s := string(*row.Surface)
				e.Surface = &s
			}
			events[k] = e
			order = append(order, k)
		}
		results[k] = append(results[k], row)
	}

	out := ThisWeek{Events: []WeekEvent{}}
	for _, k := range order {
		e, played := events[k], results[k]
		sort.SliceStable(played, func(i, j int) bool {
			if !played[i].PlayedOn.Equal(played[j].PlayedOn) {
				return played[i].PlayedOn.After(played[j].PlayedOn)
			}
			return weekRoundRank[played[i].Round] > weekRoundRank[played[j].Round]
		})
		if now.Sub(played[0].PlayedOn) > weekStale {
			continue
		}
		e.Matches = len(played)
		e.LastPlayed = played[0].PlayedOn.Format(time.DateOnly)
		for _, m := range played {
			if weekRoundRank[m.Round] > weekRoundRank[e.Round] {
				e.Round = m.Round
			}
			if m.Round == "F" {
				champion := weekPlayer(m.WinnerName, m.WinnerSlug, m.WinnerSeed)
				e.Champion = &champion
			}
		}
		for _, m := range played[:min(weekLatest, len(played))] {
			e.Latest = append(e.Latest, WeekResult{
				Date:   m.PlayedOn.Format(time.DateOnly),
				Round:  m.Round,
				Winner: weekPlayer(m.WinnerName, m.WinnerSlug, m.WinnerSeed),
				Loser:  weekPlayer(m.LoserName, m.LoserSlug, m.LoserSeed),
				Score:  m.Score,
			})
		}
		out.Events = append(out.Events, *e)
	}

	sort.SliceStable(out.Events, func(i, j int) bool {
		li, lj := levelRank(out.Events[i].Level), levelRank(out.Events[j].Level)
		if li != lj {
			return li < lj
		}
		if out.Events[i].Tour != out.Events[j].Tour {
			return out.Events[i].Tour < out.Events[j].Tour
		}
		return out.Events[i].Name < out.Events[j].Name
	})
	return out
}

func levelRank(level string) int {
	if r, ok := levelOrder[level]; ok {
		return r
	}
	return 4
}
