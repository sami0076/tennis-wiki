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
	for _, bound := range []struct {
		name string
		into **time.Time
	}{{"from", &params.FromDate}, {"to", &params.ToDate}} {
		raw := query.Get(bound.name)
		if raw == "" {
			continue
		}
		parsed, err := time.Parse(time.DateOnly, raw)
		if err != nil {
			BadRequest(w, r, bound.name+" must be a date, as YYYY-MM-DD.")
			return
		}
		*bound.into = &parsed
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
