package ingest

import (
	"context"
	"testing"
)

// findStatRow returns the index of a row whose winner has statistics.
func findStatRow(t *testing.T, rows []MatchRow) int {
	t.Helper()
	for i, r := range rows {
		if r.Winner.Stats != nil && r.Loser.Stats != nil {
			return i
		}
	}
	t.Fatal("fixture has no match with statistics on both sides")
	return 0
}

// statFingerprint describes every stat line by natural key. TableChecksum
// cannot serve here: it carries surrogate ids, and those are not stable across
// the truncate this test does between the two states it compares.
func statFingerprint(ctx context.Context, t *testing.T, store *Store) string {
	t.Helper()
	var sum string
	err := store.pool.QueryRow(ctx, `
		SELECT md5(string_agg(t, '|' ORDER BY t)) FROM (
		  SELECT tr.source_id || ':' || m.match_num || ':' || p.source_id || ':' ||
		         m.has_detailed_stats::text || ':' ||
		         COALESCE(mp.serve_points::text, 'null') || ':' ||
		         COALESCE(mp.first_in::text, 'null') || ':' ||
		         COALESCE(mp.second_won::text, 'null') || ':' ||
		         COALESCE(mp.bp_saved::text, 'null') || ':' ||
		         COALESCE(mp.aces::text, 'null') AS t
		    FROM match_players mp
		    JOIN matches m     ON m.id = mp.match_id
		    JOIN tournaments tr ON tr.id = m.tournament_id
		    JOIN players p     ON p.id = mp.player_id
		) rows`).Scan(&sum)
	if err != nil {
		t.Fatalf("fingerprint: %v", err)
	}
	return sum
}

func truncateAll(ctx context.Context, t *testing.T, store *Store) {
	t.Helper()
	if _, err := store.pool.Exec(ctx,
		`TRUNCATE match_players, matches, tournaments, players RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

// A database ingested before the parser rejected these rows still holds them,
// and a re-ingest to clear a handful costs an hour over 1.6 million rows.
// Prune is the shortcut, so it has to land on exactly the same state.
func TestPruneReachesTheStateAReingestWould(t *testing.T) {
	store, ctx := testStore(t)
	rows := fixtureRows(t, "atp_matches_2024.csv", atpTour)
	i := findStatRow(t, rows)

	// What today's parser writes: the corrupt line is dropped, the match stays.
	reingested := append([]MatchRow(nil), rows...)
	reingested[i].Winner.Stats = nil
	if _, err := store.WriteBatch(ctx, atpTour, reingested); err != nil {
		t.Fatalf("write the re-ingested state: %v", err)
	}
	want := statFingerprint(ctx, t, store)

	// What the old parser left behind: six second serves in a match that had none.
	truncateAll(ctx, t, store)
	corrupt := append([]MatchRow(nil), rows...)
	stats := *rows[i].Winner.Stats
	stats.FirstIn = stats.ServePoints
	stats.SecondWon = 6
	corrupt[i].Winner.Stats = &stats
	if _, err := store.WriteBatch(ctx, atpTour, corrupt); err != nil {
		t.Fatalf("write the corrupt state: %v", err)
	}

	res, err := store.PruneStats(ctx, true)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if res.StatLines != 1 {
		t.Errorf("dry run found %d stat lines, want 1", res.StatLines)
	}
	if statFingerprint(ctx, t, store) == want {
		t.Error("dry run cleared the row it was only supposed to count")
	}

	res, err = store.PruneStats(ctx, false)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if res.StatLines != 1 || res.Matches != 1 {
		t.Errorf("prune cleared %d stat lines over %d matches, want 1 and 1",
			res.StatLines, res.Matches)
	}

	got := statFingerprint(ctx, t, store)
	if got != want {
		t.Errorf("pruned state differs from a re-ingest:\n  re-ingest %s\n  prune     %s", want, got)
	}
}

// Only the corrupt side loses its statistics, and the match keeps both players.
func TestPruneKeepsTheMatchAndTheOtherSide(t *testing.T) {
	store, ctx := testStore(t)
	rows := fixtureRows(t, "atp_matches_2024.csv", atpTour)
	i := findStatRow(t, rows)

	stats := *rows[i].Winner.Stats
	stats.BPSaved = stats.BPFaced + 1
	rows[i].Winner.Stats = &stats
	if _, err := store.WriteBatch(ctx, atpTour, rows); err != nil {
		t.Fatalf("write: %v", err)
	}
	before, err := store.Counts(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.PruneStats(ctx, false); err != nil {
		t.Fatalf("prune: %v", err)
	}

	after, err := store.Counts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"matches", "match_players", "players"} {
		if after[table] != before[table] {
			t.Errorf("%s: %d rows before prune, %d after", table, before[table], after[table])
		}
	}

	var cleared, kept int
	err = store.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE serve_points IS NULL),
		       count(*) FILTER (WHERE serve_points IS NOT NULL)
		  FROM match_players`).Scan(&cleared, &kept)
	if err != nil {
		t.Fatal(err)
	}
	if cleared != 1 {
		t.Errorf("%d stat lines cleared, want exactly the corrupt one", cleared)
	}
	if kept == 0 {
		t.Error("prune cleared every stat line")
	}

	// A second pass has nothing left to do.
	res, err := store.PruneStats(ctx, false)
	if err != nil {
		t.Fatalf("second prune: %v", err)
	}
	if res.StatLines != 0 {
		t.Errorf("second prune cleared %d more stat lines", res.StatLines)
	}
}
