package simulate

// Baseline is one cell of the serve baseline: what a service point was worth in
// one tour, tier, surface and decade.
//
// Totals rather than a rate, because widening to a coarser population means
// re-pooling the counts and a rate cannot be re-pooled.
type Baseline struct {
	Tier        string
	Surface     string
	Decade      int
	ServePoints int64
	ServeWon    int64
}

// Scope is how far a lookup had to widen to find enough evidence.
//
// It travels with the anchor because ADR-0007 requires a simulation to name its
// inputs: an anchor drawn from the exact tier, surface and decade and one
// pooled across a whole tour are different claims, and only one of them is
// about the match being simulated.
type Scope string

const (
	// ScopeExact is the tier, surface and decade asked for.
	ScopeExact Scope = "tier_surface_decade"
	// ScopeSurface pools every decade of that tier and surface.
	ScopeSurface Scope = "tier_surface"
	// ScopeTier pools every surface and decade of that tier.
	ScopeTier Scope = "tier"
	// ScopeTour pools everything the tour has.
	ScopeTour Scope = "tour"
	// ScopeNone means there was nothing to pool. The caller gets no anchor
	// rather than a made-up one.
	ScopeNone Scope = "none"
)

// MinAnchorPoints is how many service points a cell needs before it is
// preferred to a wider one.
//
// A thousand points is roughly ten matches. Below that the cell is noise around
// the wider average rather than a refinement of it, and using it would dress up
// a small sample as local knowledge.
const MinAnchorPoints = 1000

// Anchor is the serve-point-win rate to pin the inversion on, and how far the
// lookup had to widen to find it.
//
// It widens rather than failing, because a thin cell is the normal case at the
// edges of this database -- carpet in the 2010s, ITF in the 1990s -- and the
// tour-wide average is a far better answer there than nothing. What it does not
// do is invent: a tour with no recorded serve statistics at all returns
// ScopeNone, and the caller says so instead of anchoring on a half.
func Anchor(cells []Baseline, tier, surface string, decade int) (rate float64, scope Scope, points int64) {
	type candidate struct {
		scope Scope
		match func(Baseline) bool
	}

	for _, c := range []candidate{
		{ScopeExact, func(b Baseline) bool {
			return b.Tier == tier && b.Surface == surface && b.Decade == decade
		}},
		{ScopeSurface, func(b Baseline) bool {
			return b.Tier == tier && b.Surface == surface
		}},
		{ScopeTier, func(b Baseline) bool { return b.Tier == tier }},
		{ScopeTour, func(Baseline) bool { return true }},
	} {
		var won, total int64
		for _, b := range cells {
			if c.match(b) {
				won += b.ServeWon
				total += b.ServePoints
			}
		}
		// The widest scope takes whatever it can get: if the tour has any
		// recorded points at all, that is the best answer available.
		if total >= MinAnchorPoints || (c.scope == ScopeTour && total > 0) {
			return float64(won) / float64(total), c.scope, total
		}
	}
	return 0, ScopeNone, 0
}
