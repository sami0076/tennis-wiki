package charting

import (
	"testing"
	"time"

	"github.com/sami0076/tennis-wiki/internal/ingest"
)

func day(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func charted(t *testing.T, p1, p2, date, round string) Match {
	t.Helper()
	return Match{ID: date + "-M-x-" + round, Tour: ingest.TourATP,
		Player1: p1, Player2: p2, PlayedOn: day(t, date), Round: round}
}

// Roland Garros 2026 starts Monday 25 May; the qualifying third round was
// charted on the 21st, four days before it. Zheng is Player 1 in the chart
// and the loser in the row, so the ids come back the chart's way round.
func TestResolveFindsTheRowAndOrdersThePlayersAsCharted(t *testing.T) {
	ix := NewIndex([]Candidate{
		{MatchID: 1, WinnerID: 10, LoserID: 20, Winner: "Jesper De Jong", Loser: "Michael Zheng",
			Round: "Q3", PlayedOn: day(t, "2026-05-25")},
		{MatchID: 2, WinnerID: 30, LoserID: 40, Winner: "Jesper De Jong", Loser: "Someone Else",
			Round: "Q3", PlayedOn: day(t, "2026-05-25")},
	})
	r, reason := ix.Resolve(charted(t, "Michael Zheng", "Jesper De Jong", "2026-05-21", "Q3"))
	if reason != "" {
		t.Fatalf("unresolved: %s", reason)
	}
	if r.MatchID != 1 || r.Player1ID != 20 || r.Player2ID != 10 {
		t.Errorf("got %+v, want match 1 with Zheng as player 1", r)
	}
}

func TestResolveLeavesAMatchOutsideTheWindow(t *testing.T) {
	ix := NewIndex([]Candidate{
		{MatchID: 1, WinnerID: 10, LoserID: 20, Winner: "John Mcenroe", Loser: "Bjorn Borg",
			Round: "F", PlayedOn: day(t, "1980-06-23")},
	})
	if _, reason := ix.Resolve(charted(t, "John Mcenroe", "Bjorn Borg", "1980-07-05", "F")); reason != "" {
		t.Errorf("a final twelve days after the start is inside the window: %s", reason)
	}
	if _, reason := ix.Resolve(charted(t, "John Mcenroe", "Bjorn Borg", "1980-09-07", "F")); reason == "" {
		t.Error("a final ten weeks after the start is another tournament")
	}
}

// The project's spellings against the tour files': hyphens folded, a married
// name appended, a surname the files never used. The last is not resolvable
// and is meant not to be.
func TestResolveNameForms(t *testing.T) {
	cases := []struct {
		charted, stored string
		want            bool
	}{
		{"Edouard Roger Vasselin", "Edouard Roger-Vasselin", true},
		{"Alison Riske Amritraj", "Alison Riske", true},
		{"Botic Van De Zandschulp", "Botic Van De Zandschulp", true},
		{"John Mcenroe", "John McEnroe", true},
		{"Storm Hunter", "Storm Sanders", false},
		{"Alexander Zverev", "Mischa Zverev", false},
	}
	for _, c := range cases {
		if got := samePerson(c.charted, c.stored); got != c.want {
			t.Errorf("samePerson(%q, %q) = %v, want %v", c.charted, c.stored, got, c.want)
		}
	}
}

// Two players who met in the same round of consecutive events: the one whose
// start is nearest before the charted date is the one it was played in.
func TestResolvePrefersTheEventContainingTheDate(t *testing.T) {
	ix := NewIndex([]Candidate{
		{MatchID: 1, WinnerID: 1, LoserID: 2, Winner: "A One", Loser: "B Two", Round: "R32", PlayedOn: day(t, "2025-08-04")},
		{MatchID: 2, WinnerID: 2, LoserID: 1, Winner: "B Two", Loser: "A One", Round: "R32", PlayedOn: day(t, "2025-08-11")},
	})
	r, reason := ix.Resolve(charted(t, "A One", "B Two", "2025-08-12", "R32"))
	if reason != "" || r.MatchID != 2 {
		t.Errorf("got %+v (%s), want match 2, the event that had started the day before", r, reason)
	}
	r, reason = ix.Resolve(charted(t, "A One", "B Two", "2025-08-06", "R32"))
	if reason != "" || r.MatchID != 1 {
		t.Errorf("got %+v (%s), want match 1, the only event under way", r, reason)
	}
}

func TestResolveRefusesAGenuineTie(t *testing.T) {
	ix := NewIndex([]Candidate{
		{MatchID: 1, WinnerID: 1, LoserID: 2, Winner: "A One", Loser: "B Two", Round: "RR", PlayedOn: day(t, "2025-11-10")},
		{MatchID: 2, WinnerID: 1, LoserID: 2, Winner: "A One", Loser: "B Two", Round: "RR", PlayedOn: day(t, "2025-11-10")},
	})
	if _, reason := ix.Resolve(charted(t, "A One", "B Two", "2025-11-12", "RR")); reason == "" {
		t.Error("two rows with nothing to choose between them must not be resolved to one")
	}
}
