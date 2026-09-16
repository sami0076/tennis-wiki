package charting

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"time"

	"github.com/sami0076/tennis-wiki/internal/ingest"
)

// Stats summarises a run. Charted equals resolved plus unresolved; unresolved
// is broken down by reason so the shortfall is a number with a cause.
type Stats struct {
	Charted      int
	Resolved     int
	Unresolved   map[string]int
	StatsWritten int
	RowsRejected int
	FilesRead    int
	FilesSkipped int
	FilesMissing int
}

// Loader ingests the Match Charting Project.
type Loader struct {
	Sources []ingest.ChartingSource
	Fetcher ingest.PathFetcher
	Store   Store
	Log     *slog.Logger
	// Ledger makes a run resumable; nil reads every file.
	Ledger ingest.Ledger
	// Force re-reads a file the ledger says has not changed.
	Force bool

	known map[ingest.FileKey]string
}

func (l *Loader) log() *slog.Logger {
	if l.Log != nil {
		return l.Log
	}
	return slog.Default()
}

// Run reads each tour's two files and attaches what resolves. A tour whose
// files are both unchanged since the last run is skipped whole; the matches
// file and the stats file are one record and are re-read together.
func (l *Loader) Run(ctx context.Context) (Stats, error) {
	stats := Stats{Unresolved: map[string]int{}}

	if _, conditional := l.Fetcher.(ingest.ConditionalFetcher); conditional && l.Ledger != nil && !l.Force {
		var err error
		if l.known, err = l.Ledger.IngestedFiles(ctx); err != nil {
			return stats, err
		}
	}

	for _, src := range l.Sources {
		if err := l.load(ctx, src, &stats); err != nil {
			return stats, err
		}
	}

	l.log().InfoContext(ctx, "charting ingest finished",
		"charted", stats.Charted, "resolved", stats.Resolved, "unresolved", stats.Unresolved,
		"stats_written", stats.StatsWritten, "rows_rejected", stats.RowsRejected,
		"files_read", stats.FilesRead, "files_skipped", stats.FilesSkipped,
		"files_missing", stats.FilesMissing)
	return stats, nil
}

func (l *Loader) load(ctx context.Context, src ingest.ChartingSource, stats *Stats) error {
	matchesBody, matchesTag, err := l.open(ctx, src, src.Matches)
	matchesUnchanged := errors.Is(err, ingest.ErrUnchanged)
	if err != nil && !matchesUnchanged {
		return l.absent(ctx, src, src.Matches, err, stats)
	}
	statsBody, statsTag, err := l.open(ctx, src, src.Stats)
	statsUnchanged := errors.Is(err, ingest.ErrUnchanged)
	if err != nil && !statsUnchanged {
		if !matchesUnchanged {
			_ = matchesBody.Close()
		}
		return l.absent(ctx, src, src.Stats, err, stats)
	}
	if matchesUnchanged && statsUnchanged {
		stats.FilesSkipped += 2
		l.log().InfoContext(ctx, "charting files unchanged since the last ingest", "source", src.Name)
		return nil
	}
	// One changed: read both, unconditionally for the one that did not.
	if matchesUnchanged {
		if matchesBody, err = l.Fetcher.OpenPath(ctx, src.BaseURL, src.Matches); err != nil {
			_ = statsBody.Close()
			return fmt.Errorf("open %s: %w", src.Matches, err)
		}
	}
	if statsUnchanged {
		if statsBody, err = l.Fetcher.OpenPath(ctx, src.BaseURL, src.Stats); err != nil {
			_ = matchesBody.Close()
			return fmt.Errorf("open %s: %w", src.Stats, err)
		}
	}
	defer func() { _ = matchesBody.Close() }()
	defer func() { _ = statsBody.Close() }()

	matches, mc, err := ReadMatches(src.Tour, matchesBody)
	if err != nil {
		return fmt.Errorf("%s: %w", src.Matches, err)
	}
	lines, sc, err := ReadStats(statsBody)
	if err != nil {
		return fmt.Errorf("%s: %w", src.Stats, err)
	}
	stats.FilesRead += 2
	stats.RowsRejected += mc.Rejected + sc.Rejected

	byMatch := map[string][]Stat{}
	for _, s := range lines {
		byMatch[s.MatchID] = append(byMatch[s.MatchID], s)
	}

	unresolved := map[string]string{} // charting id -> reason
	resolved, written := 0, 0
	for _, year := range years(matches) {
		index, err := l.index(ctx, src.Tour, year)
		if err != nil {
			return err
		}
		for _, m := range matches {
			if m.PlayedOn.Year() != year {
				continue
			}
			stats.Charted++
			r, reason := index.Resolve(m)
			if reason != "" {
				stats.Unresolved[reason]++
				unresolved[m.ID] = reason
				continue
			}
			n, err := l.Store.Write(ctx, src.Name, m, r, byMatch[m.ID])
			if err != nil {
				return err
			}
			stats.Resolved++
			stats.StatsWritten += n
			resolved++
			written += n
		}
	}

	byReason := map[string]map[string]int{}
	for id, reason := range unresolved {
		if byReason[reason] == nil {
			byReason[reason] = map[string]int{}
		}
		byReason[reason][id] = 1
	}
	for reason, ids := range byReason {
		if err := l.Store.RecordUnresolved(ctx, src.Name, reason, ids); err != nil {
			return err
		}
	}

	l.record(ctx, src, src.Matches, matchesTag, mc.Seen, resolved)
	l.record(ctx, src, src.Stats, statsTag, sc.Seen, written)
	return nil
}

// index loads one tour's rows whose event could contain a match charted in
// the year, and indexes them.
func (l *Loader) index(ctx context.Context, tour ingest.Tour, year int) (*Index, error) {
	from := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC).Add(-after)
	to := time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC).Add(before)
	cands, err := l.Store.Candidates(ctx, tour, from, to)
	if err != nil {
		return nil, err
	}
	return NewIndex(cands), nil
}

func years(matches []Match) []int {
	seen := map[int]struct{}{}
	for _, m := range matches {
		seen[m.PlayedOn.Year()] = struct{}{}
	}
	out := make([]int, 0, len(seen))
	for y := range seen {
		out = append(out, y)
	}
	sort.Ints(out)
	return out
}

func (l *Loader) open(ctx context.Context, src ingest.ChartingSource, path string) (io.ReadCloser, string, error) {
	if c, ok := l.Fetcher.(ingest.ConditionalFetcher); ok {
		return c.OpenPathIfChanged(ctx, src.BaseURL, path, l.known[ingest.PathKey(src.Name, path)])
	}
	body, err := l.Fetcher.OpenPath(ctx, src.BaseURL, path)
	return body, "", err
}

// absent turns a missing file into a count and a warning, and anything else
// into the error it is.
func (l *Loader) absent(ctx context.Context, src ingest.ChartingSource, path string, err error, stats *Stats) error {
	if errors.Is(err, ingest.ErrNotFound) {
		stats.FilesMissing++
		l.log().WarnContext(ctx, "charting file absent", "source", src.Name, "path", path)
		return nil
	}
	return fmt.Errorf("open %s: %w", path, err)
}

func (l *Loader) record(ctx context.Context, src ingest.ChartingSource, path, validator string, seen, written int) {
	if l.Ledger == nil || validator == "" {
		return
	}
	if err := l.Ledger.RecordFile(ctx, ingest.PathKey(src.Name, path), validator, seen, written); err != nil {
		l.log().WarnContext(ctx, "could not record an ingested file",
			"source", src.Name, "path", path, "error", err)
	}
}
