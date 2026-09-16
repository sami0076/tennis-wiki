package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// event writes an events row and points the tournaments rows at it, the way
// the events stage does after a load.
func (f *apiFixture) event(tour db.Tour, slug, name, key string, links map[int64]string) int64 {
	f.t.Helper()
	var id int64
	if err := f.tx.QueryRow(f.ctx, `
		INSERT INTO events (tour, slug, name, key, first_season, last_season)
		SELECT $1, $2, $3, $4, min(season), max(season) FROM tournaments WHERE id = ANY($5::bigint[])
		RETURNING id`,
		tour, slug, name, key, keysOf(links)).Scan(&id); err != nil {
		f.t.Fatalf("insert event %s: %v", slug, err)
	}
	for tid, link := range links {
		if _, err := f.tx.Exec(f.ctx, `UPDATE tournaments SET event_id = $1, event_link = $2 WHERE id = $3`,
			id, link, tid); err != nil {
			f.t.Fatal(err)
		}
	}
	return id
}

func keysOf(m map[int64]string) []int64 {
	out := make([]int64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// seed marks a player's entry in one match.
func (f *apiFixture) seed(tournamentID, playerID int64, seed int) {
	f.t.Helper()
	if _, err := f.tx.Exec(f.ctx, `
		UPDATE match_players mp SET seed = $3
		  FROM matches m WHERE m.id = mp.match_id AND m.tournament_id = $1 AND mp.player_id = $2`,
		tournamentID, playerID, seed); err != nil {
		f.t.Fatal(err)
	}
}

// A four-player draw, played twice under one number, plus an older edition
// joined by name and a Challenger of the same name on the other tour.
func (f *apiFixture) draws() (event int64, editions map[int]int64) {
	f.t.Helper()
	a := f.player("ev-a", "Ann Ace", db.TourWta)
	b := f.player("ev-b", "Bea Base", db.TourWta)
	c := f.player("ev-c", "Cat Court", db.TourWta)
	d := f.player("ev-d", "Dee Drop", db.TourWta)
	stats := int16(60)

	editions = map[int]int64{}
	for _, season := range []int{2015, 2024, 2025} {
		id := f.tourTournament("2024-777-"+string(rune('0'+season%10)), db.TierTour, season, db.TourWta)
		if _, err := f.tx.Exec(f.ctx, `UPDATE tournaments SET name = 'Testville', level = 'P', draw_size = 4 WHERE id = $1`, id); err != nil {
			f.t.Fatal(err)
		}
		editions[season] = id
		var serve *int16
		if season >= 2024 {
			serve = &stats
		}
		f.match(id, a, d, 1, "SF", season, serve, false)
		f.match(id, b, c, 2, "SF", season, serve, false)
		f.match(id, a, b, 3, "F", season, serve, false)
		f.seed(id, a, 1)
		f.seed(id, b, 2)
		f.seed(id, c, 3)
	}
	event = f.event(db.TourWta, "testville-wta", "Testville", "number:777", map[int64]string{
		editions[2015]: "bridged", editions[2024]: "number", editions[2025]: "number",
	})

	other := f.tournament("2025-778", db.TierChallenger, 2025)
	f.event(db.TourAtp, "testville-atp", "Testville", "number:778", map[int64]string{other: "number"})
	return event, editions
}

func TestTournamentIndexGroupsAndSearches(t *testing.T) {
	f := newAPIFixture(t)
	f.draws()

	res := f.get("/api/v1/tournaments?q=testv")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	var page Page[EventSummary]
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page.Data) != 2 {
		t.Fatalf("got %d events, want both tours' Testville: %+v", len(page.Data), page.Data)
	}
	// The tour-level event ranks before the Challenger regardless of tour.
	if page.Data[0].Slug != "testville-wta" || page.Data[0].Category != "tour" || page.Data[0].Editions != 3 {
		t.Errorf("first = %+v", page.Data[0])
	}
	if page.Data[1].Slug != "testville-atp" || page.Data[1].Category != "challenger" {
		t.Errorf("second = %+v", page.Data[1])
	}

	res = f.get("/api/v1/tournaments?tour=atp&level=challenger")
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page.Data) != 1 || page.Data[0].Slug != "testville-atp" {
		t.Errorf("filtered = %+v", page.Data)
	}
	if res := f.get("/api/v1/tournaments?level=grand"); res.StatusCode != http.StatusBadRequest {
		t.Errorf("an unknown level should be a 400, got %d", res.StatusCode)
	}
}

func TestTournamentIndexPaginates(t *testing.T) {
	f := newAPIFixture(t)
	f.draws()

	res := f.get("/api/v1/tournaments?q=testv&limit=1")
	var page Page[EventSummary]
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page.Data) != 1 || page.NextCursor == "" {
		t.Fatalf("first page = %+v", page)
	}
	res = f.get("/api/v1/tournaments?q=testv&limit=1&cursor=" + page.NextCursor)
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page.Data) != 1 || page.Data[0].Slug != "testville-atp" {
		t.Errorf("second page = %+v", page)
	}
	// A full page always carries a cursor; the page after it is empty.
	res = f.get("/api/v1/tournaments?q=testv&limit=1&cursor=" + page.NextCursor)
	var third Page[EventSummary]
	if err := json.NewDecoder(res.Body).Decode(&third); err != nil {
		t.Fatal(err)
	}
	if len(third.Data) != 0 || third.NextCursor != "" {
		t.Errorf("third page = %+v", third)
	}
}

func TestEventListsEditionsWithProvenance(t *testing.T) {
	f := newAPIFixture(t)
	f.draws()

	res := f.get("/api/v1/tournaments/testville-wta")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	var got Event
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Keyed != "number" || got.Number == nil || *got.Number != "777" {
		t.Errorf("keyed %s by %v", got.Keyed, got.Number)
	}
	if got.Provenance != (Provenance{Number: 2, Bridged: 1}) {
		t.Errorf("provenance = %+v", got.Provenance)
	}
	if len(got.Editions) != 3 || got.Editions[0].Season != 2015 || got.Editions[0].Link != "bridged" {
		t.Fatalf("editions = %+v", got.Editions)
	}
	last := got.Editions[2]
	if last.Champion == nil || last.Champion.Slug != "ev-a" || last.Finalist == nil || last.Finalist.Slug != "ev-b" {
		t.Errorf("final: %+v beat %+v", last.Champion, last.Finalist)
	}
	if last.Matches != 3 || last.DrawSize == nil || *last.DrawSize != 4 {
		t.Errorf("edition = %+v", last)
	}
	if len(got.Names) != 1 || got.Names[0] != (NameRun{Name: "Testville", FirstSeason: 2015, LastSeason: 2025}) {
		t.Errorf("names = %+v", got.Names)
	}

	if res := f.get("/api/v1/tournaments/nowhere-wta"); res.StatusCode != http.StatusNotFound {
		t.Errorf("unknown slug: %d", res.StatusCode)
	}
}

func TestEditionIsASheet(t *testing.T) {
	f := newAPIFixture(t)
	f.draws()

	res := f.get("/api/v1/tournaments/testville-wta/2025")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	var got Edition
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Event.Slug != "testville-wta" || got.Season != 2025 || got.Link != "number" {
		t.Errorf("edition = %+v", got)
	}
	if got.Serve != (EditionServe{Availability: AvailabilityRecorded, MatchesWith: 3, Matches: 3}) {
		t.Errorf("serve = %+v", got.Serve)
	}
	if len(got.Matches) != 3 || got.Matches[0].Round != "SF" || got.Matches[2].Round != "F" {
		t.Fatalf("matches = %+v", got.Matches)
	}
	final := got.Matches[2]
	if final.Players[0].Slug != "ev-a" || final.Players[0].Seed == nil || *final.Players[0].Seed != 1 {
		t.Errorf("final winner = %+v", final.Players[0])
	}
	if final.Serve[0] == nil || final.Serve[1] == nil || *final.Serve[0].ServePoints != 60 {
		t.Errorf("final serve = %+v", final.Serve)
	}
	// Seeds and where each went out: 1 won, 2 lost the final, 3 a semi.
	want := []SeedLine{
		{Seed: 1, Player: Opponent{Slug: "ev-a", Name: "Ann Ace"}, Exit: "W"},
		{Seed: 2, Player: Opponent{Slug: "ev-b", Name: "Bea Base"}, Exit: "F"},
		{Seed: 3, Player: Opponent{Slug: "ev-c", Name: "Cat Court"}, Exit: "SF"},
	}
	if len(got.Seeds) != 3 {
		t.Fatalf("seeds = %+v", got.Seeds)
	}
	for i, s := range want {
		if got.Seeds[i] != s {
			t.Errorf("seed %d = %+v, want %+v", i, got.Seeds[i], s)
		}
	}

	// The 2015 edition predates the stat line: absent once, for the edition.
	res = f.get("/api/v1/tournaments/testville-wta/2015")
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Serve.Availability != AvailabilityNotRecorded || got.Serve.MatchesWith != 0 || got.Matches[0].Serve[0] != nil {
		t.Errorf("2015 serve = %+v, first match %+v", got.Serve, got.Matches[0].Serve)
	}

	if res := f.get("/api/v1/tournaments/testville-wta/1999"); res.StatusCode != http.StatusNotFound {
		t.Errorf("a season not played: %d", res.StatusCode)
	}
	if res := f.get("/api/v1/tournaments/testville-wta/abc"); res.StatusCode != http.StatusBadRequest {
		t.Errorf("a season that is not a year: %d", res.StatusCode)
	}
}
