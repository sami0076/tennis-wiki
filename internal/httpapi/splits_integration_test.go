package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// A short career cut every way the splits cut it: wins over a No. 1 and a
// No. 40, a loss to No. 8, a match against an unranked opponent, and one
// decided by a final-set tiebreak.
func (f *apiFixture) shortCareer() int64 {
	f.t.Helper()
	me := f.player("sp-me", "Sam Splits", db.TourWta)
	top := f.player("sp-top", "Top Seed", db.TourWta)
	mid := f.player("sp-mid", "Mid Rank", db.TourWta)
	low := f.player("sp-low", "Low Rank", db.TourWta)
	none := f.player("sp-none", "No Rank", db.TourWta)
	stats := int16(60)

	t2019 := f.tourTournament("2019-sp", db.TierTour, 2019, db.TourWta)
	f.match(t2019, me, top, 1, "QF", 2019, &stats, false) // beat No. 1
	f.match(t2019, mid, me, 2, "SF", 2019, &stats, false) // lost to No. 8
	t2020 := f.tourTournament("2020-sp", db.TierTour, 2020, db.TourWta)
	f.match(t2020, me, low, 1, "SF", 2020, nil, false) // beat No. 40, no serve line
	f.match(t2020, me, none, 2, "F", 2020, nil, false) // beat an unranked player, a title

	rank := func(player int64, r *int32) {
		if _, err := f.tx.Exec(f.ctx, `UPDATE match_players SET rank = $2 WHERE player_id = $1`, player, r); err != nil {
			f.t.Fatal(err)
		}
	}
	mine, one, eight, forty := int32(25), int32(1), int32(8), int32(40)
	rank(me, &mine)
	rank(top, &one)
	rank(mid, &eight)
	rank(low, &forty)
	rank(none, nil)

	// Scores and the derived columns the ingest would have written: the win
	// over No. 1 went to a final-set tiebreak.
	if _, err := f.tx.Exec(f.ctx, `
		UPDATE matches SET score = '6-4 3-6 7-6(5)', deciding_set = true,
		       sets_winner = 2, sets_loser = 1, games_winner = 19, games_loser = 16,
		       tiebreaks_winner = 1, tiebreaks_loser = 0
		 WHERE tournament_id = $1 AND match_num = 1`, t2019); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.tx.Exec(f.ctx, `
		UPDATE matches SET score = '6-2 6-2', deciding_set = false,
		       sets_winner = 2, sets_loser = 0, games_winner = 12, games_loser = 4,
		       tiebreaks_winner = 0, tiebreaks_loser = 0
		 WHERE NOT (tournament_id = $1 AND match_num = 1)`, t2019); err != nil {
		f.t.Fatal(err)
	}
	// The fixture's serve line has no games column; the hold rate needs one.
	if _, err := f.tx.Exec(f.ctx, `UPDATE match_players SET serve_games = 10 WHERE serve_points IS NOT NULL`); err != nil {
		f.t.Fatal(err)
	}
	f.totals()
	return me
}

func TestPlayerSplitsByRankAndCloseness(t *testing.T) {
	f := newAPIFixture(t)
	f.shortCareer()

	profile := decodeProfile(t, f.get("/api/v1/players/sp-me"))
	if profile.Splits == nil {
		t.Fatal("splits = nil")
	}
	s := profile.Splits
	if s.Ranked != 3 || s.Scored != 4 {
		t.Errorf("ranked %d scored %d", s.Ranked, s.Scored)
	}
	want := map[string]RankBand{
		"1": {Band: "1", Matches: 1, Wins: 1}, "5": {Band: "5", Matches: 1, Wins: 1},
		"10": {Band: "10", Matches: 2, Wins: 1}, "20": {Band: "20", Matches: 2, Wins: 1},
		"50": {Band: "50", Matches: 3, Wins: 2}, "100": {Band: "100", Matches: 3, Wins: 2},
		"outside": {Band: "outside"}, "unranked": {Band: "unranked", Matches: 1, Wins: 1},
	}
	if len(s.ByRank) != len(want) {
		t.Fatalf("bands = %+v", s.ByRank)
	}
	for _, band := range s.ByRank {
		if band != want[band.Band] {
			t.Errorf("band %s = %+v, want %+v", band.Band, band, want[band.Band])
		}
	}
	// Ranked 25: No. 1 and No. 8 were higher, No. 40 lower.
	if s.Higher != (Record{Matches: 2, Wins: 1}) || s.Lower != (Record{Matches: 1, Wins: 1}) {
		t.Errorf("higher %+v lower %+v", s.Higher, s.Lower)
	}
	if s.FinalSetTiebreaks != (Record{Matches: 1, Wins: 1}) {
		t.Errorf("final-set tiebreaks = %+v", s.FinalSetTiebreaks)
	}
}

func TestPlayerSeasonsCarryTheirOwnDenominators(t *testing.T) {
	f := newAPIFixture(t)
	f.shortCareer()

	res := f.get("/api/v1/players/sp-me/seasons")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	var got PlayerSeasons
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got.Seasons) != 2 || got.Seasons[0].Season != 2019 || got.Seasons[1].Season != 2020 {
		t.Fatalf("seasons = %+v", got.Seasons)
	}
	first, second := got.Seasons[0], got.Seasons[1]
	// 2019: two matches with serve lines, 1-1, 2 of 5 sets, one tiebreak.
	if first.Matches != 2 || first.Wins != 1 || first.Losses != 1 || first.WithServe != 2 || first.Scored != 2 {
		t.Errorf("2019 = %+v", first)
	}
	if first.SetsPct == nil || *first.SetsPct != 40 || first.TiebreaksPct == nil || *first.TiebreaksPct != 100 || first.HoldPct == nil {
		t.Errorf("2019 rates = sets %v tiebreaks %v hold %v", first.SetsPct, first.TiebreaksPct, first.HoldPct)
	}
	// 2020: two wins and a title, no serve lines at all: the serve rates are
	// absent, not zero, while the set rate stands on both matches.
	if second.Matches != 2 || second.Wins != 2 || second.Titles != 1 || second.WithServe != 0 {
		t.Errorf("2020 = %+v", second)
	}
	if second.HoldPct != nil || second.AcePct != nil || second.Dominance != nil || second.SetsPct == nil || *second.SetsPct != 100 {
		t.Errorf("2020 rates = hold %v ace %v dominance %v sets %v", second.HoldPct, second.AcePct, second.Dominance, second.SetsPct)
	}
	// A tiebreak rate with no tiebreaks is nothing to divide by.
	if second.TiebreaksPct != nil {
		t.Errorf("2020 tiebreaks = %v, want absent", *second.TiebreaksPct)
	}

	if res := f.get("/api/v1/players/nobody/seasons"); res.StatusCode != http.StatusNotFound {
		t.Errorf("unknown player: %d", res.StatusCode)
	}
}
