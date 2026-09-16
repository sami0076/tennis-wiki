package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// chart attaches a charted record to the fixture's most recent match, with a
// total and one set for both players.
func (f *apiFixture) chart(chartingID string, winner, loser int64) {
	f.t.Helper()
	if _, err := f.tx.Exec(f.ctx, `
		INSERT INTO charted_matches (match_id, charting_id, played_on, charted_by, source)
		SELECT id, $1, played_on + 3, 'a volunteer', 'mcp-atp' FROM matches ORDER BY id DESC LIMIT 1`,
		chartingID); err != nil {
		f.t.Fatal(err)
	}
	for _, line := range []struct {
		player int64
		set    int
		serve  int
		aces   int
	}{{winner, 0, 80, 6}, {loser, 0, 70, 2}, {winner, 1, 40, 4}, {loser, 1, 35, 1}} {
		if _, err := f.tx.Exec(f.ctx, `
			INSERT INTO charted_stats (match_id, player_id, set_no, serve_points, aces, double_faults,
			        first_in, first_won, second_in, second_won, bp_faced, bp_saved, return_points,
			        return_points_won, winners, winners_fh, winners_bh, unforced, unforced_fh, unforced_bh)
			SELECT match_id, $1, $2, $3::int, $4, 1, $3::int * 6 / 10, $3::int * 4 / 10, $3::int * 3 / 10,
			       $3::int / 5, 4, 3, $3::int, $3::int / 3, 12, 8, 4, 15, 9, 6
			  FROM charted_matches WHERE charting_id = $5`,
			line.player, line.set, line.serve, line.aces, chartingID); err != nil {
			f.t.Fatal(err)
		}
	}
}

func TestChartedMatchReadsBothPlayersPerSet(t *testing.T) {
	f := newAPIFixture(t)
	a := f.player("ch-a", "Cha Aaa", db.TourAtp)
	b := f.player("ch-b", "Cha Bbb", db.TourAtp)
	open := f.tournament("ch-open", db.TierTour, 2025)
	f.match(open, a, b, 1, "F", 2025, nil, false)
	f.chart("20250505-M-Open-F-Cha_Aaa-Cha_Bbb", a, b)

	res := f.get("/api/v1/charted/20250505-M-Open-F-Cha_Aaa-Cha_Bbb")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	var got ChartedMatch
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Players[0].Slug != "ch-a" || got.Players[1].Slug != "ch-b" {
		t.Errorf("players = %+v, want the winner first", got.Players)
	}
	if got.PlayedOn != "2025-05-05" || got.ChartedBy == nil || *got.ChartedBy != "a volunteer" {
		t.Errorf("played_on %s, charted_by %v", got.PlayedOn, got.ChartedBy)
	}
	if len(got.Sets) != 2 || got.Sets[0].Set != 0 || got.Sets[1].Set != 1 {
		t.Fatalf("sets = %+v, want the match then set 1", got.Sets)
	}
	total := got.Sets[0]
	if total.Lines[0] == nil || total.Lines[1] == nil ||
		total.Lines[0].ServePoints != 80 || total.Lines[0].Aces != 6 ||
		total.Lines[1].ServePoints != 70 || total.Lines[1].Aces != 2 {
		t.Errorf("match lines = %+v / %+v", total.Lines[0], total.Lines[1])
	}
	if got.Sets[1].Lines[0].ServePoints != 40 {
		t.Errorf("set 1, player 1 = %+v", got.Sets[1].Lines[0])
	}
}

func TestChartedMatchNotFound(t *testing.T) {
	f := newAPIFixture(t)
	if res := f.get("/api/v1/charted/nothing-here"); res.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", res.StatusCode)
	}
}

// The row says it is charted by carrying the key, and says nothing when it is
// not: null, not false, not an empty string.
func TestMatchRowsCarryTheChartingIDOnlyWhenCharted(t *testing.T) {
	f := newAPIFixture(t)
	a := f.player("ch-a", "Cha Aaa", db.TourAtp)
	b := f.player("ch-b", "Cha Bbb", db.TourAtp)
	open := f.tournament("ch-open", db.TierTour, 2025)
	f.match(open, a, b, 1, "SF", 2025, nil, false)
	f.match(open, a, b, 2, "F", 2025, nil, false)
	f.chart("20250505-M-Open-F-Cha_Aaa-Cha_Bbb", a, b)

	res := f.get("/api/v1/players/ch-a/matches")
	var page struct {
		Data []PlayerMatch `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page.Data) != 2 {
		t.Fatalf("%d rows, want 2", len(page.Data))
	}
	var charted, uncharted int
	for _, m := range page.Data {
		if m.ChartingID != nil {
			charted++
			if m.Round != "F" || *m.ChartingID != "20250505-M-Open-F-Cha_Aaa-Cha_Bbb" {
				t.Errorf("charted row = %+v", m)
			}
		} else {
			uncharted++
		}
	}
	if charted != 1 || uncharted != 1 {
		t.Errorf("charted %d, uncharted %d", charted, uncharted)
	}

	h2h := f.get("/api/v1/h2h/ch-a/ch-b")
	var comparison HeadToHead
	if err := json.NewDecoder(h2h.Body).Decode(&comparison); err != nil {
		t.Fatal(err)
	}
	var meetings int
	for _, m := range comparison.Meetings {
		if m.ChartingID != nil {
			meetings++
		}
	}
	if meetings != 1 {
		t.Errorf("%d charted meetings, want 1", meetings)
	}
}
