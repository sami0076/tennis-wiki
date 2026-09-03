package ingest

import (
	"context"
	"os"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// Integration tests need a migrated database. Set TEST_DATABASE_URL to run
// them; they skip otherwise so the unit suite stays runnable anywhere.
// Issue #13 replaces this with testcontainers so CI runs them automatically.
func testStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, db.Config{DSN: dsn, MaxConns: 4, MinConns: 1})
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(pool.Close)

	// Each test starts from an empty database. Truncating rather than dropping
	// keeps the schema, which migrations own.
	if _, err := pool.Exec(ctx,
		`TRUNCATE match_players, matches, tournaments, player_aliases, players,
		          rankings, ratings, ingest_runs RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return NewStore(pool), ctx
}

func fixtureRows(t *testing.T, name string, src Source) []MatchRow {
	t.Helper()
	return all(t, fixture(t, name, src))
}

var atpTour = Source{
	Name: "fixture-atp", Tour: TourATP, Tier: "tour", Profile: "sackmann",
	BaseURL: "https://x", Path: "atp_matches_{season}.csv",
}

// The central guarantee: running twice over identical input changes nothing.
func TestIdempotentReingest(t *testing.T) {
	store, ctx := testStore(t)
	rows := fixtureRows(t, "atp_matches_2024.csv", atpTour)

	if _, err := store.WriteBatch(ctx, atpTour, rows); err != nil {
		t.Fatalf("first write: %v", err)
	}
	firstCounts, err := store.Counts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	firstSum, err := store.TableChecksum(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		if _, err := store.WriteBatch(ctx, atpTour, rows); err != nil {
			t.Fatalf("rewrite %d: %v", i, err)
		}
	}

	secondCounts, err := store.Counts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	secondSum, err := store.TableChecksum(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for table, n := range firstCounts {
		if secondCounts[table] != n {
			t.Errorf("%s: %d rows after one pass, %d after four", table, n, secondCounts[table])
		}
	}
	if firstSum != secondSum {
		t.Errorf("table checksum changed across re-ingest:\n  first  %s\n  second %s", firstSum, secondSum)
	}
	if firstCounts["matches"] == 0 {
		t.Fatal("no matches written")
	}
	if firstCounts["match_players"] != firstCounts["matches"]*2 {
		t.Errorf("match_players = %d, want twice the %d matches",
			firstCounts["match_players"], firstCounts["matches"])
	}
}

// Pre-1991 statistics must land as NULL, never as zero.
func TestPre1991StatsStoredAsNull(t *testing.T) {
	store, ctx := testStore(t)
	src := atpTour
	rows := fixtureRows(t, "atp_matches_1969.csv", src)
	if _, err := store.WriteBatch(ctx, src, rows); err != nil {
		t.Fatalf("write: %v", err)
	}

	var nulls, zeros int
	err := store.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE serve_points IS NULL),
		       count(*) FILTER (WHERE serve_points = 0)
		  FROM match_players`).Scan(&nulls, &zeros)
	if err != nil {
		t.Fatal(err)
	}
	if nulls == 0 {
		t.Error("1969 matches should have NULL serve_points")
	}
	if zeros != 0 {
		t.Errorf("%d rows stored 0 serve points; unrecorded stats must be NULL", zeros)
	}

	var detailed int
	if err := store.pool.QueryRow(ctx,
		`SELECT count(*) FROM matches WHERE has_detailed_stats`).Scan(&detailed); err != nil {
		t.Fatal(err)
	}
	if detailed != 0 {
		t.Errorf("%d 1969 matches claim detailed stats", detailed)
	}
}

// The deferred foreign key must hold: every winner actually played.
func TestWinnerIsAlwaysAParticipant(t *testing.T) {
	store, ctx := testStore(t)
	rows := fixtureRows(t, "atp_matches_2024.csv", atpTour)
	if _, err := store.WriteBatch(ctx, atpTour, rows); err != nil {
		t.Fatalf("write: %v", err)
	}

	var orphans int
	err := store.pool.QueryRow(ctx, `
		SELECT count(*) FROM matches m
		 WHERE NOT EXISTS (
		   SELECT 1 FROM match_players mp
		    WHERE mp.match_id = m.id AND mp.player_id = m.winner_id AND mp.won
		 )`).Scan(&orphans)
	if err != nil {
		t.Fatal(err)
	}
	if orphans != 0 {
		t.Errorf("%d matches have a winner_id that did not win the match", orphans)
	}
}

func TestQualifyingAndTierPersisted(t *testing.T) {
	store, ctx := testStore(t)
	src := Source{
		Name: "fixture-chall", Tour: TourATP, Tier: "challenger", Profile: "sackmann",
		BaseURL: "https://x", Path: "atp_matches_qual_chall_{season}.csv",
	}
	rows := fixtureRows(t, "atp_matches_qual_chall_2022.csv", src)
	if _, err := store.WriteBatch(ctx, src, rows); err != nil {
		t.Fatalf("write: %v", err)
	}

	var qualifying int
	if err := store.pool.QueryRow(ctx,
		`SELECT count(*) FROM matches WHERE is_qualifying`).Scan(&qualifying); err != nil {
		t.Fatal(err)
	}
	if qualifying == 0 {
		t.Error("no qualifying matches stored from the qual_chall fixture")
	}

	var tier string
	if err := store.pool.QueryRow(ctx,
		`SELECT tier::text FROM tournaments LIMIT 1`).Scan(&tier); err != nil {
		t.Fatal(err)
	}
	if tier != "challenger" {
		t.Errorf("tier = %q, want challenger", tier)
	}
}

// Two players with the same name must not collide on slug.
func TestSlugCollisionResolved(t *testing.T) {
	store, ctx := testStore(t)
	src := atpTour

	rows := fixtureRows(t, "atp_matches_2024.csv", src)
	base := rows[0]
	// Same display name, different source id.
	twin := base
	twin.MatchNum = 9001
	twin.Winner.SourceID = base.Winner.SourceID + "-twin"
	twin.Loser.SourceID = base.Loser.SourceID + "-twin"

	if _, err := store.WriteBatch(ctx, src, []MatchRow{base, twin}); err != nil {
		t.Fatalf("write: %v", err)
	}

	var slugs, distinct int
	if err := store.pool.QueryRow(ctx,
		`SELECT count(slug), count(DISTINCT slug) FROM players`).Scan(&slugs, &distinct); err != nil {
		t.Fatal(err)
	}
	if slugs != distinct {
		t.Errorf("%d players share only %d distinct slugs", slugs, distinct)
	}
	if slugs < 4 {
		t.Errorf("expected four players, got %d", slugs)
	}
}

func TestIngestRunRecorded(t *testing.T) {
	store, ctx := testStore(t)

	id, err := store.StartRun(ctx, "test-source")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if err := store.FinishRun(ctx, id, 100, 90, nil); err != nil {
		t.Fatalf("FinishRun: %v", err)
	}

	var seen, written int
	var errText *string
	err = store.pool.QueryRow(ctx,
		`SELECT rows_seen, rows_written, error FROM ingest_runs WHERE id = $1`, id).
		Scan(&seen, &written, &errText)
	if err != nil {
		t.Fatal(err)
	}
	if seen != 100 || written != 90 || errText != nil {
		t.Errorf("run recorded as seen=%d written=%d err=%v", seen, written, errText)
	}

	// A failed run must leave its error behind rather than vanish.
	failID, err := store.StartRun(ctx, "failing-source")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinishRun(ctx, failID, 5, 0, context.DeadlineExceeded); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(ctx,
		`SELECT error FROM ingest_runs WHERE id = $1`, failID).Scan(&errText); err != nil {
		t.Fatal(err)
	}
	if errText == nil || *errText == "" {
		t.Error("a failed run should store its error text")
	}
}
