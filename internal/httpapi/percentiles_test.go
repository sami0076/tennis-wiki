package httpapi

import "testing"

func f64(v float64) *float64 { return &v }

func TestPercentileRankCountsTiesHalf(t *testing.T) {
	pop := []float64{10, 20, 20, 30}
	for _, tc := range []struct {
		v    float64
		want int
	}{
		{5, 0},
		{10, 13},
		{20, 50},
		{30, 88},
		{40, 100},
	} {
		if got := percentileRank(tc.v, pop); got != tc.want {
			t.Errorf("percentileRank(%v) = %d, want %d", tc.v, got, tc.want)
		}
	}
	if got := percentileRank(7, nil); got != 50 {
		t.Errorf("empty population = %d, want 50", got)
	}
}

// Clutch averages ranks, not rates, and needs two of its four figures.
func TestClutchScoreAveragesComponentRanks(t *testing.T) {
	components := [4][]float64{{40, 60}, {30, 50}, {40, 60}, {40, 60}}

	best := windowFigures{bpSaved: f64(70), bpWon: f64(60), tiebreaks: f64(70), deciders: f64(70)}
	if got := clutchScore(best, components); got == nil || *got != 100 {
		t.Errorf("clutch of the best = %v, want 100", got)
	}

	split := windowFigures{bpSaved: f64(70), tiebreaks: f64(10)}
	if got := clutchScore(split, components); got == nil || *got != 50 {
		t.Errorf("clutch of 100 and 0 = %v, want 50", got)
	}

	if got := clutchScore(windowFigures{bpSaved: f64(70)}, components); got != nil {
		t.Errorf("clutch from one figure = %v, want absent", *got)
	}
}

func TestRankAxesLeavesUnmeasuredAxesEmpty(t *testing.T) {
	pop := []windowFigures{
		{serve: f64(60), hard: f64(1800)},
		{serve: f64(70), hard: f64(2000)},
	}
	axes := rankAxes(windowFigures{serve: f64(70), hard: f64(1900)}, pop)

	if len(axes) != len(percentileAxes) {
		t.Fatalf("axes = %d, want %d", len(axes), len(percentileAxes))
	}
	byKey := map[string]PercentileAxis{}
	for _, a := range axes {
		byKey[a.Key] = a
	}
	if p := byKey["serve"].Percentile; p == nil || *p != 75 {
		t.Errorf("serve percentile = %v, want 75", p)
	}
	if p := byKey["hard"].Percentile; p == nil || *p != 50 {
		t.Errorf("hard percentile = %v, want 50", p)
	}
	if a := byKey["grass"]; a.Percentile != nil || a.Value != nil {
		t.Errorf("grass = %+v, want no figure", a)
	}
}
