package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/db"
)

func decodeRankingPage(t *testing.T, res *http.Response) RankingPage {
	t.Helper()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var page RankingPage
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatalf("decode rankings: %v", err)
	}
	return page
}

// rankedField seeds three players with ratings in the same recent week and
// official ranks that disagree with them, which is the comparison the page is
// for.
func (f *apiFixture) rankedField(t *testing.T) {
	t.Helper()
	for _, p := range []struct {
		slug string
		elo  float64
		rank int32
	}{
		{"itg-rank-one", 2200, 1},
		{"itg-rank-two", 2100, 5},
		{"itg-rank-three", 2000, 2},
	} {
		id := f.player(p.slug, p.slug, db.TourAtp)
		// An older, lower row so "current" is genuinely the last one.
		f.rating(id, "2025-01-06", "overall", p.elo-300, 40)
		f.rating(id, "2026-01-05", "overall", p.elo, 60)
		f.ranking(id, "2026-01-05", p.rank, nil)
	}
}

func TestEloRankingsStateTheDateTheyUsed(t *testing.T) {
	f := newAPIFixture(t)
	f.rankedField(t)

	page := decodeRankingPage(t, f.get("/api/v1/rankings?type=elo"))
	if page.AsOf != "2026-01-05" {
		t.Errorf("as_of = %q, want the latest week that exists", page.AsOf)
	}
	if len(page.Data) != 3 {
		t.Fatalf("got %d rows, want the three rated players", len(page.Data))
	}
	if page.Data[0].Slug != "itg-rank-one" || page.Data[0].Position != 1 {
		t.Errorf("first row = %+v, want the highest rating at position 1", page.Data[0])
	}
	if page.Data[0].Elo == nil || *page.Data[0].Elo != 2200 {
		t.Errorf("elo = %v, want the last row rather than the first", page.Data[0].Elo)
	}
	// Peak is the highest they ever held, which here is the current one.
	if page.Data[0].PeakElo == nil || *page.Data[0].PeakElo != 2200 {
		t.Errorf("peak = %v", page.Data[0].PeakElo)
	}
}

// The delta is the whole point of putting the two lists side by side.
func TestEloRankingsCarryTheDeltaAgainstTheOfficialList(t *testing.T) {
	f := newAPIFixture(t)
	f.rankedField(t)

	page := decodeRankingPage(t, f.get("/api/v1/rankings?type=elo"))
	byslug := map[string]RankingRow{}
	for _, row := range page.Data {
		byslug[row.Slug] = row
	}

	// Officially 5th, second by Elo: the model rates them three places higher.
	two := byslug["itg-rank-two"]
	if two.Position != 2 || two.OfficialRank == nil || *two.OfficialRank != 5 {
		t.Fatalf("row = %+v", two)
	}
	if two.Delta == nil || *two.Delta != 3 {
		t.Errorf("delta = %v, want +3", two.Delta)
	}
	// Officially 2nd, third by Elo: the tour rates them higher.
	three := byslug["itg-rank-three"]
	if three.Delta == nil || *three.Delta != -1 {
		t.Errorf("delta = %v, want -1", three.Delta)
	}
}

func TestOfficialRankingsReadTheTourList(t *testing.T) {
	f := newAPIFixture(t)
	f.rankedField(t)

	page := decodeRankingPage(t, f.get("/api/v1/rankings?type=official"))
	if page.AsOf != "2026-01-05" || len(page.Data) != 3 {
		t.Fatalf("page = %+v", page)
	}
	if page.Data[0].Slug != "itg-rank-one" || page.Data[1].Slug != "itg-rank-three" {
		t.Errorf("order = %s, %s, want official rank order",
			page.Data[0].Slug, page.Data[1].Slug)
	}
	// The Elo is beside it, because the two are different claims about the
	// same player.
	if page.Data[1].Elo == nil || *page.Data[1].Elo != 2000 {
		t.Errorf("elo = %v, want the rating beside the rank", page.Data[1].Elo)
	}
}

// A rating from years ago is a real rating and not a current ranking. Without
// the window a leaderboard is a list of the retired.
func TestEloRankingsExcludeTheLongRetired(t *testing.T) {
	f := newAPIFixture(t)
	f.rankedField(t)

	retired := f.player("itg-retired", "Itg Retired", db.TourAtp)
	f.rating(retired, "2005-06-13", "overall", 2900, 1700)

	page := decodeRankingPage(t, f.get("/api/v1/rankings?type=elo"))
	for _, row := range page.Data {
		if row.Slug == "itg-retired" {
			t.Fatalf("a player last rated in 2005 is in the current rankings: %+v", row)
		}
	}
	// Asking as of their own era finds them again.
	old := decodeRankingPage(t, f.get("/api/v1/rankings?type=elo&date=2005-12-31"))
	if old.AsOf != "2005-06-13" {
		t.Errorf("as_of = %q, want the latest week at or before the one asked for", old.AsOf)
	}
	if len(old.Data) != 1 || old.Data[0].Slug != "itg-retired" {
		t.Errorf("got %+v, want the player who was active then", old.Data)
	}
	if old.Requested != "2005-12-31" {
		t.Errorf("requested = %q, want the date that was asked for", old.Requested)
	}
}

// Out of coverage is an empty answer with the date stated, not a 404.
func TestRankingsOutsideCoverageAreEmptyNotMissing(t *testing.T) {
	f := newAPIFixture(t)
	f.rankedField(t)

	for _, path := range []string{
		"/api/v1/rankings?type=elo&date=1900-01-01",
		"/api/v1/rankings?type=official&date=1900-01-01",
	} {
		page := decodeRankingPage(t, f.get(path))
		if len(page.Data) != 0 {
			t.Errorf("%s returned %d rows, want none", path, len(page.Data))
		}
		if page.AsOf != "1900-01-01" {
			t.Errorf("%s: as_of = %q, want the date it looked at", path, page.AsOf)
		}
	}
}

func TestRankingsPageThroughWithoutRepeating(t *testing.T) {
	f := newAPIFixture(t)
	f.rankedField(t)

	seen := map[string]int{}
	path := "/api/v1/rankings?type=elo&limit=2"
	for pages := 0; pages < 5; pages++ {
		page := decodeRankingPage(t, f.get(path))
		for _, row := range page.Data {
			seen[row.Slug]++
		}
		if page.NextCursor == "" {
			break
		}
		path = "/api/v1/rankings?type=elo&limit=2&cursor=" + page.NextCursor
	}
	if len(seen) != 3 {
		t.Errorf("saw %d players over the pages, want 3", len(seen))
	}
	for slug, n := range seen {
		if n != 1 {
			t.Errorf("%s appeared %d times", slug, n)
		}
	}
}

// WTA data stops four years before ATP data does. Resolving the date across
// both tours answers a request for the WTA rankings with an ATP week and an
// empty list, which is the failure this endpoint exists to avoid.
func TestRankingsResolveTheDatePerTour(t *testing.T) {
	f := newAPIFixture(t)
	f.rankedField(t)

	wta := f.player("itg-wta-leader", "Itg Wta Leader", db.TourWta)
	f.rating(wta, "2021-12-27", "overall", 2300, 500)

	page := decodeRankingPage(t, f.get("/api/v1/rankings?type=elo&tour=wta"))
	if page.AsOf != "2021-12-27" {
		t.Errorf("as_of = %q, want the last week the WTA has", page.AsOf)
	}
	if len(page.Data) != 1 || page.Data[0].Slug != "itg-wta-leader" {
		t.Errorf("got %+v, want the one WTA player", page.Data)
	}

	// The ATP answer is unaffected by the tour with the shorter history.
	atp := decodeRankingPage(t, f.get("/api/v1/rankings?type=elo&tour=atp"))
	if atp.AsOf != "2026-01-05" {
		t.Errorf("atp as_of = %q", atp.AsOf)
	}
}

func TestRankingsRejectBadInput(t *testing.T) {
	f := newAPIFixture(t)
	for _, q := range []string{
		"?type=guess",
		"?type=elo&surface=mud",
		"?type=elo&tour=itf",
		"?type=elo&date=soon",
		"?type=elo&cursor=nope",
		// The tours publish one list, so a surfaced official ranking is a
		// question with no answer rather than an empty one.
		"?type=official&surface=clay",
	} {
		res := f.get("/api/v1/rankings" + q)
		if res.StatusCode != http.StatusBadRequest {
			t.Errorf("%q returned %d, want 400", q, res.StatusCode)
			continue
		}
		if ct := res.Header.Get("Content-Type"); ct != ProblemContentType {
			t.Errorf("%q returned content type %q, want a problem document", q, ct)
		}
	}
}

func TestTrajectoriesReturnOneLinePerPlayer(t *testing.T) {
	f := newAPIFixture(t)
	f.rankedField(t)

	res := f.get("/api/v1/rankings/trajectory?players=2&months=36")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var out Trajectories
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(out.Lines) != 2 {
		t.Fatalf("got %d lines, want the two asked for", len(out.Lines))
	}
	// Leaderboard order, so a client labels the top few without sorting again.
	if out.Lines[0].Slug != "itg-rank-one" || out.Lines[0].Position != 1 {
		t.Errorf("first line = %+v, want the leader", out.Lines[0])
	}
	if len(out.Lines[0].Points) != 2 {
		t.Errorf("got %d points, want both weeks inside the window", len(out.Lines[0].Points))
	}
	if out.To != "2026-01-05" {
		t.Errorf("to = %q, want the last week that exists", out.To)
	}
}
