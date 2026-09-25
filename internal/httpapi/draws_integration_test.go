package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/db"
)

func decodeDraws(t *testing.T, res *http.Response) ReplayableDraws {
	t.Helper()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var d ReplayableDraws
	if err := json.NewDecoder(res.Body).Decode(&d); err != nil {
		t.Fatalf("decode draws: %v", err)
	}
	return d
}

// knockout writes a tournament with an event row and a bracket ending in one
// final, which is what makes a draw replayable.
func (f *apiFixture) knockout(slug, name string, season int, rounds []string) int64 {
	f.t.Helper()
	var eventID int64
	err := f.tx.QueryRow(f.ctx,
		`INSERT INTO events (tour, slug, name, key, first_season, last_season)
		 VALUES ('atp', $1, $2, $1, $3, $3) RETURNING id`,
		slug, name, season).Scan(&eventID)
	if err != nil {
		f.t.Fatalf("insert event: %v", err)
	}

	var id int64
	err = f.tx.QueryRow(f.ctx,
		`INSERT INTO tournaments (source_id, tour, name, level, tier, surface, start_date,
		                          season, event_id, draw_size)
		 VALUES ($1, 'atp', $2, 'G', 'tour', 'grass', make_date($3, 6, 24), $3, $4, 128)
		 RETURNING id`,
		slug+"-"+name, name, season, eventID).Scan(&id)
	if err != nil {
		f.t.Fatalf("insert tournament: %v", err)
	}

	winner := f.player(slug+"-w", "Itg "+slug+"w", db.TourAtp)
	loser := f.player(slug+"-l", "Itg "+slug+"l", db.TourAtp)
	for i, round := range rounds {
		f.match(id, winner, loser, i+1, round, season, nil, false)
	}
	return id
}

func TestReplayableDrawsAreAddressedTheWayTheSimulatorAddressesThem(t *testing.T) {
	f := newAPIFixture(t)
	f.knockout("itg-open", "Itg Open", 2019, []string{"QF", "SF", "F"})

	draws := decodeDraws(t, f.get("/api/v1/simulate/draws"))

	if len(draws.Data) != 1 {
		t.Fatalf("draws = %+v, want one", draws.Data)
	}
	got := draws.Data[0]
	// The simulator takes ?event=<slug>&season=, so a row without both is a
	// row a picker could not act on.
	if got.Slug != "itg-open" || got.Season != 2019 {
		t.Errorf("address = %s/%d, want itg-open/2019", got.Slug, got.Season)
	}
	if got.Name != "Itg Open" || got.Surface != "grass" {
		t.Errorf("row = %+v, want it to describe the draw", got)
	}
	if got.DrawSize == nil || *got.DrawSize != 128 {
		t.Errorf("draw size = %v, want the source's 128", got.DrawSize)
	}
}

func TestReplayableDrawsLeaveOutWhatCannotBeReplayed(t *testing.T) {
	f := newAPIFixture(t)
	f.knockout("itg-good", "Itg Good", 2019, []string{"QF", "SF", "F"})
	// A round robin has no bracket to rebuild.
	f.knockout("itg-group", "Itg Group", 2019, []string{"RR", "RR", "F"})
	// Two finals is not one draw.
	f.knockout("itg-twofinals", "Itg Twofinals", 2019, []string{"SF", "F", "F"})
	// A lone final has no rounds to play through.
	f.knockout("itg-final", "Itg Final", 2019, []string{"F"})

	draws := decodeDraws(t, f.get("/api/v1/simulate/draws"))

	slugs := map[string]bool{}
	for _, d := range draws.Data {
		slugs[d.Slug] = true
	}
	if !slugs["itg-good"] {
		t.Error("the replayable draw is missing from the list")
	}
	for _, slug := range []string{"itg-group", "itg-twofinals", "itg-final"} {
		if slugs[slug] {
			t.Errorf("%s is offered and cannot be replayed", slug)
		}
	}
}

func TestReplayableDrawsPutTheBiggestFirstWithinASeason(t *testing.T) {
	f := newAPIFixture(t)
	// Inserted smallest first, so insertion order cannot be what orders them.
	f.knockout("itg-small", "Itg Small", 2019, []string{"SF", "F"})
	f.knockout("itg-big", "Itg Big", 2019, []string{"R32", "R16", "QF", "SF", "F"})
	f.knockout("itg-old", "Itg Old", 2015, []string{"R16", "QF", "SF", "F"})

	draws := decodeDraws(t, f.get("/api/v1/simulate/draws"))

	if len(draws.Data) != 3 {
		t.Fatalf("draws = %d, want 3", len(draws.Data))
	}
	// Newest season first; within it, the draw with more of it recorded.
	if draws.Data[0].Slug != "itg-big" {
		t.Errorf("first = %s, want itg-big", draws.Data[0].Slug)
	}
	if draws.Data[2].Season != 2015 {
		t.Errorf("last season = %d, want the oldest", draws.Data[2].Season)
	}
}

func TestReplayableDrawsCanBeCutByTourAndSeason(t *testing.T) {
	f := newAPIFixture(t)
	f.knockout("itg-2019", "Itg Twentynineteen", 2019, []string{"QF", "SF", "F"})
	f.knockout("itg-2015", "Itg Twentyfifteen", 2015, []string{"QF", "SF", "F"})

	season := decodeDraws(t, f.get("/api/v1/simulate/draws?season=2015"))
	if len(season.Data) != 1 || season.Data[0].Slug != "itg-2015" {
		t.Errorf("season cut = %+v, want only the 2015 draw", season.Data)
	}
	if season.Filters.Season == nil || *season.Filters.Season != 2015 {
		t.Errorf("filters = %+v, want the cut echoed", season.Filters)
	}

	wta := decodeDraws(t, f.get("/api/v1/simulate/draws?tour=wta"))
	if len(wta.Data) != 0 {
		t.Errorf("wta cut = %+v, want none: both fixtures are ATP", wta.Data)
	}
}

func TestReplayableDrawsRejectAnUnknownTour(t *testing.T) {
	f := newAPIFixture(t)
	res := f.get("/api/v1/simulate/draws?tour=itf")
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
}

// The whole point of the list is that every row is an address the simulator
// will accept, so one of them is followed through end to end.
func TestADrawFromTheListCanActuallyBeSimulated(t *testing.T) {
	f := newAPIFixture(t)
	f.knockout("itg-playable", "Itg Playable", 2019, []string{"SF", "F"})
	f.rating(f.playerID("itg-playable-w"), "2019-06-17", "overall", 1800, 40)
	f.rating(f.playerID("itg-playable-l"), "2019-06-17", "overall", 1700, 40)

	draws := decodeDraws(t, f.get("/api/v1/simulate/draws"))
	if len(draws.Data) == 0 {
		t.Fatal("nothing to follow through")
	}
	row := draws.Data[0]

	res := f.get("/api/v1/simulate/draw?event=" + row.Slug + "&season=2019")
	// 200 is the happy path; 422 means the reconstruction declined it, which
	// the list documents as possible. A 404 would mean the address is wrong,
	// which is the one outcome this list must never produce.
	if res.StatusCode == http.StatusNotFound {
		t.Fatalf("the list offered %s/%d and the simulator has never heard of it",
			row.Slug, row.Season)
	}
}
