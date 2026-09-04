package identity

import (
	"os"
	"path/filepath"
	"testing"
)

func writeOverrides(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "overrides.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// No decisions have been made yet is not an error.
func TestLoadOverridesToleratesAMissingFile(t *testing.T) {
	o, err := LoadOverrides(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatalf("LoadOverrides: %v", err)
	}
	if len(o.Overrides) != 0 {
		t.Errorf("got %d overrides from a file that does not exist", len(o.Overrides))
	}
}

func TestLoadOverridesRejectsIncompleteEntries(t *testing.T) {
	cases := map[string]string{
		"no tour":                 `{"overrides":[{"duplicate_source_id":"A","canonical_source_id":"1"}]}`,
		"no duplicate":            `{"overrides":[{"tour":"atp","canonical_source_id":"1"}]}`,
		"merge with no canonical": `{"overrides":[{"tour":"atp","duplicate_source_id":"A"}]}`,
		"not json":                `not json at all`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadOverrides(writeOverrides(t, body)); err == nil {
				t.Errorf("accepted %s", body)
			}
		})
	}

	// Recording that two are different people needs no canonical id.
	valid := `{"overrides":[{"tour":"atp","duplicate_source_id":"A","same_person":false}]}`
	if _, err := LoadOverrides(writeOverrides(t, valid)); err != nil {
		t.Errorf("a separation decision was rejected: %v", err)
	}
}

// The committed file has to parse, or the reconcile stage fails on every run.
func TestCommittedOverridesFileParses(t *testing.T) {
	path := filepath.Join("..", "..", "configs", "player_overrides.json")
	if _, err := os.Stat(path); err != nil {
		t.Skip("no overrides file committed")
	}
	if _, err := LoadOverrides(path); err != nil {
		t.Fatalf("configs/player_overrides.json: %v", err)
	}
}

func TestDecisionsForceAMerge(t *testing.T) {
	o, err := LoadOverrides(writeOverrides(t,
		`{"overrides":[{"tour":"atp","duplicate_source_id":"CN01","canonical_source_id":"100001"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	d := o.Index()

	players := []Player{
		{ID: 1, SourceID: "100001", FullName: "Common Name", Country: "USA"},
		{ID: 2, SourceID: "CN01", FullName: "Common Name", Country: "FRA"},
	}
	// Scores 0.40 on its own, below the review floor, so nothing is proposed.
	if len(Reconcile(players)) != 0 {
		t.Fatal("the scorer proposed this pair; the test no longer covers what it means to")
	}

	got := d.Apply("atp", players, Reconcile(players))
	if len(got) != 1 {
		t.Fatalf("got %d matches, want the hand-resolved one", len(got))
	}
	if !got[0].Auto() || got[0].Confidence != 1.0 {
		t.Errorf("a hand-resolved pair should be certain, got %v", got[0].Confidence)
	}
	if got[0].Canonical.ID != 1 || got[0].Duplicate.ID != 2 {
		t.Errorf("canonical=%d duplicate=%d, want 1 and 2", got[0].Canonical.ID, got[0].Duplicate.ID)
	}
}

// Recording that two are different people is as useful as a merge, and stops
// the pair being re-proposed on every run.
func TestDecisionsSeparateAPair(t *testing.T) {
	o, err := LoadOverrides(writeOverrides(t,
		`{"overrides":[{"tour":"atp","duplicate_source_id":"JM01","canonical_source_id":"100001",
		                "same_person":false,"note":"different people"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	d := o.Index()

	born := date(t, "1990-01-01")
	players := []Player{
		{ID: 1, SourceID: "100001", FullName: "Juan Martin", Country: "ARG", BirthDate: born},
		{ID: 2, SourceID: "JM01", FullName: "Juan Martin", Country: "ARG", BirthDate: born},
	}
	proposed := Reconcile(players)
	if len(proposed) != 1 || !proposed[0].Auto() {
		t.Fatalf("expected the scorer to want this merge: %+v", proposed)
	}

	if got := d.Apply("atp", players, proposed); len(got) != 0 {
		t.Errorf("got %+v, want nothing: a human said these are different people", got)
	}
}

// An override naming an id that is not in the database has nothing to merge and
// must not invent one.
func TestDecisionsIgnoreUnknownIDs(t *testing.T) {
	o, err := LoadOverrides(writeOverrides(t,
		`{"overrides":[{"tour":"atp","duplicate_source_id":"NOPE","canonical_source_id":"ALSONOPE"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	players := []Player{{ID: 1, SourceID: "100001", FullName: "Someone Else"}}
	if got := o.Index().Apply("atp", players, nil); len(got) != 0 {
		t.Errorf("got %+v, want nothing", got)
	}
}

// A decision written either way round is the same decision.
func TestDecisionsAreOrderIndependent(t *testing.T) {
	o, err := LoadOverrides(writeOverrides(t,
		`{"overrides":[{"tour":"atp","duplicate_source_id":"A","canonical_source_id":"B","same_person":false}]}`))
	if err != nil {
		t.Fatal(err)
	}
	d := o.Index()
	if !d.Separated("atp", "A", "B") || !d.Separated("atp", "B", "A") {
		t.Error("the decision depends on which id was written first")
	}
	if d.Separated("wta", "A", "B") {
		t.Error("an ATP decision leaked into the WTA tour")
	}
}
