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

// HasTiebreak reports whether the set was decided by a tiebreak.
func (s Set) HasTiebreak() bool { return s.TiebreakWinner > 0 || s.TiebreakLoser > 0 }

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
