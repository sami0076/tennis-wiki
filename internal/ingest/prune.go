package ingest

import (
	"context"
	"fmt"
)

// inconsistentStats is Stats.consistent expressed in SQL. The two must agree:
// this clears exactly the rows the parser now refuses to write.
const inconsistentStats = `serve_points IS NOT NULL
	  AND (first_in > serve_points
	    OR first_won > first_in
	    OR second_won > serve_points - first_in
	    OR COALESCE(bp_saved, 0) > COALESCE(bp_faced, 0))`

// PruneResult counts what a prune cleared.
type PruneResult struct {
	StatLines int
	// Matches counts those no longer claiming detailed statistics, which is
	// fewer than StatLines when both participants of one match were corrupt.
	Matches int
}

// PruneStats clears stat lines that cannot describe a real match.
//
// A row already in the database predates the rejection that would stop it
// today, and a re-ingest would overwrite it -- but only after re-reading 1.6
// million rows, which takes an hour. This reaches the same state in seconds by
// doing to the stored row what the parser does to the source row: drop the
// statistics and keep the match, which is real either way.
func (s *Store) PruneStats(ctx context.Context, dryRun bool) (PruneResult, error) {
	var res PruneResult
	if dryRun {
		err := s.pool.QueryRow(ctx,
			`SELECT count(*) FROM match_players WHERE `+inconsistentStats).Scan(&res.StatLines)
		if err != nil {
			return res, fmt.Errorf("count inconsistent stat lines: %w", err)
		}
		return res, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return res, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx,
		`UPDATE match_players
		    SET aces = NULL, double_faults = NULL, serve_points = NULL, first_in = NULL,
		        first_won = NULL, second_won = NULL, serve_games = NULL,
		        bp_saved = NULL, bp_faced = NULL
		  WHERE `+inconsistentStats+`
		 RETURNING match_id`)
	if err != nil {
		return res, fmt.Errorf("clear inconsistent stat lines: %w", err)
	}
	var matchIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return res, err
		}
		matchIDs = append(matchIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, fmt.Errorf("clear inconsistent stat lines: %w", err)
	}
	res.StatLines = len(matchIDs)

	// has_detailed_stats means both participants were recorded, so clearing one
	// side falsifies it. Leaving it would trip stats_claimed_but_absent, and
	// coverage reporting would keep counting a match whose statistics are gone.
	tag, err := tx.Exec(ctx,
		`UPDATE matches SET has_detailed_stats = false
		  WHERE id = ANY($1) AND has_detailed_stats`, matchIDs)
	if err != nil {
		return res, fmt.Errorf("clear has_detailed_stats: %w", err)
	}
	res.Matches = int(tag.RowsAffected())

	if err := tx.Commit(ctx); err != nil {
		return res, err
	}
	return res, nil
}
