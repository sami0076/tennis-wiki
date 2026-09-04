package ingest

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The player tables carry placeholders for "born, date unknown". Accepting one
// would put a fabricated birthday on a player page.
func TestParseCompactDate(t *testing.T) {
	cases := []struct {
		in   string
		want string // empty means it must be rejected
	}{
		{"19870522", "1987-05-22"},
		{"19131122", "1913-11-22"},
		{"19000000", ""}, // the source's "unknown" placeholder
		{"19870000", ""}, // month and day unknown
		{"19870500", ""}, // day unknown
		{"20190231", ""}, // 31 February rounds forward if not checked
		{"19871301", ""}, // month 13
		{"", ""},
		{"1987", ""},
		{"abcdefgh", ""},
		{"18001122", ""}, // before tennis records
		{"29991122", ""}, // far future
	}
	for _, c := range cases {
		got, ok := parseCompactDate(c.in)
		if c.want == "" {
			if ok {
				t.Errorf("parseCompactDate(%q) accepted %v, want rejected", c.in, got)
			}
			continue
		}
		if !ok {
			t.Errorf("parseCompactDate(%q) rejected a real date", c.in)
			continue
		}
		if got.Format("2006-01-02") != c.want {
			t.Errorf("parseCompactDate(%q) = %v, want %s", c.in, got, c.want)
		}
	}
}

func TestRefSourceRelPaths(t *testing.T) {
	players := RefSource{Kind: RefPlayers, Path: "players/atp_players.csv"}
	if got := players.RelPaths(); len(got) != 1 || got[0] != "players/atp_players.csv" {
		t.Errorf("player paths = %v", got)
	}

	rankings := RefSource{
		Kind: RefRankings, Path: "rankings/atp_rankings_{decade}.csv",
		Decades: []string{"70s", "current"},
	}
	want := []string{"rankings/atp_rankings_70s.csv", "rankings/atp_rankings_current.csv"}
	got := rankings.RelPaths()
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("ranking paths = %v, want %v", got, want)
	}
}

func TestRefSourceValidate(t *testing.T) {
	valid := RefSource{
		Name: "s", Tour: TourATP, Kind: RefPlayers,
		BaseURL: "https://x", Path: "p.csv", Attribution: "a",
	}
	if err := valid.validate(); err != nil {
		t.Fatalf("valid source rejected: %v", err)
	}

	cases := map[string]func(*RefSource){
		"no name":             func(r *RefSource) { r.Name = "" },
		"unknown tour":        func(r *RefSource) { r.Tour = "itf" },
		"unknown kind":        func(r *RefSource) { r.Kind = "matches" },
		"no path":             func(r *RefSource) { r.Path = "" },
		"no attribution":      func(r *RefSource) { r.Attribution = "" },
		"rankings no decades": func(r *RefSource) { r.Kind = RefRankings },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			s := valid
			mutate(&s)
			if err := s.validate(); err == nil {
				t.Errorf("%+v was accepted", s)
			}
		})
	}
}

func TestPlayerReader(t *testing.T) {
	const csv = "player_id,name_first,name_last,hand,dob,ioc,height,wikidata_id\n" +
		"100001,Gardnar,Mulloy,R,19131122,USA,185,Q54544\n" +
		"200000,X,X,U,19000000,UNK,,\n" +
		"300000,No,Height,L,,FRA,12,\n" +
		",Blank,Row,R,19900101,ESP,180,\n"

	r, err := NewPlayerReader(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("NewPlayerReader: %v", err)
	}
	var got []PlayerBio
	for {
		bio, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		got = append(got, bio)
	}

	if len(got) != 3 {
		t.Fatalf("read %d players, want 3: the row with no id is not a player", len(got))
	}

	first := got[0]
	if first.FullName() != "Gardnar Mulloy" || first.Country != "USA" || first.WikidataID != "Q54544" {
		t.Errorf("first player = %+v", first)
	}
	if first.BirthDate == nil || first.HeightCm == nil || *first.HeightCm != 185 {
		t.Errorf("first player lost its date of birth or height: %+v", first)
	}

	placeholder := got[1]
	if placeholder.BirthDate != nil {
		t.Error("19000000 was accepted as a date of birth")
	}
	// UNK is the source saying it does not know, not a country code.
	if placeholder.Country != "" {
		t.Errorf("country = %q, want empty for UNK", placeholder.Country)
	}

	// A 12cm player is a typo, not a person.
	if got[2].HeightCm != nil {
		t.Errorf("height = %d, want nil: implausible", *got[2].HeightCm)
	}
}

func TestRankingReader(t *testing.T) {
	// The WTA mirror carries an extra trailing column, so resolution has to be
	// by name rather than position.
	const csv = "ranking_date,rank,player,points,tours\n" +
		"20190107,1,104745,10550,20\n" +
		"20190107,2,103819,,18\n" +
		"19000000,3,100000,500,4\n" + // unusable date
		"20190107,0,100001,500,4\n" + // rank zero
		"20190107,5,,500,4\n" // no player
	r, err := NewRankingReader(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("NewRankingReader: %v", err)
	}
	var got []RankingEntry
	for {
		e, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		got = append(got, e)
	}

	if len(got) != 2 {
		t.Fatalf("read %d rankings, want 2", len(got))
	}
	if r.Rejected() != 3 {
		t.Errorf("Rejected() = %d, want 3: unusable rows are counted, not silently dropped", r.Rejected())
	}
	if got[0].Rank != 1 || got[0].SourceID != "104745" || got[0].Points == nil || *got[0].Points != 10550 {
		t.Errorf("first ranking = %+v", got[0])
	}
	// Points are absent for much of the early history, and zero is a real value.
	if got[1].Points != nil {
		t.Errorf("missing points became %d, want nil", *got[1].Points)
	}
}

func TestPlayerReaderRejectsAWrongFile(t *testing.T) {
	_, err := NewPlayerReader(strings.NewReader("tourney_id,winner_id\n2019-001,1\n"))
	if err == nil {
		t.Fatal("a match file was accepted as a player table")
	}
	if !strings.Contains(err.Error(), "player_id") {
		t.Errorf("error = %q, should name the missing column", err)
	}
}

// The committed fixtures must parse, or the seed silently degrades.
func TestSeedReferenceFixturesParse(t *testing.T) {
	root := filepath.Join("..", "..", "testdata")
	if _, err := os.Stat(root); err != nil {
		t.Skip("seed fixtures not present")
	}

	for _, f := range []string{"atp_players.csv", "wta_players.csv"} {
		fh, err := os.Open(filepath.Join(root, f))
		if err != nil {
			t.Fatalf("open %s: %v", f, err)
		}
		r, err := NewPlayerReader(fh)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		named, dated := 0, 0
		for {
			bio, err := r.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatalf("%s: %v", f, err)
			}
			if bio.FullName() != "" {
				named++
			}
			if bio.BirthDate != nil {
				dated++
			}
		}
		_ = fh.Close()
		if named == 0 || dated == 0 {
			t.Errorf("%s: %d named, %d with a date of birth", f, named, dated)
		}
	}
}
