package httpapi

import (
	"errors"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/sami0076/tennis-wiki/internal/db"
	"github.com/sami0076/tennis-wiki/internal/rating"
)

// Percentiles places one player against their tour over a year: each axis is
// the share of qualified players they rank above, 0 to 100, so a figure in
// aces and a figure in Elo can sit on one chart.
type Percentiles struct {
	Tour string `json:"tour"`
	// From (exclusive) and To bound the window. Both null when the
	// player never played a tour-level match, and Axes is then empty.
	From *string `json:"from"`
	To   *string `json:"to"`
	// Population is how many players cleared MinMatches in the window: what
	// every percentile here is a percentile of.
	Population int `json:"population"`
	MinMatches int `json:"min_matches"`
	// Matches is the player's own count, and Qualified whether it cleared the
	// floor. A player under it is still ranked, against a population they
	// are not part of.
	Matches   int64            `json:"matches"`
	Qualified bool             `json:"qualified"`
	Axes      []PercentileAxis `json:"axes" tstype:"PercentileAxis[] | null"`
}

// PercentileAxis is one spoke. Percentile and Value are null together, where
// the player had too little of it in the window to be measured.
type PercentileAxis struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	// Value is the figure ranked, in Unit: "percent", "elo", or "elo_change".
	// Clutch is itself a mean of percentiles and has Unit "percentile".
	Value      *float64 `json:"value"`
	Unit       string   `json:"unit"`
	Percentile *int     `json:"percentile"`
}

const (
	// percentileMinMatches is the leaderboard floor, for the same reason: it
	// is where the top of a season stops being two-match players.
	percentileMinMatches = leadersMinMatches
	// bigMatchesMin keeps a 1-0 record against the top ten off the top of the
	// spoke.
	bigMatchesMin        = 5
	percentileWindowDays = 364
	formWindowDays       = 182
)

// windowFigures is one player's raw figures over the window, the ratings
// alongside the totals. A nil figure is one they cannot be measured on.
type windowFigures struct {
	serve, ret                          *float64
	bpSaved, bpWon, tiebreaks, deciders *float64
	hard, clay, grass                   *float64
	form, big                           *float64
}

func totalsFigures(row db.ListTourWindowTotalsRow) windowFigures {
	f := windowFigures{
		serve:     share(row.ServeWon, row.ServePoints),
		ret:       share(row.ReturnWon, row.ReturnPoints),
		bpSaved:   share(row.BpSaved, row.BpFaced),
		bpWon:     share(row.BpWon, row.BpChances),
		tiebreaks: share(row.TiebreaksWon, row.TiebreaksPlayed),
		deciders:  share(row.DecidersWon, row.DecidersPlayed),
	}
	if row.BigPlayed >= bigMatchesMin {
		f.big = share(row.BigWon, row.BigPlayed)
	}
	return f
}

// percentileRank is the share of population below v, ties counted half, so
// the best of a population sits near 100 and a population of one at 50.
func percentileRank(v float64, population []float64) int {
	if len(population) == 0 {
		return 50
	}
	var below, equal int
	for _, p := range population {
		switch {
		case p < v:
			below++
		case p == v:
			equal++
		}
	}
	return int(math.Round(100 * (float64(below) + float64(equal)/2) / float64(len(population))))
}

// sortedValues collects one figure across the population, skipping those
// without it.
func sortedValues(pop []windowFigures, pick func(windowFigures) *float64) []float64 {
	out := make([]float64, 0, len(pop))
	for _, f := range pop {
		if v := pick(f); v != nil {
			out = append(out, *v)
		}
	}
	sort.Float64s(out)
	return out
}

// clutchScore is the mean of whichever of the four pressure figures a player
// has, each as a percentile of the population. Averaging raw rates would let
// deciding sets, which spread widest, decide the whole spoke.
func clutchScore(f windowFigures, components [4][]float64) *float64 {
	picks := [4]*float64{f.bpSaved, f.bpWon, f.tiebreaks, f.deciders}
	var sum float64
	var n int
	for i, v := range picks {
		if v != nil && len(components[i]) > 0 {
			sum += float64(percentileRank(*v, components[i]))
			n++
		}
	}
	if n < 2 {
		return nil
	}
	mean := sum / float64(n)
	return &mean
}

type axisSpec struct {
	key, label, unit string
	pick             func(windowFigures) *float64
}

var percentileAxes = []axisSpec{
	{"serve", "Serve", "percent", func(f windowFigures) *float64 { return f.serve }},
	{"return", "Return", "percent", func(f windowFigures) *float64 { return f.ret }},
	{"clutch", "Clutch", "percentile", nil},
	{"hard", "Hard", "elo", func(f windowFigures) *float64 { return f.hard }},
	{"clay", "Clay", "elo", func(f windowFigures) *float64 { return f.clay }},
	{"grass", "Grass", "elo", func(f windowFigures) *float64 { return f.grass }},
	{"form", "Form", "elo_change", func(f windowFigures) *float64 { return f.form }},
	{"big", "Big matches", "percent", func(f windowFigures) *float64 { return f.big }},
}

// rankAxes ranks one player's figures against the population's, axis by axis.
func rankAxes(player windowFigures, pop []windowFigures) []PercentileAxis {
	var components [4][]float64
	for i, pick := range []func(windowFigures) *float64{
		func(f windowFigures) *float64 { return f.bpSaved },
		func(f windowFigures) *float64 { return f.bpWon },
		func(f windowFigures) *float64 { return f.tiebreaks },
		func(f windowFigures) *float64 { return f.deciders },
	} {
		components[i] = sortedValues(pop, pick)
	}
	clutch := func(f windowFigures) *float64 { return clutchScore(f, components) }

	axes := make([]PercentileAxis, 0, len(percentileAxes))
	for _, spec := range percentileAxes {
		pick := spec.pick
		if pick == nil {
			pick = clutch
		}
		axis := PercentileAxis{Key: spec.key, Label: spec.label, Unit: spec.unit}
		if v := pick(player); v != nil {
			rounded := round1(*v)
			if spec.unit != "percent" {
				rounded = math.Round(*v)
			}
			rank := percentileRank(*v, sortedValues(pop, pick))
			axis.Value, axis.Percentile = &rounded, &rank
		}
		axes = append(axes, axis)
	}
	return axes
}

func (a *API) handlePlayerPercentiles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	player, err := a.Queries.GetPlayerBySlug(ctx, chi.URLParam(r, "slug"))
	if errors.Is(err, pgx.ErrNoRows) {
		NotFound(w, r, "No player has that slug.")
		return
	}
	if err != nil {
		Internal(w, r, err)
		return
	}

	out := Percentiles{
		Tour: string(player.Tour), MinMatches: percentileMinMatches, Axes: []PercentileAxis{},
	}
	to, err := a.Queries.GetPlayerLastTourMatch(ctx, player.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, r, http.StatusOK, out)
		return
	}
	if err != nil {
		Internal(w, r, err)
		return
	}
	from := to.AddDate(0, 0, -percentileWindowDays)
	out.From, out.To = formatDate(&from), formatDate(&to)

	rows, err := a.Queries.ListTourWindowTotals(ctx, db.ListTourWindowTotalsParams{
		Tour: player.Tour, FromDate: from, ToDate: to,
	})
	if err != nil {
		Internal(w, r, err)
		return
	}

	ids := []int64{player.ID}
	for _, row := range rows {
		if row.Matches >= percentileMinMatches && row.PlayerID != player.ID {
			ids = append(ids, row.PlayerID)
		}
	}
	ratings, err := a.windowRatings(r, ids, to)
	if err != nil {
		Internal(w, r, err)
		return
	}

	self := ratings[player.ID]
	pop := make([]windowFigures, 0, len(ids))
	for _, row := range rows {
		f := totalsFigures(row)
		rated := ratings[row.PlayerID]
		f.hard, f.clay, f.grass, f.form = rated.hard, rated.clay, rated.grass, rated.form
		if row.PlayerID == player.ID {
			self = f
			out.Matches = row.Matches
			out.Qualified = row.Matches >= percentileMinMatches
		}
		if row.Matches >= percentileMinMatches {
			pop = append(pop, f)
		}
	}
	out.Population = len(pop)
	out.Axes = rankAxes(self, pop)
	writeJSON(w, r, http.StatusOK, out)
}

// windowRatings reads the rating figures for a set of players at the end of
// a window: each surface blended with overall as the simulator blends it, and
// the overall change over the last half of the window.
func (a *API) windowRatings(
	r *http.Request, ids []int64, asOf time.Time,
) (map[int64]windowFigures, error) {
	read := func(surface db.RatingSurface, on time.Time) (map[int64]db.CurrentEloAsOfRow, error) {
		rows, err := a.Queries.CurrentEloAsOf(r.Context(), db.CurrentEloAsOfParams{
			Surface: surface, OnDate: on, PlayerIds: ids,
		})
		out := make(map[int64]db.CurrentEloAsOfRow, len(rows))
		for _, row := range rows {
			out[row.PlayerID] = row
		}
		return out, err
	}

	overall, err := read(db.RatingSurfaceOverall, asOf)
	if err != nil {
		return nil, err
	}
	earlier, err := read(db.RatingSurfaceOverall, asOf.AddDate(0, 0, -formWindowDays))
	if err != nil {
		return nil, err
	}
	var surfaces [3]map[int64]db.CurrentEloAsOfRow
	for i, s := range []db.RatingSurface{db.RatingSurfaceHard, db.RatingSurfaceClay, db.RatingSurfaceGrass} {
		if surfaces[i], err = read(s, asOf); err != nil {
			return nil, err
		}
	}

	out := make(map[int64]windowFigures, len(overall))
	for id, o := range overall {
		var f windowFigures
		blended := [3]**float64{&f.hard, &f.clay, &f.grass}
		for i, bySurface := range surfaces {
			var sr *rating.SurfaceRating
			if row, ok := bySurface[id]; ok {
				sr = &rating.SurfaceRating{Elo: row.Elo, Matches: int(row.MatchesPlayed)}
			}
			elo, _ := rating.BlendOptional(sr, o.Elo)
			*blended[i] = &elo
		}
		if then, ok := earlier[id]; ok {
			change := o.Elo - then.Elo
			f.form = &change
		}
		out[id] = f
	}
	return out, nil
}
