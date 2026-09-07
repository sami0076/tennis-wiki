package rating

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sami0076/tennis-wiki/internal/db"
	"github.com/sami0076/tennis-wiki/internal/testdb"
)

type fixture struct {
	*Store
	pool *pgxpool.Pool
	ctx  context.Context
	t    *testing.T
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	pool, err := db.Open(ctx, db.Config{DSN: testdb.Start(t), MaxConns: 4, MinConns: 2})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, `TRUNCATE match_players, matches, tournaments, player_aliases,
		players, rankings, ratings, ingest_runs, identity_reviews RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return &fixture{Store: NewStore(pool), pool: pool, ctx: ctx, t: t}
}

func (f *fixture) player(name string) int64 {
	f.t.Helper()
	var id int64
	if err := f.pool.QueryRow(f.ctx,
		`INSERT INTO players (source_id, tour, slug, full_name)
		 VALUES ($1, 'atp', $1, $1) RETURNING id`, name).Scan(&id); err != nil {
		f.t.Fatalf("insert player %s: %v", name, err)
	}
	return id
}

// tournament puts every match on one date, which is what the source does and
// what makes the round order load-bearing.
func (f *fixture) tournament(sourceID, level, tier, on string) int64 {
	f.t.Helper()
	var id int64
	if err := f.pool.QueryRow(f.ctx, `
		INSERT INTO tournaments (source_id, tour, name, level, tier, surface, start_date, season)
		VALUES ($1, 'atp', $1, $2, $3::tier, 'hard', $4::date, extract(year FROM $4::date))
		RETURNING id`, sourceID, level, tier, on).Scan(&id); err != nil {
		f.t.Fatalf("insert tournament: %v", err)
	}
	return id
}

type matchSpec struct {
	score     string
	teamEvent bool
}

type matchOpt func(*matchSpec)

func withScore(s string) matchOpt { return func(m *matchSpec) { m.score = s } }
func asTeamEvent() matchOpt       { return func(m *matchSpec) { m.teamEvent = true } }

func (f *fixture) match(tournament, winner, loser int64, num int, round, on string, opts ...matchOpt) {
	f.t.Helper()
	spec := matchSpec{score: "6-4 6-4"}
	for _, o := range opts {
		o(&spec)
	}

	// One transaction: matches carries a deferred foreign key naming both
	// participants, so they have to land before the commit.
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		f.t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(f.ctx) }()

	var id int64
	err = tx.QueryRow(f.ctx, `
		INSERT INTO matches (tournament_id, match_num, round, best_of, surface, winner_id,
		                     loser_id, played_on, score, is_qualifying, is_team_event, source)
		VALUES ($1, $2, $3, 3, 'hard', $4, $5, $6::date, $7, $3 LIKE 'Q%', $8, 'test')
		RETURNING id`,
		tournament, num, round, winner, loser, on, spec.score, spec.teamEvent).Scan(&id)
	if err != nil {
		f.t.Fatalf("insert match: %v", err)
	}
	for _, p := range []struct {
		id  int64
		won bool
	}{{winner, true}, {loser, false}} {
		if _, err := tx.Exec(f.ctx,
			`INSERT INTO match_players (match_id, player_id, won) VALUES ($1, $2, $3)`,
			id, p.id, p.won); err != nil {
			f.t.Fatalf("insert match_player: %v", err)
		}
	}
	if err := tx.Commit(f.ctx); err != nil {
		f.t.Fatalf("commit match: %v", err)
	}
}

func (f *fixture) count(query string, args ...any) int {
	f.t.Helper()
	var n int
	if err := f.pool.QueryRow(f.ctx, query, args...).Scan(&n); err != nil {
		f.t.Fatalf("%s: %v", query, err)
	}
	return n
}

// dump renders the whole table in key order, so two recomputes can be compared
// as a whole rather than row by row.
func (f *fixture) dump() string {
	f.t.Helper()
	var out string
	if err := f.pool.QueryRow(f.ctx, `
		SELECT coalesce(string_agg(line, chr(10) ORDER BY line), '')
		  FROM (SELECT player_id || ' ' || as_of || ' ' || surface || ' ' || elo
		               || ' ' || matches_played AS line FROM ratings) x`).Scan(&out); err != nil {
		f.t.Fatalf("dump ratings: %v", err)
	}
	return out
}

// Every match of a draw carries the same date, so only the round order stops a
// final being rated before the semi-final that produced its finalist.
func TestMatchesArriveInDrawOrder(t *testing.T) {
	f := newFixture(t)
	a, b := f.player("a"), f.player("b")
	c, d := f.player("c"), f.player("d")
	open := f.tournament("open", "A", "tour", "2020-01-08")

	// The final is written first, precisely so insertion order cannot be what
	// saves the ordering.
	f.match(open, a, c, 3, "F", "2020-01-08")
	f.match(open, a, b, 1, "SF", "2020-01-08")
	f.match(open, c, d, 2, "SF", "2020-01-08")
	f.match(open, b, d, 7, "Q1", "2020-01-08")

	var order []string
	n, _, err := f.Matches(f.ctx, false, func(r Result) error {
		order = append(order, roundOf(r))
		return nil
	})
	if err != nil {
		t.Fatalf("Matches: %v", err)
	}
	if n != 4 {
		t.Fatalf("streamed %d matches, want 4", n)
	}
	want := []string{"qualifying", "semi", "semi", "final"}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

// roundOf recovers enough of the round from what the engine is given to assert
// the order, since Result carries the weighting inputs rather than the code.
func roundOf(r Result) string {
	switch {
	case r.Match.IsQualifying:
		return "qualifying"
	case r.Match.IsFinal:
		return "final"
	default:
		return "semi"
	}
}

func TestExcludedMatchesAreCountedNotRated(t *testing.T) {
	f := newFixture(t)
	a, b := f.player("a"), f.player("b")
	open := f.tournament("open", "A", "tour", "2020-01-08")
	davis := f.tournament("davis", "D", "tour", "2020-01-08")

	f.match(open, a, b, 1, "F", "2020-01-08")
	f.match(open, a, b, 2, "SF", "2020-01-08", withScore("W/O"))
	f.match(davis, a, b, 1, "RR", "2020-01-08", asTeamEvent())

	n, ex, err := f.Matches(f.ctx, false, func(Result) error { return nil })
	if err != nil {
		t.Fatalf("Matches: %v", err)
	}
	if n != 1 {
		t.Errorf("rated %d matches, want the 1 that was played", n)
	}
	if ex.Walkovers != 1 || ex.TeamEvents != 1 {
		t.Errorf("excluded %+v, want one of each", ex)
	}

	// The flag brings the team event back, and never the walkover.
	n, ex, err = f.Matches(f.ctx, true, func(Result) error { return nil })
	if err != nil {
		t.Fatalf("Matches: %v", err)
	}
	if n != 2 || ex.TeamEvents != 0 || ex.Walkovers != 1 {
		t.Errorf("with team events: rated %d, excluded %+v", n, ex)
	}
}

func TestRunWritesRatingsAndRepeatsExactly(t *testing.T) {
	f := newFixture(t)
	a, b, c := f.player("a"), f.player("b"), f.player("c")
	open := f.tournament("open", "A", "tour", "2020-01-08")
	later := f.tournament("later", "G", "tour", "2020-03-04")

	f.match(open, a, b, 1, "SF", "2020-01-08")
	f.match(open, a, c, 2, "F", "2020-01-08")
	f.match(later, b, a, 1, "F", "2020-03-04")

	first, err := Run(f.ctx, f.pool, Config{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if first.Matches != 3 {
		t.Errorf("rated %d matches, want 3", first.Matches)
	}
	if first.Players != 3 {
		t.Errorf("rated %d players, want 3", first.Players)
	}

	// Two active weeks: three players on overall and hard in the first, the two
	// who played on both series again in the second.
	if first.Snapshots != 10 {
		t.Errorf("wrote %d snapshots, want 10", first.Snapshots)
	}
	if n := f.count(`SELECT count(*) FROM ratings`); int64(n) != first.Snapshots {
		t.Errorf("the table holds %d rows, the report claims %d", n, first.Snapshots)
	}
	if n := f.count(`SELECT count(*) FROM ratings WHERE as_of = '2020-01-06'`); n != 6 {
		t.Errorf("%d rows in the first week, want 6", n)
	}
	// Nothing is written for the weeks in between.
	if n := f.count(`SELECT count(DISTINCT as_of) FROM ratings`); n != 2 {
		t.Errorf("ratings span %d weeks, want the 2 that were played", n)
	}
	// The winner of two matches is above the base rating, the player who lost
	// both is below it.
	if n := f.count(`SELECT count(*) FROM ratings
	                  WHERE as_of = '2020-01-06' AND surface = 'overall' AND elo > 1500`); n != 1 {
		t.Errorf("%d players above 1500 after the first week, want the one who won both", n)
	}

	before := f.dump()
	second, err := Run(f.ctx, f.pool, Config{})
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if second.Snapshots != first.Snapshots {
		t.Errorf("second run wrote %d snapshots, first wrote %d", second.Snapshots, first.Snapshots)
	}
	if after := f.dump(); after != before {
		t.Error("a second recompute from scratch produced different ratings")
	}
}

// A recompute that fails leaves the previous ratings alone rather than a
// half-written table.
func TestRunLeavesTheOldRatingsOnFailure(t *testing.T) {
	f := newFixture(t)
	a, b := f.player("a"), f.player("b")
	open := f.tournament("open", "A", "tour", "2020-01-08")
	f.match(open, a, b, 1, "F", "2020-01-08")

	if _, err := Run(f.ctx, f.pool, Config{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	before := f.dump()
	if before == "" {
		t.Fatal("the first run wrote nothing")
	}

	ctx, cancel := context.WithCancel(f.ctx)
	cancel()
	if _, err := Run(ctx, f.pool, Config{}); err == nil {
		t.Fatal("a cancelled run reported success")
	}
	if after := f.dump(); after != before {
		t.Error("a failed run changed the stored ratings")
	}
}
