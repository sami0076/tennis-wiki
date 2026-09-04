// Command api serves the tennis-wiki HTTP API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/sami0076/tennis-wiki/internal/db"
	"github.com/sami0076/tennis-wiki/internal/httpapi"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(log)

	// Batch and server processes alike must shut down cleanly: Kubernetes
	// sends SIGTERM and expects the process to drain rather than be killed.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, log); err != nil {
		if errors.Is(err, context.Canceled) {
			log.Info("api: cancelled")
			return
		}
		log.Error("api: failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger) error {
	cfg, err := httpapi.LoadConfig()
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}

	pool, err := db.Open(ctx, db.DefaultConfig(cfg.DatabaseURL))
	if err != nil {
		return err
	}
	defer pool.Close()

	api := httpapi.New(db.New(pool), log, cfg)
	return httpapi.Serve(ctx, cfg, api.Router(), log)
}
