package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// errBadCount is the shape of every "that number is not usable" answer here.
var errBadCount = errors.New("must be a number inside the allowed range")

func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// TrajectoryLine is one player's series.
type TrajectoryLine struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
	// Position is where the player stands at the end of the window, which is
	// what decides whether a line is labelled or drawn as part of the field.
	Position int           `json:"position"`
	Points   []RatingPoint `json:"points"`
}

// Trajectories is a multi-series chart's worth of data: one line per player
// rather than one row per player, which is what a leaderboard is.
type Trajectories struct {
	Surface string           `json:"surface"`
	Tour    *string          `json:"tour"`
	From    string           `json:"from"`
	To      string           `json:"to"`
	Lines   []TrajectoryLine `json:"lines"`
}

const (
	trajectoryDefaultPlayers = 8
	trajectoryMaxPlayers     = 20
	trajectoryDefaultMonths  = 24
	trajectoryMaxMonths      = 240
)

// handleTrajectories serves the top few players' rating lines over a window.
//
// A separate endpoint rather than a mode of the rankings one: a leaderboard is
// a page of rows and this is a handful of lines, and squeezing both through one
// response shape would make each of them worse.
func (a *API) handleTrajectories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
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

	var tour *db.Tour
	switch raw := query.Get("tour"); raw {
	case "":
	case string(db.TourAtp), string(db.TourWta):
		t := db.Tour(raw)
		tour = &t
	default:
		BadRequest(w, r, "tour must be atp or wta.")
		return
	}

	players, err := parsePositiveInt(query.Get("players"), trajectoryDefaultPlayers, trajectoryMaxPlayers)
	if err != nil {
		BadRequest(w, r, "players must be between 1 and 20.")
		return
	}
	months, err := parsePositiveInt(query.Get("months"), trajectoryDefaultMonths, trajectoryMaxMonths)
	if err != nil {
		BadRequest(w, r, "months must be between 1 and 240.")
		return
	}

	asOf, err := a.Queries.EffectiveEloDate(ctx, db.EffectiveEloDateParams{
		Surface: surface, OnOrBefore: endOfTime, Tour: tour,
	})
	if err != nil && !isNoRows(err) {
		Internal(w, r, err)
		return
	}

	out := Trajectories{Surface: string(surface), Tour: tourName(tour), Lines: []TrajectoryLine{}}
	if asOf.IsZero() {
		writeJSON(w, r, http.StatusOK, out)
		return
	}

	from := asOf.AddDate(0, -months, 0)
	out.From, out.To = from.Format(time.DateOnly), asOf.Format(time.DateOnly)

	officialDate, err := a.Queries.EffectiveOfficialDate(ctx, db.EffectiveOfficialDateParams{
		Tour: tour, OnOrBefore: asOf,
	})
	if err != nil && !isNoRows(err) {
		Internal(w, r, err)
		return
	}

	leaders, err := a.Queries.ListEloRankings(ctx, db.ListEloRankingsParams{
		Surface:      surface,
		OnDate:       asOf,
		Since:        asOf.Add(-activeWindow),
		Tour:         tour,
		OfficialDate: officialDate,
		RowLimit:     int32(players),
	})
	if err != nil {
		Internal(w, r, err)
		return
	}
	if len(leaders) == 0 {
		writeJSON(w, r, http.StatusOK, out)
		return
	}

	ids := make([]int64, 0, len(leaders))
	lines := make(map[int64]*TrajectoryLine, len(leaders))
	for _, leader := range leaders {
		ids = append(ids, leader.PlayerID)
		line := &TrajectoryLine{
			Slug: leader.Slug, Name: leader.FullName, Position: int(leader.Position),
			Points: []RatingPoint{},
		}
		lines[leader.PlayerID] = line
		out.Lines = append(out.Lines, TrajectoryLine{})
	}

	rows, err := a.Queries.ListRatingTrajectories(ctx, db.ListRatingTrajectoriesParams{
		Surface: surface, PlayerIds: ids, FromDate: from, ToDate: asOf,
	})
	if err != nil {
		Internal(w, r, err)
		return
	}
	for _, row := range rows {
		line, ok := lines[row.PlayerID]
		if !ok {
			continue
		}
		line.Points = append(line.Points, RatingPoint{
			Elo: round1(row.Elo), AsOf: row.Week.Format(time.DateOnly),
		})
	}

	// Rebuilt in leaderboard order, so the first line is the first player and a
	// client can label the top few without sorting again.
	out.Lines = out.Lines[:0]
	for _, leader := range leaders {
		out.Lines = append(out.Lines, *lines[leader.PlayerID])
	}

	writeJSON(w, r, http.StatusOK, out)
}
