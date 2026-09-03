package score

import (
	"bufio"
	"errors"
	"os"
	"strings"
	"testing"
)

// All inputs below are real strings taken from the ATP and WTA match CSVs,
// sampled across 1968-2024 and across tour, Challenger and Futures level.
func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		sets    []Set
		outcome Outcome
	}{
		{
			name: "straight sets",
			in:   "6-4 6-2",
			sets: []Set{{GamesWinner: 6, GamesLoser: 4}, {GamesWinner: 6, GamesLoser: 2}},
		},
		{
			name: "five sets with a tiebreak",
			in:   "6-4 7-6(5) 3-6 6-2",
			sets: []Set{
				{GamesWinner: 6, GamesLoser: 4},
				{GamesWinner: 7, GamesLoser: 6, TiebreakWinner: 7, TiebreakLoser: 5},
				{GamesWinner: 3, GamesLoser: 6},
				{GamesWinner: 6, GamesLoser: 2},
			},
		},
		{
			name: "tiebreak lost by the match winner",
			in:   "6-7(4) 6-3 6-4",
			sets: []Set{
				{GamesWinner: 6, GamesLoser: 7, TiebreakWinner: 7, TiebreakLoser: 4},
				{GamesWinner: 6, GamesLoser: 3},
				{GamesWinner: 6, GamesLoser: 4},
			},
		},
		{
			name: "extended tiebreak",
			in:   "7-6(11) 6-4",
			sets: []Set{
				{GamesWinner: 7, GamesLoser: 6, TiebreakWinner: 13, TiebreakLoser: 11},
				{GamesWinner: 6, GamesLoser: 4},
			},
		},
		{
			name: "tiebreak written in full",
			in:   "7-6(8-6) 6-1",
			sets: []Set{
				{GamesWinner: 7, GamesLoser: 6, TiebreakWinner: 8, TiebreakLoser: 6},
				{GamesWinner: 6, GamesLoser: 1},
			},
		},
		{
			name: "pre-tiebreak long set",
			in:   "14-12 6-4",
			sets: []Set{{GamesWinner: 14, GamesLoser: 12}, {GamesWinner: 6, GamesLoser: 4}},
		},
		{
			name: "very long set",
			in:   "6-3 6-8 11-9",
			sets: []Set{
				{GamesWinner: 6, GamesLoser: 3},
				{GamesWinner: 6, GamesLoser: 8},
				{GamesWinner: 11, GamesLoser: 9},
			},
		},
		{
			name:    "retirement mid-set",
			in:      "4-1 RET",
			sets:    []Set{{GamesWinner: 4, GamesLoser: 1}},
			outcome: Retired,
		},
		{
			name:    "retirement after a completed set",
			in:      "6-3 2-1 RET",
			sets:    []Set{{GamesWinner: 6, GamesLoser: 3}, {GamesWinner: 2, GamesLoser: 1}},
			outcome: Retired,
		},
		{
			name:    "retirement before a game was won",
			in:      "3-0 RET",
			sets:    []Set{{GamesWinner: 3, GamesLoser: 0}},
			outcome: Retired,
		},
		{name: "walkover", in: "W/O", outcome: Walkover},
		{name: "walkover lowercase", in: "w/o", outcome: Walkover},
		{name: "default", in: "DEF", outcome: Defaulted},
		{
			name:    "default after play",
			in:      "6-1 6-1 DEF",
			sets:    []Set{{GamesWinner: 6, GamesLoser: 1}, {GamesWinner: 6, GamesLoser: 1}},
			outcome: Defaulted,
		},
		{name: "unknown marker", in: "UNK", outcome: Unknown},
		{name: "empty", in: "", outcome: Unknown},
		{name: "whitespace only", in: "   ", outcome: Unknown},
		{
			name: "super tiebreak final set",
			in:   "3-6 6-4 [10-5]",
			sets: []Set{{GamesWinner: 3, GamesLoser: 6}, {GamesWinner: 6, GamesLoser: 4}, {TiebreakWinner: 10, TiebreakLoser: 5, SuperTiebreak: true}},
		},
		{
			name: "super tiebreak past ten",
			in:   "3-6 6-3 [11-9]",
			sets: []Set{{GamesWinner: 3, GamesLoser: 6}, {GamesWinner: 6, GamesLoser: 3}, {TiebreakWinner: 11, TiebreakLoser: 9, SuperTiebreak: true}},
		},
		{
			name: "sets run together without spaces",
			in:   "6-4-6-2",
			sets: []Set{{GamesWinner: 6, GamesLoser: 4}, {GamesWinner: 6, GamesLoser: 2}},
		},
		{
			name: "three sets run together",
			in:   "6-1-1-6-6-4",
			sets: []Set{{GamesWinner: 6, GamesLoser: 1}, {GamesWinner: 1, GamesLoser: 6}, {GamesWinner: 6, GamesLoser: 4}},
		},
		{
			name: "stray space inside a set",
			in:   "6-3 6- 2",
			sets: []Set{{GamesWinner: 6, GamesLoser: 3}, {GamesWinner: 6, GamesLoser: 2}},
		},
		{
			name: "stray space before a tiebreak",
			in:   "7- 6(4) 6-4",
			sets: []Set{
				{GamesWinner: 7, GamesLoser: 6, TiebreakWinner: 7, TiebreakLoser: 4},
				{GamesWinner: 6, GamesLoser: 4},
			},
		},
		{
			name: "stray space mid-string",
			in:   "5-7 6-0 6- 4",
			sets: []Set{{GamesWinner: 5, GamesLoser: 7}, {GamesWinner: 6, GamesLoser: 0}, {GamesWinner: 6, GamesLoser: 4}},
		},
		{
			name: "space before the tiebreak",
			in:   "6-7 (4) 6-4 6-1",
			sets: []Set{
				{GamesWinner: 6, GamesLoser: 7, TiebreakWinner: 7, TiebreakLoser: 4},
				{GamesWinner: 6, GamesLoser: 4},
				{GamesWinner: 6, GamesLoser: 1},
			},
		},
		{
			name:    "prose suffix, unfinished",
			in:      "6-1 5-7 3-2 Played and unfinished",
			sets:    []Set{{GamesWinner: 6, GamesLoser: 1}, {GamesWinner: 5, GamesLoser: 7}, {GamesWinner: 3, GamesLoser: 2}},
			outcome: Abandoned,
		},
		{
			name:    "prose suffix, abandoned",
			in:      "2-2 Played and abandoned",
			sets:    []Set{{GamesWinner: 2, GamesLoser: 2}},
			outcome: Abandoned,
		},
		{
			name:    "abandoned at one set all",
			in:      "6-6 Played and unfinished",
			sets:    []Set{{GamesWinner: 6, GamesLoser: 6}},
			outcome: Abandoned,
		},
		{name: "abandoned marker", in: "ABD", outcome: Abandoned},
		{
			name:    "unknown games",
			in:      "6-3 ?-?",
			sets:    []Set{{GamesWinner: 6, GamesLoser: 3}},
			outcome: Unknown,
		},
		{
			name:    "trailing question mark",
			in:      "7-5 6-2 6-1?",
			sets:    []Set{{GamesWinner: 7, GamesLoser: 5}, {GamesWinner: 6, GamesLoser: 2}},
			outcome: Unknown,
		},
		{
			name: "leading and trailing whitespace",
			in:   "  6-4 6-3  ",
			sets: []Set{{GamesWinner: 6, GamesLoser: 4}, {GamesWinner: 6, GamesLoser: 3}},
		},
		{
			name: "bagel",
			in:   "6-0 6-0",
			sets: []Set{{GamesWinner: 6, GamesLoser: 0}, {GamesWinner: 6, GamesLoser: 0}},
		},
		{
			name: "five sets",
			in:   "4-6 6-3 6-7(5) 6-4 6-2",
			sets: []Set{
				{GamesWinner: 4, GamesLoser: 6},
				{GamesWinner: 6, GamesLoser: 3},
				{GamesWinner: 6, GamesLoser: 7, TiebreakWinner: 7, TiebreakLoser: 5},
				{GamesWinner: 6, GamesLoser: 4},
				{GamesWinner: 6, GamesLoser: 2},
			},
		},
		{
			name: "tiebreak at zero",
			in:   "7-6(0) 6-2",
			sets: []Set{
				{GamesWinner: 7, GamesLoser: 6, TiebreakWinner: 7, TiebreakLoser: 0},
				{GamesWinner: 6, GamesLoser: 2},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.in)
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", tc.in, err)
			}
			if got.Outcome != tc.outcome {
				t.Errorf("Parse(%q) outcome = %v, want %v", tc.in, got.Outcome, tc.outcome)
			}
			if len(got.Sets) != len(tc.sets) {
				t.Fatalf("Parse(%q) got %d sets %v, want %d %v",
					tc.in, len(got.Sets), got.Sets, len(tc.sets), tc.sets)
			}
			for i := range tc.sets {
				if got.Sets[i] != tc.sets[i] {
					t.Errorf("Parse(%q) set %d = %+v, want %+v", tc.in, i, got.Sets[i], tc.sets[i])
				}
			}
		})
	}
}

func TestIncomplete(t *testing.T) {
	complete := []string{"6-4 6-2", "14-12 6-4", "3-6 6-4 [10-5]"}
	incomplete := []string{"4-1 RET", "W/O", "DEF", "", "UNK", "2-2 Played and abandoned", "6-3 ?-?"}

	for _, in := range complete {
		s, err := Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", in, err)
		}
		if s.Incomplete() {
			t.Errorf("Parse(%q).Incomplete() = true, want false", in)
		}
	}
	for _, in := range incomplete {
		s, err := Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", in, err)
		}
		if !s.Incomplete() {
			t.Errorf("Parse(%q).Incomplete() = false, want true", in)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	canonical := []string{
		"6-4 6-2",
		"6-4 7-6(5) 3-6 6-2",
		"6-7(4) 6-3 6-4",
		"14-12 6-4",
		"4-6 6-3 6-7(5) 6-4 6-2",
		"7-6(11) 6-4",
		"3-6 6-4 [10-5]",
		"6-3 2-1 RET",
		"6-1 6-1 DEF",
		"W/O",
	}
	for _, in := range canonical {
		s, err := Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", in, err)
		}
		if got := s.String(); got != in {
			t.Errorf("round trip: Parse(%q).String() = %q", in, got)
		}
	}
}

func TestSetsWon(t *testing.T) {
	tests := []struct {
		in            string
		winner, loser int
	}{
		{"6-4 6-2", 2, 0},
		{"6-4 3-6 6-2", 2, 1},
		{"4-6 6-3 6-7(5) 6-4 6-2", 3, 2},
		{"6-3 2-1 RET", 2, 0},
		{"W/O", 0, 0},
	}
	for _, tc := range tests {
		s, err := Parse(tc.in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.in, err)
		}
		w, l := s.SetsWon()
		if w != tc.winner || l != tc.loser {
			t.Errorf("Parse(%q).SetsWon() = %d-%d, want %d-%d", tc.in, w, l, tc.winner, tc.loser)
		}
	}
}

func TestMalformed(t *testing.T) {
	// Broken where it was clearly meant to be a score: worth surfacing.
	bad := []string{
		"6-4 7-6(5 6-2", // unclosed tiebreak
		"6-4 999-2",     // implausible games
		"6-4 7-6()",     // empty tiebreak
		"6-4 [10-",      // unclosed super tiebreak
	}
	for _, in := range bad {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) should have failed", in)
		} else if !errors.Is(err, ErrMalformed) {
			t.Errorf("Parse(%q) error = %v, want ErrMalformed", in, err)
		}
	}

	// Never a score to begin with. The source contains spreadsheet damage such
	// as "May-00"; that is a data-quality fact to report, not a parse failure.
	notAScore := []struct {
		in   string
		sets int
	}{
		{"abc", 0},
		{"May-00", 0},
		{"6-4 x-2", 1},
		{"6", 0},
		{"-4 6-2", 1},
		{"6-0 6-", 1},
	}
	for _, tc := range notAScore {
		got, err := Parse(tc.in)
		if err != nil {
			t.Errorf("Parse(%q) returned error %v, want a tolerated parse", tc.in, err)
			continue
		}
		if got.Outcome != Unknown {
			t.Errorf("Parse(%q) outcome = %v, want Unknown", tc.in, got.Outcome)
		}
		if len(got.Sets) != tc.sets {
			t.Errorf("Parse(%q) got %d sets, want %d", tc.in, len(got.Sets), tc.sets)
		}
	}
}

// TestCorpus runs every score string in the fixture, which is sampled directly
// from the source CSVs. Nothing here may panic, and anything that fails to
// parse is reported so the failure rate stays visible.
func TestCorpus(t *testing.T) {
	f, err := os.Open("testdata/corpus.txt")
	if err != nil {
		t.Fatalf("open corpus: %v", err)
	}
	defer f.Close()

	var total, failed int
	var examples []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		total++
		if _, err := Parse(line); err != nil {
			failed++
			if len(examples) < 10 {
				examples = append(examples, line)
			}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read corpus: %v", err)
	}
	if total < 500 {
		t.Fatalf("corpus has only %d lines, expected the full sample", total)
	}
	if failed > 0 {
		t.Errorf("%d/%d real score strings failed to parse:\n  %s",
			failed, total, strings.Join(examples, "\n  "))
	}
	t.Logf("parsed %d real score strings", total)
}

func TestNoPanicOnArbitraryInput(t *testing.T) {
	inputs := []string{
		"", " ", "-", "(", ")", "()", "[", "]", "[]", "6-", "-6", "6-4(",
		"6-4)", "((((", "6-4 6-4 6-4 6-4 6-4 6-4 6-4", "\t\n", "6-4 6-2",
		"RET RET RET", "W/O 6-4", "[10-", "6-4(5", strings.Repeat("6-4 ", 100),
	}
	for _, in := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Parse(%q) panicked: %v", in, r)
				}
			}()
			_, _ = Parse(in)
		}()
	}
}
