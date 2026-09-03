package ingest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// memWriter records batches instead of writing them, so the pipeline's
// concurrency can be exercised without a database.
type memWriter struct {
	mu      sync.Mutex
	batches int
	rows    []MatchRow
	err     error
}

func (m *memWriter) WriteBatch(_ context.Context, _ Source, rows []MatchRow) (Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return Result{}, m.err
	}
	m.batches++
	m.rows = append(m.rows, rows...)
	return Result{Matches: len(rows)}, nil
}

func (m *memWriter) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.rows)
}

// testRegistry points every source at the committed fixtures.
func testRegistry() *Registry {
	return &Registry{Sources: []Source{
		{
			Name: "fixture-atp", Tour: TourATP, Tier: "tour", Profile: "sackmann",
			BaseURL: "https://example.test", Path: "atp_matches_{season}.csv",
			FirstSeason: 2024, LastSeason: 2024, Precedence: 10,
		},
		{
			Name: "fixture-wta", Tour: TourWTA, Tier: "tour", Profile: "sackmann",
			BaseURL: "https://example.test", Path: "wta_matches_{season}.csv",
			FirstSeason: 2019, LastSeason: 2019, Precedence: 10,
		},
	}}
}

func TestPipelineReadsBothTours(t *testing.T) {
	w := &memWriter{}
	p := &Pipeline{
		Registry: testRegistry(),
		Fetcher:  LocalFetcher{Root: "testdata"},
		Store:    w,
		Options:  Options{Workers: 2, BatchSize: 100},
	}

	stats, err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stats.RowsSeen == 0 {
		t.Fatal("no rows seen")
	}
	if stats.RowsWritten != w.count() {
		t.Errorf("stats say %d written, writer saw %d", stats.RowsWritten, w.count())
	}
	if stats.FilesRead != 2 {
		t.Errorf("FilesRead = %d, want 2", stats.FilesRead)
	}
	if stats.RowsRejected != 0 {
		t.Errorf("RowsRejected = %d, want 0 for clean fixtures", stats.RowsRejected)
	}
}

// A batch size below the row count must produce several batches, and lose
// nothing at the boundaries.
func TestPipelineBatching(t *testing.T) {
	for _, size := range []int{1, 2, 3, 1000} {
		w := &memWriter{}
		p := &Pipeline{
			Registry: &Registry{Sources: []Source{{
				Name: "f", Tour: TourATP, Tier: "tour", Profile: "sackmann",
				BaseURL: "https://x", Path: "atp_matches_{season}.csv",
				FirstSeason: 2024, LastSeason: 2024,
			}}},
			Fetcher: LocalFetcher{Root: "testdata"},
			Store:   w,
			Options: Options{Workers: 1, BatchSize: size},
		}
		stats, err := p.Run(context.Background())
		if err != nil {
			t.Fatalf("batch size %d: %v", size, err)
		}
		if w.count() != stats.RowsSeen {
			t.Errorf("batch size %d: wrote %d rows, saw %d", size, w.count(), stats.RowsSeen)
		}
	}
}

// A season a source does not carry is normal, not an error.
func TestPipelineToleratesMissingFiles(t *testing.T) {
	w := &memWriter{}
	p := &Pipeline{
		Registry: &Registry{Sources: []Source{{
			Name: "f", Tour: TourATP, Tier: "tour", Profile: "sackmann",
			BaseURL: "https://x", Path: "atp_matches_{season}.csv",
			FirstSeason: 2020, LastSeason: 2024,
		}}},
		Fetcher: LocalFetcher{Root: "testdata"},
		Store:   w,
		Options: Options{Workers: 2, BatchSize: 100},
	}
	stats, err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Only 2024 exists in testdata; the other four seasons are absent.
	if stats.FilesRead != 1 {
		t.Errorf("FilesRead = %d, want 1", stats.FilesRead)
	}
	if w.count() == 0 {
		t.Error("no rows written from the one present file")
	}
}

func TestPipelineSurfacesWriteErrors(t *testing.T) {
	w := &memWriter{err: fmt.Errorf("database is down")}
	p := &Pipeline{
		Registry: testRegistry(),
		Fetcher:  LocalFetcher{Root: "testdata"},
		Store:    w,
		Options:  Options{Workers: 2, BatchSize: 10},
	}
	if _, err := p.Run(context.Background()); err == nil {
		t.Fatal("Run should have returned the write error")
	}
}

func TestPipelineCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := &Pipeline{
		Registry: testRegistry(),
		Fetcher:  LocalFetcher{Root: "testdata"},
		Store:    &memWriter{},
		Options:  Options{Workers: 2, BatchSize: 1},
	}
	// Must return rather than hang; the value of the error is not the point.
	if _, err := p.Run(ctx); err == nil {
		t.Log("run completed before cancellation took effect")
	}
}

func TestPipelineRejectsEmptyPlan(t *testing.T) {
	p := &Pipeline{
		Registry: &Registry{Sources: []Source{{
			Name: "f", Tour: TourATP, Profile: "sackmann", BaseURL: "https://x",
			Path: "a_{season}.csv", FirstSeason: 2024, LastSeason: 2024,
		}}},
		Fetcher: LocalFetcher{Root: "testdata"},
		Store:   &memWriter{},
		Options: Options{Seasons: []int{1800}},
	}
	if _, err := p.Run(context.Background()); err == nil {
		t.Fatal("a plan with no files should be an error")
	}
}

// A single unreadable row must be counted and skipped, not abandon the file.
func TestPipelineSkipsBadRows(t *testing.T) {
	dir := t.TempDir()
	good, err := os.ReadFile(filepath.Join("testdata", "atp_matches_2024.csv"))
	if err != nil {
		t.Fatal(err)
	}
	// Append a row whose tourney_date is unparseable.
	broken := string(good) + "2024-999,Broken,Hard,32,A,NOTADATE,1,1,,,X,R,,USA,20,2,,,Y,R,,USA,21,6-4 6-4,3,F,90\n"
	path := filepath.Join(dir, "atp_matches_2024.csv")
	if err := os.WriteFile(path, []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}

	w := &memWriter{}
	p := &Pipeline{
		Registry: &Registry{Sources: []Source{{
			Name: "f", Tour: TourATP, Tier: "tour", Profile: "sackmann",
			BaseURL: "https://x", Path: "atp_matches_{season}.csv",
			FirstSeason: 2024, LastSeason: 2024,
		}}},
		Fetcher: LocalFetcher{Root: dir},
		Store:   w,
		Options: Options{Workers: 1, BatchSize: 100},
	}
	stats, err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("one bad row should not fail the run: %v", err)
	}
	if stats.RowsRejected != 1 {
		t.Errorf("RowsRejected = %d, want 1", stats.RowsRejected)
	}
	if w.count() == 0 {
		t.Error("the good rows should still have been written")
	}
}

func TestOptionDefaults(t *testing.T) {
	var o Options
	if o.workers() < 1 {
		t.Error("workers should default to at least one")
	}
	if o.batchSize() != DefaultBatchSize {
		t.Errorf("batchSize() = %d, want %d", o.batchSize(), DefaultBatchSize)
	}
	o = Options{Workers: 3, BatchSize: 7}
	if o.workers() != 3 || o.batchSize() != 7 {
		t.Error("explicit options should win")
	}
}
