package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// datedMatch writes one finished match on a given day, so a test can control
// the order the streak query reads matches in. The fixture's own match helper
// puts every match of a season on the same date, which is enough for a record
// and not enough for a sequence.
func (f *apiFixture) datedMatch(tournamentID, winnerID, loserID int64, num int, day string) {
	f.t.Helper()
	var matchID int64
	err := f.tx.QueryRow(f.ctx,
		`INSERT INTO matches (tournament_id, match_num, round, best_of, surface, winner_id,
		                      loser_id, played_on, incomplete, has_detailed_stats, source)
		 VALUES ($1, $2, 'R32', 3, 'clay', $3, $4, $5::date, false, false, 'test')
		 RETURNING id`,
		tournamentID, num, winnerID, loserID, day).Scan(&matchID)
	if err != nil {
		f.t.Fatalf("insert match: %v", err)
	}
	for _, p := range []struct {
		id  int64
		won bool
	}{{winnerID, true}, {loserID, false}} {
		if _, err := f.tx.Exec(f.ctx,
			`INSERT INTO match_players (match_id, player_id, won) VALUES ($1, $2, $3)`,
			matchID, p.id, p.won); err != nil {
			f.t.Fatalf("insert match_player: %v", err)
		}
	}
}

func decodeHighlights(t *testing.T, res *http.Response) PlayerHighlights {
	t.Helper()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var h PlayerHighlights
	if err := json.NewDecoder(res.Body).Decode(&h); err != nil {
		t.Fatalf("decode highlights: %v", err)
	}
	return h
}

func streakOf(t *testing.T, h PlayerHighlights, kind string) Streak {
	t.Helper()
	for _, s := range h.Streaks {
		if s.Kind == kind {
			return s
		}
	}
	t.Fatalf("no %q streak in %+v", kind, h.Streaks)
	return Streak{}
}

func TestHighlightsFindsTheRunsInASequence(t *testing.T) {
	f := newAPIFixture(t)
	player := f.player("itg-run", "Itg Run", db.TourAtp)
	foil := f.player("itg-run-foil", "Itg Runfoil", db.TourAtp)
	open := f.tournament("itg-run-open", db.TierTour, 2019)

	// W W W L L W, in date order. The best run is the three at the front, the
	// worst the two in the middle, and the current one the single win at the end.
	for i, day := range []string{"2019-05-01", "2019-05-02", "2019-05-03"} {
		f.datedMatch(open, player, foil, i+1, day)
	}
	f.datedMatch(open, foil, player, 4, "2019-05-04")
	f.datedMatch(open, foil, player, 5, "2019-05-05")
	f.datedMatch(open, player, foil, 6, "2019-05-06")

	h := decodeHighlights(t, f.get("/api/v1/players/itg-run/highlights"))

	best := streakOf(t, h, "best")
	if best.Length != 3 || !best.Won {
		t.Errorf("best = %+v, want a run of 3 wins", best)
	}
	if best.From != "2019-05-01" || best.To != "2019-05-03" {
		t.Errorf("best ran %s to %s, want 2019-05-01 to 2019-05-03", best.From, best.To)
	}
	if worst := streakOf(t, h, "worst"); worst.Length != 2 || worst.Won {
		t.Errorf("worst = %+v, want a run of 2 defeats", worst)
	}
	current := streakOf(t, h, "current")
	if current.Length != 1 || !current.Won {
		t.Errorf("current = %+v, want a run of 1 win", current)
	}
}

func TestHighlightsHasNoWorstRunForAnUnbeatenCareer(t *testing.T) {
	f := newAPIFixture(t)
	player := f.player("itg-unbeaten", "Itg Unbeaten", db.TourAtp)
	foil := f.player("itg-unbeaten-foil", "Itg Unbeatenfoil", db.TourAtp)
	open := f.tournament("itg-unbeaten-open", db.TierTour, 2019)
	f.datedMatch(open, player, foil, 1, "2019-05-01")
	f.datedMatch(open, player, foil, 2, "2019-05-02")

	h := decodeHighlights(t, f.get("/api/v1/players/itg-unbeaten/highlights"))

	// Absent, not a run of length zero: there is no worst defeat to report.
	for _, s := range h.Streaks {
		if s.Kind == "worst" {
			t.Fatalf("an unbeaten career reported a worst run: %+v", s)
		}
	}
	if best := streakOf(t, h, "best"); best.Length != 2 {
		t.Errorf("best.Length = %d, want 2", best.Length)
	}
}

func TestHighlightsRatesAnOpponentAsTheyWereOnTheDay(t *testing.T) {
	f := newAPIFixture(t)
	player := f.player("itg-best", "Itg Best", db.TourAtp)
	rising := f.player("itg-rising", "Itg Rising", db.TourAtp)
	steady := f.player("itg-steady", "Itg Steady", db.TourAtp)
	open := f.tournament("itg-best-open", db.TierTour, 2019)

	// Rising was worth 1500 on the day and 2100 a year later. The win has to
	// be credited with the first, or beating a future champion early would
	// outrank beating an actual one.
	f.rating(rising, "2019-04-29", "overall", 1500, 20)
	f.rating(rising, "2020-04-27", "overall", 2100, 60)
	f.rating(steady, "2019-04-29", "overall", 1800, 40)

	f.datedMatch(open, player, rising, 1, "2019-05-01")
	f.datedMatch(open, player, steady, 2, "2019-05-02")

	h := decodeHighlights(t, f.get("/api/v1/players/itg-best/highlights"))

	if len(h.BestWins) != 2 {
		t.Fatalf("best wins = %d, want 2", len(h.BestWins))
	}
	if h.BestWins[0].Opponent.Slug != "itg-steady" {
		t.Errorf("top win was over %q, want itg-steady", h.BestWins[0].Opponent.Slug)
	}
	if h.BestWins[0].OpponentElo != 1800 {
		t.Errorf("top win Elo = %v, want 1800", h.BestWins[0].OpponentElo)
	}
	if h.BestWins[0].EloAsOf != "2019-04-29" {
		t.Errorf("Elo read from %s, want the week before the match", h.BestWins[0].EloAsOf)
	}
	if h.BestWins[1].OpponentElo != 1500 {
		t.Errorf("second win Elo = %v, want the 1500 held on the day", h.BestWins[1].OpponentElo)
	}
}

func TestHighlightsLeavesUnratedOpponentsOutOfTheSchedule(t *testing.T) {
	f := newAPIFixture(t)
	player := f.player("itg-sched", "Itg Sched", db.TourAtp)
	rated := f.player("itg-sched-rated", "Itg Schedrated", db.TourAtp)
	unrated := f.player("itg-sched-unrated", "Itg Schedunrated", db.TourAtp)
	open := f.tournament("itg-sched-open", db.TierTour, 2019)

	f.rating(rated, "2019-04-29", "overall", 1600, 30)
	f.datedMatch(open, player, rated, 1, "2019-05-01")
	f.datedMatch(open, unrated, player, 2, "2019-05-02")

	h := decodeHighlights(t, f.get("/api/v1/players/itg-sched/highlights"))

	// Two matches played, one against somebody the model had rated. Counting
	// the other at the base rating would drag the average toward a number
	// nothing measured.
	if h.Schedule.RatedMatches != 1 {
		t.Errorf("rated matches = %d, want 1", h.Schedule.RatedMatches)
	}
	if h.Schedule.AverageElo == nil || *h.Schedule.AverageElo != 1600 {
		t.Errorf("average Elo = %v, want 1600", h.Schedule.AverageElo)
	}
	if h.Schedule.EliteElo != eliteElo {
		t.Errorf("elite bar = %v, want the response to state %v", h.Schedule.EliteElo, eliteElo)
	}
}

func TestHighlightsSchedulesAveragesAreAbsentWithNothingRated(t *testing.T) {
	f := newAPIFixture(t)
	player := f.player("itg-none", "Itg None", db.TourAtp)
	foil := f.player("itg-none-foil", "Itg Nonefoil", db.TourAtp)
	open := f.tournament("itg-none-open", db.TierTour, 2019)
	f.datedMatch(open, player, foil, 1, "2019-05-01")

	h := decodeHighlights(t, f.get("/api/v1/players/itg-none/highlights"))

	if h.Schedule.AverageElo != nil || h.Schedule.HighestElo != nil {
		t.Errorf("schedule = %+v, want null averages rather than zeroes", h.Schedule)
	}
}

func TestHighlightsOrdersRoundsByDrawDepth(t *testing.T) {
	f := newAPIFixture(t)
	player := f.player("itg-rounds", "Itg Rounds", db.TourAtp)
	foil := f.player("itg-rounds-foil", "Itg Roundsfoil", db.TourAtp)
	open := f.tournament("itg-rounds-open", db.TierTour, 2019)

	// Inserted final first, so an ordering that followed insertion would fail.
	f.match(open, player, foil, 1, "F", 2019, nil, false)
	f.match(open, player, foil, 2, "R32", 2019, nil, false)
	f.match(open, foil, player, 3, "SF", 2019, nil, false)

	h := decodeHighlights(t, f.get("/api/v1/players/itg-rounds/highlights"))

	want := []string{"R32", "SF", "F"}
	if len(h.Rounds) != len(want) {
		t.Fatalf("rounds = %+v, want %v", h.Rounds, want)
	}
	for i, round := range want {
		if h.Rounds[i].Round != round {
			t.Errorf("round %d = %q, want %q", i, h.Rounds[i].Round, round)
		}
	}
	if h.Rounds[2].Wins != 1 || h.Rounds[1].Wins != 0 {
		t.Errorf("record by round = %+v, want the final won and the semi lost", h.Rounds)
	}
}

func TestHighlightsSeparatesTitlesByWhatWasWon(t *testing.T) {
	f := newAPIFixture(t)
	player := f.player("itg-titles", "Itg Titles", db.TourAtp)
	foil := f.player("itg-titles-foil", "Itg Titlesfoil", db.TourAtp)

	slam := f.tournament("itg-titles-slam", db.TierTour, 2019)
	if _, err := f.tx.Exec(f.ctx, `UPDATE tournaments SET level = 'G' WHERE id = $1`, slam); err != nil {
		t.Fatalf("set level: %v", err)
	}
	challenger := f.tournament("itg-titles-ch", db.TierChallenger, 2019)

	f.match(slam, player, foil, 1, "F", 2019, nil, false)
	f.match(challenger, player, foil, 1, "F", 2019, nil, false)
	f.match(challenger, foil, player, 2, "F", 2019, nil, false)

	h := decodeHighlights(t, f.get("/api/v1/players/itg-titles/highlights"))

	got := map[string]FinalsRecord{}
	for _, row := range h.Finals {
		got[row.Category] = row
	}
	if got["slam"].Titles != 1 || got["slam"].Finals != 1 {
		t.Errorf("slam = %+v, want one title from one final", got["slam"])
	}
	if got["challenger"].Titles != 1 || got["challenger"].Finals != 2 {
		t.Errorf("challenger = %+v, want one title from two finals", got["challenger"])
	}
}

func TestHighlightsCountsWhoKeptTurningUp(t *testing.T) {
	f := newAPIFixture(t)
	player := f.player("itg-rivals", "Itg Rivals", db.TourAtp)
	often := f.player("itg-rivals-often", "Itg Rivalsoften", db.TourAtp)
	once := f.player("itg-rivals-once", "Itg Rivalsonce", db.TourAtp)
	open := f.tournament("itg-rivals-open", db.TierTour, 2019)

	f.datedMatch(open, player, often, 1, "2019-05-01")
	f.datedMatch(open, often, player, 2, "2019-05-02")
	f.datedMatch(open, player, once, 3, "2019-05-03")

	h := decodeHighlights(t, f.get("/api/v1/players/itg-rivals/highlights"))

	if len(h.Rivals) != 2 || h.Rivals[0].Slug != "itg-rivals-often" {
		t.Fatalf("rivals = %+v, want the twice-played opponent first", h.Rivals)
	}
	if h.Rivals[0].Matches != 2 || h.Rivals[0].Wins != 1 {
		t.Errorf("record = %d-%d, want 1-1 over 2 meetings",
			h.Rivals[0].Wins, h.Rivals[0].Matches-h.Rivals[0].Wins)
	}
}

func TestHighlightsIsAProblemDocumentForAnUnknownPlayer(t *testing.T) {
	f := newAPIFixture(t)
	res := f.get("/api/v1/players/itg-nobody/highlights")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.StatusCode)
	}
}
