package httpapi

import (
	"encoding/json"
	"math"
	"net/http"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/db"
	"github.com/sami0076/tennis-wiki/internal/ingest"
)

// totals builds player_totals from whatever the fixture has written, the way
// the ingest's refresh step does, inside the test's own transaction.
func (f *apiFixture) totals() {
	f.t.Helper()
	if _, err := f.tx.Exec(f.ctx, ingest.PlayerTotalsInsert); err != nil {
		f.t.Fatalf("build player totals: %v", err)
	}
}

// scored writes the columns the score parser derives, which the fixture's
// match helper does not: a straight-sets win with one tiebreak.
func (f *apiFixture) scored(tournamentID int64) {
	f.t.Helper()
	if _, err := f.tx.Exec(f.ctx, `
		UPDATE matches SET sets_winner = 2, sets_loser = 0, games_winner = 13, games_loser = 8,
		       tiebreaks_winner = 1, tiebreaks_loser = 0, deciding_set = false
		 WHERE tournament_id = $1`, tournamentID); err != nil {
		f.t.Fatal(err)
	}
}

// A board's worth of players: three with serve lines, one of them on far
// more matches than the floor, and one with no serve line at all.
func (f *apiFixture) board() {
	f.t.Helper()
	ace := f.player("ld-ace", "Ace Server", db.TourAtp)
	base := f.player("ld-base", "Base Liner", db.TourAtp)
	few := f.player("ld-few", "Few Matches", db.TourAtp)
	none := f.player("ld-none", "No Stats", db.TourAtp)
	stats := int16(80)
	for i := range 12 {
		id := f.tournament("2019-ld-"+string(rune('a'+i)), db.TierTour, 2019)
		// Ace beats Base every week; both carry serve lines.
		f.match(id, ace, base, 1, "F", 2019, &stats, false)
		f.scored(id)
		// Few plays only the first two weeks, with lines; None never has one.
		if i < 2 {
			f.match(id, few, none, 2, "SF", 2019, &stats, false)
			f.scored(id)
			if _, err := f.tx.Exec(f.ctx, `UPDATE match_players SET serve_points = NULL, first_in = NULL,
				first_won = NULL, second_won = NULL, aces = NULL, double_faults = NULL WHERE player_id = $1`, none); err != nil {
				f.t.Fatal(err)
			}
		}
	}
	// Ace's serve line is better than Base's: the fixture writes the same
	// line for both sides, so Ace gets a stronger one.
	if _, err := f.tx.Exec(f.ctx, `UPDATE match_players SET aces = 10, first_won = 40 WHERE player_id = $1`, ace); err != nil {
		f.t.Fatal(err)
	}
	f.totals()
}

func decodeBoard(t *testing.T, res *http.Response) Leaderboard {
	t.Helper()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var b Leaderboard
	if err := json.NewDecoder(res.Body).Decode(&b); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestLeadersRankAStatWithItsDenominators(t *testing.T) {
	f := newAPIFixture(t)
	f.board()

	got := decodeBoard(t, f.get("/api/v1/leaders/aces?tour=atp&season=2019"))
	if got.Stat.Key != "aces" || got.Filters.MinMatches != 10 || got.Filters.Tour == nil || *got.Filters.Tour != "atp" {
		t.Errorf("head = %+v %+v", got.Stat, got.Filters)
	}
	// Twelve finals and two semis: fourteen matches, all with statistics.
	if got.Population.Matches != 14 || got.Population.WithStats != 14 || got.Population.Players != 4 {
		t.Errorf("population = %+v", got.Population)
	}
	// Only Ace and Base clear ten matches; Few has two and None has no line.
	if got.Population.Qualified != 2 || len(got.Data) != 2 {
		t.Fatalf("board = %+v", got.Data)
	}
	top := got.Data[0]
	if top.Slug != "ld-ace" || top.Position != 1 || top.Sample != 12 || top.Matches != 12 {
		t.Errorf("leader = %+v", top)
	}
	// 10 aces on 80 points, twelve times.
	if top.Numerator == nil || *top.Numerator != 120 || top.Denominator == nil || *top.Denominator != 960 || math.Abs(top.Value-0.125) > 1e-9 {
		t.Errorf("leader's rate = %v / %v = %v", top.Numerator, top.Denominator, top.Value)
	}
	if len(got.Stats) != len(leaderStats) {
		t.Errorf("stats listed = %d", len(got.Stats))
	}
}

func TestLeadersFloorAndDirectionAndFamilies(t *testing.T) {
	f := newAPIFixture(t)
	f.board()

	// Lowering the floor lets the two-match player on; the no-line player
	// still has no aces rate and stays off.
	got := decodeBoard(t, f.get("/api/v1/leaders/aces?min_matches=1"))
	if got.Population.Qualified != 3 {
		t.Errorf("qualified at a floor of one = %d", got.Population.Qualified)
	}

	// Double faults lead with the fewest.
	got = decodeBoard(t, f.get("/api/v1/leaders/double_faults"))
	if len(got.Data) != 2 || got.Data[0].Value > got.Data[1].Value || !got.Stat.Ascending {
		t.Errorf("double faults board = %+v", got.Data)
	}

	// A score-derived board stands on every finished match, lines or not.
	got = decodeBoard(t, f.get("/api/v1/leaders/matches_won?min_matches=1"))
	if got.Population.Qualified != 4 || got.Data[0].Slug != "ld-ace" || got.Data[0].Value != 1 {
		t.Errorf("matches won = %+v", got.Data)
	}
	got = decodeBoard(t, f.get("/api/v1/leaders/games_won?min_matches=1"))
	if got.Data[0].Numerator == nil || *got.Data[0].Numerator != 13*12 || *got.Data[0].Denominator != 21*12 {
		t.Errorf("games won = %+v", got.Data[0])
	}

	// A return figure is made of the opponents' lines: Base's return rate
	// is what Ace's serve conceded.
	got = decodeBoard(t, f.get("/api/v1/leaders/return_points_won"))
	if len(got.Data) != 2 || got.Data[0].Sample != 12 {
		t.Errorf("return board = %+v", got.Data)
	}

	// The dominance ratio has no numerator to show.
	got = decodeBoard(t, f.get("/api/v1/leaders/dominance"))
	if got.Stat.Kind != "ratio" || len(got.Data) == 0 || got.Data[0].Numerator != nil {
		t.Errorf("dominance = %+v", got.Data)
	}

	for _, path := range []string{
		"/api/v1/leaders/serves", "/api/v1/leaders/aces?tour=wtf", "/api/v1/leaders/aces?surface=ice",
		"/api/v1/leaders/aces?min_matches=0", "/api/v1/leaders/aces?season=abc", "/api/v1/leaders/aces?tier=pro",
	} {
		if res := f.get(path); res.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", path, res.StatusCode)
		}
	}
}
