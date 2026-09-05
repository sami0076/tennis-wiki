package ingest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
)

// ReferenceStore is the persistence the reference loader needs. An interface so
// the loader can be exercised without a database.
type ReferenceStore interface {
	WritePlayers(ctx context.Context, tour Tour, bios []PlayerBio) (int, error)
	PlayerIDsBySource(ctx context.Context, tour Tour) (map[string]int64, error)
	WriteRankings(ctx context.Context, batch []rankingWrite) (int64, error)
	RecordUnresolved(ctx context.Context, source, kind string, counts map[string]int) error
}

// RefStats summarises a reference run. As with matches, the numbers balance:
// rows seen equals written plus rejected plus unresolved.
type RefStats struct {
	Players             int
	RankingsWritten     int64
	RankingRowsSeen     int
	RankingRowsRejected int
	Unresolved          int
	FilesRead           int
	FilesMissing        int
	// FilesSkipped were unchanged since the last run and were not transferred.
	FilesSkipped int
}

// ReferenceLoader ingests player tables and ranking history.
//
// Sequential rather than concurrent: there are a dozen files, not three hundred,
// and rankings must not begin until every player exists to reference.
type ReferenceLoader struct {
	Sources   []RefSource
	Fetcher   PathFetcher
	Store     ReferenceStore
	Log       *slog.Logger
	BatchSize int
	// Ledger makes a run resumable; nil reads every file as before.
	Ledger Ledger
	// Force re-reads every file even if the ledger says it has not changed.
	Force bool

	known map[FileKey]string
}

func (l *ReferenceLoader) batchSize() int {
	if l.BatchSize > 0 {
		return l.BatchSize
	}
	return 5000
}

func (l *ReferenceLoader) log() *slog.Logger {
	if l.Log != nil {
		return l.Log
	}
	return slog.Default()
}

// Run loads every player table, then every ranking file. The order is a
// requirement, not a convenience: a ranking cannot be resolved to a player who
// has not been written yet.
func (l *ReferenceLoader) Run(ctx context.Context) (RefStats, error) {
	var stats RefStats

	if _, conditional := l.Fetcher.(ConditionalFetcher); conditional && l.Ledger != nil && !l.Force {
		var err error
		if l.known, err = l.Ledger.IngestedFiles(ctx); err != nil {
			return stats, err
		}
	}

	for _, src := range l.Sources {
		if src.Kind != RefPlayers {
			continue
		}
		if err := l.loadPlayers(ctx, src, &stats); err != nil {
			return stats, err
		}
	}

	for _, src := range l.Sources {
		if src.Kind != RefRankings {
			continue
		}
		if err := l.loadRankings(ctx, src, &stats); err != nil {
			return stats, err
		}
	}

	// A file skipped as unchanged still counts as found.
	if stats.FilesRead == 0 && stats.FilesSkipped == 0 {
		return stats, fmt.Errorf("no reference files found: %d missing", stats.FilesMissing)
	}
	l.log().InfoContext(ctx, "reference ingest finished",
		"players", stats.Players,
		"rankings_written", stats.RankingsWritten,
		"ranking_rows_seen", stats.RankingRowsSeen,
		"ranking_rows_rejected", stats.RankingRowsRejected,
		"unresolved_players", stats.Unresolved,
		"files_read", stats.FilesRead, "files_skipped", stats.FilesSkipped,
		"files_missing", stats.FilesMissing)
	return stats, nil
}

// open asks conditionally when the fetcher can, and returns ErrUnchanged for a
// file the mirror says is byte-for-byte what was read last time.
func (l *ReferenceLoader) open(ctx context.Context, src RefSource, path string) (
	io.ReadCloser, string, error) {
	if c, ok := l.Fetcher.(ConditionalFetcher); ok {
		return c.OpenPathIfChanged(ctx, src.BaseURL, path, l.known[PathKey(src.Name, path)])
	}
	body, err := l.Fetcher.OpenPath(ctx, src.BaseURL, path)
	return body, "", err
}

// record stores a file's validator. A failure loses the shortcut, not the data.
func (l *ReferenceLoader) record(ctx context.Context, src RefSource, path, validator string,
	seen, written int) {
	if l.Ledger == nil || validator == "" {
		return
	}
	if err := l.Ledger.RecordFile(ctx, PathKey(src.Name, path), validator, seen, written); err != nil {
		l.log().WarnContext(ctx, "could not record an ingested file",
			"source", src.Name, "path", path, "error", err)
	}
}

func (l *ReferenceLoader) loadPlayers(ctx context.Context, src RefSource, stats *RefStats) error {
	body, validator, err := l.open(ctx, src, src.Path)
	if errors.Is(err, ErrUnchanged) {
		stats.FilesSkipped++
		l.log().InfoContext(ctx, "player table unchanged since the last ingest",
			"source", src.Name, "path", src.Path)
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		stats.FilesMissing++
		l.log().WarnContext(ctx, "player table absent", "source", src.Name, "path", src.Path)
		return nil
	}
	if err != nil {
		return fmt.Errorf("open %s: %w", src.Name, err)
	}
	defer func() {
		if cerr := body.Close(); cerr != nil {
			l.log().WarnContext(ctx, "closing player table", "source", src.Name, "error", cerr)
		}
	}()

	reader, err := NewPlayerReader(body)
	if err != nil {
		return fmt.Errorf("%s: %w", src.Name, err)
	}

	batch := make([]PlayerBio, 0, l.batchSize())
	flush := func() error {
		n, err := l.Store.WritePlayers(ctx, src.Tour, batch)
		stats.Players += n
		batch = batch[:0]
		return err
	}

	for {
		bio, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("%s: %w", src.Name, err)
		}
		batch = append(batch, bio)
		if len(batch) >= l.batchSize() {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := flush(); err != nil {
		return err
	}

	stats.FilesRead++
	l.record(ctx, src, src.Path, validator, reader.Rows(), reader.Rows())
	l.log().InfoContext(ctx, "player table loaded",
		"source", src.Name, "tour", src.Tour, "rows", reader.Rows())
	return nil
}

func (l *ReferenceLoader) loadRankings(ctx context.Context, src RefSource, stats *RefStats) error {
	ids, err := l.Store.PlayerIDsBySource(ctx, src.Tour)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return fmt.Errorf("%s: no %s players are loaded; rankings must be ingested after the player table",
			src.Name, src.Tour)
	}

	// Counted per source id rather than per row, so one missing player does not
	// produce a million identical complaints.
	unresolved := map[string]int{}

	for _, path := range src.RelPaths() {
		if err := l.loadRankingFile(ctx, src, path, ids, unresolved, stats); err != nil {
			return err
		}
	}

	if len(unresolved) > 0 {
		stats.Unresolved += len(unresolved)
		if err := l.Store.RecordUnresolved(ctx, src.Name, string(RefRankings), unresolved); err != nil {
			return err
		}
		// Loud, but not fatal: the run is still useful, and cmd/dataqual
		// reports the stored detail.
		l.log().WarnContext(ctx, "ranking rows reference players that do not exist",
			"source", src.Name, "distinct_players", len(unresolved),
			"note", "see unresolved_references, reported by cmd/dataqual")
	}
	return nil
}

func (l *ReferenceLoader) loadRankingFile(ctx context.Context, src RefSource, path string,
	ids map[string]int64, unresolved map[string]int, stats *RefStats) error {
	body, validator, err := l.open(ctx, src, path)
	if errors.Is(err, ErrUnchanged) {
		stats.FilesSkipped++
		l.log().InfoContext(ctx, "ranking file unchanged since the last ingest",
			"source", src.Name, "path", path)
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		stats.FilesMissing++
		l.log().DebugContext(ctx, "ranking file absent", "source", src.Name, "path", path)
		return nil
	}
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer func() {
		if cerr := body.Close(); cerr != nil {
			l.log().WarnContext(ctx, "closing ranking file", "path", path, "error", cerr)
		}
	}()

	reader, err := NewRankingReader(body)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	batch := make([]rankingWrite, 0, l.batchSize())
	var written int64
	flush := func() error {
		n, err := l.Store.WriteRankings(ctx, batch)
		stats.RankingsWritten += n
		written += n
		batch = batch[:0]
		return err
	}

	skipped := 0
	for {
		entry, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		id, ok := ids[entry.SourceID]
		if !ok {
			unresolved[entry.SourceID]++
			skipped++
			continue
		}
		batch = append(batch, rankingWrite{
			playerID: id, date: entry.Date, rank: entry.Rank, points: entry.Points,
		})
		if len(batch) >= l.batchSize() {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := flush(); err != nil {
		return err
	}

	stats.FilesRead++
	stats.RankingRowsSeen += reader.Rows()
	stats.RankingRowsRejected += reader.Rejected()

	// A file with rows waiting on a player who does not exist yet is not
	// finished with. Recording it would skip the file on the run that finally
	// has that player, and those rankings would never arrive.
	if skipped > 0 {
		l.log().DebugContext(ctx, "not recording a ranking file with unresolved rows",
			"source", src.Name, "path", path, "rows", skipped)
		return nil
	}
	l.record(ctx, src, path, validator, reader.Rows(), int(written))
	return nil
}
