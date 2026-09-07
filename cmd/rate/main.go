// Command rate recomputes Elo ratings from scratch over every match.
//
// Never incremental: the whole history is replayed in chronological order, so a
// change to the weights or the K-factor is one rerun away from being reflected
// everywhere. The previous ratings stay in place until the new ones are ready.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sami0076/tennis-wiki/internal/db"
	"github.com/sami0076/tennis-wiki/internal/rating"
)

func main() {
	var (
		dsn        = flag.String("database-url", os.Getenv("DATABASE_URL"), "PostgreSQL connection string")
		teamEvents = flag.Bool("team-events", false, "rate Davis Cup and other team events too")
		batch      = flag.Int("batch", rating.DefaultBatch, "snapshots per COPY")
	)
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	// Batch and server processes alike must shut down cleanly: Kubernetes
	// sends SIGTERM and expects the process to drain rather than be killed.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, *dsn, rating.Config{IncludeTeamEvents: *teamEvents, Batch: *batch}); err != nil {
		if errors.Is(err, context.Canceled) {
			slog.Info("rate: cancelled")
			return
		}
		slog.Error("rate: failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, dsn string, cfg rating.Config) error {
	if dsn == "" {
		return errors.New("no database URL: set DATABASE_URL or pass -database-url")
	}

	// Two connections at least: the replay reads matches on one while the
	// transaction holding the new ratings writes on the other.
	pool, err := db.Open(ctx, db.Config{DSN: dsn, MaxConns: 4, MinConns: 2})
	if err != nil {
		return err
	}
	defer pool.Close()

	slog.InfoContext(ctx, "rate starting", "team_events", cfg.IncludeTeamEvents)

	rep, err := rating.Run(ctx, pool, cfg)
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "rate finished",
		"matches", rep.Matches,
		"players", rep.Players,
		"snapshots", rep.Snapshots,
		"excluded_team_events", rep.Excluded.TeamEvents,
		"excluded_walkovers", rep.Excluded.Walkovers,
		"took", rep.Elapsed.Round(time.Second))
	return nil
}
