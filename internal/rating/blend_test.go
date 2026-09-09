package rating

import (
	"math"
	"testing"
)

func TestBlendWeight(t *testing.T) {
	t.Parallel()

	cases := []struct {
		matches int
		want    float64
	}{
		// No evidence about the surface, so none is used.
		{0, 0},
		{-1, 0},
		{5, 0.125},
		{20, 0.5},
		// The cap is reached before the matches run out: 0.75 of 40 is 30.
		{30, 0.75},
		{40, 0.75},
		{200, 0.75},
	}

	for _, c := range cases {
		if got := BlendWeight(c.matches); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("BlendWeight(%d) = %v, want %v", c.matches, got, c.want)
		}
	}
}

func TestBlend(t *testing.T) {
	t.Parallel()

	const overall, surface = 1800.0, 2000.0

	cases := []struct {
		name    string
		matches int
		want    float64
	}{
		// Five clay matches say very little, so the answer is nearly the
		// overall rating.
		{"a handful of matches barely moves it", 5, 1825},
		{"half weight is halfway", 20, 1900},
		// At and beyond the cap the surface takes three quarters and no more.
		{"the cap holds", 40, 1950},
		{"and keeps holding", 500, 1950},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, weight := Blend(surface, c.matches, overall)
			if math.Abs(got-c.want) > 1e-9 {
				t.Errorf("Blend(%v, %d, %v) = %v, want %v", surface, c.matches, overall, got, c.want)
			}
			if weight != BlendWeight(c.matches) {
				t.Errorf("reported weight %v, want %v", weight, BlendWeight(c.matches))
			}
		})
	}
}

// A surface nobody has played is not a rating of 1500 and not a copy of the
// overall rating either. It is the absence of evidence, and the blend of no
// evidence is the overall rating at weight zero.
func TestBlendOptionalAbsentSurface(t *testing.T) {
	t.Parallel()

	elo, weight := BlendOptional(nil, 1800)
	if elo != 1800 {
		t.Errorf("blend of an unplayed surface = %v, want the overall 1800", elo)
	}
	if weight != 0 {
		t.Errorf("weight = %v, want 0: nothing was known about the surface", weight)
	}

	// And Base is emphatically not the stand-in. A player rated 1800 overall
	// who has never played grass is not 1725 on grass.
	if elo, _ := BlendOptional(&SurfaceRating{Elo: Base, Matches: 20}, 1800); elo == 1800 {
		t.Error("a real surface rating of 1500 was ignored; only a nil one means absent")
	}
}

// The blend never leaves the interval between the two ratings, whichever way
// round they are. A weighted average that overshoots would be a bug that only
// shows up on the pairs furthest apart.
func TestBlendStaysBetween(t *testing.T) {
	t.Parallel()

	for _, surface := range []float64{1200, 1500, 2400} {
		for _, overall := range []float64{1200, 1500, 2400} {
			for _, matches := range []int{0, 1, 7, 39, 40, 41, 1000} {
				got, _ := Blend(surface, matches, overall)
				lo, hi := math.Min(surface, overall), math.Max(surface, overall)
				if got < lo-1e-9 || got > hi+1e-9 {
					t.Fatalf("Blend(%v, %d, %v) = %v, outside [%v, %v]",
						surface, matches, overall, got, lo, hi)
				}
			}
		}
	}
}
