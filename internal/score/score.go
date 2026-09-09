package score

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Outcome is how a match ended.
type Outcome int

// How a match ended, as recorded in the source score string.
const (
	Complete  Outcome = iota
	Retired           // RET
	Walkover          // W/O
	Defaulted         // DEF
	Abandoned         // abandoned or unfinished
	Unknown           // UNK, ?-?, or empty
)

func (o Outcome) String() string {
	switch o {
	case Complete:
		return "complete"
	case Retired:
		return "retired"
	case Walkover:
		return "walkover"
	case Defaulted:
		return "defaulted"
	case Abandoned:
		return "abandoned"
	default:
		return "unknown"
	}
}

// Set is one set, always from the match winner's perspective — that is how the
// source encodes it, so "6-3 3-6 6-4" has a middle set the match winner lost.
type Set struct {
	GamesWinner    int
	GamesLoser     int
	TiebreakWinner int // zero when the set had no tiebreak
	TiebreakLoser  int
	// SuperTiebreak marks a match tiebreak played in place of a final set,
	// written as [10-7] in the source.
	SuperTiebreak bool
}

// HasTiebreak reports whether the source wrote the tiebreak points down.
func (s Set) HasTiebreak() bool { return s.TiebreakWinner > 0 || s.TiebreakLoser > 0 }

// TiebreakSet reports whether the set was decided by a tiebreak, points written
// down or not.
//
// A set cannot be won by a single game any other way: without a tiebreak it
// goes to two clear games. So a one-game margin at 7 or above is a tiebreak --
// 7-6, and also the 9-8 and 13-12 the longer formats produce. Requiring the
// points instead would undercount every tiebreak in the decades the files did
// not record them, and Bjorn Borg would come out with 35 in 764 matches.
func (s Set) TiebreakSet() bool {
	if s.SuperTiebreak {
		return false
	}
	if s.HasTiebreak() {
		return true
	}
	high, low := s.GamesWinner, s.GamesLoser
	if low > high {
		high, low = low, high
	}
	return high >= 7 && high-low == 1
}

func (s Set) String() string {
	if s.SuperTiebreak {
		return fmt.Sprintf("[%d-%d]", s.TiebreakWinner, s.TiebreakLoser)
	}
	games := fmt.Sprintf("%d-%d", s.GamesWinner, s.GamesLoser)
	if !s.HasTiebreak() {
		return games
	}
	// The short form only carries the loser's points, and is unambiguous
	// whenever the winner's score follows from them.
	if s.TiebreakWinner == impliedTiebreakWinner(s.TiebreakLoser) {
		return games + "(" + strconv.Itoa(s.TiebreakLoser) + ")"
	}
	return fmt.Sprintf("%s(%d-%d)", games, s.TiebreakWinner, s.TiebreakLoser)
}

// Score is a parsed score line.
type Score struct {
	Sets    []Set
	Outcome Outcome
	Raw     string
}

// Incomplete reports whether the match failed to finish, which maps to
// matches.incomplete.
func (s Score) Incomplete() bool { return s.Outcome != Complete }

// SetsWon returns sets won by the match winner and by the loser.
func (s Score) SetsWon() (winner, loser int) {
	for _, set := range s.Sets {
		switch {
		case set.GamesWinner > set.GamesLoser:
			winner++
		case set.GamesLoser > set.GamesWinner:
			loser++
		}
	}
	return winner, loser
}

// Tiebreaks counts set tiebreaks won by the match winner and by the loser.
//
// Which side won one is read from the games, not from the tiebreak points: the
// parser names the tiebreak's own winner, so "6-7(5)" carries a 7-5 tiebreak
// that the match winner lost. Reading the games is also what lets a set whose
// points were never written down still count -- see TiebreakSet.
//
// A match tiebreak played in place of a final set is not counted here. It
// decides a match rather than a set, and WentToDecider already counts it;
// counting both would put the same moment in two figures.
func (s Score) Tiebreaks() (winner, loser int) {
	for _, set := range s.Sets {
		if !set.TiebreakSet() {
			continue
		}
		switch {
		case set.GamesWinner > set.GamesLoser:
			winner++
		case set.GamesLoser > set.GamesWinner:
			loser++
		}
	}
	return winner, loser
}

// WentToDecider reports whether a finished match reached its deciding set --
// the third of a best of three, the fifth of a best of five. A match tiebreak
// standing in for that set counts: it is the deciding set, played short.
//
// Only a finished match. Somebody advanced from a third-set retirement, but
// nobody won that set, and every rate in this project leaves incomplete
// matches out.
func (s Score) WentToDecider(bestOf int) bool {
	if s.Outcome != Complete || (bestOf != 3 && bestOf != 5) {
		return false
	}
	return len(s.Sets) == bestOf
}

func (s Score) String() string {
	parts := make([]string, 0, len(s.Sets)+1)
	for _, set := range s.Sets {
		parts = append(parts, set.String())
	}
	if suffix := outcomeSuffix(s.Outcome); suffix != "" {
		parts = append(parts, suffix)
	}
	return strings.Join(parts, " ")
}

func outcomeSuffix(o Outcome) string {
	switch o {
	case Retired:
		return "RET"
	case Walkover:
		return "W/O"
	case Defaulted:
		return "DEF"
	case Unknown:
		return "UNK"
	default:
		return ""
	}
}

// ErrMalformed is returned for input that cannot be parsed at all.
var ErrMalformed = errors.New("malformed score")
