package identity

import (
	"context"
	"log/slog"
)

// Stats summarises a reconciliation pass.
type Stats struct {
	Players  int
	Proposed int
	Merged   int
	Queued   int
	// Rejected are pairs the scorer proposed and a human has already ruled out.
	Rejected int
}

// Runner reconciles one tour at a time.
type Runner struct {
	Store     *Store
	Decisions *Decisions
	Log       *slog.Logger
	// DryRun reports what would happen without touching anything, which is how
	// a change to the scoring should be reviewed before it runs.
	DryRun bool
}

func (r *Runner) log() *slog.Logger {
	if r.Log != nil {
		return r.Log
	}
	return slog.Default()
}

// Run reconciles the given tours.
func (r *Runner) Run(ctx context.Context, tours []string) (Stats, error) {
	var stats Stats
	for _, tour := range tours {
		players, err := r.Store.LoadPlayers(ctx, tour)
		if err != nil {
			return stats, err
		}
		stats.Players += len(players)

		matches := Reconcile(players)
		before := len(matches)
		if r.Decisions != nil {
			matches = r.Decisions.Apply(tour, players, matches)
			stats.Rejected += before - len(matches)
		}
		stats.Proposed += len(matches)

		for _, m := range matches {
			if !m.Auto() {
				stats.Queued++
				r.log().InfoContext(ctx, "identity needs review", "tour", tour, "pair", m.Describe())
				if r.DryRun {
					continue
				}
				if err := r.Store.Queue(ctx, tour, m); err != nil {
					return stats, err
				}
				continue
			}

			stats.Merged++
			r.log().InfoContext(ctx, "merging identity", "tour", tour, "pair", m.Describe())
			if r.DryRun {
				continue
			}
			if err := r.Store.Merge(ctx, m); err != nil {
				return stats, err
			}
		}
	}

	r.log().InfoContext(ctx, "identity reconciliation finished",
		"players", stats.Players, "proposed", stats.Proposed,
		"merged", stats.Merged, "queued_for_review", stats.Queued,
		"ruled_out_by_hand", stats.Rejected, "dry_run", r.DryRun)
	return stats, nil
}
