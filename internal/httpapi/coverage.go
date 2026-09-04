package httpapi

import (
	"net/http"
	"time"
)

// CoverageResponse describes what is actually in the database.
//
// It exists because the project has a real gap — full-schema data runs out in
// January 2026 for the ATP and December 2024 for the WTA — and ADR-0006 chose to
// accept it and say so. Saying so from a hardcoded date would drift the first
// time an ingest moved it, so every number here is queried.
type CoverageResponse struct {
	// CurrentThrough is the most recent match per tour, which is the honest
	// answer to "how up to date is this site".
	CurrentThrough map[string]string `json:"current_through"`
	Tiers          []CoverageEntry   `json:"tiers"`
}

// CoverageEntry is one tour and tier.
type CoverageEntry struct {
	Tour             string  `json:"tour"`
	Tier             string  `json:"tier"`
	Matches          int64   `json:"matches"`
	FirstMatch       string  `json:"first_match"`
	LastMatch        string  `json:"last_match"`
	MatchesWithStats int64   `json:"matches_with_stats"`
	StatsPercentage  float64 `json:"stats_percentage"`
}

func (a *API) handleCoverage(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Queries.GetCoverage(r.Context())
	if err != nil {
		Internal(w, r, err)
		return
	}

	out := CoverageResponse{
		CurrentThrough: map[string]string{},
		Tiers:          make([]CoverageEntry, 0, len(rows)),
	}
	for _, row := range rows {
		tour := string(row.Tour)
		last := row.LastMatch.Format(time.DateOnly)
		if last > out.CurrentThrough[tour] {
			out.CurrentThrough[tour] = last
		}

		entry := CoverageEntry{
			Tour:             tour,
			Tier:             string(row.Tier),
			Matches:          row.Matches,
			FirstMatch:       row.FirstMatch.Format(time.DateOnly),
			LastMatch:        last,
			MatchesWithStats: row.MatchesWithStats,
		}
		if row.Matches > 0 {
			entry.StatsPercentage = round1(100 * float64(row.MatchesWithStats) / float64(row.Matches))
		}
		out.Tiers = append(out.Tiers, entry)
	}

	writeJSON(w, r, http.StatusOK, out)
}
