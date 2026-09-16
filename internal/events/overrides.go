package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
)

// Override is a person's decision about a number, checked into the repository
// so it survives a rebuild and is reviewable like any other change. It says
// which rows it applies to — one number, one of Sackmann's M-codes, or one
// exact id — and either the number the event is filed under, a display name
// to pin, or both. An override is for a number that changed, never for a
// name: a run joined by name is the rule's job, and the page says so.
type Override struct {
	Tour string `json:"tour"`
	// Exactly one of these names the rows. SourceIDs is for a run that has
	// no number at all and is one event by a person's reading, such as the
	// WTA's year-end championships before 2014.
	Number    string   `json:"number,omitempty"`
	Code      string   `json:"code,omitempty"`
	SourceID  string   `json:"source_id,omitempty"`
	SourceIDs []string `json:"source_ids,omitempty"`
	// To is the number the rows are filed under.
	To string `json:"to,omitempty"`
	// Name pins the event's display name where the latest edition's would
	// mislead: the WTA's Canadian event alternates Montreal and Toronto.
	Name string `json:"name,omitempty"`
	Note string `json:"note,omitempty"`
}

// Overrides is the decisions file.
type Overrides struct {
	Overrides []Override `json:"overrides"`
}

var (
	digits = regexp.MustCompile(`^\d+$`)
	mCode  = regexp.MustCompile(`^M\d{3}$`)
)

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
		if err := ov.validate(); err != nil {
			return nil, fmt.Errorf("%s: override %d: %w", path, i, err)
		}
	}
	return &o, nil
}

func (o Override) validate() error {
	if o.Tour != "atp" && o.Tour != "wta" {
		return fmt.Errorf("tour %q is not atp or wta", o.Tour)
	}
	selectors := 0
	for _, s := range []string{o.Number, o.Code, o.SourceID} {
		if s != "" {
			selectors++
		}
	}
	if len(o.SourceIDs) > 0 {
		selectors++
	}
	if selectors != 1 {
		return errors.New("needs exactly one of number, code, source_id or source_ids")
	}
	if len(o.SourceIDs) > 0 && o.To == "" {
		return errors.New("source_ids needs a number to file under (to)")
	}
	if o.Number != "" && !digits.MatchString(o.Number) {
		return fmt.Errorf("number %q is not a number", o.Number)
	}
	if o.Code != "" && !mCode.MatchString(o.Code) {
		return fmt.Errorf("code %q is not an M-code like M006", o.Code)
	}
	if o.To == "" && o.Name == "" {
		return errors.New("needs a number to file under (to) or a name to pin, or both")
	}
	if o.To != "" && !digits.MatchString(o.To) {
		return fmt.Errorf("to %q is not a number", o.To)
	}
	if o.Name != "" && o.Number == "" {
		return errors.New("a name can only be pinned on a number")
	}
	return nil
}

// index prepares the overrides for lookup and counts what each one matched,
// so a decision a source has since undone is reported rather than kept.
type index struct {
	bySourceID map[string]int // tour\x00source_id -> override position
	byNumber   map[string]int
	byCode     map[string]int
	names      map[string]string // tour\x00event key -> pinned name
	hits       []int
	overrides  []Override
}

func (o *Overrides) index() *index {
	ix := &index{
		bySourceID: map[string]int{}, byNumber: map[string]int{}, byCode: map[string]int{},
		names: map[string]string{}, hits: make([]int, len(o.Overrides)), overrides: o.Overrides,
	}
	for i, ov := range o.Overrides {
		switch {
		case len(ov.SourceIDs) > 0:
			for _, id := range ov.SourceIDs {
				ix.bySourceID[ov.Tour+"\x00"+id] = i
			}
		case ov.SourceID != "":
			ix.bySourceID[ov.Tour+"\x00"+ov.SourceID] = i
		case ov.Code != "":
			ix.byCode[ov.Tour+"\x00"+ov.Code] = i
		case ov.Number != "":
			if ov.To != "" {
				ix.byNumber[ov.Tour+"\x00"+trimZeros(ov.Number)] = i
			}
			if ov.Name != "" {
				ix.names[ov.Tour+"\x00"+NumberKey(trimZeros(ov.Number))] = ov.Name
			}
		}
	}
	return ix
}

// target is the number a row is filed under by override, if any.
func (ix *index) target(r Row, raw rawKey) (string, bool) {
	if i, ok := ix.bySourceID[r.Tour+"\x00"+r.SourceID]; ok {
		return ix.hit(i)
	}
	switch raw.kind {
	case "number":
		if i, ok := ix.byNumber[r.Tour+"\x00"+raw.value]; ok {
			return ix.hit(i)
		}
	case "code":
		if i, ok := ix.byCode[r.Tour+"\x00"+raw.value]; ok {
			return ix.hit(i)
		}
	}
	return "", false
}

func (ix *index) hit(i int) (string, bool) {
	ix.hits[i]++
	if ix.overrides[i].To == "" {
		return "", false
	}
	return trimZeros(ix.overrides[i].To), true
}

func (ix *index) pinnedName(tour, key string) string {
	return ix.names[tour+"\x00"+key]
}

// unmatched lists the overrides that filed no row.
func (ix *index) unmatched() []Override {
	var out []Override
	for i, ov := range ix.overrides {
		if ov.To != "" && ix.hits[i] == 0 {
			out = append(out, ov)
		}
	}
	return out
}

func trimZeros(n string) string {
	for len(n) > 1 && n[0] == '0' {
		n = n[1:]
	}
	return n
}
