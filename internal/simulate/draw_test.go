package simulate

import (
	"errors"
	"testing"
)

// A four-player draw: 1 beats 2, 3 beats 4, then 1 beats 3.
func fourPlayerDraw() ([]BracketMatch, map[int64]Entrant) {
	matches := []BracketMatch{
		{Round: "SF", RoundIdx: 1, WinnerID: 1, LoserID: 3},
		{Round: "R16", RoundIdx: 0, WinnerID: 1, LoserID: 2},
		{Round: "R16", RoundIdx: 0, WinnerID: 3, LoserID: 4},
	}
	players := map[int64]Entrant{}
	for _, id := range []int64{1, 2, 3, 4} {
		players[id] = Entrant{PlayerID: id, Slug: "p", Name: "P"}
	}
	return matches, players
}

func TestBuildBracket(t *testing.T) {
	t.Parallel()

	matches, players := fourPlayerDraw()
	b, err := BuildBracket(matches, players)
	if err != nil {
		t.Fatalf("BuildBracket: %v", err)
	}

	if b.Size() != 4 {
		t.Fatalf("size = %d, want 4", b.Size())
	}
	if b.Champion != 1 {
		t.Errorf("champion = %d, want 1", b.Champion)
	}
	if len(b.Rounds) != 2 || b.Rounds[0] != "R16" || b.Rounds[1] != "SF" {
		t.Errorf("rounds = %v, want the first round first", b.Rounds)
	}

	// Bracket order: the two who met in the first round are adjacent, and the
	// pairs that met in the second are the halves.
	got := []int64{}
	for _, e := range b.Entrants {
		got = append(got, e.PlayerID)
	}
	want := []int64{1, 2, 3, 4}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("entrants = %v, want %v: adjacent pairs must be the first-round matches",
				got, want)
		}
	}
}

// The order matches were handed over must not matter: the tree is built from
// who beat whom, not from the order rows came back.
func TestBuildBracketIgnoresInputOrder(t *testing.T) {
	t.Parallel()

	matches, players := fourPlayerDraw()
	forward, err := BuildBracket(matches, players)
	if err != nil {
		t.Fatalf("forward: %v", err)
	}

	reversed := make([]BracketMatch, len(matches))
	for i, m := range matches {
		reversed[len(matches)-1-i] = m
	}
	backward, err := BuildBracket(reversed, players)
	if err != nil {
		t.Fatalf("backward: %v", err)
	}

	if forward.Champion != backward.Champion || forward.Size() != backward.Size() {
		t.Error("the same draw reconstructed differently from a different row order")
	}
}

// A player in the second round with no first-round match had a bye. The line
// under them stays empty, so the tree keeps its shape and the pairs above it
// still fall out of the index arithmetic.
func TestBuildBracketPlacesByes(t *testing.T) {
	t.Parallel()

	players := map[int64]Entrant{}
	for id := int64(1); id <= 6; id++ {
		players[id] = Entrant{PlayerID: id}
	}
	// Six in a draw of eight: 5 and 6 sat out the quarter-finals.
	matches := []BracketMatch{
		{Round: "QF", RoundIdx: 0, WinnerID: 1, LoserID: 2},
		{Round: "QF", RoundIdx: 0, WinnerID: 3, LoserID: 4},
		{Round: "SF", RoundIdx: 1, WinnerID: 1, LoserID: 5},
		{Round: "SF", RoundIdx: 1, WinnerID: 3, LoserID: 6},
		{Round: "F", RoundIdx: 2, WinnerID: 1, LoserID: 3},
	}
	b, err := BuildBracket(matches, players)
	if err != nil {
		t.Fatalf("BuildBracket: %v", err)
	}
	if b.Size() != 8 || b.Byes != 2 || b.Entered() != 6 {
		t.Fatalf("size %d, byes %d, entered %d; want 8, 2, 6", b.Size(), b.Byes, b.Entered())
	}
	want := []int64{1, 2, 5, 0, 3, 4, 6, 0}
	for i, e := range b.Entrants {
		if e.PlayerID != want[i] {
			t.Fatalf("entrant %d = %d, want %d: a bye sits next to the player who had it",
				i, e.PlayerID, want[i])
		}
		if e.Bye() != (want[i] == 0) {
			t.Errorf("entrant %d: Bye() = %v", i, e.Bye())
		}
	}
}

// Above the first round a missing match is not a bye: a semi-finalist who won
// no quarter-final is a hole in the source, and the draw is refused.
func TestBuildBracketRejectsAHoleAboveTheFirstRound(t *testing.T) {
	t.Parallel()

	players := map[int64]Entrant{}
	for id := int64(1); id <= 17; id++ {
		players[id] = Entrant{PlayerID: id}
	}
	var matches []BracketMatch
	for id := int64(1); id <= 16; id += 2 {
		matches = append(matches, BracketMatch{Round: "R16", RoundIdx: 0, WinnerID: id, LoserID: id + 1})
	}
	for id := int64(1); id <= 16; id += 4 {
		matches = append(matches, BracketMatch{Round: "QF", RoundIdx: 1, WinnerID: id, LoserID: id + 2})
	}
	matches = append(matches,
		BracketMatch{Round: "SF", RoundIdx: 2, WinnerID: 1, LoserID: 5},
		BracketMatch{Round: "SF", RoundIdx: 2, WinnerID: 9, LoserID: 17}, // 17 won no QF; 13 vanished
		BracketMatch{Round: "F", RoundIdx: 3, WinnerID: 1, LoserID: 9},
	)
	if _, err := BuildBracket(matches, players); !errors.Is(err, ErrBrokenTree) {
		t.Errorf("error = %v, want ErrBrokenTree", err)
	}
}

// A first round that is the right size but does not feed the round above --
// two of its winners never play again, two semi-finalists never played it --
// is not two byes. Every match eliminates one player, so the count gives it
// away.
func TestBuildBracketRejectsAFirstRoundThatDoesNotLinkUp(t *testing.T) {
	t.Parallel()

	players := map[int64]Entrant{}
	for id := int64(1); id <= 10; id++ {
		players[id] = Entrant{PlayerID: id}
	}
	matches := []BracketMatch{
		{Round: "QF", RoundIdx: 0, WinnerID: 1, LoserID: 2},
		{Round: "QF", RoundIdx: 0, WinnerID: 3, LoserID: 4},
		{Round: "QF", RoundIdx: 0, WinnerID: 7, LoserID: 8},
		{Round: "QF", RoundIdx: 0, WinnerID: 9, LoserID: 10},
		{Round: "SF", RoundIdx: 1, WinnerID: 1, LoserID: 5},
		{Round: "SF", RoundIdx: 1, WinnerID: 3, LoserID: 6},
		{Round: "F", RoundIdx: 2, WinnerID: 1, LoserID: 3},
	}
	if _, err := BuildBracket(matches, players); !errors.Is(err, ErrBrokenTree) {
		t.Errorf("error = %v, want ErrBrokenTree", err)
	}
}

func TestBuildBracketRejectsNothing(t *testing.T) {
	t.Parallel()

	if _, err := BuildBracket(nil, nil); !errors.Is(err, ErrNoMatches) {
		t.Errorf("error = %v, want ErrNoMatches", err)
	}
}

// A round that is the wrong size for its place in the tree is a round-robin
// group or a draw the source recorded partially.
func TestBuildBracketRejectsAMalformedRound(t *testing.T) {
	t.Parallel()

	players := map[int64]Entrant{}
	for id := int64(1); id <= 4; id++ {
		players[id] = Entrant{PlayerID: id}
	}
	matches := []BracketMatch{
		{Round: "RR", RoundIdx: 0, WinnerID: 1, LoserID: 2},
		{Round: "RR", RoundIdx: 0, WinnerID: 3, LoserID: 4},
		{Round: "RR", RoundIdx: 0, WinnerID: 1, LoserID: 3},
	}
	if _, err := BuildBracket(matches, players); !errors.Is(err, ErrNotPowerOfTwo) {
		t.Errorf("error = %v, want ErrNotPowerOfTwo for a group stage", err)
	}
}
