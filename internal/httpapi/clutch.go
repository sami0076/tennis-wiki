package httpapi

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// Clutch is the three under-pressure figures and what they are measured
// against.
//
// Each metric is null where the player had no opportunities of that kind, which
// is not the same as failing at them: a player who never faced a break point
// has not saved 0% of them, and a player whose matches recorded no serve line
// has no figure at all rather than a bad one.
type Clutch struct {
	Baseline         ClutchPopulation `json:"baseline"`
	BreakPointsSaved *ClutchMetric    `json:"break_points_saved"`
	TiebreaksWon     *ClutchMetric    `json:"tiebreaks_won"`
	DecidingSetsWon  *ClutchMetric    `json:"deciding_sets_won"`
	// Availability explains an absent break-points figure in the same
	// vocabulary the profile and the head-to-head use.
	Availability string `json:"availability"`
}

// ClutchPopulation is what "vs tour average" means here, in the response rather
// than in a page's caption, because a delta against an unnamed average is a
// number pretending to be a fact.
type ClutchPopulation struct {
	// Tiers and the decade range are this player's own, because the baseline is
	// weighted by where their opportunities fell. A Futures player is not
	// measured against a tour average.
	Tiers      []string `json:"tiers" tstype:"string[] | null"`
	FromDecade int16    `json:"from_decade"`
	ToDecade   int16    `json:"to_decade"`
	// Appearances is how many player-sides across the database the baseline is
	// drawn from -- two per match, the same unit the player's own figures use.
	Appearances int64 `json:"appearances"`
	// Matches and ScoredMatches are the player's own denominator. A score that
	// could not be read carries no tiebreak, and the difference is worth
	// stating rather than burying.
	Matches       int64 `json:"matches"`
	ScoredMatches int64 `json:"scored_matches"`
}

// ClutchMetric is one figure, its denominator, and the tour's figure over the
// same ground.
type ClutchMetric struct {
	Won        int64   `json:"won"`
	Played     int64   `json:"played"`
	Percentage float64 `json:"percentage"`
	// Baseline is null where no cell of the baseline had opportunities of this
	// kind -- a comparison against nothing, which is worse than no comparison.
	Baseline *float64 `json:"baseline"`
	// Delta is percentage points above the baseline, positive being better.
	Delta *float64 `json:"delta"`
}

func (a *API) handlePlayerClutch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	player, err := a.Queries.GetPlayerBySlug(ctx, chi.URLParam(r, "slug"))
	if errors.Is(err, pgx.ErrNoRows) {
		NotFound(w, r, "No player has that slug.")
		return
	}
	if err != nil {
		Internal(w, r, err)
		return
	}

	row, err := a.Queries.GetPlayerClutch(ctx, player.ID)
	if err != nil {
		Internal(w, r, err)
		return
	}

	// The same three-way explanation the profile gives, from the same two
	// queries, so a page never has to reconcile two accounts of one gap.
	summary, err := a.Queries.GetPlayerCareerSummary(ctx, player.ID)
	if err != nil {
		Internal(w, r, err)
		return
	}
	tiers, err := a.Queries.GetPlayerTierSplits(ctx, player.ID)
	if err != nil {
		Internal(w, r, err)
		return
	}

	writeJSON(w, r, http.StatusOK, buildClutch(row, buildServe(summary, tiers).Availability))
}

func buildClutch(row db.GetPlayerClutchRow, availability string) Clutch {
	return Clutch{
		Baseline: ClutchPopulation{
			Tiers:         row.Tiers,
			FromDecade:    row.FromDecade,
			ToDecade:      row.ToDecade,
			Appearances:   row.BaselineAppearances,
			Matches:       row.Appearances,
			ScoredMatches: row.Scored,
		},
		BreakPointsSaved: clutchMetric(
			row.BpSaved, row.BpFaced, row.BaselineBpWeighted, row.BaselineBpWeight),
		TiebreaksWon: clutchMetric(
			row.TiebreaksWon, row.TiebreaksPlayed,
			row.BaselineTiebreaksWeighted, row.BaselineTiebreaksWeight),
		DecidingSetsWon: clutchMetric(
			row.DecidingSetsWon, row.DecidingSetsPlayed,
			row.BaselineDecidingWeighted, row.BaselineDecidingWeight),
		Availability: availability,
	}
}

// clutchMetric assembles one figure, or nothing at all.
//
// weighted and weight come out of the query as a weighted sum and its total
// weight: the tour's rate in each cell this player played in, weighted by how
// many of their own opportunities fell there. A weight of zero means no cell
// had any, so there is a figure but nothing to compare it with.
func clutchMetric(won, played int64, weighted float64, weight int64) *ClutchMetric {
	if played == 0 {
		return nil
	}

	metric := ClutchMetric{
		Won:        won,
		Played:     played,
		Percentage: round1(float64(won) / float64(played) * 100),
	}
	if weight > 0 {
		baseline := round1(weighted / float64(weight) * 100)
		delta := round1(metric.Percentage - baseline)
		metric.Baseline, metric.Delta = &baseline, &delta
	}
	return &metric
}
