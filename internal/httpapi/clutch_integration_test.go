package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// clutchMatch writes one finished match with the columns the ingest derives
// from its score, and a break-point line for both players.
//
// bpFaced nil is a match nobody recorded serve statistics for, which is most of
// them: 83% of the database.
func (f *apiFixture) clutchMatch(
	tournamentID, winnerID, loserID int64, num, season int,
	tiebreaksWinner, tiebreaksLoser int16, decider bool, bpSaved, bpFaced *int16,
) {
	f.t.Helper()
	var matchID int64
	err := f.tx.QueryRow(f.ctx,
		`INSERT INTO matches (tournament_id, match_num, round, best_of, surface, winner_id,
		                      loser_id, played_on, incomplete, has_detailed_stats, source,
		                      tiebreaks_winner, tiebreaks_loser, deciding_set)
		 VALUES ($1, $2, 'R32', 3, 'clay', $3, $4, make_date($5, 5, 2), false, $6, 'test',
		         $7, $8, $9)
		 RETURNING id`,
		tournamentID, num, winnerID, loserID, season, bpFaced != nil,
		tiebreaksWinner, tiebreaksLoser, decider).Scan(&matchID)
	if err != nil {
		f.t.Fatalf("insert match: %v", err)
	}

	for _, p := range []struct {
		id  int64
		won bool
	}{{winnerID, true}, {loserID, false}} {
		if _, err := f.tx.Exec(f.ctx,
			`INSERT INTO match_players (match_id, player_id, won, bp_saved, bp_faced)
			 VALUES ($1, $2, $3, $4, $5)`,
			matchID, p.id, p.won, bpSaved, bpFaced); err != nil {
			f.t.Fatalf("insert match_player: %v", err)
		}
	}
}

// baseline writes one cell of what "vs tour average" is measured against. The
// ingest builds these; the API only has to weight them.
func (f *apiFixture) baseline(
	tour db.Tour, tier db.Tier, decade int, bpSaved, bpFaced, tbWon, tbPlayed int64,
) {
	f.t.Helper()
	if _, err := f.tx.Exec(f.ctx,
		`INSERT INTO clutch_baselines (tour, tier, decade, appearances, bp_saved, bp_faced,
		                               tiebreaks_won, tiebreaks_played,
		                               deciding_sets_won, deciding_sets_played)
		 VALUES ($1, $2, $3, 1000, $4, $5, $6, $7, 50, 100)`,
		tour, tier, decade, bpSaved, bpFaced, tbWon, tbPlayed); err != nil {
		f.t.Fatalf("insert baseline: %v", err)
	}
}

func decodeClutch(t *testing.T, res *http.Response) Clutch {
	t.Helper()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var c Clutch
	if err := json.NewDecoder(res.Body).Decode(&c); err != nil {
		t.Fatalf("decode clutch: %v", err)
	}
	return c
}

func TestClutchComparesAgainstTheStatedPopulation(t *testing.T) {
	f := newAPIFixture(t)
	saved, faced := int16(6), int16(10)

	player := f.player("itg-clutch", "Itg Clutch", db.TourAtp)
	foil := f.player("itg-clutch-foil", "Itg Clutchfoil", db.TourAtp)
	open := f.tournament("itg-clutch-open", db.TierTour, 2019)

	// Two wins in deciding sets and one loss, three tiebreaks won of four.
	f.clutchMatch(open, player, foil, 1, 2019, 2, 1, true, &saved, &faced)
	f.clutchMatch(open, player, foil, 2, 2019, 1, 0, true, &saved, &faced)
	f.clutchMatch(open, foil, player, 3, 2019, 0, 0, true, &saved, &faced)

	// The tour saves half its break points in this cell; this player saves 60%.
	f.baseline(db.TourAtp, db.TierTour, 2010, 500, 1000, 50, 100)

	clutch := decodeClutch(t, f.get("/api/v1/players/itg-clutch/clutch"))

	if clutch.BreakPointsSaved == nil {
		t.Fatal("break points saved is absent, want a figure")
	}
	if got := clutch.BreakPointsSaved.Percentage; got != 60 {
		t.Errorf("break points saved = %v%%, want 60", got)
	}
	if clutch.BreakPointsSaved.Baseline == nil || *clutch.BreakPointsSaved.Baseline != 50 {
		t.Errorf("baseline = %v, want the 50%% the cell records", clutch.BreakPointsSaved.Baseline)
	}
	if clutch.BreakPointsSaved.Delta == nil || *clutch.BreakPointsSaved.Delta != 10 {
		t.Errorf("delta = %v, want +10 points", clutch.BreakPointsSaved.Delta)
	}

	// Two of three deciding sets, and the third was won by the other player.
	if clutch.DecidingSetsWon == nil || clutch.DecidingSetsWon.Won != 2 ||
		clutch.DecidingSetsWon.Played != 3 {
		t.Errorf("deciding sets = %+v, want 2 of 3", clutch.DecidingSetsWon)
	}

	// The match this player lost carried one tiebreak they won, because the
	// columns are written from the match winner's side and read from theirs.
	if clutch.TiebreaksWon == nil || clutch.TiebreaksWon.Won != 3 ||
		clutch.TiebreaksWon.Played != 4 {
		t.Errorf("tiebreaks = %+v, want 3 of 4", clutch.TiebreaksWon)
	}

	// The population is in the response, because a delta against an unnamed
	// average is a number pretending to be a fact.
	if len(clutch.Baseline.Tiers) != 1 || clutch.Baseline.Tiers[0] != "tour" {
		t.Errorf("tiers = %v, want the one this player played", clutch.Baseline.Tiers)
	}
	if clutch.Baseline.FromDecade != 2010 || clutch.Baseline.ToDecade != 2010 {
		t.Errorf("decades = %d to %d, want 2010 to 2010",
			clutch.Baseline.FromDecade, clutch.Baseline.ToDecade)
	}
	if clutch.Baseline.Matches != 3 || clutch.Baseline.ScoredMatches != 3 {
		t.Errorf("denominator = %d of %d, want 3 of 3",
			clutch.Baseline.ScoredMatches, clutch.Baseline.Matches)
	}
}

// A career at a level that never recorded serve statistics has no break-point
// figure at all. Zero would say they faced break points and lost every one.
func TestClutchAbsentRatherThanZero(t *testing.T) {
	f := newAPIFixture(t)

	player := f.player("itg-futures-clutch", "Itg Futuresclutch", db.TourAtp)
	foil := f.player("itg-futures-foil", "Itg Futuresfoil", db.TourAtp)
	event := f.tournament("itg-futures-event", db.TierFutures, 2019)

	f.clutchMatch(event, player, foil, 1, 2019, 1, 0, true, nil, nil)

	clutch := decodeClutch(t, f.get("/api/v1/players/itg-futures-clutch/clutch"))

	if clutch.BreakPointsSaved != nil {
		t.Errorf("break points saved = %+v, want absent", clutch.BreakPointsSaved)
	}
	if clutch.Availability != AvailabilityNeverForTier {
		t.Errorf("availability = %q, want %q", clutch.Availability, AvailabilityNeverForTier)
	}
	// The score still says who won the tiebreak, which no serve line was needed
	// for. Absence is per figure, not per player.
	if clutch.TiebreaksWon == nil || clutch.TiebreaksWon.Won != 1 {
		t.Errorf("tiebreaks = %+v, want the one the score records", clutch.TiebreaksWon)
	}
}

// A figure with no cell to compare it against is still a figure. A baseline of
// zero would be a comparison with nothing, dressed as a comparison.
func TestClutchWithoutABaselineHasNoDelta(t *testing.T) {
	f := newAPIFixture(t)

	player := f.player("itg-unbaselined", "Itg Unbaselined", db.TourAtp)
	foil := f.player("itg-unbaselined-foil", "Itg Unbaselinedfoil", db.TourAtp)
	open := f.tournament("itg-unbaselined-open", db.TierTour, 2019)
	f.clutchMatch(open, player, foil, 1, 2019, 1, 0, true, nil, nil)

	clutch := decodeClutch(t, f.get("/api/v1/players/itg-unbaselined/clutch"))

	if clutch.TiebreaksWon == nil {
		t.Fatal("tiebreaks absent, want the figure the score records")
	}
	if clutch.TiebreaksWon.Baseline != nil || clutch.TiebreaksWon.Delta != nil {
		t.Errorf("baseline %v, delta %v; want both absent with no cell to compare",
			clutch.TiebreaksWon.Baseline, clutch.TiebreaksWon.Delta)
	}
}

func TestClutchUnknownPlayerIs404(t *testing.T) {
	f := newAPIFixture(t)
	res := f.get("/api/v1/players/itg-nobody/clutch")
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", res.StatusCode)
	}
}
