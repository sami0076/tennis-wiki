package events

import (
	"testing"
	"time"
)

var nextID int64

func row(tour, sourceID, name, tier string, season int) Row {
	nextID++
	return Row{
		ID: nextID, Tour: tour, SourceID: sourceID, Name: name, Level: "A", Tier: tier,
		Season: season, StartDate: time.Date(season, 6, 1, 0, 0, 0, 0, time.UTC),
	}
}

func team(tour, sourceID, name string, season int) Row {
	r := row(tour, sourceID, name, "tour", season)
	r.Level = "D"
	return r
}

// byKey indexes a result for assertions.
func byKey(t *testing.T, res Result) map[string]Event {
	t.Helper()
	out := map[string]Event{}
	for _, ev := range res.Events {
		out[ev.Tour+" "+ev.Key] = ev
	}
	return out
}

func links(ev Event) map[Link]int {
	out := map[Link]int{}
	for _, ed := range ev.Editions {
		out[ed.Link]++
	}
	return out
}

func TestNormalise(t *testing.T) {
	same := [][2]string{
		{"Punta Del Este CH", "Punta del Este"},
		{"Antalya $10K", "Antalya 10K"},
		{"Oeiras 2 CH", "Oeiras"},
		{"Kitzbühel", "Kitzbuhel"},
		{"'s Hertogenbosch", "s-Hertogenbosch"},
	}
	for _, c := range same {
		if Normalise(c[0]) != Normalise(c[1]) {
			t.Errorf("%q and %q should normalise alike: %q vs %q", c[0], c[1], Normalise(c[0]), Normalise(c[1]))
		}
	}
	// The category is part of what the event was.
	if Normalise("W15 Antalya") == Normalise("Antalya") {
		t.Error("a category prefix must not be stripped")
	}
}

// A name-keyed event drops the number a source wrote on it, and carries its
// own ordinal instead; a numbered event keeps its name as written.
func TestDisplayNameCarriesTheOrdinal(t *testing.T) {
	res := Resolve([]Row{
		row("wta", "1934-1077", "Scarborough 1", "tour", 1934),
		row("atp", "2025-2222", "Oeiras 2 CH", "challenger", 2025),
	}, nil)
	keys := byKey(t, res)
	if got := keys["wta name:tour:scarborough"].Name; got != "Scarborough" {
		t.Errorf("name-keyed first of its season: %q, want the bare name", got)
	}
	if got := keys["atp number:2222"].Name; got != "Oeiras 2" {
		t.Errorf("numbered event: %q, want the source's name", got)
	}
}

func TestCompetition(t *testing.T) {
	cases := map[string]string{
		"Davis Cup WG R1: ESP vs CZE":                     "davis-cup",
		"Davis Cup SAM PQ: PER vs BOL":                    "davis-cup",
		"Fed Cup G1 AM PPO: PAR vs ARG":                   "billie-jean-king-cup",
		"BJK Cup Finals":                                  "billie-jean-king-cup",
		"Billie Jean King Cup qualifying round - Group A": "billie-jean-king-cup",
		"Wightman Cup":                                    "wightman-cup",
		"Porto Alegre BRA vs COL":                         "porto-alegre-bra-vs-col",
	}
	for in, want := range cases {
		if got := Competition(in); got != want {
			t.Errorf("Competition(%q) = %q, want %q", in, got, want)
		}
	}
}

// The ATP number is the sanction, and the sanction moves city: one event,
// named after its latest edition, every row placed by number.
func TestNumberAcrossSeasonsAndNames(t *testing.T) {
	res := Resolve([]Row{
		row("atp", "1989-306", "Bari", "tour", 1989),
		row("atp", "1994-306", "St. Poelten", "tour", 1994),
		row("atp", "2009-0306", "Kitzbuhel", "tour", 2009),
		row("atp", "2025-306", "Kitzbuhel", "tour", 2025),
	}, nil)
	ev, ok := byKey(t, res)["atp number:306"]
	if !ok {
		t.Fatalf("no event number:306; got %+v", res.Events)
	}
	if len(ev.Editions) != 4 || ev.FirstSeason != 1989 || ev.LastSeason != 2025 {
		t.Errorf("editions %d, %d-%d", len(ev.Editions), ev.FirstSeason, ev.LastSeason)
	}
	if ev.Name != "Kitzbuhel" {
		t.Errorf("name %q, want the latest edition's", ev.Name)
	}
	if links(ev)[LinkNumber] != 4 {
		t.Errorf("links %v, want all by number", links(ev))
	}
}

// Sackmann's Challenger suffix is not a rename, and the display name drops it.
func TestChallengerSuffixIsNotARename(t *testing.T) {
	res := Resolve([]Row{
		row("atp", "2024-1214", "Troyes CH", "challenger", 2024),
		row("atp", "2025-1214", "Troyes", "challenger", 2025),
	}, nil)
	ev := byKey(t, res)["atp number:1214"]
	if len(ev.Editions) != 2 || ev.Name != "Troyes" {
		t.Errorf("got %d editions named %q", len(ev.Editions), ev.Name)
	}
	res = Resolve([]Row{row("atp", "2024-1214", "Troyes CH", "challenger", 2024)}, nil)
	if got := res.Events[0].Name; got != "Troyes" {
		t.Errorf("display name %q keeps the suffix", got)
	}
}

// A WTA number before 2016 is not the WTA's: a sequence within the year to
// 1987, the ITF circuit's numbering on the Challenger tier to 1995.
func TestWTASequenceNumbersAreNotIdentities(t *testing.T) {
	res := Resolve([]Row{
		row("wta", "1923-1056", "Wimbledon", "tour", 1923),
		row("wta", "1924-1056", "Bastad", "tour", 1924),
		row("wta", "1925-1114", "Wimbledon", "tour", 1925),
		row("wta", "1991-0540", "ITF Indianapolis", "challenger", 1991),
		row("wta", "2016-1056", "Tokyo", "tour", 2016),
		row("wta", "2017-1056", "Tokyo", "tour", 2017),
	}, nil)
	keys := byKey(t, res)
	if ev, ok := keys["wta name:tour:wimbledon"]; !ok || len(ev.Editions) != 2 {
		t.Errorf("Wimbledon 1923 and 1925 should be one name-keyed event: %+v", keys)
	}
	if _, ok := keys["wta name:tour:bastad"]; !ok {
		t.Error("Bastad 1924 should be its own name-keyed event")
	}
	if _, ok := keys["wta name:challenger:itf-indianapolis"]; !ok {
		t.Error("the ITF circuit's 540 of 1991 is not the WTA's 540")
	}
	if ev, ok := keys["wta number:1056"]; !ok || len(ev.Editions) != 2 || ev.FirstSeason != 2016 {
		t.Errorf("Tokyo 2016-17 should be number:1056 alone: %+v", ev)
	}
}

// The women's Slams: an override folds Sackmann's men's number onto the
// WTA's, and the ITF-style decades bridge to it by name.
func TestOverrideAndBridge(t *testing.T) {
	overrides := &Overrides{Overrides: []Override{
		{Tour: "wta", Number: "540", To: "904", Note: "Sackmann used the men's number"},
	}}
	res := Resolve([]Row{
		row("wta", "1968-W-SL-GBR-01A-1968", "Wimbledon", "tour", 1968),
		row("wta", "2015-W-SL-GBR-01A-2015", "Wimbledon", "tour", 2015),
		row("wta", "2016-540", "Wimbledon", "tour", 2016),
		row("wta", "2022-904", "Wimbledon", "tour", 2022),
		// The men's 540 is untouched by a WTA override.
		row("atp", "2016-540", "Wimbledon", "tour", 2016),
	}, overrides)
	keys := byKey(t, res)
	ev, ok := keys["wta number:904"]
	if !ok || len(ev.Editions) != 4 {
		t.Fatalf("want one women's Wimbledon with 4 editions, got %+v", keys)
	}
	got := links(ev)
	if got[LinkBridged] != 2 || got[LinkOverride] != 1 || got[LinkNumber] != 1 {
		t.Errorf("links %v", got)
	}
	if _, ok := keys["atp number:540"]; !ok {
		t.Error("the men's 540 should stand")
	}
	if len(res.Unmatched) != 0 {
		t.Errorf("override should have matched: %+v", res.Unmatched)
	}
	if res.ByLink[LinkBridged] != 2 {
		t.Errorf("ByLink %v", res.ByLink)
	}
}

// A name under two numbers in the same tour and tier bridges to neither.
func TestAmbiguousNameStaysApart(t *testing.T) {
	res := Resolve([]Row{
		row("wta", "2021-2047", "Chicago 1", "tour", 2021),
		row("wta", "2021-2048", "Chicago 2", "tour", 2021),
		row("wta", "1970-1030", "Chicago", "tour", 1970),
	}, nil)
	keys := byKey(t, res)
	if ev, ok := keys["wta name:tour:chicago"]; !ok || links(ev)[LinkName] != 1 {
		t.Errorf("Chicago 1970 should be its own event: %+v", keys)
	}
	if len(res.Ambiguous) != 1 || res.Ambiguous[0] != "wta tour chicago" {
		t.Errorf("Ambiguous = %v", res.Ambiguous)
	}
}

// The same name more than once in a season is an event per ordinal: the
// weekly ITF events of one venue, three in 2016 and two in 2017, are three
// runs, the first bare and the rest numbered in calendar order.
func TestSameNameWithinASeasonIsNumbered(t *testing.T) {
	dated := func(sourceID, name string, season int, month time.Month) Row {
		r := row("wta", sourceID, name, "itf", season)
		r.StartDate = time.Date(season, month, 1, 0, 0, 0, 0, time.UTC)
		return r
	}
	res := Resolve([]Row{
		dated("2016-W-ITF-TUR-09A-2016", "Antalya $10K", 2016, time.March),
		dated("2016-W-ITF-TUR-02A-2016", "Antalya $10K", 2016, time.January),
		dated("2016-W-ITF-TUR-05A-2016", "Antalya $10K", 2016, time.February),
		dated("2017-W-ITF-TUR-01A-2017", "Antalya 10K", 2017, time.January),
		dated("2017-W-ITF-TUR-03A-2017", "Antalya 10K", 2017, time.February),
	}, nil)
	keys := byKey(t, res)
	first, ok := keys["wta name:itf:antalya-10k"]
	if !ok || len(first.Editions) != 2 || first.Name != "Antalya 10K" || first.Ordinal != 0 {
		t.Fatalf("first of the season: %+v", first)
	}
	if first.Editions[0].Row.SourceID != "2016-W-ITF-TUR-02A-2016" {
		t.Errorf("the January event should be the first, got %s", first.Editions[0].Row.SourceID)
	}
	second, ok := keys["wta name:itf:antalya-10k:2"]
	if !ok || len(second.Editions) != 2 || second.Name != "Antalya 10K 2" || second.Ordinal != 2 {
		t.Fatalf("second of the season: %+v", second)
	}
	third, ok := keys["wta name:itf:antalya-10k:3"]
	if !ok || len(third.Editions) != 1 || third.FirstSeason != 2016 || third.Name != "Antalya $10K 3" {
		t.Fatalf("third of the season: %+v", third)
	}
	if res.Numbered != 3 || res.ByLink[LinkName] != 5 {
		t.Errorf("Numbered = %d, ByLink = %v", res.Numbered, res.ByLink)
	}
}

// Only the first of a name in a season bridges. The source's own "Adelaide 1"
// and "Adelaide 2" of 1972 normalise alike; the second is an event of its
// own, named as the source names it, rather than a second row of the numbered
// Adelaide's 1972.
func TestOnlyTheFirstOfASeasonBridges(t *testing.T) {
	jan := row("wta", "1972-1004", "Adelaide 1", "tour", 1972)
	jan.StartDate = time.Date(1972, time.January, 19, 0, 0, 0, 0, time.UTC)
	dec := row("wta", "1972-1176", "Adelaide 2", "tour", 1972)
	dec.StartDate = time.Date(1972, time.December, 11, 0, 0, 0, 0, time.UTC)
	res := Resolve([]Row{dec, jan, row("wta", "2024-2014", "Adelaide", "tour", 2024)}, nil)
	keys := byKey(t, res)
	numbered := keys["wta number:2014"]
	if len(numbered.Editions) != 2 || links(numbered)[LinkBridged] != 1 {
		t.Errorf("the numbered Adelaide should hold 1972's first and 2024: %+v", numbered)
	}
	if numbered.Editions[0].Row.SourceID != "1972-1004" {
		t.Errorf("the January event bridges, got %s", numbered.Editions[0].Row.SourceID)
	}
	second, ok := keys["wta name:tour:adelaide:2"]
	if !ok || second.Name != "Adelaide 2" || len(second.Editions) != 1 {
		t.Errorf("the December event: %+v", second)
	}
}

// A bridge never crosses tiers: a Futures Antalya is not the Challenger.
func TestBridgeStaysWithinTier(t *testing.T) {
	res := Resolve([]Row{
		row("atp", "2019-2205", "Antalya CH", "challenger", 2019),
		row("atp", "2019-M-ITF-TUR-01A-2019", "Antalya", "futures", 2019),
	}, nil)
	keys := byKey(t, res)
	if _, ok := keys["atp name:futures:antalya"]; !ok {
		t.Errorf("the Futures Antalya should stay name-keyed: %+v", keys)
	}
}

// Sackmann's M-codes reach their number through the name where it is unique,
// and through an override on the code where it is not.
func TestMCodes(t *testing.T) {
	overrides := &Overrides{Overrides: []Override{
		{Tour: "atp", Code: "M001", To: "338", Note: "Sydney sits under several numbers"},
	}}
	res := Resolve([]Row{
		row("atp", "2015-0404", "Indian Wells Masters", "tour", 2015),
		row("atp", "2016-M006", "Indian Wells Masters", "tour", 2016),
		row("atp", "2015-0338", "Sydney", "tour", 2015),
		row("atp", "2015-2817", "Sydney CH", "challenger", 2015),
		row("atp", "2015-9663", "Sydney", "tour", 2015),
		row("atp", "2016-M001", "Sydney", "tour", 2016),
	}, overrides)
	keys := byKey(t, res)
	if ev := keys["atp number:404"]; len(ev.Editions) != 2 || links(ev)[LinkBridged] != 1 {
		t.Errorf("M006 should bridge to 404: %+v", ev)
	}
	if ev := keys["atp number:338"]; len(ev.Editions) != 2 || links(ev)[LinkOverride] != 1 {
		t.Errorf("M001 should be filed under 338 by override: %+v", ev)
	}
}

// Team ties are the competition, one event per tour, renamed or not.
func TestTeamTies(t *testing.T) {
	res := Resolve([]Row{
		team("atp", "2019-M-DC-2019-FLS-A-M-FRA-JPN-01", "Davis Cup FLS A: FRA vs JPN", 2019),
		team("atp", "1968-D001", "Davis Cup EUR R1: FRA vs GBR", 1968),
		team("wta", "2019-W-FC-2019-WG-M-CZE-FRA-01", "Fed Cup WG: CZE vs FRA", 2019),
		team("wta", "2023-W-FC-2023-FLS", "BJK Cup Finals", 2023),
	}, nil)
	keys := byKey(t, res)
	dc, ok := keys["atp team:davis-cup"]
	if !ok || len(dc.Editions) != 2 || dc.Name != "Davis Cup" || dc.FirstSeason != 1968 {
		t.Errorf("Davis Cup: %+v", dc)
	}
	fc, ok := keys["wta team:billie-jean-king-cup"]
	if !ok || len(fc.Editions) != 2 || fc.Name != "Billie Jean King Cup" {
		t.Errorf("Billie Jean King Cup: %+v", fc)
	}
	if res.ByLink[LinkTeam] != 4 {
		t.Errorf("ByLink %v", res.ByLink)
	}
}

// A pinned name wins over the latest edition's, and an override that files
// nothing is reported rather than kept quietly.
func TestPinnedNameAndUnmatched(t *testing.T) {
	overrides := &Overrides{Overrides: []Override{
		{Tour: "wta", Number: "806", Name: "Canadian Open", Note: "alternates Montreal and Toronto"},
		{Tour: "wta", Number: "2030", To: "2014", Note: "Adelaide renumbered"},
	}}
	res := Resolve([]Row{
		row("wta", "2025-806", "Montreal", "tour", 2025),
		row("wta", "2026-806", "Toronto", "tour", 2026),
	}, overrides)
	ev := byKey(t, res)["wta number:806"]
	if ev.Name != "Canadian Open" {
		t.Errorf("name %q, want the pinned one", ev.Name)
	}
	if len(res.Unmatched) != 1 || res.Unmatched[0].Number != "2030" {
		t.Errorf("Unmatched = %+v", res.Unmatched)
	}
}

func TestOverrideValidation(t *testing.T) {
	bad := []Override{
		{Tour: "itf", Number: "1", To: "2"},
		{Tour: "atp", To: "2"},
		{Tour: "atp", Number: "1", Code: "M001", To: "2"},
		{Tour: "atp", Number: "x", To: "2"},
		{Tour: "atp", Code: "M1", To: "2"},
		{Tour: "atp", Number: "1"},
		{Tour: "atp", Number: "1", To: "M006"},
		{Tour: "atp", Code: "M006", Name: "Indian Wells"},
	}
	for _, o := range bad {
		if err := o.validate(); err == nil {
			t.Errorf("%+v should not validate", o)
		}
	}
	good := []Override{
		{Tour: "wta", Number: "580", To: "901"},
		{Tour: "atp", Code: "M006", To: "404"},
		{Tour: "wta", SourceID: "1924-1181", To: "650"},
		{Tour: "wta", Number: "806", Name: "Canadian Open"},
	}
	for _, o := range good {
		if err := o.validate(); err != nil {
			t.Errorf("%+v: %v", o, err)
		}
	}
}
