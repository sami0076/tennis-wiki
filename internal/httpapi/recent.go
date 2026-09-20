package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// RecentFinals is what the most recent week of tennis came to: the finals of
// every event that began that week, both tours, and the date the data is
// current to. The site does not do live; this is the fastest way to show a
// visitor it is current without saying the word.
type RecentFinals struct {
	// Through is the date the data runs to, per tour: the same figure
	// /coverage reports.
	Through map[string]string `json:"through"`
	// Week is the Monday and Sunday shown. Requested is the week that was
	// asked for when the one shown is an earlier one, because the asked-for
	// week had no final: the off-season, said rather than hidden.
	Week      Week          `json:"week"`
	Requested *Week         `json:"requested"`
	Tier      string        `json:"tier"`
	Finals    []RecentFinal `json:"finals"`
	// Without names the tours with no final in the week shown.
	Without []string `json:"without"`
}

// Week is a Monday-to-Sunday span.
type Week struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// RecentFinal is one final: a link to the sheet and to both players.
type RecentFinal struct {
	Tour       string   `json:"tour"`
	Slug       *string  `json:"slug"`
	Name       string   `json:"name"`
	Season     int16    `json:"season"`
	Level      string   `json:"level"`
	Tier       string   `json:"tier"`
	Surface    *string  `json:"surface"`
	DrawSize   *int16   `json:"draw_size"`
	StartDate  string   `json:"start_date"`
	Champion   Opponent `json:"champion"`
	Finalist   Opponent `json:"finalist"`
	FinalScore *string  `json:"final_score"`
}

func (a *API) handleRecent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	tier := db.TierTour
	switch raw := query.Get("tier"); raw {
	case "":
	case string(db.TierTour), string(db.TierChallenger), string(db.TierFutures), string(db.TierItf):
		tier = db.Tier(raw)
	default:
		BadRequest(w, r, "tier must be tour, challenger, futures or itf.")
		return
	}

	current, err := a.currentThrough(r)
	if err != nil {
		Internal(w, r, err)
		return
	}
	// The data's edge, not today: the week to show is the last complete one
	// before the last match either tour has.
	through := ""
	for _, last := range current {
		if last > through {
			through = last
		}
	}
	if raw := query.Get("through"); raw != "" {
		parsed, err := time.Parse(time.DateOnly, raw)
		if err != nil {
			BadRequest(w, r, "through must be a date, for example 2026-09-07.")
			return
		}
		through = parsed.Format(time.DateOnly)
	}
	if through == "" {
		writeJSON(w, r, http.StatusOK, RecentFinals{Through: current, Tier: string(tier), Finals: []RecentFinal{}, Without: []string{}})
		return
	}
	edge, _ := time.Parse(time.DateOnly, through)

	// The last complete Monday-to-Sunday week ending on or before the edge.
	asked := weekEnding(lastSunday(edge))
	shown := asked
	rows, err := a.finalsIn(ctx, shown, tier)
	if err != nil {
		Internal(w, r, err)
		return
	}
	out := RecentFinals{Through: current, Tier: string(tier), Finals: []RecentFinal{}, Without: []string{}}
	if len(rows) == 0 {
		// The off-season: the most recent week that had a final, said so.
		monday, _ := time.Parse(time.DateOnly, asked.From)
		latest, err := a.Queries.LatestFinalStart(ctx, db.LatestFinalStartParams{Through: monday.AddDate(0, 0, -1), Tier: tier})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			Internal(w, r, err)
			return
		}
		if err == nil {
			shown = weekEnding(lastSunday(latest.AddDate(0, 0, 6)))
			out.Requested = &asked
			if rows, err = a.finalsIn(ctx, shown, tier); err != nil {
				Internal(w, r, err)
				return
			}
		}
	}
	out.Week = shown

	seen := map[string]bool{}
	for _, row := range rows {
		seen[row.Tour] = true
		out.Finals = append(out.Finals, RecentFinal{
			Tour: row.Tour, Slug: row.EventSlug, Name: row.Name, Season: row.Season, Level: row.Level, Tier: row.Tier,
			Surface: nonEmpty(row.Surface), DrawSize: row.DrawSize, StartDate: row.StartDate.Format(time.DateOnly),
			Champion:   Opponent{Slug: row.ChampionSlug, Name: row.ChampionName},
			Finalist:   Opponent{Slug: row.FinalistSlug, Name: row.FinalistName},
			FinalScore: row.FinalScore,
		})
	}
	for _, tour := range []string{string(db.TourAtp), string(db.TourWta)} {
		if !seen[tour] {
			out.Without = append(out.Without, tour)
		}
	}
	writeJSON(w, r, http.StatusOK, out)
}

func (a *API) finalsIn(ctx context.Context, week Week, tier db.Tier) ([]db.ListFinalsBetweenRow, error) {
	from, _ := time.Parse(time.DateOnly, week.From)
	to, _ := time.Parse(time.DateOnly, week.To)
	return a.Queries.ListFinalsBetween(ctx, db.ListFinalsBetweenParams{FromDate: from, ToDate: to, Tier: tier})
}

// lastSunday is the Sunday on or before a date.
func lastSunday(d time.Time) time.Time {
	return d.AddDate(0, 0, -int(d.Weekday()))
}

// weekEnding is the Monday-to-Sunday week that ends on a Sunday.
func weekEnding(sunday time.Time) Week {
	return Week{From: sunday.AddDate(0, 0, -6).Format(time.DateOnly), To: sunday.Format(time.DateOnly)}
}
