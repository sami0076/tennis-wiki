// Package identity reconciles the same human appearing under different source
// ids.
//
// The ATP data for 2025 onward uses official ATP alphanumeric player ids
// (B0BI, MU94); everything through 2024 uses Sackmann numeric ids. They do not
// join. Left alone, Carlos Alcaraz is two players either side of 2025, every
// career total is wrong, and every rating is computed over half a career.
//
// The governing asymmetry: a wrong merge is far worse than a missed one. An
// unmerged pair is visible — two thin player pages where there should be one.
// A wrong merge is invisible, and produces totals that look entirely plausible.
// So matching is deliberately conservative, and anything short of convincing
// goes to a review queue instead of being guessed at.
package identity

import (
	"fmt"
	"strings"
	"time"

	"github.com/sami0076/tennis-wiki/internal/name"
)

// Confidence thresholds.
const (
	// AutoLink is the confidence at or above which a pair is merged without
	// asking. Reached only by a name match plus a date of birth.
	AutoLink = 0.90
	// ReviewFloor is the confidence below which a pair is not even worth
	// showing a human.
	ReviewFloor = 0.50
)

// Player is the subset of a player row reconciliation needs.
type Player struct {
	ID        int64
	SourceID  string
	Slug      string
	FullName  string
	Country   string
	BirthDate *time.Time
	// BirthYear is derived from the age recorded on a match row, for players
	// whose id space has no player table to give an exact date. Approximate,
	// but a shared name, country and birth year is a strong enough signal to
	// act on where nothing better exists.
	BirthYear *int
	// Matches is how much history this row carries. The larger of a pair
	// becomes canonical, so the merge moves the shorter career onto the longer.
	Matches int
}

// Numeric reports whether the source id is a Sackmann numeric id rather than an
// ATP alphanumeric one. The two id spaces never collide, which is what makes a
// cross-space pair worth examining in the first place.
func (p Player) Numeric() bool {
	if p.SourceID == "" {
		return false
	}
	for _, r := range p.SourceID {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Match is a proposed link between two player rows.
type Match struct {
	Canonical  Player
	Duplicate  Player
	Confidence float64
	Reason     string
}

// Auto reports whether this match is strong enough to apply without review.
func (m Match) Auto() bool { return m.Confidence >= AutoLink }

// Key is the name a candidate is grouped by: folded to ASCII, lowercased, and
// with the parts sorted so "Last, First" and "First Last" agree.
//
// Sorting the parts is what handles the ordering difference, initials that
// appear on one side only, and hyphenated surnames written both ways.
func Key(fullName string) string {
	normalised := name.Normalise(strings.ReplaceAll(fullName, ",", " "))
	parts := strings.Fields(normalised)
	if len(parts) == 0 {
		return ""
	}
	// Insertion sort: names are two or three words.
	for i := 1; i < len(parts); i++ {
		for j := i; j > 0 && parts[j] < parts[j-1]; j-- {
			parts[j], parts[j-1] = parts[j-1], parts[j]
		}
	}
	return strings.Join(parts, " ")
}

// score rates a candidate pair that already shares a name key.
//
// The date of birth is the only field that makes a name match safe at 115,000
// players: there really are two Alexander Zverevs, and country alone does not
// separate them.
func score(a, b Player) (float64, string) {
	switch {
	case a.BirthDate != nil && b.BirthDate != nil:
		if !sameDay(*a.BirthDate, *b.BirthDate) {
			// Same name, different birthday: two different people, and saying
			// so is more useful than staying silent.
			return 0, "same name, different date of birth"
		}
		if a.Country != "" && b.Country != "" && a.Country != b.Country {
			// A birthday collision across countries is possible but rare
			// enough to be worth a human glance.
			return 0.70, "name and date of birth match, country does not"
		}
		return 1.00, "name and date of birth match"
	case sameCountry(a, b) && bothYearsKnown(a, b):
		if birthYear(a) != birthYear(b) {
			return 0, "same name and country, different birth year"
		}
		// No exact date exists on one side -- the ATP id space has no player
		// table -- but sharing a name, a country and a birth year is about as
		// unlikely as sharing a birthday.
		return 0.90, "name, country and birth year match, birth year derived from age"
	case a.Country != "" && b.Country != "" && a.Country == b.Country:
		return 0.60, "name and country match, no date of birth to confirm it"
	default:
		return 0.40, "name matches, nothing else to confirm it"
	}
}

func sameCountry(a, b Player) bool {
	return a.Country != "" && b.Country != "" && a.Country == b.Country
}

// birthYear prefers an exact date and falls back to the derived year.
func birthYear(p Player) int {
	if p.BirthDate != nil {
		return p.BirthDate.Year()
	}
	if p.BirthYear != nil {
		return *p.BirthYear
	}
	return 0
}

func bothYearsKnown(a, b Player) bool {
	return birthYear(a) != 0 && birthYear(b) != 0
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// Reconcile proposes links among players of one tour.
//
// Only pairs spanning the two id spaces are considered: two Sackmann ids that
// look alike are a different problem, and merging them here would be guessing.
func Reconcile(players []Player) []Match {
	groups := map[string][]Player{}
	for _, p := range players {
		if k := Key(p.FullName); k != "" {
			groups[k] = append(groups[k], p)
		}
	}

	var out []Match
	for _, group := range groups {
		if len(group) < 2 {
			continue
		}
		out = append(out, reconcileGroup(group)...)
	}
	return out
}

func reconcileGroup(group []Player) []Match {
	var numeric, alphanumeric []Player
	for _, p := range group {
		if p.Numeric() {
			numeric = append(numeric, p)
		} else {
			alphanumeric = append(alphanumeric, p)
		}
	}
	if len(numeric) == 0 || len(alphanumeric) == 0 {
		return nil
	}

	var out []Match
	for _, dup := range alphanumeric {
		best, second := bestTwo(numeric, dup)
		if best == nil {
			continue
		}
		confidence, reason := score(*best, dup)
		if confidence < ReviewFloor {
			continue
		}
		// Two equally good candidates is exactly the case that must not be
		// resolved by picking one: it goes to review at both scores.
		if second != nil {
			if runnerUp, _ := score(*second, dup); runnerUp >= confidence {
				reason = "more than one candidate matches equally well"
				confidence = ReviewFloor
			}
		}
		out = append(out, Match{
			Canonical:  *best,
			Duplicate:  dup,
			Confidence: confidence,
			Reason:     reason,
		})
	}
	return out
}

// bestTwo returns the highest and second-highest scoring candidates.
func bestTwo(candidates []Player, dup Player) (best, second *Player) {
	bestScore, secondScore := -1.0, -1.0
	for i := range candidates {
		s, _ := score(candidates[i], dup)
		switch {
		case s > bestScore:
			second, secondScore = best, bestScore
			best, bestScore = &candidates[i], s
		case s > secondScore:
			second, secondScore = &candidates[i], s
		}
	}
	return best, second
}

// Describe renders a match for a log line or the review queue.
func (m Match) Describe() string {
	return fmt.Sprintf("%s (%s) -> %s (%s): %.2f, %s",
		m.Duplicate.SourceID, m.Duplicate.FullName,
		m.Canonical.SourceID, m.Canonical.Slug, m.Confidence, m.Reason)
}
