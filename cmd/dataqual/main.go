// Command dataqual reports data-quality anomalies found in the ingested data.
//
// Integrity violations exit non-zero so this can gate CI; anomalies and
// coverage gaps are reported but never fail the run, because they are facts
// about the sources rather than defects.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/sami0076/tennis-wiki/internal/dataqual"
	"github.com/sami0076/tennis-wiki/internal/db"
)

func main() {
	var (
		dsn        = flag.String("database-url", os.Getenv("DATABASE_URL"), "PostgreSQL connection string")
		asJSON     = flag.Bool("json", false, "emit JSON instead of a table")
		failOnWarn = flag.Bool("strict", false, "also exit non-zero on warnings")
	)
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	code, err := run(ctx, *dsn, *asJSON, *failOnWarn)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			slog.Info("dataqual: cancelled")
			return
		}
		slog.Error("dataqual: failed", "error", err)
		os.Exit(1)
	}
	os.Exit(code)
}

func run(ctx context.Context, dsn string, asJSON, strict bool) (int, error) {
	if dsn == "" {
		return 0, errors.New("no database URL: set DATABASE_URL or pass -database-url")
	}

	pool, err := db.Open(ctx, db.Config{DSN: dsn, MaxConns: 4, MinConns: 1})
	if err != nil {
		return 0, err
	}
	defer pool.Close()

	report, err := dataqual.Run(ctx, pool)
	if err != nil {
		return 0, err
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
	if strict && hasWarnings(report) {
		return 1, nil
	}
	return 0, nil
}

func hasWarnings(r dataqual.Report) bool {
	for _, f := range r.Findings {
		if f.Severity == dataqual.Warning && f.Count > 0 {
			return true
		}
	}
	return false
}
