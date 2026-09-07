package httpapi

import (
	"testing"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// The per-row version of the profile's explanation. A page listing matches has
// to label each gap, not only the career summary.
func TestExplainMatchStats(t *testing.T) {
	for _, c := range []struct {
		name   string
		tier   db.Tier
		season int
		want   string
	}{
		{"futures never recorded, in any year", db.TierFutures, 2024, AvailabilityNeverForTier},
		{"nor did ITF", db.TierItf, 2024, AvailabilityNeverForTier},
		{"tour before 1991", db.TierTour, 1969, AvailabilityNeverInEra},
		{"challenger before it followed", db.TierChallenger, 2005, AvailabilityNeverInEra},
		{"challenger after it did", db.TierChallenger, 2019, AvailabilityNotRecorded},
		{"tour in a season that recorded them", db.TierTour, 2019, AvailabilityNotRecorded},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := explainMatchStats(c.tier, c.season); got != c.want {
				t.Errorf("explainMatchStats(%s, %d) = %q, want %q", c.tier, c.season, got, c.want)
			}
		})
	}
}

// The tier decides before the era does: a 2024 Futures match has no statistics
// because of its level, and saying "not in this era" would be false.
func TestTierBeatsEraWhenBothApply(t *testing.T) {
	if got := explainMatchStats(db.TierFutures, 1969); got != AvailabilityNeverForTier {
		t.Errorf("got %q, want the tier explanation", got)
	}
}
