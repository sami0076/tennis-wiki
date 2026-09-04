package ingest

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// profile names the column layout of a source file. Layouts differ in column
// order, so every field is read by header name and never by index.
type profile struct {
	// statPrefixes maps the winner and loser stat column prefixes.
	winnerStat string
	loserStat  string
	// hasIndoor marks a layout carrying the extra indoor column.
	hasIndoor bool
	// alphanumericIDs marks player ids that are not numeric, which is true of
	// the official ATP ids and forces reconciliation against Sackmann ids.
	alphanumericIDs bool
}

var profiles = map[string]profile{
	// The 49-column layout from the Sackmann repositories, verified against
	// real ATP and WTA headers.
	"sackmann": {winnerStat: "w_", loserStat: "l_"},
	// Same fields, different column order, plus indoor. Player ids look like
	// "B0BI" rather than "104925".
	"tml": {winnerStat: "w_", loserStat: "l_", hasIndoor: true, alphanumericIDs: true},
}

// requiredColumns must be present in any layout; a missing one is a layout we
// do not understand, and guessing would silently mis-assign every field.
var requiredColumns = []string{
	"tourney_id", "tourney_name", "tourney_date", "tourney_level",
	"match_num", "round", "best_of", "score",
	"winner_id", "winner_name", "loser_id", "loser_name",
}

// columns resolves header names to positions for one file.
type columns struct {
	idx     map[string]int
	profile profile
	name    string
}

func newColumns(profileName string, header []string) (*columns, error) {
	p, ok := profiles[profileName]
	if !ok {
		return nil, fmt.Errorf("unknown schema profile %q", profileName)
	}

	idx := make(map[string]int, len(header))
	for i, h := range header {
		idx[strings.TrimSpace(strings.TrimPrefix(h, "\ufeff"))] = i
	}

	var missing []string
	for _, want := range requiredColumns {
		if _, ok := idx[want]; !ok {
			missing = append(missing, want)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("profile %q: header is missing %s",
			profileName, strings.Join(missing, ", "))
	}

	return &columns{idx: idx, profile: p, name: profileName}, nil
}

// get returns a trimmed field, or "" when the column is absent from this layout.
func (c *columns) get(rec []string, name string) string {
	i, ok := c.idx[name]
	if !ok || i >= len(rec) {
		return ""
	}
	return cleanText(rec[i])
}

// cleanText trims a field and guarantees it is valid UTF-8.
//
// Some of the WTA files are not UTF-8 -- they carry stray bytes such as a bare
// 0xC2 in player names -- and Postgres refuses the whole statement with
// "invalid byte sequence for encoding UTF8". One such byte used to abort an
// ingest of 1.6 million rows.
//
// The bad bytes are dropped rather than transcoded: the sources are
// inconsistent about which encoding they meant, guessing would silently corrupt
// names, and a name missing a character still slugs and searches correctly
// because diacritics are folded anyway.
func cleanText(s string) string {
	s = strings.TrimSpace(s)
	if utf8.ValidString(s) {
		return s
	}
	return strings.ToValidUTF8(s, "")
}

// has reports whether the layout carries a column at all, which is different
// from the column being present but empty.
func (c *columns) has(name string) bool {
	_, ok := c.idx[name]
	return ok
}
