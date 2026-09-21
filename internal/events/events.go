// Package events derives what an event is across seasons, on the terms of
// ADR-0012: a run of tournaments rows keyed by the tour's number where the id
// carries a real one, by name where it does not, and by competition for a
// team tie. The derivation is a pure function of the rows and the overrides
// file, so a reload yields the same events; the store below writes the result
// and keeps the slugs it has already minted.
package events

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sami0076/tennis-wiki/internal/name"
)

// Link says how a row got onto its event. It is provenance the page prints:
// an edition on a page by the tour's number and one there by name are two
// different claims.
type Link string

// The links, in the order the rule tries them.
const (
	LinkOverride Link = "override"
	LinkNumber   Link = "number"
	LinkBridged  Link = "bridged"
	LinkName     Link = "name"
	LinkTeam     Link = "team"
)

// Row is one tournaments row as the resolver reads it.
type Row struct {
	ID        int64
	Tour      string
	SourceID  string
	Name      string
	Level     string
	Tier      string
	Season    int
	StartDate time.Time
}

// Event is one run of editions.
type Event struct {
	Tour string
	// Key is what the run is keyed on: number:404, name:tour:wimbledon,
	// name:itf:w15-monastir:2, team:davis-cup. Unique within a tour.
	Key  string
	Name string
	// Ordinal is which of its name the run is within a season: 2 for the
	// second W15 Monastir of each year. Zero for a numbered or team event and
	// for the first of a name.
	Ordinal int
	// Slug is left empty by Resolve; the store mints it, because the slugs
	// already in the database have to be kept.
	Slug        string
	FirstSeason int
	LastSeason  int
	Editions    []Edition
}

// Edition is one row on its event.
type Edition struct {
	Row  Row
	Link Link
}

// wtaNumbersFrom is the first season a WTA numeric id is the WTA's own number.
// Measured in ADR-0012: a sequence within the year through 1987, then the ITF
// circuit's numbering on the Challenger tier to 1995 -- a space that collides
// with the WTA's, 540 being ITF Indianapolis in 1991 and Wimbledon in 2016 --
// and nothing on the tour tier until Sackmann's 2016 files.
const wtaNumbersFrom = 2016

var (
	numberID = regexp.MustCompile(`^\d{4}-(\d+)(?:-\d{4})?$`)
	mCodeID  = regexp.MustCompile(`^\d{4}-(M\d{3})$`)
	// Sackmann's Challenger suffix and an edition number within a season:
	// "Oeiras 2 CH" and "Oeiras" are one name.
	nameSuffix    = regexp.MustCompile(`(?i)\s+(CH|Challenger|\d+)$`)
	chSuffix      = regexp.MustCompile(`(?i)\s+CH$`)
	editionNumber = regexp.MustCompile(`\s+\d+$`)
	// A competition is the words up to and including "Cup": "Davis Cup WG R1:
	// ESP vs CZE" is a Davis Cup tie.
	competition = regexp.MustCompile(`(?i)^(.*?\bCup)\b`)
)

// competitionAliases folds a renamed team competition onto its current name.
// The Fed Cup became the Billie Jean King Cup in 2020 and the sources write
// it three ways.
var competitionAliases = map[string]string{
	"fed-cup": "billie-jean-king-cup",
	"bjk-cup": "billie-jean-king-cup",
}

// Normalise folds a tournament name to the form two editions are compared
// on: lowercase ASCII, punctuation folded, the Challenger suffix and a
// trailing edition number dropped. A category prefix ("W15 Antalya") stays,
// because the category is part of what the event was.
func Normalise(s string) string {
	s = nameSuffix.ReplaceAllString(strings.TrimSpace(s), "")
	// Twice: "Oeiras 2 CH" loses the suffix and then the number.
	s = nameSuffix.ReplaceAllString(s, "")
	return name.Slug(s)
}

// Competition names the team competition a tie belongs to.
func Competition(s string) string {
	if i := strings.IndexByte(s, ':'); i >= 0 {
		s = s[:i]
	}
	if m := competition.FindStringSubmatch(s); m != nil {
		s = m[1]
	}
	c := name.Slug(s)
	if alias, ok := competitionAliases[c]; ok {
		return alias
	}
	return c
}

// rawKey is the part of an id that could identify the event, and its kind.
type rawKey struct {
	kind  string // "number", "code" or ""
	value string
}

func rawKeyOf(r Row) rawKey {
	if m := numberID.FindStringSubmatch(r.SourceID); m != nil {
		if r.Tour == "atp" || r.Season >= wtaNumbersFrom {
			return rawKey{"number", trimZeros(m[1])}
		}
		return rawKey{}
	}
	if m := mCodeID.FindStringSubmatch(r.SourceID); m != nil {
		return rawKey{"code", m[1]}
	}
	return rawKey{}
}

// NumberKey is the event key for a tour's number.
func NumberKey(n string) string { return "number:" + n }

// nameKey is the event key for a run joined by name alone. The same name
// more than once in a season -- the weekly ITF events, "Adelaide 1" and
// "Adelaide 2" -- is a run per ordinal, the first bare.
func nameKey(tier, norm string, ordinal int) string {
	key := "name:" + tier + ":" + norm
	if ordinal > 1 {
		key += ":" + strconv.Itoa(ordinal)
	}
	return key
}

// teamKey is the event key for a team competition.
func teamKey(comp string) string { return "team:" + comp }

// pending is a row waiting on the name rule.
type pending struct {
	row  Row
	norm string
}

// Result is what Resolve derived, with the figures the run reports.
type Result struct {
	Events []Event
	// ByLink counts rows by how they were placed.
	ByLink map[Link]int
	// Ambiguous are the name-keyed runs that stayed apart because the name
	// sits under more than one number in the same tour and tier.
	Ambiguous []string
	// Numbered counts the rows that are the second or later of their name in
	// a season.
	Numbered int
	// Unmatched are the overrides that filed no row.
	Unmatched []Override
}

// Resolve derives the events from every tournaments row. The rule per row, in
// order: an override; the tour's number; the name within tour and tier,
// bridged to a numbered event when exactly one carries that name; and a team
// tie to its competition. A name that recurs within a season is numbered in
// calendar order, and only the first of it bridges.
func Resolve(rows []Row, overrides *Overrides) Result {
	if overrides == nil {
		overrides = &Overrides{}
	}
	index := overrides.index()
	res := Result{ByLink: map[Link]int{}}

	// Numbered events first, so the name-keyed runs can see which numbers
	// carry which names.
	assigned := map[int64]Edition{}
	keyOf := map[int64]string{}
	// namesUnder[tour][tier][norm] = set of number keys with an edition of that name.
	namesUnder := map[string]map[string]map[string]map[string]struct{}{}
	noteName := func(tour, tier, norm, key string) {
		byTier, ok := namesUnder[tour]
		if !ok {
			byTier = map[string]map[string]map[string]struct{}{}
			namesUnder[tour] = byTier
		}
		byNorm, ok := byTier[tier]
		if !ok {
			byNorm = map[string]map[string]struct{}{}
			byTier[tier] = byNorm
		}
		keys, ok := byNorm[norm]
		if !ok {
			keys = map[string]struct{}{}
			byNorm[norm] = keys
		}
		keys[key] = struct{}{}
	}

	var named []pending
	for _, r := range rows {
		if r.Level == "D" {
			keyOf[r.ID] = teamKey(Competition(r.Name))
			assigned[r.ID] = Edition{Row: r, Link: LinkTeam}
			continue
		}
		raw := rawKeyOf(r)
		if to, ok := index.target(r, raw); ok {
			keyOf[r.ID] = NumberKey(to)
			assigned[r.ID] = Edition{Row: r, Link: LinkOverride}
			noteName(r.Tour, r.Tier, Normalise(r.Name), keyOf[r.ID])
			continue
		}
		if raw.kind == "number" {
			keyOf[r.ID] = NumberKey(raw.value)
			assigned[r.ID] = Edition{Row: r, Link: LinkNumber}
			noteName(r.Tour, r.Tier, Normalise(r.Name), keyOf[r.ID])
			continue
		}
		named = append(named, pending{row: r, norm: Normalise(r.Name)})
	}

	ordinals := ordinalsWithinSeason(named)
	ambiguous := map[string]struct{}{}
	for i, p := range named {
		r := p.row
		if ordinals[i] > 1 {
			res.Numbered++
			keyOf[r.ID] = nameKey(r.Tier, p.norm, ordinals[i])
			assigned[r.ID] = Edition{Row: r, Link: LinkName}
			continue
		}
		candidates := namesUnder[r.Tour][r.Tier][p.norm]
		if len(candidates) == 1 {
			for k := range candidates {
				keyOf[r.ID] = k
			}
			assigned[r.ID] = Edition{Row: r, Link: LinkBridged}
			continue
		}
		if len(candidates) > 1 {
			ambiguous[r.Tour+" "+r.Tier+" "+p.norm] = struct{}{}
		}
		keyOf[r.ID] = nameKey(r.Tier, p.norm, 1)
		assigned[r.ID] = Edition{Row: r, Link: LinkName}
	}
	for k := range ambiguous {
		res.Ambiguous = append(res.Ambiguous, k)
	}
	sort.Strings(res.Ambiguous)

	byEvent := map[string]*Event{}
	for id, ed := range assigned {
		res.ByLink[ed.Link]++
		k := ed.Row.Tour + "\x00" + keyOf[id]
		ev, ok := byEvent[k]
		if !ok {
			ev = &Event{Tour: ed.Row.Tour, Key: keyOf[id]}
			byEvent[k] = ev
		}
		ev.Editions = append(ev.Editions, ed)
	}

	events := make([]Event, 0, len(byEvent))
	for _, ev := range byEvent {
		sort.Slice(ev.Editions, func(i, j int) bool {
			a, b := ev.Editions[i].Row, ev.Editions[j].Row
			if a.Season != b.Season {
				return a.Season < b.Season
			}
			if !a.StartDate.Equal(b.StartDate) {
				return a.StartDate.Before(b.StartDate)
			}
			return a.ID < b.ID
		})
		ev.FirstSeason = ev.Editions[0].Row.Season
		ev.LastSeason = ev.Editions[len(ev.Editions)-1].Row.Season
		ev.Ordinal = ordinalOf(ev.Key)
		ev.Name = index.pinnedName(ev.Tour, ev.Key)
		if ev.Name == "" {
			ev.Name = displayName(ev.Editions, ev.Ordinal)
		}
		events = append(events, *ev)
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].Tour != events[j].Tour {
			return events[i].Tour < events[j].Tour
		}
		return events[i].Key < events[j].Key
	})
	res.Events = events
	res.Unmatched = index.unmatched()
	return res
}

// displayName is the most recent edition's name, with the Challenger suffix
// Sackmann added and Tennismylife does not, so the switch of source is not a
// rename. Team ties name the tie; the event is the competition. A run keyed
// by name carries its ordinal the way the sources write one -- "Adelaide 2"
// -- whether or not the row did.
func displayName(editions []Edition, ordinal int) string {
	last := editions[len(editions)-1]
	if last.Link == LinkTeam {
		return titleCompetition(last.Row.Name)
	}
	n := strings.TrimSpace(chSuffix.ReplaceAllString(strings.TrimSpace(last.Row.Name), ""))
	if last.Link != LinkName {
		return n
	}
	n = strings.TrimSpace(editionNumber.ReplaceAllString(n, ""))
	if ordinal > 1 {
		n += " " + strconv.Itoa(ordinal)
	}
	return n
}

// ordinalOf reads the ordinal back off a name key.
func ordinalOf(key string) int {
	parts := strings.Split(key, ":")
	if parts[0] != "name" || len(parts) < 4 {
		return 0
	}
	n, err := strconv.Atoi(parts[3])
	if err != nil {
		return 0
	}
	return n
}

// ordinalsWithinSeason numbers the rows that share a name, tour, tier and
// season, in the order they were played. Calendar order rather than the
// source's own number, which agrees 94% of the time and otherwise counts
// backwards.
func ordinalsWithinSeason(named []pending) []int {
	type group struct {
		tour, tier, norm string
		season           int
	}
	byGroup := map[group][]int{}
	for i, p := range named {
		g := group{p.row.Tour, p.row.Tier, p.norm, p.row.Season}
		byGroup[g] = append(byGroup[g], i)
	}
	ordinals := make([]int, len(named))
	for _, idx := range byGroup {
		sort.Slice(idx, func(a, b int) bool {
			x, y := named[idx[a]].row, named[idx[b]].row
			if !x.StartDate.Equal(y.StartDate) {
				return x.StartDate.Before(y.StartDate)
			}
			if x.SourceID != y.SourceID {
				return x.SourceID < y.SourceID
			}
			return x.ID < y.ID
		})
		for k, i := range idx {
			ordinals[i] = k + 1
		}
	}
	return ordinals
}

// titleCompetition is the competition as the source spells it, before any
// stage or tie: "Davis Cup", "Billie Jean King Cup".
func titleCompetition(tieName string) string {
	s := tieName
	if i := strings.IndexByte(s, ':'); i >= 0 {
		s = s[:i]
	}
	if m := competition.FindStringSubmatch(s); m != nil {
		s = m[1]
	}
	switch name.Slug(s) {
	case "fed-cup", "bjk-cup":
		return "Billie Jean King Cup"
	}
	return strings.TrimSpace(s)
}
