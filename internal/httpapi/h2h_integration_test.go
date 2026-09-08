package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/db"
)

func decodeH2H(t *testing.T, res *http.Response) HeadToHead {
	t.Helper()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var h HeadToHead
	if err := json.NewDecoder(res.Body).Decode(&h); err != nil {
		t.Fatalf("decode head to head: %v", err)
	}
	return h
}

func TestHeadToHeadReturnsTheRecordAndTheMatchesBehindIt(t *testing.T) {
	f := newAPIFixture(t)
	pts := int16(80)

	borg := f.player("itg-borg", "Itg Borg", db.TourAtp)
	mac := f.player("itg-mac", "Itg Mac", db.TourAtp)
	other := f.player("itg-other", "Itg Other", db.TourAtp)
	open := f.tournament("itg-h2h-open", db.TierTour, 2019)

	f.historyMatch(open, borg, mac, 1, "F", "grass", "2019-07-06", "6-4 6-4", &pts)
	f.historyMatch(open, mac, borg, 2, "SF", "hard", "2020-09-05", "7-5 6-3", &pts)
	f.historyMatch(open, borg, mac, 3, "QF", "clay", "2021-06-05", "6-2 6-2", &pts)
	// A match against somebody else must not appear in the comparison.
	f.historyMatch(open, borg, other, 4, "R16", "clay", "2021-06-03", "6-0 6-0", &pts)

	h := decodeH2H(t, f.get("/api/v1/h2h/itg-borg/itg-mac"))

	if h.Record.Matches != 3 || h.Record.Wins != [2]int{2, 1} {
		t.Errorf("record = %+v, want 2-1 over three meetings", h.Record)
	}
	if len(h.Meetings) != 3 {
		t.Fatalf("got %d meetings, want the three they played", len(h.Meetings))
	}
	// Oldest first, because that is how a rivalry reads.
	if h.Meetings[0].Date != "2019-07-06" || h.Meetings[2].Date != "2021-06-05" {
		t.Errorf("meetings run %s to %s, want oldest first",
			h.Meetings[0].Date, h.Meetings[2].Date)
	}
	if h.Meetings[1].WinnerIndex != 1 {
		t.Errorf("the 2020 meeting was won by index %d, want the second player",
			h.Meetings[1].WinnerIndex)
	}
	if len(h.Surfaces) != 3 {
		t.Errorf("surfaces = %+v, want one split per surface they met on", h.Surfaces)
	}
	if h.Serve[0].Availability != AvailabilityRecorded || h.Serve[0].Rates == nil {
		t.Errorf("serve = %+v, want rates over the meetings", h.Serve[0])
	}
}

// The comparison is one thing read from two ends. Two pages that could disagree
// is the failure this is guarding against.
func TestHeadToHeadAgreesInBothDirections(t *testing.T) {
	f := newAPIFixture(t)
	borg := f.player("itg-mirror-a", "Itg Mirror A", db.TourAtp)
	mac := f.player("itg-mirror-b", "Itg Mirror B", db.TourAtp)
	open := f.tournament("itg-mirror-open", db.TierTour, 2019)

	f.historyMatch(open, borg, mac, 1, "F", "grass", "2019-07-06", "6-4 6-4", nil)
	f.historyMatch(open, borg, mac, 2, "SF", "grass", "2020-07-06", "6-4 6-4", nil)
	f.historyMatch(open, mac, borg, 3, "QF", "clay", "2021-06-05", "6-2 6-2", nil)

	forward := decodeH2H(t, f.get("/api/v1/h2h/itg-mirror-a/itg-mirror-b"))
	reverse := decodeH2H(t, f.get("/api/v1/h2h/itg-mirror-b/itg-mirror-a"))

	if forward.Record.Wins != [2]int{2, 1} || reverse.Record.Wins != [2]int{1, 2} {
		t.Errorf("forward %v and reverse %v are not the same comparison",
			forward.Record.Wins, reverse.Record.Wins)
	}
	if forward.Record.Matches != reverse.Record.Matches {
		t.Error("the two directions disagree about how many matches were played")
	}
	if len(forward.Meetings) != len(reverse.Meetings) {
		t.Fatal("the two directions list different meetings")
	}
	for i := range forward.Meetings {
		if forward.Meetings[i].Date != reverse.Meetings[i].Date {
			t.Errorf("meeting %d differs between directions", i)
		}
		// The winner index follows the order the URL asked for.
		if forward.Meetings[i].WinnerIndex == reverse.Meetings[i].WinnerIndex {
			t.Errorf("meeting %d names the same index as winner from both ends", i)
		}
	}
	if forward.Players[0].Slug != "itg-mirror-a" || reverse.Players[0].Slug != "itg-mirror-b" {
		t.Error("the response is not oriented to the order the URL asked for")
	}
}

// Two players who never met is a real answer to a real question.
func TestHeadToHeadOfAPairWhoNeverMet(t *testing.T) {
	f := newAPIFixture(t)
	f.player("itg-never-a", "Itg Never A", db.TourAtp)
	f.player("itg-never-b", "Itg Never B", db.TourAtp)

	h := decodeH2H(t, f.get("/api/v1/h2h/itg-never-a/itg-never-b"))
	if h.Record.Matches != 0 || len(h.Meetings) != 0 {
		t.Errorf("got %+v, want an empty comparison", h.Record)
	}
	if h.Players[0].Name != "Itg Never A" || h.Players[1].Name != "Itg Never B" {
		t.Error("the players are missing from a comparison with no meetings")
	}
	// Nothing recorded, and the response says which kind of nothing.
	if h.Serve[0].Availability == AvailabilityRecorded {
		t.Errorf("serve = %+v, want an absence", h.Serve[0])
	}
}

func TestHeadToHeadRejectsBadPairs(t *testing.T) {
	f := newAPIFixture(t)
	f.player("itg-solo", "Itg Solo", db.TourAtp)

	if res := f.get("/api/v1/h2h/itg-solo/itg-nobody"); res.StatusCode != http.StatusNotFound {
		t.Errorf("an unknown opponent returned %d, want 404", res.StatusCode)
	}
	if res := f.get("/api/v1/h2h/itg-nobody/itg-solo"); res.StatusCode != http.StatusNotFound {
		t.Errorf("an unknown player returned %d, want 404", res.StatusCode)
	}
	// A player against themselves is a question with no answer.
	res := f.get("/api/v1/h2h/itg-solo/itg-solo")
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("a self-comparison returned %d, want 400", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != ProblemContentType {
		t.Errorf("content type = %q, want a problem document", ct)
	}
}

// A rivalry played entirely at Futures has no statistics for a reason that has
// nothing to do with either career, and the response explains that reason.
func TestHeadToHeadStatisticsAreAbsentNotZero(t *testing.T) {
	f := newAPIFixture(t)
	a := f.player("itg-fut-a", "Itg Fut A", db.TourAtp)
	b := f.player("itg-fut-b", "Itg Fut B", db.TourAtp)
	futures := f.tournament("itg-fut-open", db.TierFutures, 2019)

	f.historyMatch(futures, a, b, 1, "F", "hard", "2019-03-02", "6-4 6-4", nil)
	f.historyMatch(futures, b, a, 2, "SF", "hard", "2019-04-02", "6-4 6-4", nil)

	h := decodeH2H(t, f.get("/api/v1/h2h/itg-fut-a/itg-fut-b"))
	if h.Record.Wins != [2]int{1, 1} {
		t.Errorf("record = %v, want one each", h.Record.Wins)
	}
	for i, serve := range h.Serve {
		if serve.Availability != AvailabilityNeverForTier {
			t.Errorf("player %d availability = %q, want the tier explanation", i, serve.Availability)
		}
		if serve.Rates != nil {
			t.Errorf("player %d has rates over matches that recorded none", i)
		}
	}
}

// Retirements belong in the record -- somebody advanced -- and are counted
// apart so a page can say so.
func TestHeadToHeadFlagsIncompleteMeetings(t *testing.T) {
	f := newAPIFixture(t)
	a := f.player("itg-ret-a", "Itg Ret A", db.TourAtp)
	b := f.player("itg-ret-b", "Itg Ret B", db.TourAtp)
	open := f.tournament("itg-ret-open", db.TierTour, 2019)

	f.historyMatch(open, a, b, 1, "F", "hard", "2019-05-02", "6-4 RET", nil)
	f.historyMatch(open, a, b, 2, "SF", "hard", "2019-05-01", "6-4 6-4", nil)

	h := decodeH2H(t, f.get("/api/v1/h2h/itg-ret-a/itg-ret-b"))
	if h.Record.Matches != 2 || h.Record.Wins[0] != 2 {
		t.Errorf("record = %+v, want the retirement counted as a win", h.Record)
	}
	if h.Record.Incomplete != 1 {
		t.Errorf("incomplete = %d, want the one retirement flagged", h.Record.Incomplete)
	}
}
