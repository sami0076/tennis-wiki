package ingest

import (
	"context"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/db"
	"github.com/sami0076/tennis-wiki/internal/testdb"
)

func testStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	dsn := testdb.Start(t)
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
		          rankings, ratings, ingest_runs, ingest_files RESTART IDENTITY CASCADE`); err != nil {
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

var wtaTour = Source{
	Name: "fixture-wta", Tour: TourWTA, Tier: "tour", Profile: "sackmann",
	BaseURL: "https://x", Path: "wta_matches_{season}.csv",
}

// Berkeley Pac Coast 1937 is filed under one tourney_id as two draw blocks,
// each numbering its matches from 1. Keyed on the number alone the two blocks'
// match 1 collapse into one row holding four players, which is two head-to-head
// records that never happened.
func TestSameMatchNumberDifferentPlayersStayApart(t *testing.T) {
	store, ctx := testStore(t)
	rows := fixtureRows(t, "wta_matches_1937.csv", wtaTour)
	if len(rows) != 6 {
		t.Fatalf("fixture has %d rows, want 6", len(rows))
	}

	res, err := store.WriteBatch(ctx, wtaTour, rows)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if res.Collapsed != 0 {
		t.Errorf("%d rows collapsed; every row is a different pair of players", res.Collapsed)
	}

	counts, err := store.Counts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if counts["matches"] != 6 {
		t.Errorf("%d matches written, want one per source row", counts["matches"])
	}
	if counts["match_players"] != 12 {
		t.Errorf("%d participants, want two per match", counts["match_players"])
	}

	var wrong int
	if err := store.pool.QueryRow(ctx, `
		SELECT count(*) FROM (
		  SELECT match_id FROM match_players GROUP BY match_id HAVING count(*) <> 2
		) x`).Scan(&wrong); err != nil {
		t.Fatal(err)
	}
	if wrong != 0 {
		t.Errorf("%d matches do not have exactly two participants", wrong)
	}

	// Both blocks kept their own match 1.
	var num1 int
	if err := store.pool.QueryRow(ctx,
		`SELECT count(*) FROM matches WHERE match_num = 1`).Scan(&num1); err != nil {
		t.Fatal(err)
	}
	if num1 != 2 {
		t.Errorf("%d matches numbered 1, want both blocks", num1)
	}
}

// The pair is unordered, so a source correcting who won updates the match
// instead of leaving the old result beside the new one.
func TestCorrectedWinnerUpdatesRatherThanDuplicates(t *testing.T) {
	store, ctx := testStore(t)
	rows := fixtureRows(t, "wta_matches_1937.csv", wtaTour)[:1]
	if _, err := store.WriteBatch(ctx, wtaTour, rows); err != nil {
		t.Fatalf("write: %v", err)
	}

	corrected := rows[0]
	corrected.Winner, corrected.Loser = rows[0].Loser, rows[0].Winner
	corrected.Score = "7-5 6-4"
	if _, err := store.WriteBatch(ctx, wtaTour, []MatchRow{corrected}); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	counts, err := store.Counts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if counts["matches"] != 1 {
		t.Errorf("%d matches, want the correction to replace the original", counts["matches"])
	}

	var score, winner string
	err = store.pool.QueryRow(ctx, `
		SELECT m.score, p.full_name FROM matches m
		  JOIN players p ON p.id = m.winner_id`).Scan(&score, &winner)
	if err != nil {
		t.Fatal(err)
	}
	if score != "7-5 6-4" || winner != corrected.Winner.Name {
		t.Errorf("match reads %s to %s, want the corrected result", score, winner)
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
