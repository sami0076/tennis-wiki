package ingest

import (
	"testing"
)

func TestLedgerRoundTrips(t *testing.T) {
	store, ctx := testStore(t)
	key := SeasonKey("fixture-wta", 1937)

	files, err := store.IngestedFiles(ctx)
	if err != nil {
		t.Fatalf("IngestedFiles: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("empty database reports %d ingested files", len(files))
	}

	if err := store.RecordFile(ctx, key, `"etag-1"`, 100, 90); err != nil {
		t.Fatalf("RecordFile: %v", err)
	}
	// Re-recording the same file replaces it rather than failing.
	if err := store.RecordFile(ctx, key, `"etag-2"`, 110, 95); err != nil {
		t.Fatalf("re-record: %v", err)
	}
	if err := store.RecordFile(ctx, PathKey("wta-players", "wta_players.csv"), `"p"`, 5, 5); err != nil {
		t.Fatalf("record reference file: %v", err)
	}

	files, err = store.IngestedFiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("ledger holds %d files, want 2", len(files))
	}
	if files[key] != `"etag-2"` {
		t.Errorf("validator = %q, want the latest", files[key])
	}
}

// Prune deletes matches, so it has to un-record the files that wrote them. If
// it did not, the next run would skip exactly the files it has to read.
func TestPruneForgetsTheFilesItEmptied(t *testing.T) {
	store, ctx := testStore(t)
	rows := fixtureRows(t, "wta_matches_1937.csv", wtaTour)
	if _, err := store.WriteBatch(ctx, wtaTour, rows); err != nil {
		t.Fatalf("write: %v", err)
	}
	key := SeasonKey(wtaTour.Name, 1937)
	if err := store.RecordFile(ctx, key, `"etag"`, len(rows), len(rows)); err != nil {
		t.Fatal(err)
	}

	// Nothing is collapsed, so nothing is forgotten.
	if _, err := store.Prune(ctx, false); err != nil {
		t.Fatalf("prune: %v", err)
	}
	files, err := store.IngestedFiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := files[key]; !ok {
		t.Fatal("prune forgot a file it did not touch")
	}

	fuseMatchOne(ctx, t, store)

	res, err := store.Prune(ctx, false)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if res.Collapsed != 1 {
		t.Fatalf("prune deleted %d collapsed matches, want 1", res.Collapsed)
	}
	files, err = store.IngestedFiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := files[key]; ok {
		t.Error("the file whose matches were deleted is still recorded as ingested")
	}
}
