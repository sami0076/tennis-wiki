package simulate

import "math"

// Format is the scoring the match is played under.
type Format struct {
	// BestOf is 3 or 5. Anything else is read as 3.
	BestOf int
	// FinalSetAdvantage plays the deciding set out rather than to a tiebreak,
	// as Wimbledon did until 2019 and the other majors until later. It applies
	// to the deciding set only: no era played every set that way.
	FinalSetAdvantage bool
}

func (f Format) sets() int {
	if f.BestOf == 5 {
		return 5
	}
	return 3
}

// Chain is every rung between a point and a match.
//
// The chain is the feature, not the last number in it. A two-point edge on
// serve becomes a twelve-point edge on the match, and a response that carried
// only the match probability would be asking the reader to take that on trust.
//
// Point and Hold are each player's own figures and do not sum to 1 -- both can
// hold most of the time. Set and Match are shares of one outcome and do.
type Chain struct {
	// Point is each player's probability of winning a point on their own serve.
	Point [2]float64
	Hold  [2]float64
	Set   [2]float64
	Match [2]float64
}

// Solve runs the chain from two serve probabilities, with A serving first.
//
// Sets are treated as independent and identically distributed, which is the
// standard simplification and not quite true: who serves first in the second
// set depends on how many games the first one took. The error it introduces is
// small, and where the probabilities came from an inversion (see Invert) it is
// absorbed entirely -- the inversion solves for whatever point probabilities
// reproduce the target match probability under this same assumption, so the
// assumption cancels at match level and survives only in the decomposition.
func Solve(pA, pB float64, f Format) Chain {
	holdA, holdB := Game(pA), Game(pB)
	setA := Set(pA, pB, false)

	decider := setA
	if f.FinalSetAdvantage {
		decider = Set(pA, pB, true)
	}
	matchA := match(setA, decider, f.sets())

	return Chain{
		Point: [2]float64{pA, pB},
		Hold:  [2]float64{holdA, holdB},
		Set:   [2]float64{setA, 1 - setA},
		Match: [2]float64{matchA, 1 - matchA},
	}
}

// Game is the probability that a server who wins p of their service points wins
// the game.
//
// Closed form, deuce included: from deuce the game is a race to a two-point
// lead, which is an absorbing chain with an exact answer rather than something
// to iterate until it stops moving.
func Game(p float64) float64 {
	q := 1 - p
	// 4-0, 4-1 and 4-2, then the 3-3 paths that reach deuce.
	straight := p * p * p * p * (1 + 4*q + 10*q*q)
	toDeuce := 20 * p * p * p * q * q * q
	return straight + toDeuce*twoPointRace(p, q)
}

// twoPointRace is the probability the first player takes a two-point lead,
// given the chance each of them wins one of the paired points.
//
// It is the shape behind deuce, behind 6-6 in a tiebreak and behind an
// advantage set, which is why it is one function: in every case the pair
// alternates so that each side wins one of the two, and only winning both ends
// it.
func twoPointRace(winBoth, loseBoth float64) float64 {
	total := winBoth*winBoth + loseBoth*loseBoth
	if total == 0 {
		// Neither side can ever win two in a row, so the race does not end.
		// Half is the only answer symmetry allows.
		return 0.5
	}
	return winBoth * winBoth / total
}

// Tiebreak is the probability A wins a first-to-seven tiebreak, A serving the
// first point.
//
// Both players' probabilities appear in it because the serve alternates: one
// point, then two, then two. That pattern is why a tiebreak is not a game with
// different numbers.
func Tiebreak(pA, pB float64) float64 {
	// win[a][b] is A's chance from that score. Filled from the end backwards,
	// so every state is written before anything reads it.
	var win [7][7]float64

	value := func(a, b int) float64 {
		if a == 7 {
			return 1
		}
		if b == 7 {
			return 0
		}
		return win[a][b]
	}

	for n := 12; n >= 0; n-- {
		for a := 0; a <= 6; a++ {
			b := n - a
			if b < 0 || b > 6 {
				continue
			}
			if a == 6 && b == 6 {
				// Beyond 6-6 the points pair up one serve each, whichever way
				// round, so a two-point race settles it.
				win[a][b] = twoPointRace(pA*(1-pB), (1-pA)*pB)
				continue
			}
			p := 1 - pB
			if serverIsA(a + b) {
				p = pA
			}
			win[a][b] = p*value(a+1, b) + (1-p)*value(a, b+1)
		}
	}
	return win[0][0]
}

// serverIsA reports who serves point n of a tiebreak, counting from zero. A
// serves one, then B serves two, then A serves two, and so on.
func serverIsA(n int) bool {
	switch n % 4 {
	case 0, 3:
		return true
	default:
		return false
	}
}

// Set is the probability A wins a set, serving the first game.
//
// advantage plays 6-6 out to a two-game lead instead of a tiebreak.
func Set(pA, pB float64, advantage bool) float64 {
	holdA, holdB := Game(pA), Game(pB)

	atSixAll := Tiebreak(pA, pB)
	if advantage {
		// The same pairing as a tiebreak's, a game at a time: each player
		// serves one of every two, and only taking both ends the set.
		atSixAll = twoPointRace(holdA*(1-holdB), (1-holdA)*holdB)
	}

	var win [7][7]float64
	value := func(a, b int) float64 {
		// Six games with two clear, or 7-5 after the set went past it.
		if a == 6 && b <= 4 {
			return 1
		}
		if b == 6 && a <= 4 {
			return 0
		}
		if a == 7 {
			return 1
		}
		if b == 7 {
			return 0
		}
		return win[a][b]
	}

	for n := 12; n >= 0; n-- {
		for a := 0; a <= 6; a++ {
			b := n - a
			if b < 0 || b > 6 {
				continue
			}
			if (a == 6 && b <= 4) || (b == 6 && a <= 4) {
				continue // terminal; value answers for it
			}
			if a == 6 && b == 6 {
				win[a][b] = atSixAll
				continue
			}
			// A serves the even-numbered games, having served the first.
			p := holdA
			if (a+b)%2 == 1 {
				p = 1 - holdB
			}
			win[a][b] = p*value(a+1, b) + (1-p)*value(a, b+1)
		}
	}
	return value(0, 0)
}

// match is the probability A wins, given their chance in a normal set and in
// the deciding one. The two differ only when the decider is an advantage set.
func match(set, decider float64, bestOf int) float64 {
	lose := 1 - set
	if bestOf == 5 {
		// 3-0, 3-1, and 3-2 where the fifth set is the decider.
		return set*set*set +
			3*set*set*set*lose +
			6*set*set*lose*lose*decider
	}
	// 2-0, and 2-1 where the third set is the decider.
	return set*set + 2*set*lose*decider
}

// The bounds Invert searches between. A serve probability at either end is not
// a tennis match, and the bisection needs somewhere to stop.
const (
	minServe = 0.01
	maxServe = 0.99
	// invertSteps is enough bisection to put the answer inside 1e-15, which is
	// past the precision anything downstream reports.
	invertSteps = 60
)

// Invert finds the two serve probabilities whose match probability is target,
// with their average pinned to anchor.
//
// Two unknowns and one equation, so the anchor supplies the second constraint:
// it is the tour-and-surface average that says what a typical service point is
// worth in this context, and it is what stops the solver answering with a pair
// that reproduces the right match probability out of two implausible halves.
//
// Returns the achieved match probability alongside. A target close enough to 0
// or 1 cannot be reached by any pair inside the bounds, and clamping quietly
// would leave the caller reporting a number the chain does not actually
// produce.
func Invert(target, anchor float64, f Format) (pA, pB, achieved float64) {
	anchor = clamp(anchor, minServe, maxServe)
	// The widest split the anchor allows before either side leaves the bounds.
	spread := math.Min(anchor-minServe, maxServe-anchor)

	lo, hi := -spread, spread
	for i := 0; i < invertSteps; i++ {
		mid := (lo + hi) / 2
		if Solve(anchor+mid, anchor-mid, f).Match[0] < target {
			lo = mid
		} else {
			hi = mid
		}
	}

	d := (lo + hi) / 2
	pA, pB = anchor+d, anchor-d
	return pA, pB, Solve(pA, pB, f).Match[0]
}

func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}
