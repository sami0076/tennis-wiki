package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// rated gives a player an Elo in one series, as of a date the fixtures share.
func (f *apiFixture) rated(playerID int64, surface db.RatingSurface, elo float64, matches int) {
	f.t.Helper()
	if _, err := f.tx.Exec(f.ctx,
		`INSERT INTO ratings (player_id, as_of, surface, elo, matches_played)
		 VALUES ($1, '2019-01-07'::date, $2, $3, $4)`,
		playerID, surface, elo, matches); err != nil {
		f.t.Fatalf("insert rating: %v", err)
	}
}

// baselineCell gives the tour something to anchor on.
func (f *apiFixture) baselineCell(
	tour db.Tour, tier db.Tier, surface string, decade int, points, won int64,
) {
	f.t.Helper()
	if _, err := f.tx.Exec(f.ctx,
		`INSERT INTO serve_baselines (tour, tier, surface, decade, appearances,
		                              serve_points, serve_won)
		 VALUES ($1, $2, $3::surface, $4, 1000, $5, $6)`,
		tour, tier, surface, decade, points, won); err != nil {
		f.t.Fatalf("insert serve baseline: %v", err)
	}
}

func decodeMatchSimulation(t *testing.T, res *http.Response) MatchSimulation {
	t.Helper()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var sim MatchSimulation
	if err := json.NewDecoder(res.Body).Decode(&sim); err != nil {
		t.Fatalf("decode simulation: %v", err)
	}
	return sim
}

func TestSimulateMatchDerivesTheChainFromRatings(t *testing.T) {
	f := newAPIFixture(t)
	pts := int16(100)

	strong := f.player("itg-sim-strong", "Itg Simstrong", db.TourAtp)
	weak := f.player("itg-sim-weak", "Itg Simweak", db.TourAtp)
	open := f.tournament("itg-sim-open", db.TierTour, 2019)
	f.match(open, strong, weak, 1, "F", 2019, &pts, false)

	f.rated(strong, db.RatingSurfaceOverall, 2100, 400)
	f.rated(strong, db.RatingSurfaceClay, 2200, 200)
	f.rated(weak, db.RatingSurfaceOverall, 1900, 400)
	f.rated(weak, db.RatingSurfaceClay, 1850, 200)
	f.baselineCell(db.TourAtp, db.TierTour, "clay", 2010, 1_000_000, 610_000)

	sim := decodeMatchSimulation(t,
		f.get("/api/v1/simulate/match?a=itg-sim-strong&b=itg-sim-weak&surface=clay&best_of=5"))

	if sim.Availability != SimulationDerived {
		t.Fatalf("availability = %q, want the derived path", sim.Availability)
	}
	if sim.Chain == nil {
		t.Fatal("no chain for two rated players")
	}

	// The edge has to compound: a small serve advantage becomes a large one.
	c := *sim.Chain
	if !(c.Point[0]-c.Point[1] < c.Hold[0]-c.Hold[1] &&
		c.Hold[0]-c.Hold[1] < c.Match[0]-c.Match[1]) {
		t.Errorf("the edge does not compound: %+v", c)
	}
	if c.Match[0] <= 0.5 {
		t.Errorf("the better-rated player is not favoured: %.4f", c.Match[0])
	}

	// The inversion has to land on what the ratings predicted, or the chain is
	// describing a different match from the one the model believes in.
	if sim.Inputs.Expected == nil || sim.Inputs.Achieved == nil {
		t.Fatal("the response does not say what it targeted or reached")
	}
	if diff := *sim.Inputs.Expected - *sim.Inputs.Achieved; diff > 1e-6 || diff < -1e-6 {
		t.Errorf("targeted %.6f, reached %.6f", *sim.Inputs.Expected, *sim.Inputs.Achieved)
	}

	// ADR-0007: the response names its own inputs.
	if sim.Inputs.Anchor == nil || sim.Inputs.AnchorScope != "tier_surface_decade" {
		t.Errorf("inputs = %+v, want the exact anchor cell named", sim.Inputs)
	}
	if sim.Players[0].SurfaceWeight != 0.75 {
		t.Errorf("surface weight = %v, want the cap for 200 clay matches",
			sim.Players[0].SurfaceWeight)
	}
}

// A player with no rating cannot be simulated, and that is an answer rather
// than an error. A 50/50 would be a coin flip wearing a model's clothes.
func TestSimulateMatchWithoutARatingIsAbsent(t *testing.T) {
	f := newAPIFixture(t)
	pts := int16(100)

	rated := f.player("itg-sim-rated", "Itg Simrated", db.TourAtp)
	never := f.player("itg-sim-never", "Itg Simnever", db.TourAtp)
	open := f.tournament("itg-sim-unrated-open", db.TierTour, 2019)
	f.match(open, rated, never, 1, "F", 2019, &pts, false)

	f.rated(rated, db.RatingSurfaceOverall, 2000, 100)
	f.baselineCell(db.TourAtp, db.TierTour, "hard", 2010, 1_000_000, 620_000)

	sim := decodeMatchSimulation(t,
		f.get("/api/v1/simulate/match?a=itg-sim-rated&b=itg-sim-never"))

	if sim.Chain != nil {
		t.Errorf("chain = %+v, want none for an unrated player", sim.Chain)
	}
	if sim.Availability != SimulationUnrated {
		t.Errorf("availability = %q, want %q", sim.Availability, SimulationUnrated)
	}
	// The rated one still reports their rating: absence is per player.
	if sim.Players[0].Elo == nil || sim.Players[1].Elo != nil {
		t.Errorf("ratings = %v and %v, want only the rated one",
			sim.Players[0].Elo, sim.Players[1].Elo)
	}
}

// No serve statistics anywhere in the tour means nothing to anchor on, which is
// a different absence from an unrated player and says so.
func TestSimulateMatchWithoutAnAnchorIsAbsent(t *testing.T) {
	f := newAPIFixture(t)
	pts := int16(100)

	a := f.player("itg-sim-noanchor-a", "Itg Simnoanchora", db.TourAtp)
	b := f.player("itg-sim-noanchor-b", "Itg Simnoanchorb", db.TourAtp)
	open := f.tournament("itg-sim-noanchor", db.TierTour, 2019)
	f.match(open, a, b, 1, "F", 2019, &pts, false)
	f.rated(a, db.RatingSurfaceOverall, 2000, 100)
	f.rated(b, db.RatingSurfaceOverall, 1900, 100)

	sim := decodeMatchSimulation(t,
		f.get("/api/v1/simulate/match?a=itg-sim-noanchor-a&b=itg-sim-noanchor-b"))
	if sim.Chain != nil || sim.Availability != SimulationNoAnchor {
		t.Errorf("availability = %q with chain %v, want %q and none",
			sim.Availability, sim.Chain, SimulationNoAnchor)
	}
}

func TestSimulateMatchRejectsBadRequests(t *testing.T) {
	f := newAPIFixture(t)
	f.player("itg-sim-one", "Itg Simone", db.TourAtp)

	for _, c := range []struct {
		path string
		want int
	}{
		{"/api/v1/simulate/match?a=itg-sim-one", http.StatusBadRequest},
		{"/api/v1/simulate/match?a=itg-sim-one&b=itg-sim-one", http.StatusBadRequest},
		{"/api/v1/simulate/match?a=itg-sim-one&b=itg-nobody", http.StatusNotFound},
		{"/api/v1/simulate/match?a=itg-sim-one&b=itg-sim-one&surface=overall", http.StatusBadRequest},
		{"/api/v1/simulate/match?a=itg-sim-one&b=itg-sim-one&best_of=4", http.StatusBadRequest},
	} {
		if got := f.get(c.path).StatusCode; got != c.want {
			t.Errorf("%s = %d, want %d", c.path, got, c.want)
		}
	}
}

func TestSimulateDrawReplaysTheBracket(t *testing.T) {
	f := newAPIFixture(t)
	event := f.tournament("Itg Sim Open", db.TierTour, 2019)

	ids := map[string]int64{}
	for i, slug := range []string{"a", "b", "c", "d"} {
		id := f.player("itg-draw-"+slug, "Itg Draw"+slug, db.TourAtp)
		ids[slug] = id
		// a is the strongest, d the weakest.
		f.rated(id, db.RatingSurfaceOverall, float64(2100-100*i), 100)
	}
	f.baselineCell(db.TourAtp, db.TierTour, "clay", 2010, 1_000_000, 610_000)

	pts := int16(100)
	f.match(event, ids["a"], ids["b"], 1, "SF", 2019, &pts, false)
	f.match(event, ids["c"], ids["d"], 2, "SF", 2019, &pts, false)
	f.match(event, ids["a"], ids["c"], 3, "F", 2019, &pts, false)

	const path = "/api/v1/simulate/draw?tour=atp&season=2019&event=Itg+Sim+Open&runs=2000&seed=5"
	res := f.get(path)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var sim DrawSimulation
	if err := json.NewDecoder(res.Body).Decode(&sim); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if sim.Entered != 4 || len(sim.Odds) != 4 {
		t.Fatalf("entered %d with %d rows, want a draw of four", sim.Entered, len(sim.Odds))
	}
	// Sorted by title odds, so the strongest is first.
	if sim.Odds[0].Slug != "itg-draw-a" {
		t.Errorf("favourite = %q, want the strongest", sim.Odds[0].Slug)
	}
	if sim.Odds[0].TitleInterval <= 0 {
		t.Error("no interval on a sampled probability")
	}
	if sim.Champion == nil || *sim.Champion != "itg-draw-a" {
		t.Errorf("champion = %v, want the one who actually won", sim.Champion)
	}
	if sim.Event.RatingsAsOf == "" {
		t.Error("the response does not say which week its ratings are from")
	}

	// The same seed twice is the same answer, or a published figure cannot be
	// reproduced.
	var repeat DrawSimulation
	if err := json.NewDecoder(f.get(path).Body).Decode(&repeat); err != nil {
		t.Fatalf("decode repeat: %v", err)
	}
	if repeat.Odds[0].Title != sim.Odds[0].Title {
		t.Errorf("same seed gave %.4f then %.4f", sim.Odds[0].Title, repeat.Odds[0].Title)
	}
}

// A draw that is not a complete bracket is refused with the reason, not a 500.
func TestSimulateDrawRefusesAnIncompleteBracket(t *testing.T) {
	f := newAPIFixture(t)
	event := f.tournament("Itg Group Finals", db.TierTour, 2019)
	pts := int16(100)

	a := f.player("itg-rr-a", "Itg Rra", db.TourAtp)
	b := f.player("itg-rr-b", "Itg Rrb", db.TourAtp)
	c := f.player("itg-rr-c", "Itg Rrc", db.TourAtp)
	f.match(event, a, b, 1, "RR", 2019, &pts, false)
	f.match(event, b, c, 2, "RR", 2019, &pts, false)
	f.match(event, a, c, 3, "RR", 2019, &pts, false)

	res := f.get("/api/v1/simulate/draw?tour=atp&season=2019&event=Itg+Group+Finals")
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422 for a group stage", res.StatusCode)
	}
}

func TestSimulateDrawUnknownEventIs404(t *testing.T) {
	f := newAPIFixture(t)
	res := f.get("/api/v1/simulate/draw?tour=atp&season=2019&event=Nowhere")
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", res.StatusCode)
	}
}
