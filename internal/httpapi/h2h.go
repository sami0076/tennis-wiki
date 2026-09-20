package httpapi

import (
	"errors"
	"net/http"
	"strconv"
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
	// tstype Pair, because a Go array of two marshals to a plain JSON array and
	// the generated type would otherwise promise only "some players" -- losing
	// the one guarantee this shape is built on.
	Players [2]HeadToHeadPlayer `json:"players" tstype:"Pair<HeadToHeadPlayer>"`
	Record  HeadToHeadRecord    `json:"record"`
	// Surfaces and Tiers exist because a 3-1 record that is really 3-1 at
	// Futures is a different claim from 3-1 at tour level.
	Surfaces []HeadToHeadSplit `json:"surfaces"`
	Tiers    []HeadToHeadSplit `json:"tiers"`
	Serve    [2]ServeStats     `json:"serve" tstype:"Pair<ServeStats>"`
	Meetings []Meeting         `json:"meetings"`
	// Filters echoes the cut the record, the splits, the serve figures and
	// the meetings are under; TotalMeetings is the rivalry's whole count, so
	// a page can write "3-1 in finals, of 40 meetings".
	Filters       HeadToHeadFilters `json:"filters"`
	TotalMeetings int               `json:"total_meetings"`
	// Closeness is the rivalry's summary and is never filtered: the meetings
	// that went the distance and the tiebreaks between them, and who won each.
	Closeness HeadToHeadCloseness `json:"closeness"`
}

// HeadToHeadFilters is the cut a comparison was read under.
type HeadToHeadFilters struct {
	Level     *string `json:"level"`
	Round     *string `json:"round"`
	BestOf    *int    `json:"best_of"`
	Surface   *string `json:"surface"`
	Deciders  bool    `json:"deciders"`
	Tiebreaks bool    `json:"tiebreaks"`
	From      *int    `json:"from"`
	To        *int    `json:"to"`
}

// HeadToHeadCloseness is how close the rivalry has been, from the scores.
type HeadToHeadCloseness struct {
	// Deciders is the meetings that reached a deciding set and who won them;
	// Scored is the finished meetings whose score could be read, which is
	// what it is a share of.
	Deciders HeadToHeadRecord `json:"deciders"`
	Scored   int              `json:"scored"`
	// Tiebreaks is every set tiebreak between the two and who won it.
	Tiebreaks HeadToHeadSplit `json:"tiebreaks"`
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
	Wins    [2]int `json:"wins" tstype:"Pair<number>"`
	// Incomplete counts retirements and walkovers. They belong in the record --
	// somebody advanced -- and are excluded from every rate.
	Incomplete int `json:"incomplete"`
}

// HeadToHeadSplit is the record inside one surface or one tier.
type HeadToHeadSplit struct {
	Name    string `json:"name"`
	Matches int    `json:"matches"`
	Wins    [2]int `json:"wins" tstype:"Pair<number>"`
}

// Meeting is one match between the two, from neither side.
type Meeting struct {
	Date       string  `json:"date"`
	Tournament string  `json:"tournament"`
	EventSlug  *string `json:"event_slug"`
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
	// ChartingID is the Match Charting Project's id when this meeting was
	// charted, and the key to /charted/{id}; null otherwise.
	ChartingID *string `json:"charting_id"`
	BestOf     int16   `json:"best_of"`
	// DecidingSet is whether the meeting went the distance; null where the
	// score could not be read or the match did not finish.
	DecidingSet *bool `json:"deciding_set"`
	// Tiebreaks is the set tiebreaks each side won in this meeting, in
	// Players order; null on the same terms as DecidingSet.
	Tiebreaks *[2]int `json:"tiebreaks" tstype:"Pair<number> | null"`
}

// meetingFilter is HeadToHeadFilters as a predicate over the rows.
type meetingFilter struct {
	HeadToHeadFilters
	bestOf int16
}

// parseMeetingFilter reads the cut from the query, validated the way the
// match history validates its own: a value outside the vocabulary is a 400,
// never silently everything.
func parseMeetingFilter(r *http.Request) (meetingFilter, string) {
	q := r.URL.Query()
	var f meetingFilter
	if raw := q.Get("level"); raw != "" {
		if !eventCategories[raw] {
			return f, "level must be slam, masters, finals, olympics, team, tour, challenger, futures or itf."
		}
		f.Level = &raw
	}
	if raw := q.Get("round"); raw != "" {
		switch raw {
		case "F", "SF", "QF", "R16", "R32", "R64", "R128", "RR", "Q":
			f.Round = &raw
		default:
			return f, "round must be F, SF, QF, R16, R32, R64, R128, RR or Q."
		}
	}
	if raw := q.Get("best_of"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || (n != 3 && n != 5) {
			return f, "best_of must be 3 or 5."
		}
		f.BestOf, f.bestOf = &n, int16(n)
	}
	if raw := q.Get("surface"); raw != "" {
		switch raw {
		case string(db.SurfaceHard), string(db.SurfaceClay), string(db.SurfaceGrass), string(db.SurfaceCarpet):
			f.Surface = &raw
		default:
			return f, "surface must be hard, clay, grass or carpet."
		}
	}
	for _, flag := range []struct {
		name string
		into *bool
	}{{"deciders", &f.Deciders}, {"tiebreaks", &f.Tiebreaks}} {
		switch raw := q.Get(flag.name); raw {
		case "", "false", "0":
		case "true", "1":
			*flag.into = true
		default:
			return f, flag.name + " must be true or false."
		}
	}
	for _, bound := range []struct {
		name string
		into **int
	}{{"from", &f.From}, {"to", &f.To}} {
		if raw := q.Get(bound.name); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < firstSeason || n > lastSeason {
				return f, bound.name + " must be a four-digit season."
			}
			*bound.into = &n
		}
	}
	if f.From != nil && f.To != nil && *f.From > *f.To {
		return f, "from must not be after to."
	}
	return f, ""
}

// keep says whether a meeting is inside the cut.
func (f meetingFilter) keep(row db.ListHeadToHeadMeetingsRow) bool {
	if f.Level != nil && categoryOf(row.Level, string(row.Tier)) != *f.Level {
		return false
	}
	if f.Round != nil {
		if *f.Round == "Q" {
			if !row.IsQualifying {
				return false
			}
		} else if row.Round != *f.Round || row.IsQualifying {
			return false
		}
	}
	if f.BestOf != nil && row.BestOf != f.bestOf {
		return false
	}
	if f.Surface != nil && (row.Surface == nil || string(*row.Surface) != *f.Surface) {
		return false
	}
	if f.Deciders && (row.DecidingSet == nil || !*row.DecidingSet) {
		return false
	}
	if f.Tiebreaks && (row.TiebreaksWinner == nil || row.TiebreaksLoser == nil ||
		*row.TiebreaksWinner+*row.TiebreaksLoser == 0) {
		return false
	}
	if f.From != nil && int(row.Season) < *f.From {
		return false
	}
	if f.To != nil && int(row.Season) > *f.To {
		return false
	}
	return true
}

// categoryOf folds the two tours' level vocabularies into the index's list,
// the same way ListEvents does in SQL.
func categoryOf(level, tier string) string {
	switch level {
	case "D":
		return "team"
	case "G":
		return "slam"
	case "M", "PM", "1000":
		return "masters"
	case "F":
		return "finals"
	case "O":
		return "olympics"
	}
	return tier
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

	filter, problem := parseMeetingFilter(r)
	if problem != "" {
		BadRequest(w, r, problem)
		return
	}

	rows, err := a.Queries.ListHeadToHeadMeetings(ctx, db.ListHeadToHeadMeetingsParams{
		PlayerA: first.ID, PlayerB: second.ID,
	})
	if err != nil {
		Internal(w, r, err)
		return
	}

	writeJSON(w, r, http.StatusOK, buildHeadToHead(first, second, rows, filter))
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
	first, second db.GetPlayerBySlugRow, rows []db.ListHeadToHeadMeetingsRow, filter meetingFilter,
) HeadToHead {
	h := HeadToHead{
		Players:       [2]HeadToHeadPlayer{headToHeadPlayer(first), headToHeadPlayer(second)},
		Meetings:      make([]Meeting, 0, len(rows)),
		Filters:       filter.HeadToHeadFilters,
		TotalMeetings: len(rows),
		Closeness:     HeadToHeadCloseness{Tiebreaks: HeadToHeadSplit{Name: "tiebreaks"}},
	}

	// The closeness summary is over every meeting, whatever the cut below.
	for _, row := range rows {
		side := 0
		if row.WinnerID == second.ID {
			side = 1
		}
		if row.DecidingSet != nil {
			h.Closeness.Scored++
			if *row.DecidingSet {
				h.Closeness.Deciders.Matches++
				h.Closeness.Deciders.Wins[side]++
			}
		}
		if row.TiebreaksWinner != nil && row.TiebreaksLoser != nil {
			won, lost := int(*row.TiebreaksWinner), int(*row.TiebreaksLoser)
			h.Closeness.Tiebreaks.Matches += won + lost
			h.Closeness.Tiebreaks.Wins[side] += won
			h.Closeness.Tiebreaks.Wins[1-side] += lost
		}
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
		if !filter.keep(row) {
			continue
		}
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
			EventSlug:   row.EventSlug,
			Tier:        string(row.Tier),
			Level:       row.Level,
			Season:      row.Season,
			Round:       row.Round,
			Qualifying:  row.IsQualifying,
			WinnerIndex: side,
			Score:       row.Score,
			Incomplete:  row.Incomplete,
			ChartingID:  row.ChartingID,
			BestOf:      row.BestOf,
			DecidingSet: row.DecidingSet,
		}
		if row.Surface != nil {
			value := string(*row.Surface)
			meeting.Surface = &value
		}
		if row.TiebreaksWinner != nil && row.TiebreaksLoser != nil {
			var pair [2]int
			pair[side], pair[1-side] = int(*row.TiebreaksWinner), int(*row.TiebreaksLoser)
			meeting.Tiebreaks = &pair
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
