package rating

import (
	"math"
	"testing"
	"time"
)

func day(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		t.Fatalf("parse %s: %v", s, err)
	}
	return d
}

// collect runs a replay and returns everything it emitted.
func collect(t *testing.T, results []Result) ([]Snapshot, *Engine) {
	t.Helper()
	var out []Snapshot
	e := NewEngine(DefaultWeights(), func(s Snapshot) error {
		out = append(out, s)
		return nil
	})
	for _, r := range results {
		if err := e.Add(r); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	if err := e.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return out, e
}

func tourMatch(winner, loser int64, on time.Time, surface Series) Result {
	return Result{
		WinnerID: winner, LoserID: loser, PlayedOn: on, Surface: surface,
		Match: Match{Tier: TierTour, Level: LevelATP},
	}
}

func TestOneMatchMovesBothPlayers(t *testing.T) {
	snaps, e := collect(t, []Result{tourMatch(1, 2, day(t, "2020-01-08"), Hard)})

	// Two debutants at 1500: K(0)/2 each way, at a tour weight of 1.
	want := K(0) / 2
	if elo, n := e.Rating(1, Overall); math.Abs(elo-(Base+want)) > 1e-9 || n != 1 {
		t.Errorf("winner = %v after %d matches, want %v after 1", elo, n, Base+want)
	}
	if elo, _ := e.Rating(2, Overall); math.Abs(elo-(Base-want)) > 1e-9 {
		t.Errorf("loser = %v, want %v", elo, Base-want)
	}

	// Overall and hard for both players, in the week the match was played.
	if len(snaps) != 4 {
		t.Fatalf("got %d snapshots, want 4", len(snaps))
	}
	monday := day(t, "2020-01-06")
	for _, s := range snaps {
		if !s.AsOf.Equal(monday) {
			t.Errorf("snapshot as_of %s, want the Monday %s", s.AsOf, monday)
		}
	}
}

// A surface series only ever sees matches on that surface.
func TestSurfaceSeriesAreIndependent(t *testing.T) {
	_, e := collect(t, []Result{
		tourMatch(1, 2, day(t, "2020-01-08"), Clay),
		tourMatch(1, 2, day(t, "2020-01-09"), Clay),
	})

	if elo, n := e.Rating(1, Clay); n != 2 || elo <= Base {
		t.Errorf("clay = %v after %d matches, want two wins above base", elo, n)
	}
	if elo, n := e.Rating(1, Grass); n != 0 || elo != Base {
		t.Errorf("grass = %v after %d matches, want an untouched series", elo, n)
	}
	if _, n := e.Rating(1, Overall); n != 2 {
		t.Errorf("overall counted %d matches, want both", n)
	}
}

// A match with no recorded surface still counts, but only overall.
func TestUnknownSurfaceFeedsOverallOnly(t *testing.T) {
	snaps, e := collect(t, []Result{tourMatch(1, 2, day(t, "2020-01-08"), "")})
	if _, n := e.Rating(1, Overall); n != 1 {
		t.Errorf("overall counted %d matches, want 1", n)
	}
	if len(snaps) != 2 {
		t.Errorf("got %d snapshots, want one per player", len(snaps))
	}
}

// The snapshot rule: a row per series per week it moved, and nothing for the
// weeks in between.
func TestOnlyActiveWeeksAreSnapshotted(t *testing.T) {
	snaps, _ := collect(t, []Result{
		tourMatch(1, 2, day(t, "2020-01-08"), Hard),
		// Six weeks later, and only one of the two players is involved.
		tourMatch(1, 3, day(t, "2020-02-19"), Hard),
	})

	weeks := map[string]int{}
	for _, s := range snaps {
		weeks[s.AsOf.Format(time.DateOnly)]++
	}
	if len(weeks) != 2 {
		t.Fatalf("snapshots span %d weeks, want the 2 that were played", len(weeks))
	}
	if got := weeks["2020-01-06"]; got != 4 {
		t.Errorf("first week has %d rows, want 4", got)
	}
	if got := weeks["2020-02-17"]; got != 4 {
		t.Errorf("second week has %d rows, want 4 for the two players who played", got)
	}
	// Player 2 sat out the second week and has no row in it.
	for _, s := range snaps {
		if s.PlayerID == 2 && s.AsOf.Equal(day(t, "2020-02-17")) {
			t.Error("a player who did not play was snapshotted")
		}
	}
}

// Several matches in one week collapse to a single end-of-week row per series.
func TestAWeekYieldsOneRowPerSeries(t *testing.T) {
	snaps, e := collect(t, []Result{
		tourMatch(1, 2, day(t, "2020-01-06"), Hard),
		tourMatch(1, 3, day(t, "2020-01-08"), Hard),
		tourMatch(1, 4, day(t, "2020-01-10"), Hard),
	})

	var rows int
	for _, s := range snaps {
		if s.PlayerID == 1 && s.Series == Overall {
			rows++
			if s.Matches != 3 {
				t.Errorf("snapshot records %d matches, want all 3 from the week", s.Matches)
			}
			if elo, _ := e.Rating(1, Overall); s.Elo != elo {
				t.Errorf("snapshot elo %v, want the end-of-week %v", s.Elo, elo)
			}
		}
	}
	if rows != 1 {
		t.Errorf("%d overall rows for one week, want 1", rows)
	}
}

// K is read before the counts move, so the first match of a career is rated at
// K(0) rather than K(1).
func TestKUsesMatchesBeforeTheResult(t *testing.T) {
	_, e := collect(t, []Result{tourMatch(1, 2, day(t, "2020-01-08"), Hard)})
	elo, _ := e.Rating(1, Overall)
	if math.Abs(elo-(Base+K(0)/2)) > 1e-9 {
		t.Errorf("first match rated at %v, want K(0) of %v", elo-Base, K(0)/2)
	}
}

// The weight scales the whole move: a Grand Slam final counts for more than a
// Futures first round between the same two players.
func TestWeightScalesTheMove(t *testing.T) {
	slam := Result{WinnerID: 1, LoserID: 2, PlayedOn: day(t, "2020-01-08"), Surface: Hard,
		Match: Match{Tier: TierTour, Level: LevelGrandSlam, IsFinal: true}}
	futures := Result{WinnerID: 3, LoserID: 4, PlayedOn: day(t, "2020-01-08"), Surface: Hard,
		Match: Match{Tier: TierFutures, Level: LevelSatellite}}

	_, e := collect(t, []Result{slam, futures})
	big, _ := e.Rating(1, Overall)
	small, _ := e.Rating(3, Overall)
	if big-Base <= small-Base {
		t.Errorf("slam final moved %v, futures moved %v", big-Base, small-Base)
	}
	if ratio := (big - Base) / (small - Base); math.Abs(ratio-1.20/0.60) > 1e-9 {
		t.Errorf("ratio of moves = %v, want the ratio of the weights", ratio)
	}
}

// Weeks are closed as soon as the next one opens, so a result arriving late
// would be snapshotted into a week that has already been written.
func TestOutOfOrderResultsAreRefused(t *testing.T) {
	e := NewEngine(DefaultWeights(), func(Snapshot) error { return nil })
	if err := e.Add(tourMatch(1, 2, day(t, "2020-02-19"), Hard)); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := e.Add(tourMatch(1, 2, day(t, "2020-01-08"), Hard)); err == nil {
		t.Error("a result from an earlier week was accepted")
	}
}

// Two replays of the same matches produce the same rows in the same order,
// which is what makes a recompute safe to repeat.
func TestReplayIsDeterministic(t *testing.T) {
	results := []Result{
		tourMatch(7, 3, day(t, "2020-01-08"), Hard),
		tourMatch(3, 9, day(t, "2020-01-09"), Clay),
		tourMatch(9, 7, day(t, "2020-01-15"), ""),
	}
	first, _ := collect(t, results)
	second, _ := collect(t, results)

	if len(first) != len(second) {
		t.Fatalf("%d rows then %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("row %d differs: %+v then %+v", i, first[i], second[i])
		}
	}
	// Sorted within a week, so the order does not depend on map iteration.
	for i := 1; i < len(first); i++ {
		a, b := first[i-1], first[i]
		if a.AsOf.Equal(b.AsOf) && a.PlayerID > b.PlayerID {
			t.Errorf("rows %d and %d are out of order within a week", i-1, i)
		}
	}
}

func TestWeekOf(t *testing.T) {
	monday := day(t, "2020-01-06")
	for _, d := range []string{"2020-01-06", "2020-01-08", "2020-01-12"} {
		if got := WeekOf(day(t, d)); !got.Equal(monday) {
			t.Errorf("WeekOf(%s) = %s, want %s", d, got, monday)
		}
	}
	if got := WeekOf(day(t, "2020-01-13")); !got.Equal(day(t, "2020-01-13")) {
		t.Errorf("WeekOf(2020-01-13) = %s, want the Monday itself", got)
	}
}
