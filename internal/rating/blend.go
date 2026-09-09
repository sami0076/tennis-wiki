package rating

// The blend between a surface rating and the overall one, as the README states
// it:
//
//	blended = w·surface + (1-w)·overall,  w = min(0.75, surface matches / 40)
//
// A clay specialist with a hundred clay matches is described by their clay
// rating; someone with five is not, and leaning on it would be reading a number
// off a sample that cannot carry it. The weight is what moves between those two
// readings as evidence accumulates.
const (
	// BlendCap is the most weight a surface rating is ever given. It stops
	// short of 1 on purpose: a player's overall record is evidence about them
	// on any surface, and no amount of clay makes it irrelevant.
	BlendCap = 0.75
	// BlendMatches is how many matches on a surface reach the cap.
	BlendMatches = 40
)

// Blend combines a surface rating with the overall one and reports the weight
// it gave the surface, because the weight is part of the answer: 1900 from a
// hundred clay matches and 1900 from three are the same number and different
// claims.
func Blend(surfaceElo float64, surfaceMatches int, overallElo float64) (elo, weight float64) {
	weight = BlendWeight(surfaceMatches)
	return weight*surfaceElo + (1-weight)*overallElo, weight
}

// BlendWeight is how much a surface rating is worth after that many matches on
// it.
func BlendWeight(surfaceMatches int) float64 {
	if surfaceMatches <= 0 {
		return 0
	}
	w := float64(surfaceMatches) / BlendMatches
	if w > BlendCap {
		return BlendCap
	}
	return w
}

// BlendOptional is Blend for a surface the player may never have played.
//
// An absent surface rating is not 1500 and not the overall rating either: it is
// the absence of evidence about that surface, and the honest blend of no
// evidence is the overall rating at weight zero. Passing Base here instead
// would invent a rating and then average it in.
func BlendOptional(surface *SurfaceRating, overallElo float64) (elo, weight float64) {
	if surface == nil {
		return overallElo, 0
	}
	return Blend(surface.Elo, surface.Matches, overallElo)
}

// SurfaceRating is one surface series: what the player is worth on it and how
// much of it they have played.
type SurfaceRating struct {
	Elo     float64
	Matches int
}
