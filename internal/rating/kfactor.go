package rating

import "math"

// K-factor decay constants from build spec section 7.2.
const (
	kNumerator = 250.0
	kOffset    = 5.0
	kExponent  = 0.4
)

// K returns the K-factor for a player who has completed n matches before this
// one, so early results move a rating far more than late ones.
//
// K(0) = 131.33, K(500) = 20.73.
func K(n int) float64 {
	if n < 0 {
		n = 0
	}
	return kNumerator / math.Pow(float64(n)+kOffset, kExponent)
}

// Match describes the context needed to weight a single result.
type Match struct {
	Tier         Tier
	Level        Level
	IsFinal      bool
	IsQualifying bool
}

// Weights scale K by how much a result should count. Values are provisional
// below tour level until the promotion-continuity check runs; see ADR-0004.
type Weights struct {
	GrandSlamFinal float64
	GrandSlam      float64
	TourFinals     float64
	Masters        float64
	Tour           float64
	TeamEvent      float64
	Challenger     float64
	Futures        float64
	Qualifying     float64
}

// DefaultWeights returns the values agreed in ADR-0004.
func DefaultWeights() Weights {
	return Weights{
		GrandSlamFinal: 1.20,
		GrandSlam:      1.10,
		TourFinals:     1.10,
		Masters:        1.05,
		Tour:           1.00,
		TeamEvent:      0.80,
		Challenger:     0.80,
		Futures:        0.60,
		Qualifying:     0.90,
	}
}

// For returns the importance weight for a match.
func (w Weights) For(m Match) float64 {
	weight := w.base(m)
	if m.IsQualifying {
		weight *= w.Qualifying
	}
	return weight
}

func (w Weights) base(m Match) float64 {
	// Tier dominates: a Challenger final is still a Challenger match.
	switch m.Tier {
	case TierChallenger:
		return w.Challenger
	case TierFutures, TierITF:
		return w.Futures
	}

	switch m.Level {
	case LevelGrandSlam:
		if m.IsFinal {
			return w.GrandSlamFinal
		}
		return w.GrandSlam
	case LevelTourFinals:
		return w.TourFinals
	case LevelMasters:
		return w.Masters
	case LevelDavisCup:
		return w.TeamEvent
	default:
		return w.Tour
	}
}
