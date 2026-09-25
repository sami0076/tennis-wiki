package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/db"
)

func decodeCommon(t *testing.T, res *http.Response) CommonOpponents {
	t.Helper()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var c CommonOpponents
	if err := json.NewDecoder(res.Body).Decode(&c); err != nil {
		t.Fatalf("decode common opponents: %v", err)
	}
	return c
}

// sharedField seeds two players, an opponent they have both played and one
// only the first has, plus one meeting between the two themselves.
func (f *apiFixture) sharedField() (open, a, b int64) {
	f.t.Helper()
	a = f.player("itg-common-a", "Itg Commona", db.TourAtp)
	b = f.player("itg-common-b", "Itg Commonb", db.TourAtp)
	shared := f.player("itg-common-shared", "Itg Commonshared", db.TourAtp)
	onlyA := f.player("itg-common-only", "Itg Commononly", db.TourAtp)
	open = f.tournament("itg-common-open", db.TierTour, 2019)

	// A is 2-0 against the shared opponent, B is 0-1.
	f.datedMatch(open, a, shared, 1, "2019-05-01")
	f.datedMatch(open, a, shared, 2, "2019-05-02")
	f.datedMatch(open, shared, b, 3, "2019-05-03")
	f.datedMatch(open, a, onlyA, 4, "2019-05-04")
	f.datedMatch(open, b, a, 5, "2019-05-05")
	return open, a, b
}

func TestCommonOpponentsCountsOnlyTheSharedField(t *testing.T) {
	f := newAPIFixture(t)
	f.sharedField()

	c := decodeCommon(t, f.get("/api/v1/h2h/itg-common-a/itg-common-b/common"))

	if len(c.Opponents) != 1 {
		t.Fatalf("opponents = %+v, want only the shared one", c.Opponents)
	}
	shared := c.Opponents[0]
	if shared.Slug != "itg-common-shared" {
		t.Errorf("opponent = %q, want itg-common-shared", shared.Slug)
	}
	if shared.Matches != [2]int{2, 1} || shared.Wins != [2]int{2, 0} {
		t.Errorf("records = %v of %v, want A 2-0 and B 0-1", shared.Wins, shared.Matches)
	}
	if c.Totals[0] != (CommonRecord{Matches: 2, Wins: 2}) {
		t.Errorf("A's total = %+v, want 2 wins from 2", c.Totals[0])
	}
	if c.Totals[1] != (CommonRecord{Matches: 1, Wins: 0}) {
		t.Errorf("B's total = %+v, want 0 wins from 1", c.Totals[1])
	}
}

func TestCommonOpponentsReadsTheSameFromEitherEnd(t *testing.T) {
	f := newAPIFixture(t)
	f.sharedField()

	forward := decodeCommon(t, f.get("/api/v1/h2h/itg-common-a/itg-common-b/common"))
	reverse := decodeCommon(t, f.get("/api/v1/h2h/itg-common-b/itg-common-a/common"))

	if forward.Players[0].Slug != reverse.Players[1].Slug {
		t.Fatalf("players did not swap: %v then %v", forward.Players, reverse.Players)
	}
	// The pairs are ordered by the URL, so reversing the URL reverses them and
	// nothing else. The same comparison, read from the other end.
	if forward.Opponents[0].Wins[0] != reverse.Opponents[0].Wins[1] {
		t.Errorf("A's record moved: %v then %v",
			forward.Opponents[0].Wins, reverse.Opponents[0].Wins)
	}
}

func TestCommonOpponentsExcludesThePairThemselves(t *testing.T) {
	f := newAPIFixture(t)
	f.sharedField()

	c := decodeCommon(t, f.get("/api/v1/h2h/itg-common-a/itg-common-b/common"))

	// They have played each other, so each is an opponent of the other. That
	// meeting is the head to head, not a third party to compare through.
	for _, o := range c.Opponents {
		if o.Slug == "itg-common-a" || o.Slug == "itg-common-b" {
			t.Fatalf("the pair appeared in their own shared field: %+v", o)
		}
	}
}

func TestCommonOpponentsRejectsAPlayerAgainstThemselves(t *testing.T) {
	f := newAPIFixture(t)
	f.player("itg-common-self", "Itg Commonself", db.TourAtp)

	res := f.get("/api/v1/h2h/itg-common-self/itg-common-self/common")
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
}

func TestCommonOpponentsIsNotFoundForAnUnknownSlug(t *testing.T) {
	f := newAPIFixture(t)
	f.player("itg-common-known", "Itg Commonknown", db.TourAtp)

	res := f.get("/api/v1/h2h/itg-common-known/itg-nobody/common")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.StatusCode)
	}
}
