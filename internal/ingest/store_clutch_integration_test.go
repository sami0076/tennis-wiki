package ingest

import (
	"testing"
	"time"
)

// clutchRows are scores chosen to exercise the derivation rather than to be a
// tournament: a tiebreak the match winner lost, a tiebreak with no points
// written down, a deciding set each way, and a retirement in a third set that
// nobody won.
func clutchRows() []MatchRow {
	scores := []string{
		"7-6(5) 6-4",      // one tiebreak, no deciding set
		"6-7(4) 6-3 6-4",  // a tiebreak the match winner lost
		"7-6 6-7 6-4",     // two tiebreaks, points never written down
		"6-4 3-6 7-6(2)",  // a deciding set won in a tiebreak
		"6-2 6-1",         // neither
		"6-4 4-6 2-1 RET", // a third set nobody won
	}
	rows := make([]MatchRow, 0, len(scores))
	for i, s := range scores {
		// Alternating winners, so neither side can come out right by accident.
		winner, loser := "clutch-a", "clutch-b"
		if i%2 == 1 {
			winner, loser = loser, winner
		}
		rows = append(rows, MatchRow{
			TourneyID: "clutch-open", TourneyName: "Clutch Open", Surface: "hard",
			Level: "A", TourneyDate: time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC),
			MatchNum: i + 1, Round: "R32", BestOf: 3, Score: s,
			Winner: Player{SourceID: winner, Name: "Clutch " + winner, Country: "USA", Hand: "R"},
			Loser:  Player{SourceID: loser, Name: "Clutch " + loser, Country: "USA", Hand: "R"},
		})
	}
	return rows
}

// The property that says the derivation is balanced: every tiebreak has a
// winner and a loser, both of them in the same match, so a population that
// counts both sides wins exactly half of its own tiebreaks. The same holds for
// deciding sets.
//
// It is worth asserting rather than assuming. A baseline that came out at
// anything other than half would mean the two columns disagree about who won,
// which is the one way this derivation can be wrong without looking wrong.
func TestClutchBaselineIsBalanced(t *testing.T) {
	store, ctx := testStore(t)

	if _, err := store.WriteBatch(ctx, atpTour, clutchRows()); err != nil {
		t.Fatalf("write matches: %v", err)
	}
	if _, err := store.RefreshClutch(ctx, false); err != nil {
		t.Fatalf("refresh clutch: %v", err)
	}

	var tbWon, tbPlayed, decWon, decPlayed int64
	err := store.pool.QueryRow(ctx, `
		SELECT coalesce(sum(tiebreaks_won), 0), coalesce(sum(tiebreaks_played), 0),
		       coalesce(sum(deciding_sets_won), 0), coalesce(sum(deciding_sets_played), 0)
		  FROM clutch_baselines`).Scan(&tbWon, &tbPlayed, &decWon, &decPlayed)
	if err != nil {
		t.Fatalf("read baselines: %v", err)
	}

	// Five finished matches carry five tiebreaks between them and three deciding
	// sets, each counted from both sides. The retirement carries neither,
	// because nobody won that third set.
	if tbPlayed != 10 {
		t.Errorf("tiebreaks played = %d, want 10: five tiebreaks counted from both sides", tbPlayed)
	}
	if decPlayed != 6 {
		t.Errorf("deciding sets played = %d, want 6: three matches counted from both sides", decPlayed)
	}
	if tbWon*2 != tbPlayed {
		t.Errorf("tiebreaks: %d won of %d played, want exactly half", tbWon, tbPlayed)
	}
	if decWon*2 != decPlayed {
		t.Errorf("deciding sets: %d won of %d played, want exactly half", decWon, decPlayed)
	}
}

// A match nobody could parse a score for keeps NULL columns. Zero would say the
// match was played and had no tiebreaks, which is a different claim.
func TestClutchColumnsAbsentRatherThanZero(t *testing.T) {
	store, ctx := testStore(t)

	if _, err := store.WriteBatch(ctx, atpTour, clutchRows()); err != nil {
		t.Fatalf("write matches: %v", err)
	}
	if _, err := store.RefreshClutch(ctx, false); err != nil {
		t.Fatalf("refresh clutch: %v", err)
	}

	var derived, incompleteWithColumns int64
	if err := store.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE deciding_set IS NOT NULL),
		       count(*) FILTER (WHERE incomplete AND deciding_set IS NOT NULL)
		  FROM matches`).Scan(&derived, &incompleteWithColumns); err != nil {
		t.Fatalf("count derived: %v", err)
	}

	if derived == 0 {
		t.Fatal("nothing was derived, so the fixture proves nothing")
	}
	if incompleteWithColumns != 0 {
		t.Errorf("%d unfinished matches carry a deciding set; nobody won those",
			incompleteWithColumns)
	}
}

// Running it twice writes nothing the second time, which is what makes it safe
// to end every ingest with.
func TestRefreshClutchIsIdempotent(t *testing.T) {
	store, ctx := testStore(t)

	if _, err := store.WriteBatch(ctx, atpTour, clutchRows()); err != nil {
		t.Fatalf("write matches: %v", err)
	}

	// The ingest already derives on write, so the first refresh has nothing
	// left to do either. Both runs being zero is the point.
	first, err := store.RefreshClutch(ctx, false)
	if err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	second, err := store.RefreshClutch(ctx, false)
	if err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	if second != 0 {
		t.Errorf("second refresh derived %d matches after %d, want none", second, first)
	}

	// Forcing it re-derives every match, which is what a change to the
	// derivation itself needs.
	forced, err := store.RefreshClutch(ctx, true)
	if err != nil {
		t.Fatalf("forced refresh: %v", err)
	}
	if forced == 0 {
		t.Error("forcing derived nothing, so a change to the derivation could not be applied")
	}
}

// The serve baseline anchors ADR-0007's inversion, so the totals it stores have
// to be the ones the anchor lookup expects: serve points and points won, per
// tour, tier, surface and decade.
func TestServeBaselinesAreBuiltFromRecordedLinesOnly(t *testing.T) {
	store, ctx := testStore(t)
	rows := fixtureRows(t, "atp_matches_2024.csv", atpTour)

	if _, err := store.WriteBatch(ctx, atpTour, rows); err != nil {
		t.Fatalf("write matches: %v", err)
	}
	if err := store.RefreshServeBaselines(ctx); err != nil {
		t.Fatalf("refresh serve baselines: %v", err)
	}

	var cells, points, won int64
	if err := store.pool.QueryRow(ctx, `
		SELECT count(*), coalesce(sum(serve_points), 0), coalesce(sum(serve_won), 0)
		  FROM serve_baselines`).Scan(&cells, &points, &won); err != nil {
		t.Fatalf("read serve baselines: %v", err)
	}

	if cells == 0 || points == 0 {
		t.Fatalf("baselines are empty: %d cells over %d points", cells, points)
	}
	// A rate outside this range would mean the numerator and denominator are
	// not what the column names say.
	if rate := float64(won) / float64(points); rate < 0.4 || rate > 0.8 {
		t.Errorf("serve points won = %.3f, which is not a tennis number", rate)
	}

	// Every point counted must come from a match_players row that actually had
	// one. Nulls counted as zero would drag the anchor down invisibly.
	var recorded int64
	if err := store.pool.QueryRow(ctx, `
		SELECT coalesce(sum(mp.serve_points), 0)
		  FROM match_players mp
		  JOIN matches m ON m.id = mp.match_id
		 WHERE mp.serve_points IS NOT NULL AND m.surface IS NOT NULL
		   AND NOT m.is_team_event`).Scan(&recorded); err != nil {
		t.Fatalf("count recorded points: %v", err)
	}
	if points != recorded {
		t.Errorf("baselines hold %d points, the recorded lines hold %d", points, recorded)
	}
}

// Running it twice leaves the same table, which is what makes it safe at the
// end of every ingest.
func TestServeBaselinesRebuildCleanly(t *testing.T) {
	store, ctx := testStore(t)
	rows := fixtureRows(t, "atp_matches_2024.csv", atpTour)

	if _, err := store.WriteBatch(ctx, atpTour, rows); err != nil {
		t.Fatalf("write matches: %v", err)
	}
	var first, second int64
	for _, into := range []*int64{&first, &second} {
		if err := store.RefreshServeBaselines(ctx); err != nil {
			t.Fatalf("refresh serve baselines: %v", err)
		}
		if err := store.pool.QueryRow(ctx,
			`SELECT count(*) FROM serve_baselines`).Scan(into); err != nil {
			t.Fatalf("count cells: %v", err)
		}
	}
	if first != second || first == 0 {
		t.Errorf("rebuild changed the table: %d cells then %d", first, second)
	}
}
