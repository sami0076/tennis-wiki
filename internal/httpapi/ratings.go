package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// RatingPoint is one rating and the week it was taken.
type RatingPoint struct {
	Elo  float64 `json:"elo"`
	AsOf string  `json:"as_of"`
}

// SeriesRating is what a player is worth in one series.
//
// Current and peak are both here because a page needs both: a player who
// retired in 1983 has a current rating -- the last one they were given -- and
// it is a real number that would be the wrong one to lead with.
type SeriesRating struct {
	Surface string `json:"surface"`
	// Matches is how many matches feed this series, which is what says whether
	// a surface rating is worth as much as the overall one.
	Matches int32       `json:"matches"`
	Current RatingPoint `json:"current"`
	Peak    RatingPoint `json:"peak"`
}

// RatingSeries is a whole trajectory, for a chart.
type RatingSeries struct {
	Surface string `json:"surface"`
	// From and To are the range actually covered, which is not the range asked
	// for: a player has ratings for the weeks they played and no others.
	From   string        `json:"from"`
	To     string        `json:"to"`
	Points []RatingPoint `json:"points"`
}

// ratingSurfaces are the values of the rating_surface enum, which is the
// overall series alongside one per surface.
var ratingSurfaces = []db.RatingSurface{
	db.RatingSurfaceOverall,
	db.RatingSurfaceHard,
	db.RatingSurfaceClay,
	db.RatingSurfaceGrass,
	db.RatingSurfaceCarpet,
}

func parseRatingSurface(raw string) (db.RatingSurface, bool) {
	for _, s := range ratingSurfaces {
		if string(s) == raw {
			return s, true
		}
	}
	return "", false
}

// playerRatings reads the current and peak of every series a player has one in.
//
// A series the player never played is absent rather than 1500: an unplayed
// surface is not a rating of average, it is the absence of a rating, and the
// same rule that keeps a missing ace count from being a zero applies here.
func (a *API) playerRatings(r *http.Request, playerID int64) ([]SeriesRating, error) {
	rows, err := a.Queries.GetPlayerRatings(r.Context(), playerID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		// Everyone whose only matches were team events or walkovers: rated by
		// nothing, so the block is null rather than a list of base ratings.
		return nil, nil
	}

	out := make([]SeriesRating, 0, len(rows))
	for _, row := range rows {
		out = append(out, SeriesRating{
			Surface: string(row.Surface),
			Matches: row.MatchesPlayed,
			Current: RatingPoint{Elo: row.CurrentElo, AsOf: row.CurrentAsOf.Format(time.DateOnly)},
			Peak:    RatingPoint{Elo: row.PeakElo, AsOf: row.PeakAsOf.Format(time.DateOnly)},
		})
	}
	return out, nil
}

// RankingPoint is one published ranking and the week it was published.
type RankingPoint struct {
	Date string `json:"date"`
	Rank int32  `json:"rank"`
	// Points is null for the decades before the tours published them, which is
	// not a week the player scored nothing.
	Points *int32 `json:"points"`
}

// RankingHistory is the published ATP or WTA ranking over time, which is a
// different claim from the Elo series beside it: one is what the tour said, the
// other is what this project computes.
type RankingHistory struct {
	From   string         `json:"from"`
	To     string         `json:"to"`
	Best   *RankingPoint  `json:"best"`
	Points []RankingPoint `json:"points"`
}

func (a *API) handlePlayerRankingHistory(w http.ResponseWriter, r *http.Request) {
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

	params := db.ListPlayerRankingHistoryParams{PlayerID: player.ID}
	if err := dateRange(r, &params.FromDate, &params.ToDate); err != nil {
		BadRequest(w, r, err.Error())
		return
	}

	rows, err := a.Queries.ListPlayerRankingHistory(ctx, params)
	if err != nil {
		Internal(w, r, err)
		return
	}

	history := RankingHistory{Points: make([]RankingPoint, 0, len(rows))}
	for _, row := range rows {
		point := RankingPoint{
			Date: row.RankingDate.Format(time.DateOnly), Rank: row.Rank, Points: row.Points,
		}
		history.Points = append(history.Points, point)
		// Best is the lowest number, and the first week they reached it.
		if history.Best == nil || point.Rank < history.Best.Rank {
			best := point
			history.Best = &best
		}
	}
	if len(history.Points) > 0 {
		history.From = history.Points[0].Date
		history.To = history.Points[len(history.Points)-1].Date
	}

	writeJSON(w, r, http.StatusOK, history)
}

// dateRange reads the from and to bounds shared by the two series endpoints.
func dateRange(r *http.Request, from, to **time.Time) error {
	query := r.URL.Query()
	for _, bound := range []struct {
		name string
		into **time.Time
	}{{"from", from}, {"to", to}} {
		raw := query.Get(bound.name)
		if raw == "" {
			continue
		}
		parsed, err := time.Parse(time.DateOnly, raw)
		if err != nil {
			return errors.New(bound.name + " must be a date, as YYYY-MM-DD.")
		}
		*bound.into = &parsed
	}
	return nil
}

func (a *API) handlePlayerRatingSeries(w http.ResponseWriter, r *http.Request) {
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

	query := r.URL.Query()
	surface := db.RatingSurfaceOverall
	if raw := query.Get("surface"); raw != "" {
		parsed, ok := parseRatingSurface(raw)
		if !ok {
			BadRequest(w, r, "surface must be overall, hard, clay, grass or carpet.")
			return
		}
		surface = parsed
	}

	params := db.ListPlayerRatingSeriesParams{PlayerID: player.ID, Surface: surface}
	if err := dateRange(r, &params.FromDate, &params.ToDate); err != nil {
		BadRequest(w, r, err.Error())
		return
	}

	rows, err := a.Queries.ListPlayerRatingSeries(ctx, params)
	if err != nil {
		Internal(w, r, err)
		return
	}

	series := RatingSeries{Surface: string(surface), Points: make([]RatingPoint, 0, len(rows))}
	for _, row := range rows {
		series.Points = append(series.Points, RatingPoint{
			Elo: row.Elo, AsOf: row.AsOf.Format(time.DateOnly),
		})
	}
	// The covered range, not the range asked for. A player rated in nine weeks
	// of 1996 does not have a trajectory spanning the decade a caller requested.
	if len(series.Points) > 0 {
		series.From = series.Points[0].AsOf
		series.To = series.Points[len(series.Points)-1].AsOf
	}

	writeJSON(w, r, http.StatusOK, series)
}
