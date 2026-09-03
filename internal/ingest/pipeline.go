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
	RowsSeen     int
	RowsWritten  int
	RowsRejected int
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
}

// job is one source file.
type job struct {
	source Source
	season int
}

// chunk is a batch of parsed rows travelling to the writer.
type chunk struct {
	source Source
	rows   []MatchRow
	seen   int
	// rejected counts rows this chunk could not parse.
	rejected int
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
	log.InfoContext(ctx, "ingest starting",
		"files", len(jobs), "workers", p.Options.workers(), "batch", p.Options.batchSize())

	jobCh := make(chan job)
	chunkCh := make(chan chunk, p.Options.workers())

	var readers sync.WaitGroup
	readErrs := make(chan error, len(jobs))

	for i := 0; i < p.Options.workers(); i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for j := range jobCh {
				if err := p.readFile(ctx, j, chunkCh, log); err != nil {
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
	for c := range chunkCh {
		stats.RowsSeen += c.seen
		stats.RowsRejected += c.rejected
		if c.rows == nil {
			stats.FilesRead++
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
		stats.RowsWritten += res.Matches
	}

	for err := range readErrs {
		if err != nil && writeErr == nil {
			writeErr = err
		}
	}
	if writeErr != nil {
		return stats, writeErr
	}

	log.InfoContext(ctx, "ingest finished",
		"rows_seen", stats.RowsSeen, "rows_written", stats.RowsWritten,
		"rejected", stats.RowsRejected, "files", stats.FilesRead)
	return stats, nil
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
func (p *Pipeline) readFile(ctx context.Context, j job, out chan<- chunk, log *slog.Logger) error {
	body, err := p.Fetcher.Open(ctx, j.source, j.season)
	if errors.Is(err, ErrNotFound) {
		log.DebugContext(ctx, "source file absent", "source", j.source.Name, "season", j.season)
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
		c := chunk{source: j.source, rows: batch, seen: seen, rejected: rejected}
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

	// A nil-rows chunk marks the file as finished.
	select {
	case out <- chunk{source: j.source}:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
