package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// ChartedMatch is one match as the Match Charting Project's volunteers
// recorded it: the two players and their figures per set, with the match
// total as set 0. It is keyed by the project's own id, which is also how a
// match row says it is charted (ADR-0011).
type ChartedMatch struct {
	ChartingID string `json:"charting_id"`
	// PlayedOn is the day the match was played, which the project records and
	// the tour files do not.
	PlayedOn   string         `json:"played_on"`
	ChartedBy  *string        `json:"charted_by"`
	Tournament string         `json:"tournament"`
	Season     int16          `json:"season"`
	Round      string         `json:"round"`
	Score      *string        `json:"score"`
	Players    [2]ChartedSide `json:"players"`
	Sets       []ChartedSet   `json:"sets"`
}

// ChartedSide names a player of the charted match; the winner is first.
type ChartedSide struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// ChartedSet is one set's figures for both players, in Players order. Set 0
// is the match.
type ChartedSet struct {
	Set   int16              `json:"set"`
	Lines [2]*ChartedFigures `json:"lines"`
}

// ChartedFigures are the project's stats-Overview columns. Counts, not rates:
// the rates are the reader's to derive, with the denominator in view.
type ChartedFigures struct {
	ServePoints     int16 `json:"serve_points"`
	Aces            int16 `json:"aces"`
	DoubleFaults    int16 `json:"double_faults"`
	FirstIn         int16 `json:"first_in"`
	FirstWon        int16 `json:"first_won"`
	SecondIn        int16 `json:"second_in"`
	SecondWon       int16 `json:"second_won"`
	BPFaced         int16 `json:"bp_faced"`
	BPSaved         int16 `json:"bp_saved"`
	ReturnPoints    int16 `json:"return_points"`
	ReturnPointsWon int16 `json:"return_points_won"`
	Winners         int16 `json:"winners"`
	WinnersFH       int16 `json:"winners_fh"`
	WinnersBH       int16 `json:"winners_bh"`
	Unforced        int16 `json:"unforced"`
	UnforcedFH      int16 `json:"unforced_fh"`
	UnforcedBH      int16 `json:"unforced_bh"`
}

func (a *API) handleChartedMatch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	match, err := a.Queries.GetChartedMatch(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		NotFound(w, r, "No charted match with that id.")
		return
	}
	if err != nil {
		Internal(w, r, err)
		return
	}
	rows, err := a.Queries.ListChartedStats(r.Context(), id)
	if err != nil {
		Internal(w, r, err)
		return
	}

	out := ChartedMatch{
		ChartingID: match.ChartingID,
		PlayedOn:   match.PlayedOn.Format(time.DateOnly),
		ChartedBy:  match.ChartedBy,
		Tournament: match.Tournament,
		Season:     match.Season,
		Round:      match.Round,
		Score:      match.Score,
		Players: [2]ChartedSide{
			{Slug: match.WinnerSlug, Name: match.WinnerName},
			{Slug: match.LoserSlug, Name: match.LoserName},
		},
		Sets: []ChartedSet{},
	}
	bySet := map[int16]*ChartedSet{}
	for _, row := range rows {
		set := bySet[row.SetNo]
		if set == nil {
			out.Sets = append(out.Sets, ChartedSet{Set: row.SetNo})
			set = &out.Sets[len(out.Sets)-1]
			bySet[row.SetNo] = set
		}
		side := 0
		if row.PlayerID == match.LoserID {
			side = 1
		}
		set.Lines[side] = &ChartedFigures{
			ServePoints: row.ServePoints, Aces: row.Aces, DoubleFaults: row.DoubleFaults,
			FirstIn: row.FirstIn, FirstWon: row.FirstWon, SecondIn: row.SecondIn, SecondWon: row.SecondWon,
			BPFaced: row.BpFaced, BPSaved: row.BpSaved,
			ReturnPoints: row.ReturnPoints, ReturnPointsWon: row.ReturnPointsWon,
			Winners: row.Winners, WinnersFH: row.WinnersFh, WinnersBH: row.WinnersBh,
			Unforced: row.Unforced, UnforcedFH: row.UnforcedFh, UnforcedBH: row.UnforcedBh,
		}
	}

	writeJSON(w, r, http.StatusOK, out)
}
