package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// HeadToHead is one comparison between two players.
//
// Everything paired is a two-element array in the order the URL asked for, so
// /h2h/a/b and /h2h/b/a are the same comparison read from opposite ends rather
// than two pages that could disagree.
type HeadToHead struct {
	Players [2]HeadToHeadPlayer `json:"players"`
	Record  HeadToHeadRecord    `json:"record"`
	// Surfaces and Tiers exist because a 3-1 record that is really 3-1 at
	// Futures is a different claim from 3-1 at tour level.
	Surfaces []HeadToHeadSplit `json:"surfaces"`
	Tiers    []HeadToHeadSplit `json:"tiers"`
	Serve    [2]ServeStats     `json:"serve"`
	Meetings []Meeting         `json:"meetings"`
}

// HeadToHeadPlayer is enough of a player to head a column.
type HeadToHeadPlayer struct {
	Slug    string  `json:"slug"`
	Name    string  `json:"name"`
	Tour    string  `json:"tour"`
	Country *string `json:"country"`
}

// HeadToHeadRecord is the score between them.
type HeadToHeadRecord struct {
	Matches int    `json:"matches"`
	Wins    [2]int `json:"wins"`
	// Incomplete counts retirements and walkovers. They belong in the record --
	// somebody advanced -- and are excluded from every rate.
	Incomplete int `json:"incomplete"`
}

// HeadToHeadSplit is the record inside one surface or one tier.
type HeadToHeadSplit struct {
	Name    string `json:"name"`
	Matches int    `json:"matches"`
	Wins    [2]int `json:"wins"`
}

// Meeting is one match between the two, from neither side.
type Meeting struct {
	Date       string  `json:"date"`
	Tournament string  `json:"tournament"`
	Tier       string  `json:"tier"`
	Level      string  `json:"level"`
	Season     int16   `json:"season"`
	Round      string  `json:"round"`
	Qualifying bool    `json:"qualifying"`
	Surface    *string `json:"surface"`
	// WinnerIndex is 0 or 1, indexing Players. An index rather than a slug so
	// a caller cannot mix up which end of the comparison it is reading.
	WinnerIndex int     `json:"winner_index"`
	Score       *string `json:"score"`
	Incomplete  bool    `json:"incomplete"`
}

func (a *API) handleHeadToHead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	first, err := a.Queries.GetPlayerBySlug(ctx, chi.URLParam(r, "slug"))
	if errors.Is(err, pgx.ErrNoRows) {
		NotFound(w, r, "No player has that slug.")
		return
	}
	if err != nil {
		Internal(w, r, err)
		return
	}

	second, err := a.Queries.GetPlayerBySlug(ctx, chi.URLParam(r, "opponent"))
	if errors.Is(err, pgx.ErrNoRows) {
		NotFound(w, r, "No player has that slug.")
		return
	}
	if err != nil {
		Internal(w, r, err)
		return
	}

	if first.ID == second.ID {
		BadRequest(w, r, "A player cannot be compared with themselves.")
		return
	}

	rows, err := a.Queries.ListHeadToHeadMeetings(ctx, db.ListHeadToHeadMeetingsParams{
		PlayerA: first.ID, PlayerB: second.ID,
	})
	if err != nil {
		Internal(w, r, err)
		return
	}

	writeJSON(w, r, http.StatusOK, buildHeadToHead(first, second, rows))
}

func headToHeadPlayer(p db.GetPlayerBySlugRow) HeadToHeadPlayer {
	return HeadToHeadPlayer{
		Slug: p.Slug, Name: p.FullName, Tour: string(p.Tour), Country: p.Country,
	}
}

// serveTotals accumulates one player's serve line across the meetings.
type serveTotals struct {
	matches                                            int64
	aces, doubleFaults, servePoints, firstIn, firstWon int64
	secondWon, bpSaved, bpFaced                        int64
}

func (s *serveTotals) add(aces, df, points, firstIn, firstWon, secondWon, bpSaved, bpFaced *int16) {
	// serve_points is the column the ingest check hangs off: present means the
	// row carries a stat line, absent means it never did.
	if points == nil {
		return
	}
	s.matches++
	s.servePoints += int64(*points)
	for _, pair := range []struct {
		into  *int64
		value *int16
	}{
		{&s.aces, aces}, {&s.doubleFaults, df}, {&s.firstIn, firstIn},
		{&s.firstWon, firstWon}, {&s.secondWon, secondWon},
		{&s.bpSaved, bpSaved}, {&s.bpFaced, bpFaced},
	} {
		if pair.value != nil {
			*pair.into += int64(*pair.value)
		}
	}
}

// stats renders the totals, or explains why there are none.
//
// The explanation is drawn from the meetings themselves rather than from either
// career: two tour players who only ever met at a Futures event in 1989 have no
// statistics for reasons that have nothing to do with the rest of their lives.
func (s *serveTotals) stats(tiers map[db.Tier]struct{}, lastSeason int) ServeStats {
	out := ServeStats{MatchesWithData: s.matches}
	if s.matches == 0 {
		out.Availability = explainMeetings(tiers, lastSeason)
		return out
	}

	out.Availability = AvailabilityRecorded
	matches := float64(s.matches)
	out.Rates = &ServeRates{
		AcesPerMatch:         round1(float64(s.aces) / matches),
		DoubleFaultsPerMatch: round1(float64(s.doubleFaults) / matches),
		FirstServeIn:         percent(s.firstIn, s.servePoints),
		FirstServeWon:        percent(s.firstWon, s.firstIn),
		SecondServeWon:       percent(s.secondWon, s.servePoints-s.firstIn),
		BreakPointsSaved:     percent(s.bpSaved, s.bpFaced),
	}
	return out
}

// explainMeetings says why a set of meetings carries no serve line, using the
// same three-way vocabulary the profile does.
func explainMeetings(tiers map[db.Tier]struct{}, lastSeason int) string {
	if len(tiers) == 0 {
		return AvailabilityNotRecorded
	}

	everyTier := func(allowed ...db.Tier) bool {
		for tier := range tiers {
			if !containsTier(allowed, tier) {
				return false
			}
		}
		return true
	}

	if everyTier(db.TierFutures, db.TierItf) {
		return AvailabilityNeverForTier
	}
	if lastSeason < firstStatisticalSeason {
		return AvailabilityNeverInEra
	}
	if everyTier(db.TierChallenger, db.TierFutures, db.TierItf) &&
		lastSeason < challengerStatisticalSeason {
		return AvailabilityNeverInEra
	}
	return AvailabilityNotRecorded
}

func buildHeadToHead(
	first, second db.GetPlayerBySlugRow, rows []db.ListHeadToHeadMeetingsRow,
) HeadToHead {
	h := HeadToHead{
		Players:  [2]HeadToHeadPlayer{headToHeadPlayer(first), headToHeadPlayer(second)},
		Meetings: make([]Meeting, 0, len(rows)),
	}

	// Insertion order is kept so the splits come out oldest-surface-first
	// rather than in whatever order a map happened to iterate.
	var surfaceOrder, tierOrder []string
	surfaces := map[string]*HeadToHeadSplit{}
	tierSplits := map[string]*HeadToHeadSplit{}
	meetingTiers := map[db.Tier]struct{}{}

	var totals [2]serveTotals
	lastSeason := 0

	for _, row := range rows {
		side := 0
		if row.WinnerID == second.ID {
			side = 1
		}

		h.Record.Matches++
		h.Record.Wins[side]++
		if row.Incomplete {
			h.Record.Incomplete++
		}
		if int(row.Season) > lastSeason {
			lastSeason = int(row.Season)
		}
		meetingTiers[row.Tier] = struct{}{}

		// The source leaves surface blank for some events; "unknown" keeps the
		// matches visible instead of dropping them from the split.
		surface := "unknown"
		if row.Surface != nil {
			surface = string(*row.Surface)
		}
		addSplit(surfaces, &surfaceOrder, surface, side)
		addSplit(tierSplits, &tierOrder, string(row.Tier), side)

		totals[0].add(row.AAces, row.ADoubleFaults, row.AServePoints, row.AFirstIn,
			row.AFirstWon, row.ASecondWon, row.ABpSaved, row.ABpFaced)
		totals[1].add(row.BAces, row.BDoubleFaults, row.BServePoints, row.BFirstIn,
			row.BFirstWon, row.BSecondWon, row.BBpSaved, row.BBpFaced)

		meeting := Meeting{
			Date:        row.PlayedOn.Format(time.DateOnly),
			Tournament:  row.Tournament,
			Tier:        string(row.Tier),
			Level:       row.Level,
			Season:      row.Season,
			Round:       row.Round,
			Qualifying:  row.IsQualifying,
			WinnerIndex: side,
			Score:       row.Score,
			Incomplete:  row.Incomplete,
		}
		if row.Surface != nil {
			value := string(*row.Surface)
			meeting.Surface = &value
		}
		h.Meetings = append(h.Meetings, meeting)
	}

	h.Surfaces = collectSplits(surfaces, surfaceOrder)
	h.Tiers = collectSplits(tierSplits, tierOrder)
	h.Serve = [2]ServeStats{
		totals[0].stats(meetingTiers, lastSeason),
		totals[1].stats(meetingTiers, lastSeason),
	}
	return h
}

func addSplit(into map[string]*HeadToHeadSplit, order *[]string, name string, side int) {
	split, ok := into[name]
	if !ok {
		split = &HeadToHeadSplit{Name: name}
		into[name] = split
		*order = append(*order, name)
	}
	split.Matches++
	split.Wins[side]++
}

func collectSplits(from map[string]*HeadToHeadSplit, order []string) []HeadToHeadSplit {
	out := make([]HeadToHeadSplit, 0, len(order))
	for _, name := range order {
		out = append(out, *from[name])
	}
	return out
}
