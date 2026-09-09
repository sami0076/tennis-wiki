package simulate

import (
	"math"
	"testing"
)

const tolerance = 1e-9

func near(t *testing.T, got, want float64, what string) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Errorf("%s = %.12f, want %.12f", what, got, want)
	}
}

func TestGame(t *testing.T) {
	t.Parallel()

	// The standard check on this formula. Written as the exact fraction the
	// algebra gives for p = 3/5, so the test pins the closed form rather than a
	// rounding of it: 0.7357292307...
	near(t, Game(0.6), 149445.0/203125.0, "Game(0.6)")
	near(t, Game(0.5), 0.5, "Game(0.5)")
	near(t, Game(0), 0, "Game(0)")
	near(t, Game(1), 1, "Game(1)")

	// Monotone: winning more points never wins fewer games.
	previous := 0.0
	for p := 0.0; p <= 1.0; p += 0.01 {
		got := Game(p)
		if got < previous-tolerance {
			t.Fatalf("Game(%.2f) = %v, below Game of a smaller p (%v)", p, got, previous)
		}
		previous = got
	}
}

// Two identical players are even at every rung, whichever format they play and
// whoever serves first.
func TestEvenPlayersAreEven(t *testing.T) {
	t.Parallel()

	for _, f := range []Format{{BestOf: 3}, {BestOf: 5}, {BestOf: 5, FinalSetAdvantage: true}} {
		c := Solve(0.5, 0.5, f)
		near(t, c.Hold[0], 0.5, "hold")
		near(t, c.Set[0], 0.5, "set")
		near(t, c.Match[0], 0.5, "match")
	}

	// And at a realistic serve rate, where holds are common and the set is
	// decided by the rare break.
	near(t, Set(0.65, 0.65, false), 0.5, "even set at 65%")
	near(t, Tiebreak(0.65, 0.65), 0.5, "even tiebreak at 65%")
}

// Swapping the two players mirrors every rung exactly.
//
// Not obvious, because A serves first throughout: the closed form says that
// serving first is worth nothing over a whole set, since the advantage of
// serving the odd games is exactly the disadvantage of receiving the even ones.
func TestSwappingMirrors(t *testing.T) {
	t.Parallel()

	for _, pair := range [][2]float64{{0.65, 0.63}, {0.7, 0.5}, {0.55, 0.9}, {0.61, 0.61}} {
		a, b := pair[0], pair[1]
		near(t, Tiebreak(a, b), 1-Tiebreak(b, a), "tiebreak")
		near(t, Set(a, b, false), 1-Set(b, a, false), "set")
		near(t, Set(a, b, true), 1-Set(b, a, true), "advantage set")

		for _, f := range []Format{{BestOf: 3}, {BestOf: 5}, {BestOf: 5, FinalSetAdvantage: true}} {
			near(t, Solve(a, b, f).Match[0], 1-Solve(b, a, f).Match[0], "match")
		}
	}
}

// The claim the page is built to make: the scoring system is an amplifier, and
// the edge grows at every rung.
func TestTheEdgeCompounds(t *testing.T) {
	t.Parallel()

	c := Solve(0.65, 0.63, Format{BestOf: 5})
	point := c.Point[0] - c.Point[1]
	hold := c.Hold[0] - c.Hold[1]
	set := c.Set[0] - c.Set[1]
	match := c.Match[0] - c.Match[1]

	if !(point < hold && hold < set && set < match) {
		t.Errorf("edges do not compound: point %.4f, hold %.4f, set %.4f, match %.4f",
			point, hold, set, match)
	}
	// "A 2-point edge on serve becomes a 12-point edge on the match."
	if ratio := match / point; ratio < 10 || ratio > 14 {
		t.Errorf("a %.0f-point edge became a %.0f-point edge (x%.1f); the design says about x12",
			point*100, match*100, ratio)
	}
}

// The longer the match, the less room for an upset.
func TestBestOfFiveAmplifiesMore(t *testing.T) {
	t.Parallel()

	three := Solve(0.65, 0.63, Format{BestOf: 3}).Match[0]
	five := Solve(0.65, 0.63, Format{BestOf: 5}).Match[0]
	if five <= three {
		t.Errorf("best of five = %.4f, not above best of three = %.4f", five, three)
	}

	// An unrecognised BestOf is read as three rather than producing something
	// that is neither.
	near(t, Solve(0.65, 0.63, Format{}).Match[0], three, "default format")
	near(t, Solve(0.65, 0.63, Format{BestOf: 4}).Match[0], three, "best of four")
}

// An advantage decider is more games, and more games favour the better player.
func TestAdvantageDeciderFavoursTheFavourite(t *testing.T) {
	t.Parallel()

	tiebreak := Solve(0.65, 0.63, Format{BestOf: 5}).Match[0]
	advantage := Solve(0.65, 0.63, Format{BestOf: 5, FinalSetAdvantage: true}).Match[0]
	if advantage <= tiebreak {
		t.Errorf("advantage decider = %.4f, not above the tiebreak decider = %.4f",
			advantage, tiebreak)
	}
	// It only reaches the deciding set, so the effect is small.
	if advantage-tiebreak > 0.02 {
		t.Errorf("advantage decider moved the match by %.4f, which is too much for one set",
			advantage-tiebreak)
	}
}

// A tiebreak is not a game with different numbers: the serve alternates, so
// both players' probabilities are in it.
func TestTiebreakUsesBothServers(t *testing.T) {
	t.Parallel()

	// Holding the first player fixed and improving the second must lower the
	// first player's chances.
	previous := 1.0
	for pB := 0.4; pB <= 0.9; pB += 0.05 {
		got := Tiebreak(0.65, pB)
		if got > previous+tolerance {
			t.Fatalf("Tiebreak(0.65, %.2f) = %v rose against a better opponent", pB, got)
		}
		previous = got
	}

	// Serving the first point is worth something in a tiebreak, unlike in a
	// set: the sequence is not long enough for it to wash out.
	if Tiebreak(0.65, 0.65) != 0.5 {
		t.Error("equal players are not even in a tiebreak")
	}
}

// The inversion is the whole Elo-derived path from ADR-0007: it has to land on
// the target, not near it.
func TestInvertRoundTrips(t *testing.T) {
	t.Parallel()

	for _, f := range []Format{{BestOf: 3}, {BestOf: 5}, {BestOf: 5, FinalSetAdvantage: true}} {
		for _, anchor := range []float64{0.559, 0.626, 0.654} {
			for _, target := range []float64{0.5, 0.55, 0.62, 0.75, 0.9, 0.25} {
				pA, pB, achieved := Invert(target, anchor, f)
				near(t, achieved, target, "achieved")
				near(t, (pA+pB)/2, anchor, "anchor")
				near(t, Solve(pA, pB, f).Match[0], target, "resolved chain")
			}
		}
	}
}

// A target no pair of plausible serve probabilities can reach is reported as
// what was actually achieved. Clamping quietly would leave the caller
// publishing a number the chain does not produce.
func TestInvertReportsWhatItCouldNotReach(t *testing.T) {
	t.Parallel()

	f := Format{BestOf: 5}
	_, _, achieved := Invert(0.999999, 0.62, f)
	if achieved >= 0.999999 {
		t.Errorf("achieved %.8f, expected the bounds to stop short of the target", achieved)
	}
	if achieved < 0.9 {
		t.Errorf("achieved %.4f, expected the bounds to still allow a lopsided match", achieved)
	}
}

// Nothing in the chain may produce a NaN, whatever it is handed. A NaN would
// travel all the way to the page as "null%".
func TestDegenerateInputsStayFinite(t *testing.T) {
	t.Parallel()

	for _, pair := range [][2]float64{{0, 0}, {1, 1}, {0, 1}, {1, 0}, {0.5, 0}, {1, 0.5}} {
		for _, f := range []Format{{BestOf: 3}, {BestOf: 5, FinalSetAdvantage: true}} {
			c := Solve(pair[0], pair[1], f)
			for _, rung := range [][2]float64{c.Hold, c.Set, c.Match} {
				for _, v := range rung {
					if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
						t.Fatalf("Solve(%v, %v, %+v) produced %v", pair[0], pair[1], f, v)
					}
				}
			}
		}
	}
}

// The figures in simulator.png, which the chain should reproduce rather than
// merely resemble.
func TestReproducesTheDesignNumbers(t *testing.T) {
	t.Parallel()

	c := Solve(0.65, 0.63, Format{BestOf: 5})
	for _, c := range []struct {
		got, want float64
		what      string
	}{
		{c.Hold[0], 0.830, "A's hold"},
		{c.Hold[1], 0.795, "B's hold"},
		{c.Set[0], 0.567, "A's set"},
		{c.Match[0], 0.625, "A's match"},
	} {
		if math.Abs(c.got-c.want) > 0.001 {
			t.Errorf("%s = %.4f, the design shows %.3f", c.what, c.got, c.want)
		}
	}
}

// The draw simulator plays 127 matches per run and wants ten thousand runs, so
// a solve has to cost microseconds rather than milliseconds. Benchmarked here
// rather than assumed, because the whole reason a match is solved exactly is
// that it is cheap enough to be.
func BenchmarkSolve(b *testing.B) {
	f := Format{BestOf: 5}
	for i := 0; i < b.N; i++ {
		_ = Solve(0.65, 0.63, f)
	}
}

func BenchmarkInvert(b *testing.B) {
	f := Format{BestOf: 5}
	for i := 0; i < b.N; i++ {
		Invert(0.62, 0.638, f)
	}
}
