package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// tourWeek writes a tournament with a final starting on a date, on one tour.
func (f *apiFixture) tourWeek(sourceID string, tour db.Tour, start string, season int, winner, loser int64) int64 {
	f.t.Helper()
	id := f.tourTournament(sourceID, db.TierTour, season, tour)
	if _, err := f.tx.Exec(f.ctx, `UPDATE tournaments SET start_date = $2::date, name = $3 WHERE id = $1`, id, start, sourceID); err != nil {
		f.t.Fatal(err)
	}
	f.match(id, winner, loser, 1, "F", season, nil, false)
	if _, err := f.tx.Exec(f.ctx, `UPDATE matches SET played_on = $2::date WHERE tournament_id = $1`, id, start); err != nil {
		f.t.Fatal(err)
	}
	return id
}

func decodeRecent(t *testing.T, res *http.Response) RecentFinals {
	t.Helper()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var out RecentFinals
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestRecentFinalsAreTheLastCompleteWeek(t *testing.T) {
	f := newAPIFixture(t)
	a := f.player("rc-a", "Ann Ace", db.TourAtp)
	b := f.player("rc-b", "Bea Base", db.TourAtp)
	c := f.player("rc-c", "Cat Court", db.TourWta)
	d := f.player("rc-d", "Dee Drop", db.TourWta)
	// Week of Monday 24 August 2026: a final on each tour. The ATP's data then
	// runs to Wednesday 2 September, inside the following week, which is not
	// complete and must not be shown.
	f.tourWeek("Rome", db.TourAtp, "2026-08-24", 2026, a, b)
	f.tourWeek("Cincinnati", db.TourWta, "2026-08-25", 2026, c, d)
	later := f.tourWeek("Metz", db.TourAtp, "2026-09-02", 2026, b, a)
	_ = later

	got := decodeRecent(t, f.get("/api/v1/recent"))
	if got.Through["atp"] != "2026-09-02" || got.Through["wta"] != "2026-08-25" {
		t.Errorf("through = %v", got.Through)
	}
	// The last complete week before Wednesday 2 September ends Sunday 30 August.
	if got.Week != (Week{From: "2026-08-24", To: "2026-08-30"}) || got.Requested != nil {
		t.Errorf("week = %+v requested %+v", got.Week, got.Requested)
	}
	if len(got.Finals) != 2 || got.Finals[0].Tour != "atp" || got.Finals[0].Champion.Slug != "rc-a" || got.Finals[1].Tour != "wta" {
		t.Errorf("finals = %+v", got.Finals)
	}
	if len(got.Without) != 0 {
		t.Errorf("without = %v", got.Without)
	}

	// Nothing began the week of 7 September: the strip falls back to the last
	// week that had a final, says which week it was asked for, and names the
	// tour with nothing that week.
	one := decodeRecent(t, f.get("/api/v1/recent?through=2026-09-13"))
	if one.Week != (Week{From: "2026-08-31", To: "2026-09-06"}) || len(one.Finals) != 1 || len(one.Without) != 1 || one.Without[0] != "wta" {
		t.Errorf("shown = %+v, finals %+v, without %v", one.Week, one.Finals, one.Without)
	}
	if one.Requested == nil || one.Requested.From != "2026-09-07" {
		t.Errorf("requested = %+v", one.Requested)
	}

	if res := f.get("/api/v1/recent?through=yesterday"); res.StatusCode != http.StatusBadRequest {
		t.Errorf("bad date: %d", res.StatusCode)
	}
	if res := f.get("/api/v1/recent?tier=pro"); res.StatusCode != http.StatusBadRequest {
		t.Errorf("bad tier: %d", res.StatusCode)
	}
}
