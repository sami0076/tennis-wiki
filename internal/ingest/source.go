package ingest

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Tour is the governing body of a source's matches.
type Tour string

// The two tours this project covers.
const (
	TourATP Tour = "atp"
	TourWTA Tour = "wta"
)

// Source is one file family from one mirror. Sources are configuration rather
// than code because the project's original upstream repositories were removed
// mid-build; see ADR-0002.
type Source struct {
	Name    string `json:"name"`
	Tour    Tour   `json:"tour"`
	Tier    string `json:"tier"`
	Profile string `json:"profile"`
	BaseURL string `json:"base_url"`
	// Path is a template containing {season}, e.g. "results/atp_matches_{season}.csv".
	Path        string `json:"path"`
	FirstSeason int    `json:"first_season"`
	LastSeason  int    `json:"last_season"`
	// Through marks a final season that is incomplete, as YYYY-MM-DD. Coverage
	// has to be a date rather than a year: the deepest mirror stops mid-January.
	Through string `json:"through,omitempty"`
	// Precedence breaks ties where sources overlap; higher wins.
	Precedence  int    `json:"precedence"`
	Attribution string `json:"attribution"`
	// Qualifying says whether this family bundles qualifying draws in with the
	// main draw, as atp_matches_qual_chall does.
	Qualifying bool `json:"bundles_qualifying,omitempty"`
}

// Covers reports whether the source carries the given season.
func (s Source) Covers(season int) bool {
	return season >= s.FirstSeason && season <= s.LastSeason
}

// ThroughDate parses Through, returning the zero time when the final season is
// complete.
func (s Source) ThroughDate() (time.Time, error) {
	if s.Through == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse("2006-01-02", s.Through)
	if err != nil {
		return time.Time{}, fmt.Errorf("source %q: bad through date %q: %w", s.Name, s.Through, err)
	}
	return t, nil
}

// URL returns the location of the season's file.
func (s Source) URL(season int) string {
	return strings.TrimSuffix(s.BaseURL, "/") + "/" + s.RelPath(season)
}

// RelPath returns the season's path relative to the source root, which is also
// its location under a local clone.
func (s Source) RelPath(season int) string {
	return strings.ReplaceAll(s.Path, "{season}", strconv.Itoa(season))
}

// Registry is the configured set of sources.
type Registry struct {
	Sources []Source `json:"sources"`
	// Reference holds the player tables and ranking history, which are not
	// seasonal and so do not fit Source.
	Reference []RefSource `json:"reference,omitempty"`
}

// ReferenceFor returns the reference sources of one kind.
func (r *Registry) ReferenceFor(kind RefKind) []RefSource {
	var out []RefSource
	for _, s := range r.Reference {
		if s.Kind == kind {
			out = append(out, s)
		}
	}
	return out
}

// LoadRegistry reads a registry from a JSON file.
func LoadRegistry(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read source registry: %w", err)
	}
	var r Registry
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse source registry %s: %w", path, err)
	}
	if err := r.validate(); err != nil {
		return nil, err
	}
	return &r, nil
}

func (r *Registry) validate() error {
	if len(r.Sources) == 0 {
		return fmt.Errorf("source registry is empty")
	}
	for _, ref := range r.Reference {
		if err := ref.validate(); err != nil {
			return err
		}
	}
	seen := make(map[string]struct{}, len(r.Sources))
	for _, s := range r.Sources {
		switch {
		case s.Name == "":
			return fmt.Errorf("source with no name")
		case s.Tour != TourATP && s.Tour != TourWTA:
			return fmt.Errorf("source %q: unknown tour %q", s.Name, s.Tour)
		case s.Profile == "":
			return fmt.Errorf("source %q: no profile", s.Name)
		case s.BaseURL == "" || s.Path == "":
			return fmt.Errorf("source %q: base_url and path are both required", s.Name)
		case !strings.Contains(s.Path, "{season}"):
			return fmt.Errorf("source %q: path must contain {season}", s.Name)
		case s.FirstSeason > s.LastSeason:
			return fmt.Errorf("source %q: first_season after last_season", s.Name)
		}
		if _, err := s.ThroughDate(); err != nil {
			return err
		}
		if _, ok := profiles[s.Profile]; !ok {
			return fmt.Errorf("source %q: unknown profile %q", s.Name, s.Profile)
		}
		if _, dup := seen[s.Name]; dup {
			return fmt.Errorf("duplicate source name %q", s.Name)
		}
		seen[s.Name] = struct{}{}
	}
	return nil
}

// For returns every source covering the tour and season, highest precedence
// first. Ties break on name so the order is stable across runs.
func (r *Registry) For(tour Tour, season int) []Source {
	var out []Source
	for _, s := range r.Sources {
		if s.Tour == tour && s.Covers(season) {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Precedence != out[j].Precedence {
			return out[i].Precedence > out[j].Precedence
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Seasons returns every season any source covers, ascending.
func (r *Registry) Seasons() []int {
	seen := map[int]struct{}{}
	for _, s := range r.Sources {
		for y := s.FirstSeason; y <= s.LastSeason; y++ {
			seen[y] = struct{}{}
		}
	}
	out := make([]int, 0, len(seen))
	for y := range seen {
		out = append(out, y)
	}
	sort.Ints(out)
	return out
}

// Attributions returns the distinct attribution strings, for the footer and
// the methodology page.
func (r *Registry) Attributions() []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range r.Sources {
		if s.Attribution == "" {
			continue
		}
		if _, ok := seen[s.Attribution]; !ok {
			seen[s.Attribution] = struct{}{}
			out = append(out, s.Attribution)
		}
	}
	sort.Strings(out)
	return out
}
