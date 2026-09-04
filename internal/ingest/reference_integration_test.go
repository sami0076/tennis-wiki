package ingest

import (
	"testing"
	"time"
)

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("bad date %q: %v", s, err)
	}
	return d
}

func i16(v int16) *int16 { return &v }
func i32(v int32) *int32 { return &v }

// The central guarantee, as for matches: running twice changes nothing.
func TestWritePlayersIsIdempotent(t *testing.T) {
	store, ctx := testStore(t)
	dob := mustDate(t, "1987-05-22")
	bios := []PlayerBio{
		{SourceID: "1", FirstName: "Novak", LastName: "Djokovic", Hand: "R",
			BirthDate: &dob, Country: "SRB", HeightCm: i16(188), WikidataID: "Q5812"},
		{SourceID: "2", FirstName: "Rafael", LastName: "Nadal", Hand: "L", Country: "ESP"},
	}

	if _, err := store.WritePlayers(ctx, TourATP, bios); err != nil {
		t.Fatalf("first write: %v", err)
	}
	first, err := store.TableChecksum(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		if _, err := store.WritePlayers(ctx, TourATP, bios); err != nil {
			t.Fatalf("rewrite %d: %v", i, err)
		}
	}
	second, err := store.TableChecksum(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Errorf("re-ingest changed the database:\n  first  %s\n  second %s", first, second)
	}

	var name, country, wikidata string
	var height int16
	var birth time.Time
	err = store.pool.QueryRow(ctx,
		`SELECT full_name, country, wikidata_id, height_cm, birth_date
		   FROM players WHERE source_id = '1' AND tour = 'atp'`).
		Scan(&name, &country, &wikidata, &height, &birth)
	if err != nil {
		t.Fatal(err)
	}
	if name != "Novak Djokovic" || country != "SRB" || wikidata != "Q5812" || height != 188 {
		t.Errorf("stored %s / %s / %s / %d", name, country, wikidata, height)
	}
	if !birth.Equal(dob) {
		t.Errorf("birth_date = %v, want %v", birth, dob)
	}
}

// Match ingest writes a player from a match file; the player table then fills in
// the biography. The slug must not move, because it is the URL.
func TestPlayerTableFillsBiographyWithoutMovingTheSlug(t *testing.T) {
	store, ctx := testStore(t)

	// As match ingest would write it: a name, and nothing else.
	if _, err := store.WritePlayers(ctx, TourATP, []PlayerBio{
		{SourceID: "7", FirstName: "Roger", LastName: "Federer"},
	}); err != nil {
		t.Fatal(err)
	}
	var slugBefore string
	if err := store.pool.QueryRow(ctx,
		`SELECT slug FROM players WHERE source_id = '7' AND tour = 'atp'`).Scan(&slugBefore); err != nil {
		t.Fatal(err)
	}

	dob := mustDate(t, "1981-08-08")
	if _, err := store.WritePlayers(ctx, TourATP, []PlayerBio{
		{SourceID: "7", FirstName: "Roger", LastName: "Federer", Hand: "R",
			BirthDate: &dob, Country: "SUI", HeightCm: i16(185)},
	}); err != nil {
		t.Fatal(err)
	}

	var slugAfter, country string
	var birth time.Time
	if err := store.pool.QueryRow(ctx,
		`SELECT slug, country, birth_date FROM players WHERE source_id = '7' AND tour = 'atp'`).
		Scan(&slugAfter, &country, &birth); err != nil {
		t.Fatal(err)
	}
	if slugAfter != slugBefore {
		t.Errorf("slug moved from %q to %q; that breaks every URL to the page", slugBefore, slugAfter)
	}
	if country != "SUI" || !birth.Equal(dob) {
		t.Errorf("biography was not filled in: country=%s birth=%v", country, birth)
	}
}

func TestWriteRankingsIsIdempotent(t *testing.T) {
	store, ctx := testStore(t)
	if _, err := store.WritePlayers(ctx, TourATP, []PlayerBio{
		{SourceID: "1", FirstName: "A", LastName: "One"},
		{SourceID: "2", FirstName: "B", LastName: "Two"},
	}); err != nil {
		t.Fatal(err)
	}
	ids, err := store.PlayerIDsBySource(ctx, TourATP)
	if err != nil {
		t.Fatal(err)
	}

	date := mustDate(t, "2019-01-07")
	batch := []rankingWrite{
		{playerID: ids["1"], date: date, rank: 1, points: i32(10550)},
		{playerID: ids["2"], date: date, rank: 2, points: nil},
	}
	if _, err := store.WriteRankings(ctx, batch); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if _, err := store.WriteRankings(ctx, batch); err != nil {
		t.Fatalf("second write: %v", err)
	}

	var n int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM rankings`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("%d ranking rows after two identical writes, want 2", n)
	}

	var points *int32
	if err := store.pool.QueryRow(ctx,
		`SELECT points FROM rankings WHERE player_id = $1`, ids["2"]).Scan(&points); err != nil {
		t.Fatal(err)
	}
	if points != nil {
		t.Errorf("absent points became %d; zero points is a real value and must stay distinct", *points)
	}
}

// Postgres refuses an ON CONFLICT that would touch the same row twice in one
// statement, and the sources do repeat a player on a date.
func TestWriteRankingsToleratesDuplicatesInOneBatch(t *testing.T) {
	store, ctx := testStore(t)
	if _, err := store.WritePlayers(ctx, TourATP,
		[]PlayerBio{{SourceID: "1", FirstName: "A", LastName: "One"}}); err != nil {
		t.Fatal(err)
	}
	ids, err := store.PlayerIDsBySource(ctx, TourATP)
	if err != nil {
		t.Fatal(err)
	}

	date := mustDate(t, "2019-01-07")
	batch := []rankingWrite{
		{playerID: ids["1"], date: date, rank: 5, points: i32(100)},
		{playerID: ids["1"], date: date, rank: 5, points: i32(100)},
	}
	if _, err := store.WriteRankings(ctx, batch); err != nil {
		t.Fatalf("a duplicated row in one batch failed the whole batch: %v", err)
	}
}

func TestRecordUnresolvedIsIdempotent(t *testing.T) {
	store, ctx := testStore(t)
	counts := map[string]int{"999999": 12, "888888": 3}

	for i := 0; i < 2; i++ {
		if err := store.RecordUnresolved(ctx, "test-source", "rankings", counts); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}

	var rows int
	var occurrences int64
	if err := store.pool.QueryRow(ctx,
		`SELECT count(*), coalesce(max(occurrences), 0) FROM unresolved_references`).
		Scan(&rows, &occurrences); err != nil {
		t.Fatal(err)
	}
	if rows != 2 {
		t.Errorf("%d unresolved rows after two identical writes, want 2", rows)
	}
	if occurrences != 12 {
		t.Errorf("occurrences = %d, want 12: the count is replaced, not accumulated", occurrences)
	}
}

// Rankings cannot resolve against players that were never written, so the
// loader has to refuse rather than record every row as unresolved.
func TestReferenceLoaderRefusesRankingsWithoutPlayers(t *testing.T) {
	store, ctx := testStore(t)
	loader := &ReferenceLoader{
		Sources: []RefSource{{
			Name: "r", Tour: TourATP, Kind: RefRankings, BaseURL: "https://x",
			Path: "atp_rankings_{decade}.csv", Decades: []string{"10s"}, Attribution: "a",
		}},
		Fetcher: LocalFetcher{Root: "testdata"},
		Store:   store,
	}
	if _, err := loader.Run(ctx); err == nil {
		t.Fatal("rankings were loaded with no players in the database")
	}
}

// End to end through the committed seed fixtures.
func TestReferenceLoaderLoadsSeedFixtures(t *testing.T) {
	store, ctx := testStore(t)
	const root = "../../testdata"

	loader := &ReferenceLoader{
		Sources: []RefSource{
			{Name: "atp-players", Tour: TourATP, Kind: RefPlayers, BaseURL: "https://x",
				Path: "atp_players.csv", Attribution: "a"},
			{Name: "atp-rankings", Tour: TourATP, Kind: RefRankings, BaseURL: "https://x",
				Path: "atp_rankings_{decade}.csv", Decades: []string{"10s", "70s", "90s"},
				Attribution: "a"},
		},
		Fetcher:   LocalFetcher{Root: root},
		Store:     store,
		BatchSize: 500,
	}

	stats, err := loader.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stats.Players == 0 || stats.RankingsWritten == 0 {
		t.Fatalf("loaded nothing: %+v", stats)
	}
	// The 90s file is not in the fixture; a missing decade is normal.
	if stats.FilesMissing == 0 {
		t.Error("expected the absent decade to be counted as missing")
	}

	var withDOB, withWikidata int
	if err := store.pool.QueryRow(ctx,
		`SELECT count(birth_date), count(wikidata_id) FROM players`).Scan(&withDOB, &withWikidata); err != nil {
		t.Fatal(err)
	}
	if withDOB == 0 {
		t.Error("no player got a date of birth; #7 matches on it")
	}
	if withWikidata == 0 {
		t.Error("no player got a wikidata id")
	}

	// Every ranking must point at a real player, or the foreign key is a lie.
	var orphans int
	if err := store.pool.QueryRow(ctx, `
		SELECT count(*) FROM rankings r
		 WHERE NOT EXISTS (SELECT 1 FROM players p WHERE p.id = r.player_id)`).Scan(&orphans); err != nil {
		t.Fatal(err)
	}
	if orphans != 0 {
		t.Errorf("%d rankings reference a missing player", orphans)
	}
}
