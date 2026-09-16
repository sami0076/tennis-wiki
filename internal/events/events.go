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
	// team:davis-cup. Unique within a tour.
	Key  string
	Name string
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

// wtaNumbersFrom is the first season a WTA numeric id is the WTA's own number
// rather than a sequence within the year. Measured in ADR-0012: contiguous
// blocks through 1983, a trailing handful of exhibitions numbered the same way
// to 1987, real ids after.
const wtaNumbersFrom = 1988

var (
	numberID = regexp.MustCompile(`^\d{4}-(\d+)(?:-\d{4})?$`)
	mCodeID  = regexp.MustCompile(`^\d{4}-(M\d{3})$`)
	// Sackmann's Challenger suffix and an edition number within a season:
	// "Oeiras 2 CH" and "Oeiras" are one name.
	nameSuffix = regexp.MustCompile(`(?i)\s+(CH|Challenger|\d+)$`)
	chSuffix   = regexp.MustCompile(`(?i)\s+CH$`)
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

// nameKey is the event key for a run joined by name alone.
func nameKey(tier, norm string) string { return "name:" + tier + ":" + norm }

// teamKey is the event key for a team competition.
func teamKey(comp string) string { return "team:" + comp }

// Result is what Resolve derived, with the figures the run reports.
type Result struct {
	Events []Event
	// ByLink counts rows by how they were placed.
	ByLink map[Link]int
	// Ambiguous are the name-keyed runs that stayed apart because the name
	// sits under more than one number in the same tour and tier.
	Ambiguous []string
	// Unmatched are the overrides that filed no row.
	Unmatched []Override
}

// Resolve derives the events from every tournaments row. The rule per row, in
// order: an override; the tour's number; the name within tour and tier,
// bridged to a numbered event when exactly one carries that name; and a team
// tie to its competition.
func Resolve(rows []Row, overrides *Overrides) Result {
	if overrides == nil {
		overrides = &Overrides{}
	}
	index := overrides.index()
	res := Result{ByLink: map[Link]int{}}

	type pending struct {
		row  Row
		norm string
	}
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

	ambiguous := map[string]struct{}{}
	for _, p := range named {
		r := p.row
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
		keyOf[r.ID] = nameKey(r.Tier, p.norm)
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
		ev.Name = index.pinnedName(ev.Tour, ev.Key)
		if ev.Name == "" {
			ev.Name = displayName(ev.Editions)
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
// rename. Team ties name the tie; the event is the competition.
func displayName(editions []Edition) string {
	last := editions[len(editions)-1]
	if last.Link == LinkTeam {
		return titleCompetition(last.Row.Name)
	}
	return strings.TrimSpace(chSuffix.ReplaceAllString(strings.TrimSpace(last.Row.Name), ""))
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
