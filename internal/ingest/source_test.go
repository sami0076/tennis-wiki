package ingest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The registry shipped in configs/ must stay valid: a typo there breaks every
// ingest run, and it is edited by hand whenever a mirror changes.
func TestShippedRegistryIsValid(t *testing.T) {
	r, err := LoadRegistry(filepath.Join("..", "..", "configs", "sources.json"))
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if len(r.Sources) < 6 {
		t.Errorf("got %d sources, expected the full set", len(r.Sources))
	}

	var atp, wta bool
	tiers := map[string]bool{}
	for _, s := range r.Sources {
		switch s.Tour {
		case TourATP:
			atp = true
		case TourWTA:
			wta = true
		}
		tiers[s.Tier] = true
		if s.Attribution == "" {
			t.Errorf("source %q has no attribution; the data licence requires one", s.Name)
		}
	}
	if !atp || !wta {
		t.Error("registry must cover both tours")
	}
	for _, want := range []string{"tour", "challenger", "futures", "itf"} {
		if !tiers[want] {
			t.Errorf("registry has no %s source", want)
		}
	}
}

// Where two sources cover the same season, the fresher one must win.
func TestPrecedenceResolvesOverlap(t *testing.T) {
	r := &Registry{Sources: []Source{
		{Name: "old", Tour: TourATP, Profile: "sackmann", BaseURL: "https://x", Path: "a_{season}.csv",
			FirstSeason: 1968, LastSeason: 2025, Precedence: 50},
		{Name: "fresh", Tour: TourATP, Profile: "tml", BaseURL: "https://y", Path: "{season}.csv",
			FirstSeason: 2025, LastSeason: 2026, Precedence: 60},
	}}

	got := r.For(TourATP, 2025)
	if len(got) != 2 {
		t.Fatalf("got %d sources for 2025, want 2", len(got))
	}
	if got[0].Name != "fresh" {
		t.Errorf("first source = %q, want the higher-precedence %q", got[0].Name, "fresh")
	}

	if only := r.For(TourATP, 1990); len(only) != 1 || only[0].Name != "old" {
		t.Errorf("1990 sources = %v, want just the old one", only)
	}
	if none := r.For(TourWTA, 2000); len(none) != 0 {
		t.Errorf("WTA sources = %v, want none", none)
	}
}

func TestCoverageAndURLs(t *testing.T) {
	s := Source{
		Name: "x", Tour: TourATP, Profile: "sackmann",
		BaseURL: "https://example.test/repo/", Path: "results/atp_matches_{season}.csv",
		FirstSeason: 1968, LastSeason: 2024,
	}
	if !s.Covers(1968) || !s.Covers(2024) {
		t.Error("boundary seasons should be covered")
	}
	if s.Covers(1967) || s.Covers(2025) {
		t.Error("seasons outside the window should not be covered")
	}
	want := "https://example.test/repo/results/atp_matches_2024.csv"
	if got := s.URL(2024); got != want {
		t.Errorf("URL = %q, want %q", got, want)
	}
	if got := s.RelPath(1999); got != "results/atp_matches_1999.csv" {
		t.Errorf("RelPath = %q", got)
	}
}

// A final season is often partial, so coverage is expressed as a date.
func TestThroughDate(t *testing.T) {
	s := Source{Through: "2026-01-17"}
	d, err := s.ThroughDate()
	if err != nil {
		t.Fatalf("ThroughDate: %v", err)
	}
	if d.Year() != 2026 || d.Month() != 1 || d.Day() != 17 {
		t.Errorf("ThroughDate = %v", d)
	}

	if d, err := (Source{}).ThroughDate(); err != nil || !d.IsZero() {
		t.Errorf("empty Through should give the zero time, got %v %v", d, err)
	}
	if _, err := (Source{Name: "b", Through: "17-01-2026"}).ThroughDate(); err == nil {
		t.Error("a malformed through date should fail")
	}
}

func TestRegistryValidation(t *testing.T) {
	bad := map[string]Registry{
		"empty":            {},
		"no name":          {Sources: []Source{{Tour: TourATP, Profile: "sackmann", BaseURL: "u", Path: "{season}"}}},
		"bad tour":         {Sources: []Source{{Name: "a", Tour: "itf", Profile: "sackmann", BaseURL: "u", Path: "{season}"}}},
		"unknown profile":  {Sources: []Source{{Name: "a", Tour: TourATP, Profile: "nope", BaseURL: "u", Path: "{season}"}}},
		"no season token":  {Sources: []Source{{Name: "a", Tour: TourATP, Profile: "sackmann", BaseURL: "u", Path: "fixed.csv"}}},
		"reversed seasons": {Sources: []Source{{Name: "a", Tour: TourATP, Profile: "sackmann", BaseURL: "u", Path: "{season}", FirstSeason: 2020, LastSeason: 1990}}},
		"duplicate names": {Sources: []Source{
			{Name: "a", Tour: TourATP, Profile: "sackmann", BaseURL: "u", Path: "{season}"},
			{Name: "a", Tour: TourWTA, Profile: "sackmann", BaseURL: "u", Path: "{season}"},
		}},
	}
	for name, r := range bad {
		reg := r
		if err := reg.validate(); err == nil {
			t.Errorf("%s: validate() should have failed", name)
		}
	}
}

func TestLoadRegistryErrors(t *testing.T) {
	if _, err := LoadRegistry(filepath.Join("testdata", "does-not-exist.json")); err == nil {
		t.Error("missing file should fail")
	}

	tmp := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(tmp, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRegistry(tmp); err == nil {
		t.Error("malformed JSON should fail")
	}
}

func TestAttributions(t *testing.T) {
	r := &Registry{Sources: []Source{
		{Attribution: "B"}, {Attribution: "A"}, {Attribution: "A"}, {Attribution: ""},
	}}
	got := r.Attributions()
	if len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Errorf("Attributions() = %v, want [A B]", got)
	}
}

func TestSeasons(t *testing.T) {
	r := &Registry{Sources: []Source{
		{FirstSeason: 2000, LastSeason: 2002},
		{FirstSeason: 2001, LastSeason: 2003},
	}}
	got := r.Seasons()
	want := []int{2000, 2001, 2002, 2003}
	if len(got) != len(want) {
		t.Fatalf("Seasons() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Seasons()[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

// Every profile the shipped registry names must exist, or ingest fails at run
// time rather than at load time.
func TestShippedProfilesExist(t *testing.T) {
	r, err := LoadRegistry(filepath.Join("..", "..", "configs", "sources.json"))
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	for _, s := range r.Sources {
		if _, ok := profiles[s.Profile]; !ok {
			t.Errorf("source %q names unknown profile %q", s.Name, s.Profile)
		}
		if !strings.HasPrefix(s.BaseURL, "https://") {
			t.Errorf("source %q base_url is not https", s.Name)
		}
	}
}
