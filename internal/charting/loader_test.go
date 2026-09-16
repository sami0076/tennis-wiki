package charting

import (
	"context"
	"testing"
	"time"

	"github.com/sami0076/tennis-wiki/internal/ingest"
)

// fakeStore answers with a fixed set of rows and remembers what was written.
type fakeStore struct {
	cands      []Candidate
	written    []string
	stats      int
	unresolved map[string]map[string]int
}

func (f *fakeStore) Candidates(_ context.Context, _ ingest.Tour, _, _ time.Time) ([]Candidate, error) {
	return f.cands, nil
}

func (f *fakeStore) Write(_ context.Context, _ string, batch []Attachment) (int, error) {
	n := 0
	for _, a := range batch {
		f.written = append(f.written, a.Match.ID)
		f.stats += len(a.Stats)
		n += len(a.Stats)
	}
	return n, nil
}

func (f *fakeStore) RecordUnresolved(_ context.Context, _, kind string, counts map[string]int) error {
	if f.unresolved == nil {
		f.unresolved = map[string]map[string]int{}
	}
	f.unresolved[kind] = counts
	return nil
}

// The fixture's five rows: four read, of which the database "has" two. The
// run resolves those two with their figures, leaves the other two with a
// reason each, and the rejected row is counted and nothing else.
func TestLoaderAttachesWhatResolvesAndCountsTheRest(t *testing.T) {
	store := &fakeStore{cands: []Candidate{
		{MatchID: 1, WinnerID: 10, LoserID: 20, Winner: "Jesper De Jong", Loser: "Michael Zheng",
			Round: "Q3", PlayedOn: day(t, "2026-05-25")},
		{MatchID: 2, WinnerID: 30, LoserID: 40, Winner: "Bjorn Borg", Loser: "John McEnroe",
			Round: "F", PlayedOn: day(t, "1980-06-23")},
	}}
	loader := &Loader{
		Sources: []ingest.ChartingSource{{Name: "mcp-atp", Tour: ingest.TourATP,
			BaseURL: "x", Matches: "charting-m-matches.csv", Stats: "charting-m-stats-Overview.csv"}},
		Fetcher: ingest.LocalFetcher{Root: "testdata"},
		Store:   store,
	}

	stats, err := loader.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Charted != 4 || stats.Resolved != 2 || stats.RowsRejected != 1 {
		t.Errorf("stats = %+v, want 4 charted, 2 resolved, 1 row rejected", stats)
	}
	if len(store.written) != 2 || store.stats == 0 {
		t.Errorf("wrote %v with %d stat rows", store.written, store.stats)
	}
	if n := stats.Unresolved["no match on that date with those players in that round"]; n != 2 {
		t.Errorf("unresolved by reason = %v, want the Davis Cup tie and the Bucharest semi-final", stats.Unresolved)
	}
	if len(store.unresolved) != 1 {
		t.Errorf("unresolved recorded as %v, want one reason", store.unresolved)
	}
	if stats.FilesRead != 2 {
		t.Errorf("files read = %d, want both", stats.FilesRead)
	}
}

// A ledger that has seen both files at their current validators skips the
// source; one that has seen only one re-reads both.
type fakeLedger struct{ known map[ingest.FileKey]string }

func (l fakeLedger) IngestedFiles(context.Context) (map[ingest.FileKey]string, error) {
	return l.known, nil
}
func (l fakeLedger) RecordFile(_ context.Context, k ingest.FileKey, v string, _, _ int) error {
	l.known[k] = v
	return nil
}

func TestLoaderSkipsASourceWhoseFilesAreUnchanged(t *testing.T) {
	ledger := fakeLedger{known: map[ingest.FileKey]string{}}
	src := ingest.ChartingSource{Name: "mcp-atp", Tour: ingest.TourATP,
		BaseURL: "x", Matches: "charting-m-matches.csv", Stats: "charting-m-stats-Overview.csv"}
	loader := &Loader{Sources: []ingest.ChartingSource{src},
		Fetcher: ingest.LocalFetcher{Root: "testdata"}, Store: &fakeStore{}, Ledger: ledger}

	first, err := loader.Run(context.Background())
	if err != nil || first.FilesRead != 2 {
		t.Fatalf("first run: %+v, %v", first, err)
	}
	second, err := loader.Run(context.Background())
	if err != nil || second.FilesSkipped != 2 || second.FilesRead != 0 {
		t.Fatalf("second run: %+v, %v; want both files skipped", second, err)
	}

	delete(ledger.known, ingest.PathKey(src.Name, src.Stats))
	third, err := loader.Run(context.Background())
	if err != nil || third.FilesRead != 2 {
		t.Fatalf("third run: %+v, %v; want both re-read when one changed", third, err)
	}
}
