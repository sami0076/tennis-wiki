package ingest

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// RefreshPlayerTotals rebuilds player_totals: every total a leaderboard or a
// year-by-year row is a rate over, per player, season, tier and surface.
//
// One pass over every appearance, joined to the other side of its match for
// the return figures. Not a per-request query for the same reason the
// baselines are not: the population is the whole database, summing it took
// eleven seconds on the full load, and the answer moves only when an ingest
// does. Replaced wholesale: a reconcile can move matches between players, and
// a table rebuilt from scratch cannot carry the old attribution.
func (s *Store) RefreshPlayerTotals(ctx context.Context) (err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin player totals: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			err = errors.Join(err, rbErr)
		}
	}()

	if _, err := tx.Exec(ctx, `DELETE FROM player_totals`); err != nil {
		return fmt.Errorf("clear player totals: %w", err)
	}

	// Finished matches only, team events out, the same exclusions every rate
	// on the site makes. A serve line counts when the source wrote the points
	// served; a score counts when the parser read it. Each figure carries the
	// count of matches it stands on, so a rate can name its denominator.
	if _, err := tx.Exec(ctx, `
		INSERT INTO player_totals
		       (player_id, tour, season, tier, surface,
		        matches, wins, titles, with_stats,
		        with_serve, aces, double_faults, serve_points, first_in, first_won, second_won,
		        serve_games, bp_saved, bp_faced,
		        with_return, op_serve_points, op_first_in, op_first_won, op_second_won,
		        op_serve_games, op_bp_saved, op_bp_faced,
		        scored, sets_won, sets_played, games_won, games_played,
		        tiebreaks_won, tiebreaks_played, deciders_won, deciders_played)
		SELECT mp.player_id, t.tour, t.season, t.tier, m.surface,
		       count(*),
		       count(*) FILTER (WHERE mp.won),
		       count(*) FILTER (WHERE mp.won AND m.round = 'F' AND NOT m.is_qualifying),
		       count(*) FILTER (WHERE m.has_detailed_stats),

		       count(*) FILTER (WHERE mp.serve_points IS NOT NULL),
		       coalesce(sum(mp.aces), 0),
		       coalesce(sum(mp.double_faults), 0),
		       coalesce(sum(mp.serve_points), 0),
		       coalesce(sum(mp.first_in), 0),
		       coalesce(sum(mp.first_won), 0),
		       coalesce(sum(mp.second_won), 0),
		       coalesce(sum(mp.serve_games), 0),
		       coalesce(sum(mp.bp_saved), 0),
		       coalesce(sum(mp.bp_faced), 0),

		       count(*) FILTER (WHERE op.serve_points IS NOT NULL),
		       coalesce(sum(op.serve_points), 0),
		       coalesce(sum(op.first_in), 0),
		       coalesce(sum(op.first_won), 0),
		       coalesce(sum(op.second_won), 0),
		       coalesce(sum(op.serve_games), 0),
		       coalesce(sum(op.bp_saved), 0),
		       coalesce(sum(op.bp_faced), 0),

		       count(*) FILTER (WHERE m.sets_winner IS NOT NULL),
		       coalesce(sum(CASE WHEN mp.won THEN m.sets_winner ELSE m.sets_loser END), 0),
		       coalesce(sum(m.sets_winner + m.sets_loser), 0),
		       coalesce(sum(CASE WHEN mp.won THEN m.games_winner ELSE m.games_loser END), 0),
		       coalesce(sum(m.games_winner + m.games_loser), 0),
		       coalesce(sum(CASE WHEN mp.won THEN m.tiebreaks_winner ELSE m.tiebreaks_loser END), 0),
		       coalesce(sum(m.tiebreaks_winner + m.tiebreaks_loser), 0),
		       count(*) FILTER (WHERE m.deciding_set AND mp.won),
		       count(*) FILTER (WHERE m.deciding_set)
		  FROM match_players mp
		  JOIN matches m       ON m.id = mp.match_id
		  JOIN tournaments t   ON t.id = m.tournament_id
		  JOIN match_players op ON op.match_id = m.id AND op.player_id <> mp.player_id
		 WHERE NOT m.incomplete AND NOT m.is_team_event
		 GROUP BY mp.player_id, t.tour, t.season, t.tier, m.surface`); err != nil {
		return fmt.Errorf("rebuild player totals: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit player totals: %w", err)
	}
	return nil
}
