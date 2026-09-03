package score

import (
	"fmt"
	"strings"
)

// maxGames bounds a plausible set. Pre-tiebreak sets ran long — 14-12 and worse
// — but three digits is a corrupt row, not a marathon.
const maxGames = 99

// impliedTiebreakWinner returns the winning score of a tiebreak given the
// loser's, since the short form "7-6(5)" records only the latter. First to
// seven, win by two.
func impliedTiebreakWinner(loser int) int {
	if loser+2 > 7 {
		return loser + 2
	}
	return 7
}

// Parse reads a source score string into sets plus an outcome.
//
// It is deliberately tolerant: the corpus contains stray spaces inside a set
// ("6- 2"), sets run together ("6-4-6-2"), prose suffixes ("Played and
// abandoned"), and unknown games ("?-?"). Input that carries no usable
// information yields Unknown rather than an error; only genuinely
// unintelligible input returns ErrMalformed.
func Parse(raw string) (Score, error) {
	s := Score{Raw: raw, Outcome: Complete}

	normalised, outcome := normalise(raw)
	if outcome != Complete {
		s.Outcome = outcome
	}
	if normalised == "" {
		if s.Outcome == Complete {
			s.Outcome = Unknown
		}
		return s, nil
	}

	skipped := false
	for _, token := range strings.Fields(normalised) {
		if !looksLikeSet(token) {
			skipped = true
			continue
		}
		sets, err := parseToken(token)
		if err != nil {
			return Score{Raw: raw}, fmt.Errorf("parse %q: %w", raw, err)
		}
		if len(sets) == 0 {
			skipped = true
		}
		s.Sets = append(s.Sets, sets...)
	}

	if s.Outcome == Complete && (skipped || len(s.Sets) == 0) {
		s.Outcome = Unknown
	}
	return s, nil
}

// normalise strips status markers and repairs the formatting damage present in
// the source, returning the remaining games text and any outcome found.
func normalise(raw string) (string, Outcome) {
	text := strings.TrimSpace(raw)
	outcome := Complete

	// Prose suffixes: "6-1 5-7 3-2 Played and unfinished".
	if i := indexFold(text, "played and"); i >= 0 {
		outcome = Abandoned
		text = text[:i]
	}
	for _, word := range []string{"unfinished", "abandoned"} {
		if i := indexFold(text, word); i >= 0 {
			outcome = Abandoned
			text = text[:i]
		}
	}

	markers := []struct {
		token string
		out   Outcome
	}{
		{"W/O", Walkover}, {"WO", Walkover},
		{"RET", Retired}, {"DEF", Defaulted},
		{"ABD", Abandoned}, {"UNK", Unknown},
	}

	var kept []string
	for _, field := range strings.Fields(text) {
		upper := strings.ToUpper(strings.Trim(field, ".,"))
		matched := false
		for _, m := range markers {
			if upper == m.token {
				outcome, matched = m.out, true
				break
			}
		}
		if matched {
			continue
		}
		// "?-?" and "6-1?" carry no reliable games.
		if strings.Contains(field, "?") {
			if outcome == Complete {
				outcome = Unknown
			}
			continue
		}
		kept = append(kept, field)
	}

	return healSpacing(strings.Join(kept, " ")), outcome
}

// healSpacing repairs sets split by a stray space: "6- 2", "7- 6(4)" and
// "6-7 (4)" all appear in the source.
func healSpacing(text string) string {
	fields := strings.Fields(text)
	out := make([]string, 0, len(fields))
	for i := 0; i < len(fields); i++ {
		if strings.HasSuffix(fields[i], "-") && i+1 < len(fields) {
			out = append(out, fields[i]+fields[i+1])
			i++
			continue
		}
		if strings.HasPrefix(fields[i], "(") && len(out) > 0 {
			out[len(out)-1] += fields[i]
			continue
		}
		out = append(out, fields[i])
	}
	return strings.Join(out, " ")
}

// looksLikeSet distinguishes a broken score from text that was never a score.
// "7-6(5" is worth an error; "May-00" is a spreadsheet accident to skip.
func looksLikeSet(token string) bool {
	if token == "" {
		return false
	}
	if token[0] == '[' {
		return true
	}
	return token[0] >= '0' && token[0] <= '9' && strings.Contains(token, "-")
}

func indexFold(s, substr string) int {
	return strings.Index(strings.ToLower(s), substr)
}

// parseToken reads one whitespace-delimited token. It may yield more than one
// set, because the source sometimes runs them together as "6-4-6-2".
func parseToken(token string) ([]Set, error) {
	if strings.HasPrefix(token, "[") {
		set, err := parseSuperTiebreak(token)
		if err != nil {
			return nil, err
		}
		return []Set{set}, nil
	}

	// A truncated set such as "6-" carries no games worth keeping.
	if strings.HasSuffix(token, "-") {
		return nil, nil
	}

	sets, err := scanSets(token, 0)
	if err == nil {
		return sets, nil
	}
	// Games are almost always single digits, so a token that fails greedy
	// scanning is usually two sets run together: "6-36-3" is "6-3 6-3".
	if retry, retryErr := scanSets(token, 1); retryErr == nil {
		return retry, nil
	}
	return nil, err
}

func scanSets(token string, maxDigits int) ([]Set, error) {
	p := &scanner{src: token, maxDigits: maxDigits}
	var sets []Set
	for {
		set, err := p.set()
		if err != nil {
			return nil, err
		}
		sets = append(sets, set)
		if p.done() {
			return sets, nil
		}
		// "6-4-6-2" separates sets with a hyphen; "6-36-3" not at all.
		if !p.accept('-') && !p.atDigit() {
			return nil, fmt.Errorf("%w: unexpected %q in %q", ErrMalformed, p.peek(), token)
		}
	}
}

func parseSuperTiebreak(token string) (Set, error) {
	body := strings.TrimSuffix(strings.TrimPrefix(token, "["), "]")
	p := &scanner{src: body}
	w, err := p.int()
	if err != nil {
		return Set{}, err
	}
	if !p.accept('-') {
		return Set{}, fmt.Errorf("%w: expected '-' in %q", ErrMalformed, token)
	}
	l, err := p.int()
	if err != nil {
		return Set{}, err
	}
	if !p.done() {
		return Set{}, fmt.Errorf("%w: trailing text in %q", ErrMalformed, token)
	}
	return Set{TiebreakWinner: w, TiebreakLoser: l, SuperTiebreak: true}, nil
}

type scanner struct {
	src string
	pos int
	// maxDigits caps how many digits one number may take, used to re-scan a
	// token whose sets run together. Zero means no cap.
	maxDigits int
}

func (p *scanner) atDigit() bool {
	return !p.done() && p.src[p.pos] >= '0' && p.src[p.pos] <= '9'
}

func (p *scanner) done() bool { return p.pos >= len(p.src) }

func (p *scanner) peek() string {
	if p.done() {
		return ""
	}
	return string(p.src[p.pos])
}

func (p *scanner) accept(c byte) bool {
	if !p.done() && p.src[p.pos] == c {
		p.pos++
		return true
	}
	return false
}

func (p *scanner) int() (int, error) {
	start := p.pos
	for p.atDigit() {
		if p.maxDigits > 0 && p.pos-start >= p.maxDigits {
			break
		}
		p.pos++
	}
	if start == p.pos {
		return 0, fmt.Errorf("%w: expected a number at %d in %q", ErrMalformed, start, p.src)
	}
	n := 0
	for _, c := range p.src[start:p.pos] {
		n = n*10 + int(c-'0')
	}
	if n > maxGames {
		return 0, fmt.Errorf("%w: implausible score %d in %q", ErrMalformed, n, p.src)
	}
	return n, nil
}

// set reads "6-4", "7-6(5)" or "7-6(8-6)".
func (p *scanner) set() (Set, error) {
	w, err := p.int()
	if err != nil {
		return Set{}, err
	}
	if !p.accept('-') {
		return Set{}, fmt.Errorf("%w: expected '-' in %q", ErrMalformed, p.src)
	}
	l, err := p.int()
	if err != nil {
		return Set{}, err
	}
	set := Set{GamesWinner: w, GamesLoser: l}

	if !p.accept('(') {
		return set, nil
	}
	first, err := p.int()
	if err != nil {
		return Set{}, err
	}
	if p.accept('-') {
		second, err := p.int()
		if err != nil {
			return Set{}, err
		}
		set.TiebreakWinner, set.TiebreakLoser = first, second
	} else {
		set.TiebreakWinner, set.TiebreakLoser = impliedTiebreakWinner(first), first
	}
	if !p.accept(')') {
		return Set{}, fmt.Errorf("%w: unclosed tiebreak in %q", ErrMalformed, p.src)
	}
	return set, nil
}
