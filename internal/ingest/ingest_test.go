package ingest

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture opens a real CSV sample committed under testdata.
func fixture(t *testing.T, name string, src Source) *Reader {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("open fixture %s: %v", name, err)
	}
	t.Cleanup(func() { f.Close() })
	r, err := NewReader(src, f)
	if err != nil {
		t.Fatalf("NewReader(%s): %v", name, err)
	}
	return r
}

func all(t *testing.T, r *Reader) []MatchRow {
	t.Helper()
	var rows []MatchRow
	for {
		row, err := r.Next()
		if errors.Is(err, io.EOF) {
			return rows
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		rows = append(rows, row)
	}
}

var (
	sackmannATP = Source{Name: "test-atp", Tour: TourATP, Profile: "sackmann", BaseURL: "https://x", Path: "atp_matches_{season}.csv"}
	sackmannWTA = Source{Name: "test-wta", Tour: TourWTA, Profile: "sackmann", BaseURL: "https://x", Path: "wta_matches_{season}.csv"}
	tmlATP      = Source{Name: "test-tml", Tour: TourATP, Profile: "tml", BaseURL: "https://x", Path: "{season}.csv"}
)

func TestReadSackmannATP(t *testing.T) {
	rows := all(t, fixture(t, "atp_matches_2024.csv", sackmannATP))
	if len(rows) == 0 {
		t.Fatal("no rows read")
	}
	r := rows[0]
	if r.TourneyID == "" || r.Winner.Name == "" || r.Loser.Name == "" {
		t.Errorf("row not populated: %+v", r)
	}
	if r.TourneyDate.Year() != 2024 {
		t.Errorf("tourney date year = %d, want 2024", r.TourneyDate.Year())
	}
	if !r.HasDetailedStats() {
		t.Error("2024 match should carry serve statistics")
	}
	if r.Winner.Stats.ServePoints <= 0 {
		t.Errorf("serve points = %d, want positive", r.Winner.Stats.ServePoints)
	}
}

func TestReadSackmannWTA(t *testing.T) {
	rows := all(t, fixture(t, "wta_matches_2019.csv", sackmannWTA))
	if len(rows) == 0 {
		t.Fatal("no rows read")
	}
	// The same profile must read both tours: the layouts are identical.
	if rows[0].Winner.SourceID == "" {
		t.Error("WTA row has no winner id")
	}
}

// Statistics do not exist before roughly 1991. They must arrive as nil, never
// as zeros, because a zeroed stat line is wrong but plausible-looking.
func TestPre1991StatsAreNil(t *testing.T) {
	rows := all(t, fixture(t, "atp_matches_1969.csv", sackmannATP))
	if len(rows) == 0 {
		t.Fatal("no rows read")
	}
	for _, r := range rows {
		if r.Winner.Stats != nil || r.Loser.Stats != nil {
			t.Errorf("1969 match %s/%d has stats %+v, want nil",
				r.TourneyID, r.MatchNum, r.Winner.Stats)
		}
		if r.HasDetailedStats() {
			t.Error("HasDetailedStats() = true for a 1969 match")
		}
		if r.Minutes != nil {
			t.Errorf("1969 match has minutes %v, want nil", *r.Minutes)
		}
	}
}

// Futures carry no serve statistics in any year.
func TestFuturesStatsAreNil(t *testing.T) {
	src := sackmannATP
	src.Tier = "futures"
	rows := all(t, fixture(t, "atp_matches_futures_2015.csv", src))
	if len(rows) == 0 {
		t.Fatal("no rows read")
	}
	for _, r := range rows {
		if r.HasDetailedStats() {
			t.Errorf("futures match %s/%d reports detailed stats", r.TourneyID, r.MatchNum)
		}
	}
}

// Qualifying is bundled into the same file and tournament as the main draw, so
// it has to come from the round.
func TestQualifyingDetection(t *testing.T) {
	rows := all(t, fixture(t, "atp_matches_qual_chall_2022.csv", sackmannATP))
	if len(rows) == 0 {
		t.Fatal("no rows read")
	}
	var qualifying int
	for _, r := range rows {
		if r.IsQualifying() {
			qualifying++
		}
	}
	if qualifying == 0 {
		t.Error("no qualifying rounds detected in the qual_chall fixture")
	}

	// QF is a quarterfinal, not a qualifying round. Confusing the two would
	// mislabel every main-draw quarterfinal in the database.
	cases := map[string]bool{
		"Q1": true, "Q2": true, "Q3": true, "q1": true,
		"QF": false, "SF": false, "F": false, "R16": false, "RR": false, "BR": false,
	}
	for round, want := range cases {
		got := MatchRow{Round: round}.IsQualifying()
		if got != want {
			t.Errorf("round %q: IsQualifying() = %v, want %v", round, got, want)
		}
	}
}

// The TML layout orders columns differently and adds one. Reading by header
// name rather than position is what makes that safe.
func TestReadTMLProfile(t *testing.T) {
	rows := all(t, fixture(t, "tml_2026.csv", tmlATP))
	if len(rows) == 0 {
		t.Fatal("no rows read")
	}
	r := rows[0]
	if r.Winner.Name == "" || r.Loser.Name == "" {
		t.Errorf("names not resolved: winner=%q loser=%q", r.Winner.Name, r.Loser.Name)
	}
	if r.TourneyDate.Year() != 2026 {
		t.Errorf("year = %d, want 2026", r.TourneyDate.Year())
	}
	if r.Indoor == nil {
		t.Error("TML layout should populate indoor")
	}
	if !r.HasDetailedStats() {
		t.Error("TML rows should carry serve statistics")
	}
	// Official ATP ids are alphanumeric, which is why reconciliation is needed.
	if _, err := strconvAtoi(r.Winner.SourceID); err == nil {
		t.Logf("winner id %q happens to be numeric", r.Winner.SourceID)
	}
	if r.Winner.SourceID == "" {
		t.Error("TML winner id is empty")
	}
}

func strconvAtoi(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errors.New("not numeric")
		}
		n = n*10 + int(c-'0')
	}
	if s == "" {
		return 0, errors.New("empty")
	}
	return n, nil
}

// Reading a reordered header by position would silently mis-assign every field,
// so an unrecognised header must fail loudly instead.
func TestUnknownHeaderFails(t *testing.T) {
	bad := strings.NewReader("foo,bar,baz\n1,2,3\n")
	if _, err := NewReader(sackmannATP, bad); err == nil {
		t.Fatal("NewReader should reject a header missing required columns")
	} else if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error = %v, want it to name the missing columns", err)
	}
}

func TestUnknownProfileFails(t *testing.T) {
	src := sackmannATP
	src.Profile = "nonexistent"
	r := strings.NewReader("tourney_id\n1\n")
	if _, err := NewReader(src, r); err == nil {
		t.Fatal("NewReader should reject an unknown profile")
	}
}

// Column order must not matter.
func TestReorderedHeaderStillParses(t *testing.T) {
	csvText := "score,match_num,winner_name,loser_name,tourney_date,tourney_id," +
		"tourney_name,tourney_level,round,best_of,winner_id,loser_id\n" +
		"6-4 6-2,1,A Player,B Player,20240115,2024-001,Test Open,A,F,3,100,200\n"
	rows := all(t, mustReader(t, sackmannATP, csvText))
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	r := rows[0]
	if r.Winner.Name != "A Player" || r.Loser.Name != "B Player" {
		t.Errorf("names mis-assigned: %+v", r)
	}
	if r.Score != "6-4 6-2" || r.MatchNum != 1 {
		t.Errorf("fields mis-assigned: score=%q match_num=%d", r.Score, r.MatchNum)
	}
}

func mustReader(t *testing.T, src Source, csvText string) *Reader {
	t.Helper()
	r, err := NewReader(src, strings.NewReader(csvText))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	return r
}

func TestSurfaceAndHandNormalisation(t *testing.T) {
	surfaces := map[string]string{
		"Hard": "hard", "Clay": "clay", "Grass": "grass", "Carpet": "carpet",
		"": "", "None": "", "unknown": "",
	}
	for in, want := range surfaces {
		if got := normaliseSurface(in); got != want {
			t.Errorf("normaliseSurface(%q) = %q, want %q", in, got, want)
		}
	}
	hands := map[string]string{"R": "R", "L": "L", "U": "U", "A": "U", "": "", "x": ""}
	for in, want := range hands {
		if got := normaliseHand(in); got != want {
			t.Errorf("normaliseHand(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOptIntKeepsBlanksNil(t *testing.T) {
	if optInt("") != nil {
		t.Error(`optInt("") should be nil, not zero`)
	}
	if optInt("NA") != nil {
		t.Error(`optInt("NA") should be nil`)
	}
	if v := optInt("0"); v == nil || *v != 0 {
		t.Error(`optInt("0") should be a real zero`)
	}
	if v := optInt("183.0"); v == nil || *v != 183 {
		t.Errorf(`optInt("183.0") = %v, want 183`, v)
	}
}

func TestLocalFetcher(t *testing.T) {
	src := Source{Name: "t", Tour: TourATP, Profile: "sackmann", BaseURL: "https://x", Path: "atp_matches_{season}.csv"}
	f := LocalFetcher{Root: "testdata"}

	rc, err := f.Open(context.Background(), src, 2024)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	rc.Close()

	if _, err := f.Open(context.Background(), src, 1800); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing season error = %v, want ErrNotFound", err)
	}
}
