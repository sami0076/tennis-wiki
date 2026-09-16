package charting

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sami0076/tennis-wiki/internal/db"
	"github.com/sami0076/tennis-wiki/internal/ingest"
	"github.com/sami0076/tennis-wiki/internal/testdb"
)

type fixture struct {
	pool *pgxpool.Pool
	ctx  context.Context
	t    *testing.T
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	pool, err := db.Open(ctx, db.Config{DSN: testdb.Start(t), MaxConns: 4, MinConns: 1})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE charted_stats, charted_matches, match_players, matches,
		tournaments, players, unresolved_references, ingest_files RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return &fixture{pool: pool, ctx: ctx, t: t}
}

func (f *fixture) player(sourceID, full string) int64 {
	f.t.Helper()
	var id int64
	if err := f.pool.QueryRow(f.ctx,
		`INSERT INTO players (source_id, tour, slug, full_name) VALUES ($1, 'atp', $1, $2) RETURNING id`,
		sourceID, full).Scan(&id); err != nil {
		f.t.Fatal(err)
	}
	return id
}

func (f *fixture) match(event string, start time.Time, round string, winner, loser int64) int64 {
	f.t.Helper()
	var tid int64
	if err := f.pool.QueryRow(f.ctx, `
		INSERT INTO tournaments (source_id, tour, name, level, tier, surface, start_date, season)
		VALUES ($1, 'atp', $1, 'G', 'tour', 'clay', $2, $3)
		ON CONFLICT (source_id, season, tour) DO UPDATE SET name = EXCLUDED.name RETURNING id`,
		event, start, start.Year()).Scan(&tid); err != nil {
		f.t.Fatal(err)
	}
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		f.t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(f.ctx) }()
	var id int64
	if err := tx.QueryRow(f.ctx, `
		INSERT INTO matches (tournament_id, match_num, round, best_of, winner_id, loser_id, played_on, source)
		VALUES ($1, (SELECT coalesce(max(match_num), 0) + 1 FROM matches WHERE tournament_id = $1),
		        $2, 3, $3, $4, $5, 'test') RETURNING id`,
		tid, round, winner, loser, start).Scan(&id); err != nil {
		f.t.Fatal(err)
	}
	for _, p := range []struct {
		id  int64
		won bool
	}{{winner, true}, {loser, false}} {
		if _, err := tx.Exec(f.ctx,
			`INSERT INTO match_players (match_id, player_id, won) VALUES ($1, $2, $3)`, id, p.id, p.won); err != nil {
			f.t.Fatal(err)
		}
	}
	if err := tx.Commit(f.ctx); err != nil {
		f.t.Fatal(err)
	}
	return id
}

func (f *fixture) count(q string, args ...any) int {
	f.t.Helper()
	var n int
	if err := f.pool.QueryRow(f.ctx, q, args...).Scan(&n); err != nil {
		f.t.Fatalf("%s: %v", q, err)
	}
	return n
}

func (f *fixture) loader() *Loader {
	return &Loader{
		Sources: []ingest.ChartingSource{{Name: "mcp-atp", Tour: ingest.TourATP,
			BaseURL: "x", Matches: "charting-m-matches.csv", Stats: "charting-m-stats-Overview.csv"}},
		Fetcher: ingest.LocalFetcher{Root: "testdata"},
		Store:   NewStore(f.pool),
		Ledger:  ingest.NewStore(f.pool),
	}
}

// The fixture's Roland Garros qualifier against a database that has the row:
// one charted match, its figures under the right player ids, and a second run
// that changes nothing.
func TestRunAttachesFiguresAndIsIdempotent(t *testing.T) {
	f := newFixture(t)
	deJong := f.player("100001", "Jesper De Jong")
	zheng := f.player("100002", "Michael Zheng")
	rg := f.match("roland-garros", day(t, "2026-05-25"), "Q3", deJong, zheng)

	first, err := f.loader().Run(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if first.Resolved != 1 || first.Charted != 4 {
		t.Fatalf("first run: %+v, want 1 of 4 resolved", first)
	}
	if n := f.count(`SELECT count(*) FROM charted_matches WHERE match_id = $1`, rg); n != 1 {
		t.Errorf("%d charted rows for the match, want 1", n)
	}
	if n := f.count(`SELECT serve_points FROM charted_stats WHERE match_id = $1 AND player_id = $2 AND set_no = 0`,
		rg, deJong); n != 80 {
		t.Errorf("De Jong's total serve points = %d, want 80", n)
	}
	if n := f.count(`SELECT count(*) FROM charted_stats WHERE match_id = $1 AND player_id = $2 AND set_no > 0`,
		rg, zheng); n == 0 {
		t.Error("no per-set rows for Zheng")
	}
	if n := f.count(`SELECT count(*) FROM unresolved_references WHERE source = 'mcp-atp'`); n != 3 {
		t.Errorf("%d unresolved recorded, want the other three", n)
	}

	second, err := f.loader().Run(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if second.FilesSkipped != 2 {
		t.Errorf("second run read the files again: %+v", second)
	}
	stats := f.count(`SELECT count(*) FROM charted_stats`)
	forced := f.loader()
	forced.Force = true
	third, err := forced.Run(f.ctx)
	if err != nil || third.Resolved != 1 {
		t.Fatalf("forced third run: %+v, %v", third, err)
	}
	if n := f.count(`SELECT count(*) FROM charted_stats`); n != stats {
		t.Errorf("a re-read changed the row count from %d to %d", stats, n)
	}
}

// A match that resolved once and later resolves elsewhere -- the row it was
// attached to was re-ingested under another id -- moves rather than doubles,
// and a match that was unresolved and now resolves leaves the unresolved list.
func TestWriteReplacesUnderEitherKey(t *testing.T) {
	f := newFixture(t)
	deJong := f.player("100001", "Jesper De Jong")
	zheng := f.player("100002", "Michael Zheng")
	old := f.match("roland-garros", day(t, "2026-05-25"), "Q3", deJong, zheng)

	loader := f.loader()
	loader.Force = true
	if _, err := loader.Run(f.ctx); err != nil {
		t.Fatal(err)
	}
	// Now the same charting id resolves to a different row.
	if _, err := f.pool.Exec(f.ctx, `UPDATE matches SET round = 'Q2' WHERE id = $1`, old); err != nil {
		t.Fatal(err)
	}
	fresh := f.match("roland-garros", day(t, "2026-05-25"), "Q3", deJong, zheng)
	if _, err := loader.Run(f.ctx); err != nil {
		t.Fatal(err)
	}
	if n := f.count(`SELECT count(*) FROM charted_matches`); n != 1 {
		t.Errorf("%d charted rows, want 1", n)
	}
	if n := f.count(`SELECT count(*) FROM charted_matches WHERE match_id = $1`, fresh); n != 1 {
		t.Error("the charted match did not move to the row it now resolves to")
	}

	// The Davis Cup tie was unresolved; give it a row and it resolves.
	botic := f.player("100003", "Botic Van De Zandschulp")
	berrettini := f.player("100004", "Matteo Berrettini")
	f.match("davis-cup", day(t, "2024-09-13"), "RR", berrettini, botic)
	if _, err := loader.Run(f.ctx); err != nil {
		t.Fatal(err)
	}
	if n := f.count(`SELECT count(*) FROM unresolved_references WHERE source_id LIKE '20240915-M-Davis%'`); n != 0 {
		t.Error("a charted match that now resolves is still listed as unresolved")
	}

	// And the reverse: take the row away and the attachment goes with it.
	if _, err := f.pool.Exec(f.ctx, `UPDATE matches SET round = 'Q1' WHERE id = $1`, fresh); err != nil {
		t.Fatal(err)
	}
	if _, err := loader.Run(f.ctx); err != nil {
		t.Fatal(err)
	}
	if n := f.count(`SELECT count(*) FROM charted_matches WHERE match_id = $1`, fresh); n != 0 {
		t.Error("a charted match that no longer resolves is still attached")
	}
}
