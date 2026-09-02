// Command rate recomputes Elo ratings from scratch over every match.
//
// Not yet implemented. See the Phase 1 issues for scope.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	// Batch and server processes alike must shut down cleanly: Kubernetes
	// sends SIGTERM and expects the process to drain rather than be killed.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			slog.Info("rate: cancelled")
			return
		}
		slog.Error("rate: failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	slog.InfoContext(ctx, "rate: not implemented yet")
	return nil
}
