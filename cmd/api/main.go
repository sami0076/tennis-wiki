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

	"github.com/sami0076/tennis-wiki/internal/cache"
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

	// A cache that cannot be built is a configuration error and worth failing
	// on; one that cannot be reached is not, and is handled at every call site.
	redis, err := cache.FromEnv(log)
	if err != nil {
		return fmt.Errorf("cache configuration: %w", err)
	}
	defer func() {
		if cerr := redis.Close(); cerr != nil {
			log.Warn("closing cache failed", "error", cerr)
		}
	}()
	log.Info("api: read cache", "enabled", redis.Enabled(), "ttl", redis.TTLFor())

	api := httpapi.New(db.New(pool), log, cfg)
	api.Cache = redis
	return httpapi.Serve(ctx, cfg, api.Router(), log)
}
