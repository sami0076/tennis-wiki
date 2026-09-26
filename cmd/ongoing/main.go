// Command ongoing refreshes this week's provisional results from the source's
// ongoing files. Hourly, on its own: it touches nothing a full load derives
// and flushes no cache, because /this-week is not cached.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sami0076/tennis-wiki/internal/db"
	"github.com/sami0076/tennis-wiki/internal/ingest"
	"github.com/sami0076/tennis-wiki/internal/ongoing"
)

func main() {
	dsn := flag.String("database-url", os.Getenv("DATABASE_URL"), "PostgreSQL connection string")
	base := flag.String("base-url", ongoing.BaseURL, "where the ongoing files are published")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, *dsn, *base); err != nil {
		slog.Error("ongoing: failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, dsn, base string) error {
	if dsn == "" {
		return errors.New("no database URL: set DATABASE_URL or pass -database-url")
	}
	pool, err := db.Open(ctx, db.Config{DSN: dsn, MaxConns: 2})
	if err != nil {
		return err
	}
	defer pool.Close()

	fetch := ingest.HTTPFetcher{Client: &http.Client{Timeout: time.Minute}}
	var failed error
	// One file failing leaves the other tour's week as fresh as it can be.
	for _, f := range ongoing.Files {
		res, err := ongoing.Refresh(ctx, pool, fetch, base, f)
		if err != nil {
			failed = errors.Join(failed, err)
			continue
		}
		slog.Info("ongoing: refreshed", "file", f.Name,
			"changed", res.Changed, "rows", res.Rows, "skipped", res.Skipped)
	}
	return failed
}
