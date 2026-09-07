package rating

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config tunes a recompute.
type Config struct {
	Weights Weights
	// IncludeTeamEvents rates Davis Cup and the like, which are excluded by
	// default: a tie is played for a country, not a ranking, and the selection
	// is not the player's.
	IncludeTeamEvents bool
	// Batch is how many snapshots are copied at once.
	Batch int
}

// DefaultBatch is large enough that COPY dominates the round trip.
const DefaultBatch = 50_000

// Report is what one recompute did. Every figure is counted, not estimated.
type Report struct {
	Matches   int
	Excluded  Excluded
	Players   int
	Snapshots int64
	Elapsed   time.Duration
}

// Run recomputes every rating from scratch.
//
// From scratch is the whole design: ratings are never patched incrementally, so
// a fix to the weights or the K-factor is one rerun away from being reflected
// everywhere. Two runs over the same data write byte-identical rows, because
// the match order is total and each week's snapshots are emitted sorted.
//
// The pool needs at least two connections: the replay reads matches on one
// while the transaction holding the new ratings writes on the other.
func Run(ctx context.Context, pool *pgxpool.Pool, cfg Config) (Report, error) {
	if cfg.Batch <= 0 {
		cfg.Batch = DefaultBatch
	}
	if (cfg.Weights == Weights{}) {
		cfg.Weights = DefaultWeights()
	}

	started := time.Now()
	store := NewStore(pool)
	var rep Report

	rows, err := store.Replace(ctx, cfg.Batch, func(emit func(Snapshot) error) error {
		engine := NewEngine(cfg.Weights, emit)
		n, excluded, err := store.Matches(ctx, cfg.IncludeTeamEvents, engine.Add)
		if err != nil {
			return err
		}
		if err := engine.Close(); err != nil {
			return err
		}
		rep.Matches, rep.Excluded, rep.Players = n, excluded, engine.Players()
		return nil
	})
	if err != nil {
		return rep, err
	}

	rep.Snapshots = rows
	rep.Elapsed = time.Since(started)
	return rep, nil
}
