package ingest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"sync"
)

// DefaultBatchSize is how many rows go into one transaction.
const DefaultBatchSize = 2000

// Options configure a run.
type Options struct {
	// Workers bounds concurrent file readers. Defaults to GOMAXPROCS.
	Workers int
	// BatchSize is rows per transaction.
	BatchSize int
	// Seasons limits the run; empty means every season the registry covers.
	Seasons []int
	// Tours limits the run; empty means both.
	Tours []Tour
	// Force re-reads every file even if the ledger says it has not changed.
	Force bool
}

func (o Options) workers() int {
	if o.Workers > 0 {
		return o.Workers
	}
	return runtime.GOMAXPROCS(0)
}

func (o Options) batchSize() int {
	if o.BatchSize > 0 {
		return o.BatchSize
	}
	return DefaultBatchSize
}

// RunStats summarise a run.
type RunStats struct {
	FilesRead    int
	FilesMissing int
	// FilesSkipped were unchanged since the last run and were not transferred.
	FilesSkipped int
	RowsSeen     int
	RowsWritten  int
	RowsRejected int
	// RowsCollapsed are rows that shared a natural key with another row in the
	// same batch. Seen = written + rejected + collapsed, and the summary should
	// balance.
	RowsCollapsed int
}

// BatchWriter persists a batch. It is an interface so the pipeline's
// concurrency can be tested without a database.
type BatchWriter interface {
	WriteBatch(ctx context.Context, src Source, rows []MatchRow) (Result, error)
}

// Pipeline reads every configured source and writes it to the database.
//
// Files are read concurrently and fanned in to a single writer, because forty
// goroutines competing over the connection pool is slower than one writer with
// batched transactions, not faster.
type Pipeline struct {
	Registry *Registry
	Fetcher  Fetcher
	Store    BatchWriter
	Log      *slog.Logger
	Options  Options
	// Ledger makes a run resumable. Nil, or a Fetcher that cannot fetch
	// conditionally, and every file is read as before.
	Ledger Ledger
}

// job is one source file.
type job struct {
	source Source
	season int
}

func (j job) key() FileKey { return SeasonKey(j.source.Name, j.season) }

// chunk is a batch of parsed rows travelling to the writer.
type chunk struct {
	source Source
	key    FileKey
	rows   []MatchRow
	seen   int
	// rejected counts rows this chunk could not parse.
	rejected int
	// missing marks a source file the fetcher did not have.
	missing bool
	// skipped marks a file the ledger and the mirror agreed had not changed.
	skipped bool
	// validator describes the content read, and is recorded once every row of
	// the file is committed. Set on the chunk that closes a file.
	validator string
}

// Run executes the pipeline.
func (p *Pipeline) Run(ctx context.Context) (RunStats, error) {
	log := p.Log
	if log == nil {
		log = slog.Default()
	}

	jobs := p.plan()
	if len(jobs) == 0 {
		return RunStats{}, errors.New("no source files selected")
	}
	known, err := p.ledger(ctx)
	if err != nil {
		return RunStats{}, err
	}
	log.InfoContext(ctx, "ingest starting",
		"files", len(jobs), "workers", p.Options.workers(), "batch", p.Options.batchSize(),
		"already_ingested", len(known))

	jobCh := make(chan job)
	chunkCh := make(chan chunk, p.Options.workers())

	var readers sync.WaitGroup
	readErrs := make(chan error, len(jobs))

	for i := 0; i < p.Options.workers(); i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for j := range jobCh {
				if err := p.readFile(ctx, j, known[j.key()], chunkCh, log); err != nil {
					readErrs <- err
					return
				}
			}
		}()
	}

	go func() {
		defer close(jobCh)
		for _, j := range jobs {
			select {
			case jobCh <- j:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		readers.Wait()
		close(chunkCh)
		close(readErrs)
	}()

	// Single writer: batches arrive here and nowhere else.
	var stats RunStats
	var writeErr error
	// Rows are counted per file so that the ledger entry, written when the file
	// closes, describes that file rather than the run.
	tally := map[FileKey]*fileTally{}
	for c := range chunkCh {
		stats.RowsSeen += c.seen
		stats.RowsRejected += c.rejected
		if c.skipped {
			stats.FilesSkipped++
			continue
		}
		if c.missing {
			stats.FilesMissing++
			continue
		}
		t, ok := tally[c.key]
		if !ok {
			t = &fileTally{}
			tally[c.key] = t
		}
		t.seen += c.seen
		if c.rows == nil {
			stats.FilesRead++
			// Recorded only after every batch of this file committed, so a
			// recorded file is one the database genuinely holds.
			if writeErr == nil {
				p.record(ctx, c.key, c.validator, *t, log)
			}
			delete(tally, c.key)
			continue
		}
		if writeErr != nil {
			continue // drain so readers are not blocked
		}
		res, err := p.Store.WriteBatch(ctx, c.source, c.rows)
		if err != nil {
			writeErr = err
			continue
		}
		t.written += res.Matches
		stats.RowsWritten += res.Matches
		stats.RowsCollapsed += res.Collapsed
	}

	for err := range readErrs {
		if err != nil && writeErr == nil {
			writeErr = err
		}
	}
	if writeErr != nil {
		return stats, writeErr
	}

	// An ingest that read nothing at all is a misconfiguration, not an empty
	// result. Reporting success here is how `make ingest` silently did nothing
	// for as long as it pointed at a directory that did not exist. A file
	// skipped as unchanged still counts as found: that is a resumed run
	// working, not a broken one.
	if stats.FilesRead == 0 && stats.FilesSkipped == 0 {
		return stats, fmt.Errorf("no source files found: %d planned, %d missing", len(jobs), stats.FilesMissing)
	}

	log.InfoContext(ctx, "ingest finished",
		"rows_seen", stats.RowsSeen, "rows_written", stats.RowsWritten,
		"rejected", stats.RowsRejected, "collapsed", stats.RowsCollapsed,
		"files_read", stats.FilesRead, "files_skipped", stats.FilesSkipped,
		"files_missing", stats.FilesMissing)
	return stats, nil
}

// fileTally counts one file's rows as its batches go past the writer.
type fileTally struct{ seen, written int }

// ledger returns what has already been ingested, or nothing when the run cannot
// resume: no ledger, a fetcher that cannot ask, or --force.
func (p *Pipeline) ledger(ctx context.Context) (map[FileKey]string, error) {
	_, conditional := p.Fetcher.(ConditionalFetcher)
	if p.Ledger == nil || !conditional || p.Options.Force {
		return nil, nil
	}
	return p.Ledger.IngestedFiles(ctx)
}

// record stores a file's validator. A failure here loses the shortcut, not the
// data, so it is reported rather than fatal.
func (p *Pipeline) record(ctx context.Context, k FileKey, validator string,
	t fileTally, log *slog.Logger) {
	if p.Ledger == nil || validator == "" {
		return
	}
	if err := p.Ledger.RecordFile(ctx, k, validator, t.seen, t.written); err != nil {
		log.WarnContext(ctx, "could not record an ingested file",
			"source", k.Source, "unit", k.Unit, "error", err)
	}
}

// plan expands the registry into one job per source file.
func (p *Pipeline) plan() []job {
	tours := p.Options.Tours
	if len(tours) == 0 {
		tours = []Tour{TourATP, TourWTA}
	}
	seasons := p.Options.Seasons
	if len(seasons) == 0 {
		seasons = p.Registry.Seasons()
	}

	var jobs []job
	for _, t := range tours {
		for _, season := range seasons {
			for _, src := range p.Registry.For(t, season) {
				jobs = append(jobs, job{source: src, season: season})
			}
		}
	}
	return jobs
}

// readFile streams one file, emitting batches. A file the source does not have
// is normal and not an error: mirrors differ in coverage.
func (p *Pipeline) readFile(ctx context.Context, j job, validator string,
	out chan<- chunk, log *slog.Logger) error {
	body, current, err := p.open(ctx, j, validator)
	if errors.Is(err, ErrUnchanged) {
		log.DebugContext(ctx, "source file unchanged since the last ingest",
			"source", j.source.Name, "season", j.season)
		select {
		case out <- chunk{source: j.source, key: j.key(), skipped: true}:
		case <-ctx.Done():
		}
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		log.DebugContext(ctx, "source file absent", "source", j.source.Name, "season", j.season)
		select {
		case out <- chunk{source: j.source, key: j.key(), missing: true}:
		case <-ctx.Done():
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("open %s %d: %w", j.source.Name, j.season, err)
	}
	defer func() {
		if cerr := body.Close(); cerr != nil {
			log.WarnContext(ctx, "closing source file", "source", j.source.Name, "error", cerr)
		}
	}()

	reader, err := NewReader(j.source, body)
	if err != nil {
		return fmt.Errorf("read %s %d: %w", j.source.Name, j.season, err)
	}

	batch := make([]MatchRow, 0, p.Options.batchSize())
	seen, rejected := 0, 0

	flush := func() bool {
		c := chunk{source: j.source, key: j.key(), rows: batch, seen: seen, rejected: rejected}
		seen, rejected = 0, 0
		select {
		case out <- c:
			batch = make([]MatchRow, 0, p.Options.batchSize())
			return true
		case <-ctx.Done():
			return false
		}
	}

	for {
		row, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		seen++
		if err != nil {
			// One malformed row must not abandon the file; it is reported and
			// counted so cmd/dataqual can surface it.
			rejected++
			log.WarnContext(ctx, "skipping unreadable row",
				"source", j.source.Name, "season", j.season, "error", err)
			continue
		}
		batch = append(batch, row)
		if len(batch) >= p.Options.batchSize() {
			if !flush() {
				return ctx.Err()
			}
		}
	}

	if len(batch) > 0 || seen > 0 {
		if !flush() {
			return ctx.Err()
		}
	}

	// A nil-rows chunk marks the file as finished, and carries the validator
	// the writer records once it knows every batch went in.
	select {
	case out <- chunk{source: j.source, key: j.key(), validator: current}:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// open asks conditionally when the fetcher can, and unconditionally otherwise.
func (p *Pipeline) open(ctx context.Context, j job, validator string) (
	io.ReadCloser, string, error) {
	if c, ok := p.Fetcher.(ConditionalFetcher); ok {
		return c.OpenPathIfChanged(ctx, j.source.BaseURL, j.source.RelPath(j.season), validator)
	}
	body, err := p.Fetcher.Open(ctx, j.source, j.season)
	return body, "", err
}
