package simulate

import (
	"errors"
	"fmt"
	"sort"
)

// BracketMatch is one completed main-draw match, as the reconstruction needs
// it: who played, who won, and which round it was.
type BracketMatch struct {
	Round    string
	RoundIdx int
	WinnerID int64
	LoserID  int64
}

// Entrant is one player in a draw. The zero Entrant is a bye: the empty line
// under a seed who skipped the first round.
type Entrant struct {
	PlayerID int64
	Slug     string
	Name     string
	Seed     *int
}

// Bye reports whether this line of the draw is empty.
func (e Entrant) Bye() bool { return e.PlayerID == 0 }

// Bracket is a draw as a flat binary tree.
//
// Entrants is a power of two in bracket order: 0 plays 1, and the winner meets
// the winner of 2 against 3. That single slice is the whole tree, and it is
// what lets a simulation play the draw forward with an index shift instead of
// pointer chasing. A bye is a zero Entrant next to the player who had it.
type Bracket struct {
	Entrants []Entrant
	// Byes is how many of the Entrants are empty lines.
	Byes int
	// Rounds are the round codes from the first to the final, so a result can
	// name the round a player reached rather than counting them.
	Rounds []string
	// Champion is who actually won it. A reconstruction is of an event that
	// was played, so the answer is available to score a simulation against.
	Champion int64
}

// Size is the tree: the number of lines, byes included.
func (b Bracket) Size() int { return len(b.Entrants) }

// Entered is how many players were in the draw.
func (b Bracket) Entered() int { return len(b.Entrants) - b.Byes }

// Errors a draw can fail to reconstruct with. Each is a real shape in the data
// rather than a defect, so they are reported and skipped rather than fixed.
var (
	// ErrNotPowerOfTwo covers round-robin groups and rounds the source recorded
	// partially, either of which leaves a round the wrong size for the tree.
	ErrNotPowerOfTwo = errors.New("the draw is not a power-of-two bracket")
	// ErrBrokenTree means a player appeared in a round above the first without
	// having won one in the round below.
	ErrBrokenTree = errors.New("a round does not link to the one below it")
	ErrNoMatches  = errors.New("no main-draw matches")
)

// BuildBracket reconstructs a draw from the matches that were played.
//
// Built downward from the final rather than upward from the first round,
// because the tree is only unambiguous in that direction: the final names two
// players, each of them won a semi-final, and so on down to the pairs that
// opened the event. Going the other way would mean guessing which first-round
// matches feed which second-round slot, and the source does not record draw
// positions.
//
// A player in the second round with no first-round match had a bye, which no
// source records as a row; the sheet reads it the same way. Higher up, a
// missing match is a hole in the data, and the draw is rejected rather than
// patched. Every match eliminates one player, so a draw that reconstructs has
// exactly one more entrant than matches -- which is what catches a round that
// is present but does not feed the one above it.
func BuildBracket(matches []BracketMatch, players map[int64]Entrant) (Bracket, error) {
	if len(matches) == 0 {
		return Bracket{}, ErrNoMatches
	}

	// Group by round, in the order they were played.
	byRound := map[int][]BracketMatch{}
	var order []int
	for _, m := range matches {
		if _, seen := byRound[m.RoundIdx]; !seen {
			order = append(order, m.RoundIdx)
		}
		byRound[m.RoundIdx] = append(byRound[m.RoundIdx], m)
	}
	sort.Ints(order)

	// The tree is as deep as there are rounds. Every round above the first is
	// full; the first may be short, by the number of byes.
	total := len(matches)
	rounds := make([]string, 0, len(order))
	want := 1 << (len(order) - 1)
	for i, idx := range order {
		n := len(byRound[idx])
		if n > want || (i > 0 && n != want) {
			return Bracket{}, fmt.Errorf("%w: round %q has %d matches, want %d",
				ErrNotPowerOfTwo, byRound[idx][0].Round, n, want)
		}
		rounds = append(rounds, byRound[idx][0].Round)
		want /= 2
	}

	// wonBy[round][player] is the match that player won in that round.
	wonBy := make(map[int]map[int64]BracketMatch, len(order))
	for idx, ms := range byRound {
		wonBy[idx] = make(map[int64]BracketMatch, len(ms))
		for _, m := range ms {
			wonBy[idx][m.WinnerID] = m
		}
	}

	final := byRound[order[len(order)-1]][0]
	entrants := make([]Entrant, 0, 1<<len(order))
	byes := 0

	// walk appends the leaves under the match `winner` won in `round`, left to
	// right, which is the bracket order the flat slice needs.
	var walk func(depth int, winner int64) error
	walk = func(depth int, winner int64) error {
		m, ok := wonBy[order[depth]][winner]
		if !ok && depth == 0 {
			entrants = append(entrants, players[winner], Entrant{})
			byes++
			return nil
		}
		if !ok {
			return fmt.Errorf("%w: nobody won a %s that %d played above",
				ErrBrokenTree, rounds[depth], winner)
		}
		if depth == 0 {
			entrants = append(entrants, players[m.WinnerID], players[m.LoserID])
			return nil
		}
		if err := walk(depth-1, m.WinnerID); err != nil {
			return err
		}
		return walk(depth-1, m.LoserID)
	}

	if err := walk(len(order)-1, final.WinnerID); err != nil {
		return Bracket{}, err
	}
	if len(entrants)-byes != total+1 {
		return Bracket{}, fmt.Errorf("%w: reached %d of %d entrants",
			ErrBrokenTree, len(entrants)-byes, total+1)
	}

	return Bracket{Entrants: entrants, Byes: byes, Rounds: rounds, Champion: final.WinnerID}, nil
}
