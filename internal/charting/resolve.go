package charting

import (
	"sort"
	"strings"
	"time"

	"github.com/sami0076/tennis-wiki/internal/name"
)

// Candidate is a row of matches a charted match might be: the two players by
// name, the round, and the event's start date, which is what played_on holds.
type Candidate struct {
	MatchID  int64
	WinnerID int64
	LoserID  int64
	Winner   string
	Loser    string
	Round    string
	PlayedOn time.Time
}

// Resolution names the row a charted match is, and which player each of the
// charted names is.
type Resolution struct {
	MatchID   int64
	Player1ID int64
	Player2ID int64
}

// The window a charted match's date may fall in around its event's start:
// qualifying runs the week before the main draw's Monday, and a Grand Slam
// runs a fortnight after it.
const (
	before = 7 * 24 * time.Hour
	after  = 21 * 24 * time.Hour
)

// Index holds one tour's candidates, keyed by round and the two surnames, so
// a charted match is answered from memory.
type Index struct {
	byKey map[string][]Candidate
}

// NewIndex builds the index, keying each row under every surname either of
// its players might be known by.
func NewIndex(cands []Candidate) *Index {
	ix := &Index{byKey: map[string][]Candidate{}}
	for _, c := range cands {
		for _, a := range surnames(c.Winner) {
			for _, b := range surnames(c.Loser) {
				k := key(c.Round, a, b)
				ix.byKey[k] = append(ix.byKey[k], c)
			}
		}
	}
	return ix
}

// Resolve finds the one row a charted match is. The reason is empty on
// success and says why otherwise; a charted match with no row, or with two,
// is left rather than guessed.
func (ix *Index) Resolve(m Match) (Resolution, string) {
	if m.Round == "" {
		return Resolution{}, "no round"
	}
	var found []Candidate
	seen := map[int64]struct{}{}
	for _, s1 := range surnames(m.Player1) {
		for _, s2 := range surnames(m.Player2) {
			for _, c := range ix.byKey[key(m.Round, s1, s2)] {
				if _, dup := seen[c.MatchID]; dup || !inWindow(m.PlayedOn, c.PlayedOn) {
					continue
				}
				if samePerson(m.Player1, c.Winner) && samePerson(m.Player2, c.Loser) ||
					samePerson(m.Player1, c.Loser) && samePerson(m.Player2, c.Winner) {
					found = append(found, c)
					seen[c.MatchID] = struct{}{}
				}
			}
		}
	}
	switch len(found) {
	case 0:
		return Resolution{}, "no match on that date with those players in that round"
	case 1:
	default:
		// The same two players in the same round within a month of each
		// other: the event whose start is nearest before the charted date
		// is the one it was played in, if only one is.
		sort.Slice(found, func(i, j int) bool {
			return distance(m.PlayedOn, found[i].PlayedOn) < distance(m.PlayedOn, found[j].PlayedOn)
		})
		if distance(m.PlayedOn, found[0].PlayedOn) == distance(m.PlayedOn, found[1].PlayedOn) {
			return Resolution{}, "more than one match on that date with those players in that round"
		}
		found = found[:1]
	}

	c := found[0]
	r := Resolution{MatchID: c.MatchID, Player1ID: c.WinnerID, Player2ID: c.LoserID}
	if !samePerson(m.Player1, c.Winner) {
		r.Player1ID, r.Player2ID = c.LoserID, c.WinnerID
	}
	return r, ""
}

func inWindow(charted, eventStart time.Time) bool {
	return !charted.Before(eventStart.Add(-before)) && !charted.After(eventStart.Add(after))
}

// distance is how far the charted date is from the event's start, with a
// date before the start counted as further than the same gap after it.
func distance(charted, eventStart time.Time) time.Duration {
	d := charted.Sub(eventStart)
	if d < 0 {
		return -d + after
	}
	return d
}

// samePerson is the whole name folded to ASCII, with or without its spaces
// -- "Christopher Oconnell" and "Christopher O'Connell" -- or, failing that,
// a surname in common and either the same initial or a given name in common.
// The project writes "Coco Gauff" where the files say "Cori Gauff" and
// "Daria Kasatkina" for "Darya Kasatkina", which the initial covers; "Alison
// Riske Amritraj" for "Alison Riske" and "Camila Osorio" for "Maria Camila
// Osorio Serrano", which the shared given name covers. Particles are not
// surnames: "Van" is not what Botic Van De Zandschulp and Bart Van Den Berg
// have in common.
func samePerson(charted, stored string) bool {
	a, b := name.Normalise(charted), name.Normalise(stored)
	if a == b || strings.ReplaceAll(a, " ", "") == strings.ReplaceAll(b, " ", "") {
		return true
	}
	if a == "" || b == "" || (a[0] != b[0] && !sharesGivenName(a, b)) {
		return false
	}
	for _, x := range surnames(charted) {
		for _, y := range surnames(stored) {
			if x == y {
				return true
			}
		}
	}
	return false
}

// sharesGivenName reports whether either name's first word is a word of the
// other.
func sharesGivenName(a, b string) bool {
	wa, wb := strings.Fields(a), strings.Fields(b)
	if len(wa) == 0 || len(wb) == 0 {
		return false
	}
	for _, w := range wb {
		if w == wa[0] {
			return true
		}
	}
	for _, w := range wa {
		if w == wb[0] {
			return true
		}
	}
	return false
}

// particles are the words in a name that are not a surname on their own.
var particles = map[string]bool{
	"de": true, "del": true, "della": true, "der": true, "den": true, "di": true, "da": true,
	"do": true, "dos": true, "du": true, "van": true, "von": true, "la": true, "le": true,
	"el": true, "al": true, "y": true, "e": true, "jr": true, "sr": true, "ii": true, "iii": true,
}

// surnames is every word of a name but the first, particles aside, plus all
// of them run together, which is the form an apostrophe or a space in the
// wrong place produces on one side and not the other.
func surnames(full string) []string {
	words := strings.Fields(name.Normalise(full))
	if len(words) <= 1 {
		return words
	}
	rest := words[1:]
	out := make([]string, 0, len(rest)+1)
	for _, w := range rest {
		if !particles[w] {
			out = append(out, w)
		}
	}
	if len(rest) > 1 {
		out = append(out, strings.Join(rest, ""))
	}
	if len(out) == 0 {
		out = rest // a name that is all particles is still a name
	}
	return out
}

func key(round, a, b string) string {
	if a > b {
		a, b = b, a
	}
	return strings.TrimSpace(round) + "|" + a + "|" + b
}
