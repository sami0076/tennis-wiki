// Command validate reports whether the rating engine's output stands up.
//
// Predictive accuracy and calibration per tier, mean reversion across the pool,
// and promotion continuity -- the check that says which way the tier weights
// have erred. See spec section 7.5 as restated by ADR-0004.
//
// Nothing here fails the run except being unable to measure at all. Every other
// finding is a judgement about weights that a person should make.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/sami0076/tennis-wiki/internal/db"
	"github.com/sami0076/tennis-wiki/internal/rating"
	"github.com/sami0076/tennis-wiki/internal/validate"
)

func main() {
	var (
		dsn        = flag.String("database-url", os.Getenv("DATABASE_URL"), "PostgreSQL connection string")
		asJSON     = flag.Bool("json", false, "emit JSON instead of a table")
		weightPath = flag.String("weights", "", "JSON file of tier weights to try instead of the defaults")
		minMatches = flag.Int("min-matches", validate.DefaultMinMatches,
			"matches of history each player needs before a prediction is scored")
		promotedAfter = flag.Int("promoted-after", validate.DefaultPromotedAfter,
			"lower-tier matches before a first tour-level match counts as a promotion")
		promotionWindow = flag.Int("promotion-window", validate.DefaultPromotionWindow,
			"tour-level matches measured after a promotion")
		systematicZ = flag.Float64("systematic-z", validate.DefaultSystematicZ,
			"standard deviations at which a promotion gap is called systematic")
		teamEvents = flag.Bool("team-events", false, "include team events, as cmd/rate would")
		simTour    = flag.String("simulation-tour", "atp", "tour to validate the simulator against")
		simMatches = flag.Int("simulation-matches", 0, "matches sampled for the deciding-set check")
		simEvents  = flag.Int("simulation-events", 0, "draws sampled for the draw check")
		skipSim    = flag.Bool("no-simulation", false, "skip the simulator checks")
	)
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := validate.Config{
		IncludeTeamEvents: *teamEvents,
		MinMatches:        *minMatches,
		PromotedAfter:     *promotedAfter,
		PromotionWindow:   *promotionWindow,
		SystematicZ:       *systematicZ,
	}

	simCfg := validate.SimulationConfig{
		Tour: *simTour, Matches: *simMatches, Events: *simEvents,
	}
	if *skipSim {
		simCfg.Tour = ""
	}

	code, err := run(ctx, *dsn, *weightPath, *asJSON, cfg, simCfg, !*skipSim)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			slog.Info("validate: cancelled")
			return
		}
		slog.Error("validate: failed", "error", err)
		os.Exit(1)
	}
	os.Exit(code)
}

func run(
	ctx context.Context, dsn, weightPath string, asJSON bool,
	cfg validate.Config, simCfg validate.SimulationConfig, withSimulation bool,
) (int, error) {
	if dsn == "" {
		return 0, errors.New("no database URL: set DATABASE_URL or pass -database-url")
	}

	if weightPath != "" {
		weights, err := loadWeights(weightPath)
		if err != nil {
			return 0, err
		}
		cfg.Weights = weights
	}

	pool, err := db.Open(ctx, db.Config{DSN: dsn, MaxConns: 2, MinConns: 1})
	if err != nil {
		return 0, err
	}
	defer pool.Close()

	slog.InfoContext(ctx, "validate: replaying every match")

	report, err := validate.Run(ctx, pool, cfg)
	if err != nil {
		return 0, err
	}

	if withSimulation {
		slog.InfoContext(ctx, "validate: checking what the simulation chain adds")
		sim, serr := validate.RunSimulation(ctx, pool, simCfg)
		if serr != nil {
			return 0, serr
		}
		report.Simulation = &sim
	}

	if asJSON {
		err = report.WriteJSON(os.Stdout)
	} else {
		err = report.WriteText(os.Stdout)
	}
	if err != nil {
		return 0, err
	}

	if report.Failed() {
		return 1, nil
	}
	return 0, nil
}

// loadWeights reads an alternative weight set.
//
// A file rather than a rebuild, per ADR-0004: the numbers below tour level are
// a judgement, and trying another one should not need a compiler. Every field
// must be present, because a zero weight is a real setting and cannot double as
// "unspecified".
func loadWeights(path string) (rating.Weights, error) {
	var w rating.Weights

	raw, err := os.ReadFile(path)
	if err != nil {
		return w, fmt.Errorf("read weights: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	// An unknown key is almost always a misspelled one, and silently ignoring
	// it would report on the default weight while claiming to test another.
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&w); err != nil {
		return w, fmt.Errorf("read weights from %s: %w", path, err)
	}
	if (w == rating.Weights{}) {
		return w, fmt.Errorf("read weights from %s: every weight is zero, which rates nothing", path)
	}
	return w, nil
}
