package rating

import "fmt"

// Tier is the competitive standard of an event. The Postgres enum created in
// migrations/ is authoritative; this mirrors it.
type Tier string

const (
	TierTour       Tier = "tour"
	TierChallenger Tier = "challenger"
	TierFutures    Tier = "futures"
	TierITF        Tier = "itf"
)

// Level is the source CSV's tourney_level code.
type Level string

const (
	LevelGrandSlam  Level = "G"
	LevelMasters    Level = "M"
	LevelTourFinals Level = "F"
	LevelATP        Level = "A"
	LevelDavisCup   Level = "D"
	LevelChallenger Level = "C"
	LevelSatellite  Level = "S"
)

var tiers = map[Tier]struct{}{
	TierTour: {}, TierChallenger: {}, TierFutures: {}, TierITF: {},
}

// ParseTier validates a tier read from the database or from configuration.
func ParseTier(s string) (Tier, error) {
	t := Tier(s)
	if _, ok := tiers[t]; !ok {
		return "", fmt.Errorf("unknown tier %q", s)
	}
	return t, nil
}

func (t Tier) String() string { return string(t) }
