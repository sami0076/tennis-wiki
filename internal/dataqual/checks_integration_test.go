package dataqual

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/sami0076/tennis-wiki/internal/db"
	"github.com/sami0076/tennis-wiki/internal/testdb"
)

// A stat line can contradict itself in four ways. The check counted three of
// them, so five second-serve rows in the full dataset went unreported while a
// player page served them as a percentage above 100.
func TestImpossibleServeNumbersCatchesEveryCondition(t *testing.T) {
	cases := []struct {
		name                                             string
		svpt, firstIn, firstWon, secondWon, saved, faced int
		schemaRejects                                    bool
		want                                             int
	}{
		{name: "consistent", svpt: 70, firstIn: 42, firstWon: 30, secondWon: 18, saved: 2, faced: 4},
		{name: "more first serves in than points served",
			svpt: 70, firstIn: 90, firstWon: 30, schemaRejects: true},
		{name: "more first serves won than made",
			svpt: 70, firstIn: 42, firstWon: 50, schemaRejects: true},
		{name: "second serves won when none were played",
			svpt: 25, firstIn: 25, firstWon: 20, secondWon: 6, want: 1},
		{name: "more second serves won than played",
			svpt: 135, firstIn: 122, firstWon: 90, secondWon: 21, want: 1},
		{name: "more break points saved than faced",
			svpt: 70, firstIn: 42, firstWon: 30, saved: 4, faced: 2, want: 1},
	}

	check := checkByName(t, "impossible_serve_numbers")
	ctx := context.Background()
	pool, err := db.Open(ctx, db.Config{DSN: testdb.Start(t), MaxConns: 4, MinConns: 1})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Rolled back, so these rows never reach another test's counts.
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatalf("begin: %v", err)
			}
			defer tx.Rollback(ctx)

			matchID, playerID := seedMatch(ctx, t, tx)
			_, err = tx.Exec(ctx,
				`INSERT INTO match_players (match_id, player_id, won, serve_points,
				                            first_in, first_won, second_won, bp_saved, bp_faced)
				 VALUES ($1, $2, true, $3, $4, $5, $6, $7, $8)`,
				matchID, playerID, c.svpt, c.firstIn, c.firstWon, c.secondWon, c.saved, c.faced)
			if c.schemaRejects {
				if err == nil || !strings.Contains(err.Error(), "match_players_stats_consistent") {
					t.Fatalf("schema accepted an impossible row: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("insert: %v", err)
			}

			var got int
			if err := tx.QueryRow(ctx, check.Query).Scan(&got); err != nil {
				t.Fatalf("run check: %v", err)
			}
			if got != c.want {
				t.Errorf("check counted %d rows, want %d", got, c.want)
			}
			if c.want > 0 {
				var sample string
				if err := tx.QueryRow(ctx, check.Sample).Scan(&sample); err != nil {
					t.Fatalf("run sample: %v", err)
				}
				if !strings.Contains(sample, "serve_points") {
					t.Errorf("sample %q does not describe the row", sample)
				}
			}
		})
	}
}

func checkByName(t *testing.T, name string) Check {
	t.Helper()
	for _, c := range integrityChecks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no check named %q", name)
	return Check{}
}

func seedMatch(ctx context.Context, t *testing.T, tx pgx.Tx) (matchID, playerID int64) {
	t.Helper()
	err := tx.QueryRow(ctx,
		`INSERT INTO players (source_id, tour, slug, full_name)
		 VALUES ('1', 'atp', 'test-player', 'Test Player') RETURNING id`).Scan(&playerID)
	if err != nil {
		t.Fatalf("insert player: %v", err)
	}
	var tournamentID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO tournaments (source_id, tour, name, level, tier, start_date, season)
		 VALUES ('t', 'atp', 'Test', 'A', 'tour', make_date(2019, 1, 1), 2019) RETURNING id`).
		Scan(&tournamentID)
	if err != nil {
		t.Fatalf("insert tournament: %v", err)
	}
	err = tx.QueryRow(ctx,
		`INSERT INTO matches (tournament_id, match_num, round, best_of, winner_id,
		                      loser_id, played_on, has_detailed_stats, source)
		 VALUES ($1, 1, 'F', 3, $2, $2, make_date(2019, 6, 1), true, 'test') RETURNING id`,
		tournamentID, playerID).Scan(&matchID)
	if err != nil {
		t.Fatalf("insert match: %v", err)
	}
	return matchID, playerID
}
