package ingest

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// RefreshServeBaselines rebuilds the anchor the simulator's inversion needs:
// serve-point totals by tour, tier, surface and decade.
//
// One pass over every appearance that carried a serve line. Not a per-request
// query for the same reason the clutch baselines are not: the population is the
// whole database and the answer only moves when an ingest does.
func (s *Store) RefreshServeBaselines(ctx context.Context) (err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin serve baselines: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			err = errors.Join(err, rbErr)
		}
	}()

	// Replaced wholesale rather than upserted: a cell that loses its last match
	// has to disappear, and the table is a few hundred rows.
	if _, err := tx.Exec(ctx, `DELETE FROM serve_baselines`); err != nil {
		return fmt.Errorf("clear serve baselines: %w", err)
	}

	// A match with no recorded surface is left out rather than bucketed as a
	// fifth surface: the anchor is a claim about a surface, and "unknown" is
	// not one.
	if _, err := tx.Exec(ctx, `
		INSERT INTO serve_baselines
		       (tour, tier, surface, decade, appearances, serve_points, serve_won)
		SELECT t.tour,
		       t.tier,
		       m.surface,
		       ((t.season / 10) * 10)::smallint,
		       count(*),
		       sum(mp.serve_points),
		       sum(mp.first_won + mp.second_won)
		  FROM match_players mp
		  JOIN matches m     ON m.id = mp.match_id
		  JOIN tournaments t ON t.id = m.tournament_id
		 WHERE NOT m.is_team_event
		   AND m.surface IS NOT NULL
		   AND mp.serve_points IS NOT NULL
		   AND mp.first_won IS NOT NULL
		   AND mp.second_won IS NOT NULL
		 GROUP BY 1, 2, 3, 4`); err != nil {
		return fmt.Errorf("rebuild serve baselines: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit serve baselines: %w", err)
	}
	return nil
}
