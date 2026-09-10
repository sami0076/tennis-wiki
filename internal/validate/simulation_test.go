package validate

import (
	"math"
	"testing"
)

// The chance a match reaches its last set, which is the first thing the chain
// claims that the rating alone does not.
func TestDecidingSetProbability(t *testing.T) {
	t.Parallel()

	cases := []struct {
		set    float64
		bestOf int
		want   float64
	}{
		// Even players go the distance most often, and a best of three reaches
		// its third set half the time.
		{0.5, 3, 0.5},
		{0.5, 5, 0.375},
		// A one-sided match rarely does.
		{0.9, 3, 0.18},
		{0.9, 5, 0.0486},
		// Certainty never gets there at all.
		{1, 3, 0},
		{0, 5, 0},
	}

	for _, c := range cases {
		if got := decidingSetProbability(c.set, c.bestOf); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("decidingSetProbability(%v, %d) = %v, want %v",
				c.set, c.bestOf, got, c.want)
		}
	}
}

// It peaks at even and falls away either side, which is why the calibration
// buckets above 50% are empty rather than missing.
func TestDecidingSetProbabilityPeaksAtEven(t *testing.T) {
	t.Parallel()

	for _, bestOf := range []int{3, 5} {
		peak := decidingSetProbability(0.5, bestOf)
		for s := 0.05; s < 1; s += 0.05 {
			if got := decidingSetProbability(s, bestOf); got > peak+1e-9 {
				t.Errorf("best of %d: %v at s=%.2f exceeds the %v at even", bestOf, got, s, peak)
			}
		}
	}
}
