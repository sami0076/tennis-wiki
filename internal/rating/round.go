package rating

import (
	"strconv"
	"strings"
)

// rounds are the round codes in the order they are played within one
// tournament. Qualifying comes first, and the challenge round -- the old format
// where the defending champion waits for the winner of the all-comers draw --
// comes after the final, because that is when it was played.
//
// Ordering by date alone is not enough: 63,676 of the 63,681 tournaments in the
// database carry a single played_on for every match, so a final would otherwise
// be rated before the semi-final that produced its finalist.
var rounds = []string{
	"Q1", "Q2", "Q3", "Q4", "Q5", // qualifying draws
	"ER",                                    // early rounds, Davis Cup ties
	"RR", "R128", "R64", "R32", "R16", "QF", // round robin sits with the first main-draw round
	"SF", "BR", "F", "CR",
}

// RoundRank orders one round within its tournament. An unrecognised code sorts
// last, which keeps the order total without pretending to know where it goes.
func RoundRank(round string) int {
	for i, r := range rounds {
		if r == round {
			return i
		}
	}
	return len(rounds)
}

// RoundRankSQL renders RoundRank as an expression over the given column, so the
// database sorts by exactly the order the engine expects and there is one place
// to change it.
func RoundRankSQL(column string) string {
	var b strings.Builder
	b.WriteString("CASE " + column)
	for i, r := range rounds {
		b.WriteString(" WHEN '" + r + "' THEN " + strconv.Itoa(i))
	}
	b.WriteString(" ELSE " + strconv.Itoa(len(rounds)) + " END")
	return b.String()
}
