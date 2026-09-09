package identity

import (
	"testing"
	"time"
)

func date(t *testing.T, s string) *time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("bad date %q: %v", s, err)
	}
	return &d
}

// The sources write the same name several ways. Grouping has to see through
// all of them, or the pair is never even considered.
func TestKeyNormalisesTheWaysNamesDiffer(t *testing.T) {
	same := [][]string{
		{"Carlos Alcaraz", "Alcaraz, Carlos", "carlos alcaraz", "  Carlos   Alcaraz  "},
		{"Novak Djokovic", "Novak Djoković", "Djokovic, Novak"},
		{"Jo-Wilfried Tsonga", "Tsonga, Jo Wilfried", "jo wilfried tsonga"},
		{"Karolina Pliskova", "Karolína Plíšková"},
	}
	for _, group := range same {
		want := Key(group[0])
		if want == "" {
			t.Fatalf("empty key for %q", group[0])
		}
		for _, variant := range group[1:] {
			if got := Key(variant); got != want {
				t.Errorf("Key(%q) = %q, but Key(%q) = %q", variant, got, group[0], want)
			}
		}
	}

	// Different people must not collide.
	if Key("Alexander Zverev") == Key("Mischa Zverev") {
		t.Error("two brothers share a key")
	}
	if Key("") != "" {
		t.Error("an empty name should give an empty key")
	}
}

func TestNumericDistinguishesTheTwoIDSpaces(t *testing.T) {
	cases := map[string]bool{
		"104745": true, "207518": true,
		"B0BI": false, "MU94": false, "": false, "12A": false,
	}
	for id, want := range cases {
		if got := (Player{SourceID: id}).Numeric(); got != want {
			t.Errorf("Numeric(%q) = %v, want %v", id, got, want)
		}
	}
}

// The date of birth is what makes a name match safe at 115,000 players.
func TestScoring(t *testing.T) {
	born1990 := date(t, "1990-01-01")
	born1991 := date(t, "1991-01-01")

	cases := []struct {
		name     string
		a, b     Player
		want     float64
		wantAuto bool
	}{
		{
			name:     "name and date of birth",
			a:        Player{BirthDate: born1990, Country: "ESP"},
			b:        Player{BirthDate: born1990, Country: "ESP"},
			want:     1.00,
			wantAuto: true,
		},
		{
			name: "same name, different birthday: different people",
			a:    Player{BirthDate: born1990},
			b:    Player{BirthDate: born1991},
			want: 0,
		},
		{
			name: "birthday matches but country does not",
			a:    Player{BirthDate: born1990, Country: "ESP"},
			b:    Player{BirthDate: born1990, Country: "ARG"},
			want: 0.70,
		},
		{
			name: "no date of birth, same country",
			a:    Player{Country: "ESP"},
			b:    Player{Country: "ESP"},
			want: 0.60,
		},
		{
			name: "nothing but the name",
			a:    Player{},
			b:    Player{},
			want: 0.40,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, reason := score(c.a, c.b)
			if got != c.want {
				t.Errorf("score = %v (%s), want %v", got, reason, c.want)
			}
			if reason == "" {
				t.Error("every score should carry a reason")
			}
			if auto := (Match{Confidence: got}).Auto(); auto != c.wantAuto {
				t.Errorf("Auto() = %v, want %v at confidence %v", auto, c.wantAuto, got)
			}
		})
	}
}

// The case the whole package exists for.
func TestReconcileLinksAcrossIDSpaces(t *testing.T) {
	born := date(t, "2003-05-05")
	players := []Player{
		{ID: 1, SourceID: "207989", FullName: "Carlos Alcaraz", Country: "ESP", BirthDate: born, Matches: 300},
		{ID: 2, SourceID: "AC30", FullName: "Carlos Alcaraz", Country: "ESP", BirthDate: born, Matches: 40},
	}

	matches := Reconcile(players)
	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1: %+v", len(matches), matches)
	}
	m := matches[0]
	if !m.Auto() {
		t.Errorf("confidence %v: a name and date of birth match should link automatically", m.Confidence)
	}
	// The longer career is canonical, so the merge moves the shorter one.
	if m.Canonical.ID != 1 || m.Duplicate.ID != 2 {
		t.Errorf("canonical=%d duplicate=%d, want 1 and 2", m.Canonical.ID, m.Duplicate.ID)
	}
}

// Two Sackmann ids that look alike are a different problem, and merging them
// here would be guessing.
func TestReconcileIgnoresPairsInsideOneIDSpace(t *testing.T) {
	born := date(t, "1997-04-20")
	players := []Player{
		{ID: 1, SourceID: "100001", FullName: "Alexander Zverev", BirthDate: born},
		{ID: 2, SourceID: "100002", FullName: "Alexander Zverev", BirthDate: born},
	}
	if got := Reconcile(players); len(got) != 0 {
		t.Errorf("got %d matches, want none: %+v", len(got), got)
	}
}

// Same name, different birthday: two people, and the pair must not even reach
// the review queue as a candidate merge.
func TestReconcileRejectsDifferentBirthdays(t *testing.T) {
	players := []Player{
		{ID: 1, SourceID: "100001", FullName: "Alexander Zverev", BirthDate: date(t, "1997-04-20")},
		{ID: 2, SourceID: "ZV11", FullName: "Alexander Zverev", BirthDate: date(t, "1985-08-22")},
	}
	if got := Reconcile(players); len(got) != 0 {
		t.Errorf("got %+v, want no match: the birthdays differ", got)
	}
}

// Ambiguity must go to a human, not be resolved by picking one.
func TestReconcileQueuesAmbiguousCandidates(t *testing.T) {
	players := []Player{
		{ID: 1, SourceID: "100001", FullName: "Juan Martin", Country: "ARG"},
		{ID: 2, SourceID: "100002", FullName: "Juan Martin", Country: "ARG"},
		{ID: 3, SourceID: "JM01", FullName: "Juan Martin", Country: "ARG"},
	}
	matches := Reconcile(players)
	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1: %+v", len(matches), matches)
	}
	if matches[0].Auto() {
		t.Error("two equally good candidates were merged without review")
	}
	if matches[0].Reason == "" {
		t.Error("an ambiguous match should say why it is ambiguous")
	}
}

// Weak matches are not worth a human's time either.
func TestReconcileDropsMatchesBelowTheReviewFloor(t *testing.T) {
	players := []Player{
		{ID: 1, SourceID: "100001", FullName: "Common Name", Country: "USA"},
		{ID: 2, SourceID: "CN01", FullName: "Common Name", Country: "FRA"},
	}
	// Countries differ and there is no date of birth: 0.40, below the floor.
	if got := Reconcile(players); len(got) != 0 {
		t.Errorf("got %+v, want nothing: too weak to be worth reviewing", got)
	}
}

// The real numbers from the 179 pairs this was silently discarding. Sinner was
// born in August 2001 and both of his rows derive 2000 from a match age; the
// old comparison put the exact 2001 against the derived 2000, scored zero, and
// dropped the pair below the review floor.
func TestExactAndDerivedYearsAreNotComparedDirectly(t *testing.T) {
	derived := 2000
	sackmann := Player{
		SourceID: "206173", FullName: "Jannik Sinner", Country: "ITA",
		BirthDate: date(t, "2001-08-16"), BirthYear: &derived,
	}
	atp := Player{
		SourceID: "S0AG", FullName: "Jannik Sinner", Country: "ITA",
		BirthYear: &derived,
	}

	got, reason := score(sackmann, atp)
	if got < AutoLink {
		t.Errorf("score = %.2f (%s), want an automatic merge", got, reason)
	}
}

// The rounding cancels only when both sides carry it. Two players whose derived
// years genuinely differ are still two people.
func TestDerivedYearsThatDisagreeAreNotMerged(t *testing.T) {
	a, b := 1998, 2001
	got, _ := score(
		Player{SourceID: "100001", FullName: "Same Name", Country: "USA", BirthYear: &a},
		Player{SourceID: "S0AA", FullName: "Same Name", Country: "USA", BirthYear: &b},
	)
	if got != 0 {
		t.Errorf("score = %.2f, want nothing: the two derived years disagree", got)
	}
}

// One side with no derived year at all is the only case where an exact year
// meets an approximate one. A single year of disagreement is then a question
// for a person, not an answer either way.
func TestAnExactYearAgainstAnApproximateOneGoesToReview(t *testing.T) {
	derived := 2000
	exact := Player{
		SourceID: "100001", FullName: "Some Player", Country: "USA",
		BirthDate: date(t, "2001-08-16"),
	}
	approximate := Player{
		SourceID: "S0AA", FullName: "Some Player", Country: "USA", BirthYear: &derived,
	}

	got, reason := score(exact, approximate)
	if got >= AutoLink {
		t.Errorf("score = %.2f (%s), want review rather than an automatic merge", got, reason)
	}
	if got < ReviewFloor {
		t.Errorf("score = %.2f (%s), want it to reach a human at all", got, reason)
	}

	// Three years apart is not rounding.
	far := 1998
	if got, _ := score(exact, Player{
		SourceID: "S0AB", FullName: "Some Player", Country: "USA", BirthYear: &far,
	}); got != 0 {
		t.Errorf("score = %.2f, want nothing three years apart", got)
	}
}
