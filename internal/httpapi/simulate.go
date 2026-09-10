package httpapi

import (
	"context"
	"errors"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/sami0076/tennis-wiki/internal/db"
	"github.com/sami0076/tennis-wiki/internal/rating"
	"github.com/sami0076/tennis-wiki/internal/simulate"
)

// Why a simulation could not be run. A pair the model cannot serve is a normal
// answer at this depth, not an error: most of 115,000 players have never been
// rated on a surface, and a 50/50 would be a coin flip wearing a model's
// clothes.
const (
	// SimulationDerived means the point probabilities came from the ratings,
	// which is the only path ADR-0007 provides.
	SimulationDerived = "elo_derived"
	// SimulationUnrated means one of the two has no rating to derive from.
	SimulationUnrated = "unrated"
	// SimulationNoAnchor means no serve statistics exist anywhere in that tour,
	// so there is nothing to pin the inversion's second degree of freedom on.
	SimulationNoAnchor = "no_anchor"
)

// SimulatedPlayer is one side of a simulation, and what was known about them.
type SimulatedPlayer struct {
	Slug    string  `json:"slug"`
	Name    string  `json:"name"`
	Tour    string  `json:"tour"`
	Country *string `json:"country"`
	// Elo is the blended rating actually used, null where they have none.
	Elo *float64 `json:"elo"`
	// SurfaceWeight is how much of Elo came from the surface series rather than
	// the overall one, so a clay rating built on five matches is visibly that.
	SurfaceWeight  float64 `json:"surface_weight"`
	SurfaceMatches int32   `json:"surface_matches"`
}

// SimulationChain is every rung between a point and a match.
type SimulationChain struct {
	Point [2]float64 `json:"point" tstype:"Pair<number>"`
	Hold  [2]float64 `json:"hold" tstype:"Pair<number>"`
	Set   [2]float64 `json:"set" tstype:"Pair<number>"`
	Match [2]float64 `json:"match" tstype:"Pair<number>"`
}

// SimulationInputs is where the numbers came from.
//
// ADR-0007 requires this: a simulation that will not say whether its point
// probability was observed or derived, and against what population, is the same
// failure as a "+4" against an unnamed average.
type SimulationInputs struct {
	Source string `json:"source"`
	// Anchor is the serve-point-win rate the inversion pinned the pair to, and
	// Scope how far the lookup had to widen to find it.
	Anchor       *float64 `json:"anchor"`
	AnchorScope  string   `json:"anchor_scope"`
	AnchorPoints int64    `json:"anchor_points"`
	Surface      string   `json:"surface"`
	Tier         string   `json:"tier"`
	Decade       int      `json:"decade"`
	// Expected is what the ratings predicted and Achieved what the chain
	// produced. They differ only where no plausible pair of serve
	// probabilities could reach the target, which is worth seeing rather than
	// smoothing over.
	Expected *float64 `json:"expected"`
	Achieved *float64 `json:"achieved"`
}

// MatchSimulation is the response for one hypothetical match.
type MatchSimulation struct {
	Players [2]SimulatedPlayer `json:"players" tstype:"Pair<SimulatedPlayer>"`
	BestOf  int                `json:"best_of"`
	Surface string             `json:"surface"`
	// Chain is null when the pair cannot be simulated, and Availability says
	// which of the two reasons applies.
	Chain        *SimulationChain `json:"chain"`
	Inputs       SimulationInputs `json:"inputs"`
	Availability string           `json:"availability"`
}

const simulateDefaultSurface = "hard"

func (a *API) handleSimulateMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	surface := query.Get("surface")
	if surface == "" {
		surface = simulateDefaultSurface
	}
	series, ok := parseRatingSurface(surface)
	if !ok || series == db.RatingSurfaceOverall {
		BadRequest(w, r, "surface must be hard, clay, grass or carpet.")
		return
	}

	bestOf := 3
	if raw := query.Get("best_of"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || (n != 3 && n != 5) {
			BadRequest(w, r, "best_of must be 3 or 5.")
			return
		}
		bestOf = n
	}

	var players [2]db.GetSimulationPlayerRow
	for i, name := range []string{"a", "b"} {
		slug := query.Get(name)
		if slug == "" {
			BadRequest(w, r, "Name two players, as a and b.")
			return
		}
		row, err := a.Queries.GetSimulationPlayer(ctx, slug)
		if errors.Is(err, pgx.ErrNoRows) {
			NotFound(w, r, "No player has that slug.")
			return
		}
		if err != nil {
			Internal(w, r, err)
			return
		}
		players[i] = row
	}
	if players[0].ID == players[1].ID {
		BadRequest(w, r, "A player cannot be simulated against themselves.")
		return
	}

	sim, err := a.simulateMatch(ctx, players, series, surface, bestOf)
	if err != nil {
		Internal(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, sim)
}

// simulateMatch blends both ratings, finds the anchor, and inverts the chain.
func (a *API) simulateMatch(
	ctx context.Context, players [2]db.GetSimulationPlayerRow,
	series db.RatingSurface, surface string, bestOf int,
) (MatchSimulation, error) {
	sim := MatchSimulation{BestOf: bestOf, Surface: surface, Availability: SimulationDerived}

	var elos [2]float64
	rated := true
	for i, p := range players {
		out := SimulatedPlayer{
			Slug: p.Slug, Name: p.FullName, Tour: p.Tour, Country: p.Country,
		}
		rows, err := a.Queries.GetPlayerRatings(ctx, p.ID)
		if err != nil {
			return sim, err
		}
		elo, weight, matches, ok := blendedRating(rows, series)
		if ok {
			out.Elo, out.SurfaceWeight, out.SurfaceMatches = &elo, weight, matches
			elos[i] = elo
		} else {
			rated = false
		}
		sim.Players[i] = out
	}

	// The context the anchor is drawn from: the better of the two levels, and
	// the later of the two careers. A tour player against a Futures player is a
	// tour-level hypothetical, and it is the more recent era that a reader
	// imagines the match being played in.
	tier := betterTier(players[0].BestTier, players[1].BestTier)
	season := players[0].LastSeason
	if players[1].LastSeason > season {
		season = players[1].LastSeason
	}
	decade := (int(season) / 10) * 10

	cells, err := a.serveBaseline(ctx, players[0].Tour)
	if err != nil {
		return sim, err
	}
	anchor, scope, points := simulate.Anchor(cells, tier, surface, decade)

	sim.Inputs = SimulationInputs{
		Source: SimulationDerived, AnchorScope: string(scope), AnchorPoints: points,
		Surface: surface, Tier: tier, Decade: decade,
	}
	if scope != simulate.ScopeNone {
		sim.Inputs.Anchor = &anchor
	}

	switch {
	case !rated:
		sim.Availability = SimulationUnrated
		return sim, nil
	case scope == simulate.ScopeNone:
		sim.Availability = SimulationNoAnchor
		return sim, nil
	}

	format := simulate.Format{BestOf: bestOf}
	expected := rating.Expected(elos[0], elos[1])
	pA, pB, achieved := simulate.Invert(expected, anchor, format)
	chain := simulate.Solve(pA, pB, format)

	sim.Inputs.Expected, sim.Inputs.Achieved = &expected, &achieved
	sim.Chain = &SimulationChain{
		Point: chain.Point, Hold: chain.Hold, Set: chain.Set, Match: chain.Match,
	}
	return sim, nil
}

// blendedRating is the surface series weighted against the overall one, as
// rating.Blend defines it. A player with no overall rating has no blend and no
// simulation.
func blendedRating(rows []db.GetPlayerRatingsRow, series db.RatingSurface) (
	elo, weight float64, matches int32, ok bool,
) {
	var overall *db.GetPlayerRatingsRow
	var surface *db.GetPlayerRatingsRow
	for i := range rows {
		switch rows[i].Surface {
		case db.RatingSurfaceOverall:
			overall = &rows[i]
		case series:
			surface = &rows[i]
		}
	}
	if overall == nil {
		return 0, 0, 0, false
	}
	if surface == nil {
		elo, weight = rating.BlendOptional(nil, overall.CurrentElo)
		return elo, weight, 0, true
	}
	elo, weight = rating.Blend(surface.CurrentElo, int(surface.MatchesPlayed), overall.CurrentElo)
	return elo, weight, surface.MatchesPlayed, true
}

// betterTier is the higher standard of the two, using the enum's own order:
// tour, challenger, futures, itf.
func betterTier(a, b string) string {
	order := map[string]int{"tour": 0, "challenger": 1, "futures": 2, "itf": 3}
	if order[a] <= order[b] {
		return a
	}
	return b
}

// serveBaseline reads the anchor cells for a tour.
func (a *API) serveBaseline(ctx context.Context, tour string) ([]simulate.Baseline, error) {
	rows, err := a.Queries.ListServeBaselines(ctx, db.Tour(tour))
	if err != nil {
		return nil, err
	}
	cells := make([]simulate.Baseline, 0, len(rows))
	for _, r := range rows {
		cells = append(cells, simulate.Baseline{
			Tier: r.Tier, Surface: r.Surface, Decade: int(r.Decade),
			ServePoints: r.ServePoints, ServeWon: r.ServeWon,
		})
	}
	return cells, nil
}

// DrawOdds is one entrant's chances.
type DrawOdds struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
	Seed *int   `json:"seed"`
	// Title and its 95% half-width. The interval travels with the figure
	// because a sampled probability without one is pretending to be exact.
	Title         float64 `json:"title"`
	TitleInterval float64 `json:"title_interval"`
	// Reached is the probability of winning each round, indexed as Rounds is.
	Reached []float64 `json:"reached"`
	// Rating is the blended figure the simulation used, as of the week the
	// event started rather than as of today.
	Rating *float64 `json:"rating"`
}

// DrawSimulation is a whole event, played many times.
type DrawSimulation struct {
	Event   SimulatedEvent   `json:"event"`
	Rounds  []string         `json:"rounds"`
	Odds    []DrawOdds       `json:"odds"`
	Runs    int              `json:"runs"`
	Seed    uint64           `json:"seed"`
	Inputs  SimulationInputs `json:"inputs"`
	Entered int              `json:"entered"`
	// Champion is who actually won it, so a simulation can be read against what
	// happened rather than only admired.
	Champion *string `json:"champion"`
}

// SimulatedEvent names the draw that was replayed.
type SimulatedEvent struct {
	Name    string `json:"name"`
	Season  int    `json:"season"`
	Tour    string `json:"tour"`
	Tier    string `json:"tier"`
	Surface string `json:"surface"`
	// RatingsAsOf is the week the event began. A draw simulation is always as
	// of then: using today's ratings would be reading the answer off the back
	// of the book.
	RatingsAsOf string `json:"ratings_as_of"`
}

// drawMaxRuns stops a caller asking for ten million and getting a worker
// instead of an answer. The interval barely moves past this.
const drawMaxRuns = 200_000

func (a *API) handleSimulateDraw(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	tour := query.Get("tour")
	if tour != string(db.TourAtp) && tour != string(db.TourWta) {
		BadRequest(w, r, "tour must be atp or wta.")
		return
	}
	season, err := strconv.Atoi(query.Get("season"))
	if err != nil || season < 1877 || season > 2100 {
		BadRequest(w, r, "season must be a year, for example 2019.")
		return
	}
	name := query.Get("event")
	if name == "" {
		BadRequest(w, r, "Name the event, for example event=Wimbledon.")
		return
	}

	runs := simulate.DefaultRuns
	if raw := query.Get("runs"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > drawMaxRuns {
			BadRequest(w, r, "runs must be between 1 and 200000.")
			return
		}
		runs = n
	}
	var seed uint64 = 1
	if raw := query.Get("seed"); raw != "" {
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			BadRequest(w, r, "seed must be a whole number.")
			return
		}
		seed = n
	}

	event, err := a.Queries.FindTournament(ctx, db.FindTournamentParams{
		Tour: db.Tour(tour), Season: int16(season), Name: name,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		NotFound(w, r, "No event of that name was played that season.")
		return
	}
	if err != nil {
		Internal(w, r, err)
		return
	}

	sim, err := a.simulateDraw(ctx, event, runs, seed)
	if errors.Is(err, simulate.ErrNotPowerOfTwo) || errors.Is(err, simulate.ErrBrokenTree) ||
		errors.Is(err, simulate.ErrNoMatches) {
		WriteProblem(w, r, http.StatusUnprocessableEntity, TypeBadRequest,
			"That event's draw cannot be reconstructed: "+err.Error()+
				". Round-robin finals and draws with byes have no complete bracket.")
		return
	}
	if err != nil {
		Internal(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, sim)
}

func (a *API) simulateDraw(
	ctx context.Context, event db.FindTournamentRow, runs int, seed uint64,
) (DrawSimulation, error) {
	var out DrawSimulation

	rows, err := a.Queries.ListDrawMatches(ctx, event.ID)
	if err != nil {
		return out, err
	}

	matches := make([]simulate.BracketMatch, 0, len(rows))
	entrants := map[int64]simulate.Entrant{}
	for _, m := range rows {
		matches = append(matches, simulate.BracketMatch{
			Round: m.Round, RoundIdx: rating.RoundRank(m.Round),
			WinnerID: m.WinnerID, LoserID: m.LoserID,
		})
		entrants[m.WinnerID] = simulate.Entrant{
			PlayerID: m.WinnerID, Slug: m.WinnerSlug, Name: m.WinnerName,
			Seed: seedOf(m.WinnerSeed),
		}
		entrants[m.LoserID] = simulate.Entrant{
			PlayerID: m.LoserID, Slug: m.LoserSlug, Name: m.LoserName,
			Seed: seedOf(m.LoserSeed),
		}
	}

	bracket, err := simulate.BuildBracket(matches, entrants)
	if err != nil {
		return out, err
	}

	// As of the week it started. The ratings table is sparse, so this is each
	// player's last row at or before that date.
	asOf := event.StartDate
	ids := make([]int64, 0, bracket.Size())
	for _, e := range bracket.Entrants {
		ids = append(ids, e.PlayerID)
	}

	series := db.RatingSurfaceOverall
	if s, ok := parseRatingSurface(event.Surface); ok {
		series = s
	}
	overall, err := a.ratingsAsOf(ctx, ids, db.RatingSurfaceOverall, asOf)
	if err != nil {
		return out, err
	}
	surfaceRatings := overall
	if series != db.RatingSurfaceOverall {
		if surfaceRatings, err = a.ratingsAsOf(ctx, ids, series, asOf); err != nil {
			return out, err
		}
	}

	cells, err := a.serveBaseline(ctx, event.Tour)
	if err != nil {
		return out, err
	}
	decade := (int(event.Season) / 10) * 10
	anchor, scope, points := simulate.Anchor(cells, event.Tier, event.Surface, decade)

	// Every entrant's blended rating, once.
	elos := make(map[int64]float64, len(ids))
	for _, id := range ids {
		base, ok := overall[id]
		if !ok {
			continue
		}
		elo := base.elo
		if s, ok := surfaceRatings[id]; ok && series != db.RatingSurfaceOverall {
			elo, _ = rating.Blend(s.elo, s.matches, base.elo)
		}
		elos[id] = elo
	}

	format := simulate.Format{BestOf: 3}
	if event.Level == "G" {
		format.BestOf = 5
	}

	// An entrant with no rating as of that week -- a qualifier in their first
	// event -- is treated as even against anyone. Dropping them would change
	// the size of the draw, and inventing a rating for them would be worse.
	win := func(x, y simulate.Entrant) float64 {
		ex, okx := elos[x.PlayerID]
		ey, oky := elos[y.PlayerID]
		if !okx || !oky {
			return 0.5
		}
		target := rating.Expected(ex, ey)
		if scope == simulate.ScopeNone {
			return target
		}
		pA, pB, _ := simulate.Invert(target, anchor, format)
		return simulate.Solve(pA, pB, format).Match[0]
	}

	result := simulate.RunDraw(bracket, win, runs, seed)

	out = DrawSimulation{
		Event: SimulatedEvent{
			Name: event.Name, Season: int(event.Season), Tour: event.Tour,
			Tier: event.Tier, Surface: event.Surface,
			RatingsAsOf: asOf.Format(time.DateOnly),
		},
		Rounds:  bracket.Rounds,
		Runs:    result.Runs,
		Seed:    result.Seed,
		Entered: bracket.Size(),
		Inputs: SimulationInputs{
			Source: SimulationDerived, AnchorScope: string(scope), AnchorPoints: points,
			Surface: event.Surface, Tier: event.Tier, Decade: decade,
		},
		Odds: make([]DrawOdds, 0, len(result.Odds)),
	}
	if scope != simulate.ScopeNone {
		out.Inputs.Anchor = &anchor
	}
	if champion, ok := entrants[bracket.Champion]; ok {
		out.Champion = &champion.Slug
	}

	for _, o := range result.Odds {
		row := DrawOdds{
			Slug: o.Entrant.Slug, Name: o.Entrant.Name, Seed: o.Entrant.Seed,
			Title: round3(o.Title), TitleInterval: round3(o.TitleInterval),
			Reached: make([]float64, len(o.Reached)),
		}
		for i, v := range o.Reached {
			row.Reached[i] = round3(v)
		}
		if elo, ok := elos[o.Entrant.PlayerID]; ok {
			e := elo
			row.Rating = &e
		}
		out.Odds = append(out.Odds, row)
	}
	sortOddsByTitle(out.Odds)
	return out, nil
}

type asOfRating struct {
	elo     float64
	matches int
}

func (a *API) ratingsAsOf(
	ctx context.Context, ids []int64, series db.RatingSurface, on time.Time,
) (map[int64]asOfRating, error) {
	rows, err := a.Queries.CurrentEloAsOf(ctx, db.CurrentEloAsOfParams{
		Surface: series, OnDate: on, PlayerIds: ids,
	})
	if err != nil {
		return nil, err
	}
	out := make(map[int64]asOfRating, len(rows))
	for _, r := range rows {
		out[r.PlayerID] = asOfRating{elo: r.Elo, matches: int(r.MatchesPlayed)}
	}
	return out, nil
}

func seedOf(v *int16) *int {
	if v == nil {
		return nil
	}
	n := int(*v)
	return &n
}

// round3 keeps a probability at three places, which is the precision the
// interval justifies and the page shows.
func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}

// sortOddsByTitle puts the favourites first, which is the order a reader wants
// and the order the design shows.
func sortOddsByTitle(odds []DrawOdds) {
	sort.SliceStable(odds, func(i, j int) bool { return odds[i].Title > odds[j].Title })
}
