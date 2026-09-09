package simulate

import (
	"math"
	"testing"
)

// A realistic slice: ATP tour hard courts are well covered, carpet in the 2010s
// is nearly gone, and ITF has almost nothing anywhere.
func cells() []Baseline {
	return []Baseline{
		{Tier: "tour", Surface: "hard", Decade: 2010, ServePoints: 3442419, ServeWon: 2179051},
		{Tier: "tour", Surface: "hard", Decade: 2020, ServePoints: 1692363, ServeWon: 1079752},
		{Tier: "tour", Surface: "clay", Decade: 2010, ServePoints: 1924682, ServeWon: 1181754},
		{Tier: "tour", Surface: "carpet", Decade: 2010, ServePoints: 400, ServeWon: 260},
		{Tier: "challenger", Surface: "hard", Decade: 2010, ServePoints: 2000000, ServeWon: 1224000},
	}
}

func TestAnchorPrefersTheExactCell(t *testing.T) {
	t.Parallel()

	rate, scope, points := Anchor(cells(), "tour", "hard", 2020)
	if scope != ScopeExact {
		t.Errorf("scope = %q, want the exact cell", scope)
	}
	if points != 1692363 {
		t.Errorf("points = %d, want the cell's own", points)
	}
	if math.Abs(rate-0.638) > 0.001 {
		t.Errorf("rate = %.4f, want the measured 63.8%%", rate)
	}
}

// A cell too thin to mean anything widens to the next population up, and says
// that it did. Carpet in the 2010s has 400 points, which is about four matches.
func TestAnchorWidensOnAThinCell(t *testing.T) {
	t.Parallel()

	rate, scope, points := Anchor(cells(), "tour", "carpet", 2010)
	if scope != ScopeTier {
		t.Errorf("scope = %q, want it widened past the surface to the tier", scope)
	}
	if points <= 400 {
		t.Errorf("points = %d, want a wider population than the 400-point cell", points)
	}
	// Pooled across every tour-level cell, not the four carpet matches.
	if rate < 0.6 || rate > 0.65 {
		t.Errorf("rate = %.4f, outside anything the tour-level pool could give", rate)
	}
}

// Widening re-pools the counts rather than averaging the rates, which is why
// the table stores totals. A tier whose decades are lopsided would come out
// wrong under a mean of rates.
func TestAnchorRepoolsRatherThanAveraging(t *testing.T) {
	t.Parallel()

	lopsided := []Baseline{
		{Tier: "tour", Surface: "hard", Decade: 1990, ServePoints: 1000, ServeWon: 900},
		{Tier: "tour", Surface: "hard", Decade: 2020, ServePoints: 999000, ServeWon: 599400},
	}
	rate, scope, _ := Anchor(lopsided, "tour", "hard", 2000)
	if scope != ScopeSurface {
		t.Fatalf("scope = %q, want the surface pool", scope)
	}
	// Pooled: 600300 / 1000000. A mean of the two rates would give 0.75.
	if math.Abs(rate-0.6003) > 1e-6 {
		t.Errorf("rate = %.6f, want the pooled 0.6003 rather than a mean of rates", rate)
	}
}

// A tour with a little data anywhere still answers, because the tour-wide
// average beats no anchor at all.
func TestAnchorFallsBackToTheTour(t *testing.T) {
	t.Parallel()

	sparse := []Baseline{
		{Tier: "itf", Surface: "clay", Decade: 1990, ServePoints: 500, ServeWon: 300},
	}
	rate, scope, points := Anchor(sparse, "tour", "grass", 2020)
	if scope != ScopeTour {
		t.Errorf("scope = %q, want the tour-wide fallback", scope)
	}
	if points != 500 || math.Abs(rate-0.6) > 1e-9 {
		t.Errorf("rate %.4f over %d points, want 0.6 over 500", rate, points)
	}
}

// Nothing at all is absent, not a half. A coin flip presented as a measured
// anchor is exactly the failure ADR-0007 forbids.
func TestAnchorWithNoDataIsAbsent(t *testing.T) {
	t.Parallel()

	rate, scope, points := Anchor(nil, "tour", "hard", 2020)
	if scope != ScopeNone {
		t.Errorf("scope = %q, want none", scope)
	}
	if rate != 0 || points != 0 {
		t.Errorf("rate %v over %d points, want nothing at all", rate, points)
	}
}
