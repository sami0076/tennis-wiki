package identity

import (
	"context"
	"testing"
	"time"

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
	pool, err := db.Open(ctx, db.Config{DSN: testdb.Start(t), MaxConns: 4, MinConns: 1})
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

func (f *fixture) player(sourceID, full, country string, born *time.Time) int64 {
	f.t.Helper()
	var id int64
	err := f.pool.QueryRow(f.ctx,
		`INSERT INTO players (source_id, tour, slug, full_name, country, birth_date)
		 VALUES ($1, 'atp', $1, $2, $3, $4) RETURNING id`,
		sourceID, full, country, born).Scan(&id)
	if err != nil {
		f.t.Fatalf("insert player %s: %v", sourceID, err)
	}
	return id
}

func (f *fixture) tournament(sourceID string, season int) int64 {
	f.t.Helper()
	var id int64
	err := f.pool.QueryRow(f.ctx,
		`INSERT INTO tournaments (source_id, tour, name, level, tier, surface, start_date, season)
		 VALUES ($1, 'atp', $1, 'A', 'tour', 'hard', make_date($2, 5, 1), $2) RETURNING id`,
		sourceID, season).Scan(&id)
	if err != nil {
		f.t.Fatalf("insert tournament: %v", err)
	}
	return id
}

// One transaction: matches carries a deferred foreign key naming the winner as
// a participant, so the participants have to land before the commit.
func (f *fixture) match(tournamentID, winner, loser int64, num int, source string) {
	f.t.Helper()
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		f.t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(f.ctx) }()

	var matchID int64
	err = tx.QueryRow(f.ctx,
		`INSERT INTO matches (tournament_id, match_num, round, best_of, surface, winner_id,
		                      loser_id, played_on, source)
		 VALUES ($1, $2, 'F', 3, 'hard', $3, $4, make_date(2025, 5, 2), $5) RETURNING id`,
		tournamentID, num, winner, loser, source).Scan(&matchID)
	if err != nil {
		f.t.Fatalf("insert match: %v", err)
	}
	for _, p := range []struct {
		id  int64
		won bool
	}{{winner, true}, {loser, false}} {
		if _, err := tx.Exec(f.ctx,
			`INSERT INTO match_players (match_id, player_id, won) VALUES ($1, $2, $3)`,
			matchID, p.id, p.won); err != nil {
			f.t.Fatalf("insert match_player: %v", err)
		}
	}
	if err := tx.Commit(f.ctx); err != nil {
		f.t.Fatalf("commit match: %v", err)
	}
}

func (f *fixture) ranking(playerID int64, date string, rank int32) {
	f.t.Helper()
	if _, err := f.pool.Exec(f.ctx,
		`INSERT INTO rankings (player_id, ranking_date, rank) VALUES ($1, $2::date, $3)`,
		playerID, date, rank); err != nil {
		f.t.Fatalf("insert ranking: %v", err)
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

// The whole point: one career, not two.
func TestMergeMovesTheEntireCareer(t *testing.T) {
	f := newFixture(t)
	born := date(t, "2003-05-05")

	canonical := f.player("207989", "Carlos Alcaraz", "ESP", born)
	duplicate := f.player("AC30", "Carlos Alcaraz", "ESP", born)
	opponent := f.player("100001", "Some Opponent", "FRA", nil)

	old := f.tournament("old-open", 2024)
	recent := f.tournament("new-open", 2025)
	f.match(old, canonical, opponent, 1, "sackmann-atp-tour")
	f.match(recent, duplicate, opponent, 2, "tml-atp-current")
	f.ranking(canonical, "2024-01-01", 2)
	f.ranking(duplicate, "2025-01-01", 1)

	err := f.Merge(f.ctx, Match{
		Canonical:  Player{ID: canonical, SourceID: "207989"},
		Duplicate:  Player{ID: duplicate, SourceID: "AC30"},
		Confidence: 1.0,
		Reason:     "test",
	})
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}

	if n := f.count(`SELECT count(*) FROM players WHERE id = $1`, duplicate); n != 0 {
		t.Error("the duplicate player row survived the merge")
	}
	if n := f.count(`SELECT count(*) FROM match_players WHERE player_id = $1`, canonical); n != 2 {
		t.Errorf("canonical player has %d matches, want both", n)
	}
	if n := f.count(`SELECT count(*) FROM matches WHERE winner_id = $1`, canonical); n != 2 {
		t.Errorf("canonical player won %d matches, want both", n)
	}
	if n := f.count(`SELECT count(*) FROM rankings WHERE player_id = $1`, canonical); n != 2 {
		t.Errorf("canonical player has %d ranking rows, want both", n)
	}

	// Nothing may be left pointing at an id that no longer exists.
	for _, q := range []string{
		`SELECT count(*) FROM match_players WHERE player_id = $1`,
		`SELECT count(*) FROM rankings WHERE player_id = $1`,
		`SELECT count(*) FROM matches WHERE winner_id = $1`,
	} {
		if n := f.count(q, duplicate); n != 0 {
			t.Errorf("%d rows still reference the merged-away player: %s", n, q)
		}
	}

	// The alias is what makes the merge survive a re-ingest, and it records the
	// source the duplicate id came from.
	var source string
	var linked int64
	if err := f.pool.QueryRow(f.ctx,
		`SELECT source, player_id FROM player_aliases WHERE source_id = 'AC30'`).
		Scan(&source, &linked); err != nil {
		t.Fatalf("no alias was recorded: %v", err)
	}
	if linked != canonical {
		t.Errorf("alias points at %d, want the canonical %d", linked, canonical)
	}
	if source != "tml-atp-current" {
		t.Errorf("alias source = %q, want the source the duplicate id came from", source)
	}
}

// Both sources carrying the same match is redundancy, not a conflict.
func TestMergeToleratesAMatchPresentOnBothSides(t *testing.T) {
	f := newFixture(t)
	canonical := f.player("207989", "Carlos Alcaraz", "ESP", nil)
	duplicate := f.player("AC30", "Carlos Alcaraz", "ESP", nil)
	opponent := f.player("100001", "Some Opponent", "FRA", nil)

	shared := f.tournament("shared-open", 2025)
	f.match(shared, canonical, opponent, 1, "sackmann-atp-tour")
	// The same fixture appearing again for the duplicate, and the same ranking
	// date, which would collide on the primary keys.
	other := f.tournament("other-open", 2025)
	f.match(other, duplicate, opponent, 2, "tml-atp-current")
	f.ranking(canonical, "2025-01-01", 1)
	f.ranking(duplicate, "2025-01-01", 1)

	err := f.Merge(f.ctx, Match{
		Canonical: Player{ID: canonical, SourceID: "207989"},
		Duplicate: Player{ID: duplicate, SourceID: "AC30"}, Confidence: 1.0, Reason: "test",
	})
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if n := f.count(`SELECT count(*) FROM rankings WHERE player_id = $1`, canonical); n != 1 {
		t.Errorf("%d ranking rows for the shared date, want the one that was already there", n)
	}
}

func TestQueueRecordsForReviewAndIsIdempotent(t *testing.T) {
	f := newFixture(t)
	canonical := f.player("100001", "Juan Martin", "ARG", nil)
	duplicate := f.player("JM01", "Juan Martin", "ARG", nil)
	opponent := f.player("100002", "Some Opponent", "FRA", nil)
	open := f.tournament("open", 2025)
	f.match(open, duplicate, opponent, 1, "tml-atp-current")

	m := Match{
		Canonical: Player{ID: canonical, SourceID: "100001"},
		Duplicate: Player{ID: duplicate, SourceID: "JM01"},
		// Two candidates matched equally well.
		Confidence: ReviewFloor,
		Reason:     "more than one candidate matches equally well",
	}
	for i := 0; i < 2; i++ {
		if err := f.Queue(f.ctx, "atp", m); err != nil {
			t.Fatalf("Queue %d: %v", i, err)
		}
	}
	if n := f.count(`SELECT count(*) FROM identity_reviews`); n != 1 {
		t.Errorf("%d review rows after queueing twice, want 1", n)
	}
	// Nothing was merged: a review is a question, not a decision.
	if n := f.count(`SELECT count(*) FROM players WHERE id = $1`, duplicate); n != 1 {
		t.Error("queueing for review removed the player")
	}
	if n := f.count(`SELECT count(*) FROM player_aliases`); n != 0 {
		t.Error("queueing for review recorded an alias")
	}
}

// Running the whole pass twice must change nothing the second time.
func TestRunnerIsIdempotent(t *testing.T) {
	f := newFixture(t)
	born := date(t, "2003-05-05")
	f.player("207989", "Carlos Alcaraz", "ESP", born)
	dup := f.player("AC30", "Carlos Alcaraz", "ESP", born)
	opponent := f.player("100001", "Some Opponent", "FRA", nil)
	open := f.tournament("open", 2025)
	f.match(open, dup, opponent, 1, "tml-atp-current")

	runner := &Runner{Store: f.Store, Decisions: (&Overrides{}).Index()}

	first, err := runner.Run(f.ctx, []string{"atp"})
	if err != nil {
		t.Fatalf("first pass: %v", err)
	}
	if first.Merged != 1 {
		t.Fatalf("first pass merged %d, want 1", first.Merged)
	}

	second, err := runner.Run(f.ctx, []string{"atp"})
	if err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if second.Merged != 0 || second.Proposed != 0 {
		t.Errorf("second pass proposed %d and merged %d, want nothing left to do",
			second.Proposed, second.Merged)
	}
	if n := f.count(`SELECT count(*) FROM player_aliases`); n != 1 {
		t.Errorf("%d aliases after two passes, want 1", n)
	}
}

// A dry run is how a change to the scoring should be reviewed.
func TestRunnerDryRunChangesNothing(t *testing.T) {
	f := newFixture(t)
	born := date(t, "2003-05-05")
	f.player("207989", "Carlos Alcaraz", "ESP", born)
	dup := f.player("AC30", "Carlos Alcaraz", "ESP", born)
	opponent := f.player("100001", "Some Opponent", "FRA", nil)
	open := f.tournament("open", 2025)
	f.match(open, dup, opponent, 1, "tml-atp-current")

	runner := &Runner{Store: f.Store, Decisions: (&Overrides{}).Index(), DryRun: true}
	stats, err := runner.Run(f.ctx, []string{"atp"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stats.Merged != 1 {
		t.Errorf("dry run reported %d merges, want the 1 it would have made", stats.Merged)
	}
	if n := f.count(`SELECT count(*) FROM players WHERE id = $1`, dup); n != 1 {
		t.Error("a dry run removed a player")
	}
	if n := f.count(`SELECT count(*) FROM player_aliases`); n != 0 {
		t.Error("a dry run wrote an alias")
	}
}

func TestAliasedPlayer(t *testing.T) {
	f := newFixture(t)
	canonical := f.player("207989", "Carlos Alcaraz", "ESP", nil)
	if _, err := f.pool.Exec(f.ctx,
		`INSERT INTO player_aliases (source, source_id, player_id, confidence)
		 VALUES ('tml-atp-current', 'AC30', $1, 1.0)`, canonical); err != nil {
		t.Fatal(err)
	}

	got, ok, err := f.AliasedPlayer(f.ctx, "tml-atp-current", "AC30")
	if err != nil || !ok || got != canonical {
		t.Errorf("AliasedPlayer = %d, %v, %v", got, ok, err)
	}
	if _, ok, err := f.AliasedPlayer(f.ctx, "tml-atp-current", "NOPE"); err != nil || ok {
		t.Errorf("an unknown alias returned ok=%v err=%v", ok, err)
	}
}

// The derived birth year comes from the age on a match row, and age is elapsed
// years. Rounding it instead of flooring makes anyone born in the second half
// of the year come out a year too young, which silently failed to merge about
// half of all players.
func TestDerivedBirthYearUsesElapsedYears(t *testing.T) {
	f := newFixture(t)
	// The fixture match is played on 2025-05-02. A player aged 27.4 that day was
	// born around 1997-12-12: floor(2025 - 27.4) is 1997, round would say 1998.
	born := date(t, "1997-12-12")
	canonical := f.player("126774", "Stefanos Tsitsipas", "GRE", born)
	duplicate := f.player("TE51", "Stefanos Tsitsipas", "GRE", nil)
	opponent := f.player("100001", "Some Opponent", "FRA", nil)

	open := f.tournament("open", 2026)
	f.match(open, duplicate, opponent, 1, "tml-atp-current")
	if _, err := f.pool.Exec(f.ctx,
		`UPDATE match_players SET age = 27.4 WHERE player_id = $1`, duplicate); err != nil {
		t.Fatal(err)
	}

	players, err := f.LoadPlayers(f.ctx, "atp")
	if err != nil {
		t.Fatalf("LoadPlayers: %v", err)
	}
	var dup Player
	for _, p := range players {
		if p.ID == duplicate {
			dup = p
		}
	}
	if dup.BirthYear == nil {
		t.Fatal("no birth year was derived from the match age")
	}
	if *dup.BirthYear != 1997 {
		t.Errorf("derived birth year = %d, want 1997", *dup.BirthYear)
	}

	matches := Reconcile(players)
	if len(matches) != 1 || !matches[0].Auto() {
		t.Fatalf("got %+v, want one automatic merge", matches)
	}
	if matches[0].Canonical.ID != canonical {
		t.Errorf("canonical = %d, want %d", matches[0].Canonical.ID, canonical)
	}
}
