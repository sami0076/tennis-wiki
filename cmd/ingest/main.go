// Command ingest loads match, player, and ranking data from the configured sources.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sami0076/tennis-wiki/internal/db"
	"github.com/sami0076/tennis-wiki/internal/identity"
	"github.com/sami0076/tennis-wiki/internal/ingest"
)

type config struct {
	sources   string
	localPath string
	dsn       string
	seasons   string
	tours     string
	workers   int
	batchSize int
	stage     string
	overrides string
	dryRun    bool
	verbose   bool
}

// Stages. Reference data runs after matches so the player tables, which carry
// the canonical name, get the last word on it -- and so rankings resolve
// against every player, however they were created.
const (
	stageAll       = "all"
	stageMatches   = "matches"
	stageReference = "reference"
	stageReconcile = "reconcile"
)

func main() {
	var cfg config
	flag.StringVar(&cfg.sources, "sources", "configs/sources.json", "path to the source registry")
	flag.StringVar(&cfg.localPath, "local-path", "", "read from a local clone instead of the network")
	flag.StringVar(&cfg.dsn, "database-url", os.Getenv("DATABASE_URL"), "PostgreSQL connection string")
	flag.StringVar(&cfg.seasons, "seasons", "", "seasons to ingest, e.g. 2020-2024 or 1999 (default: all)")
	flag.StringVar(&cfg.tours, "tours", "", "tours to ingest: atp, wta (default: both)")
	flag.IntVar(&cfg.workers, "workers", 0, "concurrent file readers (default: GOMAXPROCS)")
	flag.IntVar(&cfg.batchSize, "batch", ingest.DefaultBatchSize, "rows per transaction")
	flag.StringVar(&cfg.stage, "stage", stageAll,
		"what to run: all, matches, reference (player tables and rankings), or reconcile")
	flag.StringVar(&cfg.overrides, "overrides", "configs/player_overrides.json",
		"identity decisions made by hand")
	flag.BoolVar(&cfg.dryRun, "dry-run", false,
		"for the reconcile stage: report what would be merged without doing it")
	flag.BoolVar(&cfg.verbose, "v", false, "verbose logging")
	flag.Parse()

	level := slog.LevelInfo
	if cfg.verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg); err != nil {
		if errors.Is(err, context.Canceled) {
			slog.Info("ingest: cancelled")
			return
		}
		slog.Error("ingest: failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config) error {
	registry, err := ingest.LoadRegistry(cfg.sources)
	if err != nil {
		return err
	}

	seasons, err := parseSeasons(cfg.seasons)
	if err != nil {
		return err
	}
	tours, err := parseTours(cfg.tours)
	if err != nil {
		return err
	}

	if cfg.dsn == "" {
		return errors.New("no database URL: set DATABASE_URL or pass -database-url")
	}
	pool, err := db.Open(ctx, db.Config{
		DSN: cfg.dsn,
		// Ingest is one writer, so a large pool would sit idle.
		MaxConns: 4, MinConns: 1,
	})
	if err != nil {
		return err
	}
	defer pool.Close()

	switch cfg.stage {
	case stageAll, stageMatches, stageReference, stageReconcile:
	default:
		return fmt.Errorf("unknown stage %q: want all, matches, reference or reconcile", cfg.stage)
	}

	store := ingest.NewStore(pool)
	fetcher := chooseFetcher(cfg.localPath)

	if cfg.stage == stageAll || cfg.stage == stageMatches {
		if err := runMatches(ctx, cfg, registry, fetcher, store, seasons, tours); err != nil {
			return err
		}
	}
	if cfg.stage == stageAll || cfg.stage == stageReference {
		if err := runReference(ctx, cfg, registry, fetcher, store); err != nil {
			return err
		}
	}
	// Last: it needs every player that any source is going to create.
	if cfg.stage == stageAll || cfg.stage == stageReconcile {
		if err := runReconcile(ctx, cfg, pool, tours); err != nil {
			return err
		}
	}

	// Search ranks off this view, so it is stale the moment matches change.
	if cfg.stage != stageReconcile {
		started := time.Now()
		if err := store.RefreshProminence(ctx); err != nil {
			return err
		}
		slog.Info("refreshed search ranking", "took", time.Since(started).Round(time.Millisecond))
	}

	counts, err := store.Counts(ctx)
	if err != nil {
		return err
	}
	slog.Info("database totals",
		"players", counts["players"], "tournaments", counts["tournaments"],
		"matches", counts["matches"], "match_players", counts["match_players"],
		"rankings", counts["rankings"])
	return nil
}

func runReconcile(ctx context.Context, cfg config, pool *pgxpool.Pool, tours []ingest.Tour) error {
	overrides, err := identity.LoadOverrides(cfg.overrides)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(tours))
	for _, t := range tours {
		names = append(names, string(t))
	}
	if len(names) == 0 {
		names = []string{string(ingest.TourATP), string(ingest.TourWTA)}
	}

	runner := &identity.Runner{
		Store:     identity.NewStore(pool),
		Decisions: overrides.Index(),
		Log:       slog.Default(),
		DryRun:    cfg.dryRun,
	}
	_, err = runner.Run(ctx, names)
	return err
}

func runMatches(ctx context.Context, cfg config, registry *ingest.Registry,
	fetcher ingest.Fetcher, store *ingest.Store, seasons []int, tours []ingest.Tour) error {
	runID, err := store.StartRun(ctx, sourceLabel(cfg))
	if err != nil {
		return err
	}

	pipeline := &ingest.Pipeline{
		Registry: registry,
		Fetcher:  fetcher,
		Store:    store,
		Log:      slog.Default(),
		Options: ingest.Options{
			Workers:   cfg.workers,
			BatchSize: cfg.batchSize,
			Seasons:   seasons,
			Tours:     tours,
		},
	}

	stats, runErr := pipeline.Run(ctx)
	// The run record is written whether or not the run succeeded, so a failure
	// leaves its error text behind rather than vanishing.
	if err := store.FinishRun(ctx, runID, stats.RowsSeen, stats.RowsWritten, runErr); err != nil {
		slog.Error("could not record ingest run", "error", err)
	}
	return runErr
}

func runReference(ctx context.Context, cfg config, registry *ingest.Registry,
	fetcher ingest.Fetcher, store *ingest.Store) error {
	if len(registry.Reference) == 0 {
		slog.Warn("no reference sources configured; skipping player tables and rankings")
		return nil
	}
	paths, ok := fetcher.(ingest.PathFetcher)
	if !ok {
		return errors.New("fetcher cannot open reference files")
	}

	runID, err := store.StartRun(ctx, sourceLabel(cfg)+" (reference)")
	if err != nil {
		return err
	}

	loader := &ingest.ReferenceLoader{
		Sources:   registry.Reference,
		Fetcher:   paths,
		Store:     store,
		Log:       slog.Default(),
		BatchSize: cfg.batchSize,
	}
	stats, runErr := loader.Run(ctx)
	written := int(stats.RankingsWritten) + stats.Players
	if err := store.FinishRun(ctx, runID, stats.RankingRowsSeen+stats.Players, written, runErr); err != nil {
		slog.Error("could not record reference run", "error", err)
	}
	return runErr
}

func chooseFetcher(localPath string) ingest.Fetcher {
	if localPath != "" {
		return ingest.LocalFetcher{Root: localPath}
	}
	return ingest.HTTPFetcher{}
}

func sourceLabel(cfg config) string {
	if cfg.localPath != "" {
		return "local:" + cfg.localPath
	}
	return "registry:" + cfg.sources
}

// parseSeasons accepts "2024", "2020-2024" or a comma-separated mix.
func parseSeasons(s string) ([]int, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	var out []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if lo, hi, ok := strings.Cut(part, "-"); ok {
			start, err := strconv.Atoi(strings.TrimSpace(lo))
			if err != nil {
				return nil, fmt.Errorf("bad season range %q: %w", part, err)
			}
			end, err := strconv.Atoi(strings.TrimSpace(hi))
			if err != nil {
				return nil, fmt.Errorf("bad season range %q: %w", part, err)
			}
			if start > end {
				return nil, fmt.Errorf("season range %q runs backwards", part)
			}
			for y := start; y <= end; y++ {
				out = append(out, y)
			}
			continue
		}
		y, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("bad season %q: %w", part, err)
		}
		out = append(out, y)
	}
	return out, nil
}

func parseTours(s string) ([]ingest.Tour, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	var out []ingest.Tour
	for _, part := range strings.Split(s, ",") {
		switch t := ingest.Tour(strings.ToLower(strings.TrimSpace(part))); t {
		case ingest.TourATP, ingest.TourWTA:
			out = append(out, t)
		default:
			return nil, fmt.Errorf("unknown tour %q", part)
		}
	}
	return out, nil
}
