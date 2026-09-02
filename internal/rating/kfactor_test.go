package rating

import (
	"math"
	"testing"
)

func TestK(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want float64
	}{
		{"debutant", 0, 131.33},
		{"20 matches", 20, 68.99},
		{"100 matches", 100, 38.86},
		{"311 matches", 311, 25.01},
		{"veteran", 500, 20.73},
		// Spec section 7.2 says K is driven by matches played "before" this
		// one, so a negative count is a caller bug, not a valid input.
		{"negative clamps to debutant", -1, 131.33},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := K(tc.n)
			if math.Abs(got-tc.want) > 0.01 {
				t.Errorf("K(%d) = %.4f, want %.2f", tc.n, got, tc.want)
			}
		})
	}
}

func TestKDecreasesMonotonically(t *testing.T) {
	prev := math.Inf(1)
	for n := 0; n <= 1000; n++ {
		got := K(n)
		if got >= prev {
			t.Fatalf("K(%d) = %f did not decrease from %f", n, got, prev)
		}
		if got <= 0 {
			t.Fatalf("K(%d) = %f is not positive", n, got)
		}
		prev = got
	}
}

func TestWeightsFor(t *testing.T) {
	w := DefaultWeights()

	tests := []struct {
		name  string
		match Match
		want  float64
	}{
		{"grand slam final", Match{Tier: TierTour, Level: LevelGrandSlam, IsFinal: true}, 1.20},
		{"grand slam round", Match{Tier: TierTour, Level: LevelGrandSlam}, 1.10},
		{"tour finals", Match{Tier: TierTour, Level: LevelTourFinals}, 1.10},
		{"masters", Match{Tier: TierTour, Level: LevelMasters}, 1.05},
		{"ordinary tour", Match{Tier: TierTour, Level: LevelATP}, 1.00},
		{"davis cup", Match{Tier: TierTour, Level: LevelDavisCup}, 0.80},
		{"challenger", Match{Tier: TierChallenger, Level: LevelChallenger}, 0.80},
		{"futures", Match{Tier: TierFutures, Level: LevelSatellite}, 0.60},
		{"itf", Match{Tier: TierITF, Level: LevelSatellite}, 0.60},
		{"tour qualifying", Match{Tier: TierTour, Level: LevelATP, IsQualifying: true}, 0.90},
		{"slam qualifying", Match{Tier: TierTour, Level: LevelGrandSlam, IsQualifying: true}, 0.99},
		{"challenger qualifying", Match{Tier: TierChallenger, IsQualifying: true}, 0.72},
		// A Challenger final is still a Challenger match: tier wins over round.
		{"challenger final is not a slam final", Match{Tier: TierChallenger, Level: LevelGrandSlam, IsFinal: true}, 0.80},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := w.For(tc.match)
			if math.Abs(got-tc.want) > 1e-9 {
				t.Errorf("For(%+v) = %.4f, want %.2f", tc.match, got, tc.want)
			}
		})
	}
}

func TestWeightsOrdering(t *testing.T) {
	w := DefaultWeights()
	slam := w.For(Match{Tier: TierTour, Level: LevelGrandSlam, IsFinal: true})
	tour := w.For(Match{Tier: TierTour, Level: LevelATP})
	chall := w.For(Match{Tier: TierChallenger})
	fut := w.For(Match{Tier: TierFutures})

	if !(slam > tour && tour > chall && chall > fut) {
		t.Errorf("weights not ordered by standard: slam=%v tour=%v chall=%v futures=%v",
			slam, tour, chall, fut)
	}
}

func TestParseTier(t *testing.T) {
	for _, s := range []string{"tour", "challenger", "futures", "itf"} {
		if _, err := ParseTier(s); err != nil {
			t.Errorf("ParseTier(%q) returned error: %v", s, err)
		}
	}
	for _, s := range []string{"", "TOUR", "qualifying", "grand_slam"} {
		if _, err := ParseTier(s); err == nil {
			t.Errorf("ParseTier(%q) should have failed", s)
		}
	}
}
