package identity

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Override is a human's decision about one pair, checked into the repository so
// it survives a rebuild and is reviewable like any other change.
type Override struct {
	Tour string `json:"tour"`
	// DuplicateSourceID is the id being folded away, and CanonicalSourceID the
	// one being kept.
	DuplicateSourceID string `json:"duplicate_source_id"`
	CanonicalSourceID string `json:"canonical_source_id"`
	// SamePerson defaults to true. Setting it false records that two rows which
	// look alike are different people, which is as useful a decision as a merge
	// and stops them being re-proposed on every run.
	SamePerson *bool  `json:"same_person,omitempty"`
	Note       string `json:"note,omitempty"`
}

// Merges reports whether this override links the pair or separates it.
func (o Override) Merges() bool { return o.SamePerson == nil || *o.SamePerson }

// Overrides is the decisions file.
type Overrides struct {
	Overrides []Override `json:"overrides"`
}

// LoadOverrides reads the decisions file. A missing file is not an error: no
// decisions have been made yet.
func LoadOverrides(path string) (*Overrides, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Overrides{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var o Overrides
	if err := json.Unmarshal(data, &o); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for i, ov := range o.Overrides {
		switch {
		case ov.Tour == "":
			return nil, fmt.Errorf("%s: override %d has no tour", path, i)
		case ov.DuplicateSourceID == "":
			return nil, fmt.Errorf("%s: override %d has no duplicate_source_id", path, i)
		case ov.CanonicalSourceID == "" && ov.Merges():
			return nil, fmt.Errorf("%s: override %d merges but names no canonical_source_id", path, i)
		}
	}
	return &o, nil
}

// key identifies a pair regardless of which way round it was written.
func key(tour, a, b string) string {
	if a > b {
		a, b = b, a
	}
	return strings.ToLower(tour) + "\x00" + a + "\x00" + b
}

// Decisions indexes overrides for lookup during reconciliation.
type Decisions struct {
	merge    map[string]Override
	separate map[string]struct{}
}

// Index prepares the overrides for lookup.
func (o *Overrides) Index() *Decisions {
	d := &Decisions{merge: map[string]Override{}, separate: map[string]struct{}{}}
	for _, ov := range o.Overrides {
		k := key(ov.Tour, ov.DuplicateSourceID, ov.CanonicalSourceID)
		if ov.Merges() {
			d.merge[k] = ov
		} else {
			d.separate[k] = struct{}{}
		}
	}
	return d
}

// Separated reports whether a human has said these two are different people.
func (d *Decisions) Separated(tour, a, b string) bool {
	_, ok := d.separate[key(tour, a, b)]
	return ok
}

// Merged reports whether a human has said these two are the same person.
func (d *Decisions) Merged(tour, a, b string) bool {
	_, ok := d.merge[key(tour, a, b)]
	return ok
}

// Apply folds human decisions into the proposed matches: a decided pair is
// forced to full confidence or dropped, and a merge nobody proposed is added.
func (d *Decisions) Apply(tour string, players []Player, matches []Match) []Match {
	bySourceID := make(map[string]Player, len(players))
	for _, p := range players {
		bySourceID[p.SourceID] = p
	}

	out := make([]Match, 0, len(matches))
	seen := map[string]struct{}{}
	for _, m := range matches {
		k := key(tour, m.Duplicate.SourceID, m.Canonical.SourceID)
		if d.Separated(tour, m.Duplicate.SourceID, m.Canonical.SourceID) {
			continue // a human has already said no
		}
		if _, ok := d.merge[k]; ok {
			m.Confidence, m.Reason = 1.0, "resolved by hand in the overrides file"
		}
		seen[k] = struct{}{}
		out = append(out, m)
	}

	// A decision can also name a pair scoring too low to be proposed at all,
	// which is most of the point of having the file.
	for k, ov := range d.merge {
		if _, done := seen[k]; done || !strings.HasPrefix(k, strings.ToLower(tour)+"\x00") {
			continue
		}
		dup, okDup := bySourceID[ov.DuplicateSourceID]
		canonical, okCanonical := bySourceID[ov.CanonicalSourceID]
		if !okDup || !okCanonical {
			continue // one side is not in the database, so there is nothing to merge
		}
		out = append(out, Match{
			Canonical:  canonical,
			Duplicate:  dup,
			Confidence: 1.0,
			Reason:     "resolved by hand in the overrides file",
		})
	}
	return out
}
