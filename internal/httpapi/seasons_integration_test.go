package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// A calendar: the Testville draws (WTA 2015, 2024, 2025 and an ATP Challenger
// in 2025), a Slam on each tour in 2025, and a team competition's two ties.
func (f *apiFixture) calendar() {
	f.t.Helper()
	f.draws()
	a := f.playerID("ev-a")
	b := f.playerID("ev-b")
	for _, tour := range []db.Tour{db.TourAtp, db.TourWta} {
		id := f.tourTournament("2025-580-"+string(tour), db.TierTour, 2025, tour)
		if _, err := f.tx.Exec(f.ctx, `UPDATE tournaments SET name = 'Big Slam', level = 'G', surface = 'hard', draw_size = 128 WHERE id = $1`, id); err != nil {
			f.t.Fatal(err)
		}
		f.match(id, a, b, 1, "F", 2025, nil, false)
		f.event(tour, "big-slam-"+string(tour), "Big Slam", "number:580", map[int64]string{id: "number"})
	}
	ties := map[int64]string{}
	for i, tie := range []string{"Davis Cup: Ann v Bea", "Davis Cup: Bea v Ann"} {
		id := f.tournament("2025-M-DC-0"+string(rune('1'+i)), db.TierTour, 2025)
		if _, err := f.tx.Exec(f.ctx, `UPDATE tournaments SET name = $2, level = 'D', surface = NULL WHERE id = $1`, id, tie); err != nil {
			f.t.Fatal(err)
		}
		if _, err := f.tx.Exec(f.ctx, `UPDATE matches SET is_team_event = true WHERE tournament_id = $1`, id); err != nil {
			f.t.Fatal(err)
		}
		f.match(id, a, b, 1, "RR", 2025, nil, false)
		ties[id] = "team"
	}
	f.event(db.TourAtp, "davis-cup", "Davis Cup", "team:davis-cup", ties)
}

func TestSeasonsIsARowPerYearWithBothTours(t *testing.T) {
	f := newAPIFixture(t)
	f.calendar()

	res := f.get("/api/v1/seasons")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	var got SeasonsResponse
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got.Data) != 3 || got.Data[0].Season != 2025 || got.Data[2].Season != 2015 {
		t.Fatalf("seasons = %+v", got.Data)
	}
	latest := got.Data[0]
	if latest.ATP == nil || latest.WTA == nil {
		t.Fatalf("2025 = %+v", latest)
	}
	// The ATP's 2025: the Challenger and the Slam as events, the two ties apart.
	if latest.ATP.Events != 2 || latest.ATP.Ties != 2 || latest.ATP.Surfaces != (SurfaceCounts{Hard: 1, Clay: 1}) {
		t.Errorf("atp 2025 = %+v", latest.ATP)
	}
	if len(latest.ATP.Slams) != 1 || latest.ATP.Slams[0].Slug != "big-slam-atp" || latest.ATP.Slams[0].Champion == nil || latest.ATP.Slams[0].Champion.Slug != "ev-a" {
		t.Errorf("atp slams = %+v", latest.ATP.Slams)
	}
	if latest.WTA.Events != 2 || len(latest.WTA.Slams) != 1 {
		t.Errorf("wta 2025 = %+v", latest.WTA)
	}
	// 2025 is the last season either tour has a match in, so both are in progress.
	if !latest.ATP.Partial || !latest.WTA.Partial || got.CurrentThrough["atp"] != "2025-05-02" {
		t.Errorf("partial: atp %v wta %v through %v", latest.ATP.Partial, latest.WTA.Partial, got.CurrentThrough)
	}
	// 2015 is the women's alone, and complete.
	first := got.Data[2]
	if first.ATP != nil || first.WTA == nil || first.WTA.Partial || first.WTA.Events != 1 {
		t.Errorf("2015 = atp %+v wta %+v", first.ATP, first.WTA)
	}
}

func TestSeasonEventsListsAYearByTier(t *testing.T) {
	f := newAPIFixture(t)
	f.calendar()

	res := f.get("/api/v1/seasons/2025")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	var got SeasonEventsResponse
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Season != 2025 || got.Tour != nil || got.Tier != "tour" {
		t.Errorf("head = %+v", got)
	}
	// Tour tier, both tours: two Slams, Testville, and the Davis Cup as one row.
	if len(got.Events) != 4 {
		t.Fatalf("events = %+v", got.Events)
	}
	var cup *SeasonEvent
	for i := range got.Events {
		if got.Events[i].Category == "team" {
			cup = &got.Events[i]
		}
	}
	if cup == nil || cup.Ties != 2 || cup.Matches != 2 || cup.Name != "Davis Cup" || cup.Champion != nil {
		t.Errorf("davis cup = %+v", cup)
	}
	if got.Partial["atp"] != "2025-05-02" || got.Partial["wta"] != "2025-05-02" {
		t.Errorf("partial = %v", got.Partial)
	}

	res = f.get("/api/v1/seasons/2025?tour=atp&tier=challenger")
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got.Events) != 1 || got.Events[0].Slug == nil || *got.Events[0].Slug != "testville-atp" || got.Events[0].Category != "challenger" {
		t.Errorf("challengers = %+v", got.Events)
	}

	// Decoded afresh: a map decoded into twice keeps the first call's keys.
	var old SeasonEventsResponse
	res = f.get("/api/v1/seasons/2015")
	if err := json.NewDecoder(res.Body).Decode(&old); err != nil {
		t.Fatal(err)
	}
	if len(old.Events) != 1 || len(old.Partial) != 0 || old.Events[0].Champion == nil || old.Events[0].Champion.Slug != "ev-a" {
		t.Errorf("2015 = %+v", old)
	}

	// A year with nothing is an empty list, not a 404: the calendar has the year.
	var none SeasonEventsResponse
	res = f.get("/api/v1/seasons/1950")
	if err := json.NewDecoder(res.Body).Decode(&none); err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK || len(none.Events) != 0 {
		t.Errorf("1950: %d %+v", res.StatusCode, none.Events)
	}
	if res := f.get("/api/v1/seasons/abc"); res.StatusCode != http.StatusBadRequest {
		t.Errorf("not a year: %d", res.StatusCode)
	}
	if res := f.get("/api/v1/seasons/2025?tier=pro"); res.StatusCode != http.StatusBadRequest {
		t.Errorf("not a tier: %d", res.StatusCode)
	}
}
