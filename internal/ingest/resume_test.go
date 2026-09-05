package ingest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// memLedger is the ledger without a database.
type memLedger struct {
	mu       sync.Mutex
	files    map[FileKey]string
	recorded int
}

func newMemLedger() *memLedger { return &memLedger{files: map[FileKey]string{}} }

func (m *memLedger) IngestedFiles(context.Context) (map[FileKey]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[FileKey]string, len(m.files))
	for k, v := range m.files {
		out[k] = v
	}
	return out, nil
}

func (m *memLedger) RecordFile(_ context.Context, k FileKey, validator string, _, _ int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[k] = validator
	m.recorded++
	return nil
}

// The whole point of #55: the second run over unchanged sources does no work.
func TestSecondRunSkipsUnchangedFiles(t *testing.T) {
	ledger := newMemLedger()
	run := func(force bool) (RunStats, *memWriter) {
		w := &memWriter{}
		p := &Pipeline{
			Registry: testRegistry(),
			Fetcher:  LocalFetcher{Root: "testdata"},
			Store:    w,
			Ledger:   ledger,
			Options:  Options{Workers: 2, BatchSize: 100, Force: force},
		}
		stats, err := p.Run(context.Background())
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		return stats, w
	}

	first, firstWriter := run(false)
	if first.FilesRead != 2 || first.FilesSkipped != 0 {
		t.Fatalf("first run read %d and skipped %d, want 2 and 0",
			first.FilesRead, first.FilesSkipped)
	}
	if ledger.recorded != 2 {
		t.Fatalf("%d files recorded, want one per file read", ledger.recorded)
	}

	second, secondWriter := run(false)
	if second.FilesSkipped != 2 || second.FilesRead != 0 {
		t.Errorf("second run read %d and skipped %d, want 0 and 2",
			second.FilesRead, second.FilesSkipped)
	}
	if secondWriter.count() != 0 {
		t.Errorf("second run wrote %d rows; an unchanged file should cost nothing",
			secondWriter.count())
	}
	if second.RowsSeen != 0 {
		t.Errorf("second run read %d rows, want none", second.RowsSeen)
	}

	forced, forcedWriter := run(true)
	if forced.FilesRead != 2 || forced.FilesSkipped != 0 {
		t.Errorf("forced run read %d and skipped %d, want 2 and 0",
			forced.FilesRead, forced.FilesSkipped)
	}
	if forcedWriter.count() != firstWriter.count() {
		t.Errorf("forced run wrote %d rows, first wrote %d",
			forcedWriter.count(), firstWriter.count())
	}
}

// A changed file is read again. Without this the ledger would be a way to miss
// corrections rather than a way to skip work.
func TestChangedFileIsReadAgain(t *testing.T) {
	root := t.TempDir()
	original, err := os.ReadFile(filepath.Join("testdata", "atp_matches_2024.csv"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "atp_matches_2024.csv")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}

	registry := &Registry{Sources: []Source{{
		Name: "f", Tour: TourATP, Tier: "tour", Profile: "sackmann",
		BaseURL: "https://x", Path: "atp_matches_{season}.csv",
		FirstSeason: 2024, LastSeason: 2024,
	}}}
	ledger := newMemLedger()
	run := func() RunStats {
		p := &Pipeline{
			Registry: registry,
			Fetcher:  LocalFetcher{Root: root},
			Store:    &memWriter{},
			Ledger:   ledger,
			Options:  Options{Workers: 1, BatchSize: 100},
		}
		stats, err := p.Run(context.Background())
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		return stats
	}

	run()
	if s := run(); s.FilesSkipped != 1 {
		t.Fatalf("unchanged file was not skipped: %+v", s)
	}

	// Rewriting the file changes its size and modification time, which is the
	// validator a clone on disk offers in place of an ETag.
	if err := os.WriteFile(path, append(original, original[len(original)-80:]...), 0o600); err != nil {
		t.Fatal(err)
	}
	if s := run(); s.FilesRead != 1 || s.FilesSkipped != 0 {
		t.Errorf("changed file read %d skipped %d, want 1 and 0", s.FilesRead, s.FilesSkipped)
	}
}

// The mirrors answer If-None-Match with 304 and no body, which is where the
// time actually goes: a full run transfers a few hundred files.
func TestHTTPFetcherSendsAndHonoursTheValidator(t *testing.T) {
	const etag = `"abc123"`
	var conditional int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if match := r.Header.Get("If-None-Match"); match != "" {
			conditional++
			if match == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
		w.Header().Set("ETag", etag)
		if _, err := w.Write([]byte("body")); err != nil {
			t.Errorf("write: %v", err)
		}
	}))
	defer srv.Close()

	f := HTTPFetcher{Client: srv.Client()}
	ctx := context.Background()

	body, got, err := f.OpenPathIfChanged(ctx, srv.URL, "file.csv", "")
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}
	if got != etag {
		t.Errorf("validator = %q, want the ETag %q", got, etag)
	}
	if conditional != 0 {
		t.Errorf("an empty validator must not send If-None-Match")
	}

	if _, _, err := f.OpenPathIfChanged(ctx, srv.URL, "file.csv", etag); err == nil ||
		!errors.Is(err, ErrUnchanged) {
		t.Errorf("matching validator returned %v, want ErrUnchanged", err)
	}
	if conditional != 1 {
		t.Errorf("If-None-Match was sent %d times, want 1", conditional)
	}

	body, _, err = f.OpenPathIfChanged(ctx, srv.URL, "file.csv", `"stale"`)
	if err != nil {
		t.Fatalf("stale validator: %v", err)
	}
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}
}
