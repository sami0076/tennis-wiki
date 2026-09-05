package ingest

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// querier is the part of a pool and a transaction that prune reads through.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// inconsistentStats is Stats.consistent expressed in SQL. The two must agree:
// this clears exactly the rows the parser now refuses to write.
const inconsistentStats = `serve_points IS NOT NULL
	  AND (first_in > serve_points
	    OR first_won > first_in
	    OR second_won > serve_points - first_in
	    OR COALESCE(bp_saved, 0) > COALESCE(bp_faced, 0))`

// collapsedMatch matches a row holding other than two players.
const collapsedMatch = `(SELECT count(*) FROM match_players mp WHERE mp.match_id = m.id) <> 2`

// PruneResult counts what a prune cleared.
type PruneResult struct {
	StatLines int
	// Matches counts those no longer claiming detailed statistics, which is
	// fewer than StatLines when both participants of one match were corrupt.
	Matches int
	// Collapsed counts matches deleted for holding other than two players.
	Collapsed int
	// Refill names the (source, season) pairs a re-ingest has to revisit to put
	// the deleted matches back, separated this time.
	Refill []SourceSeason
}

// SourceSeason is one unit of re-ingestible work.
type SourceSeason struct {
	Source string
	Season int
	Count  int
}

// Prune brings stored rows to what today's ingest would have written.
//
// Every write is an upsert, so tightening a rule leaves behind the rows written
// under the old one. A re-ingest would replace them, but only after re-reading
// 1.6 million rows, which takes an hour. This reaches the same state in seconds.
//
// Two repairs, because there are two ways a row can predate its rule:
//
//   - A stat line that cannot describe a real match loses its statistics and
//     keeps its match, which is what the parser does to the source row.
//   - A match holding other than two players is two matches fused by the
//     natural key migration 00011 replaced. There is no in-place repair -- the
//     row is a mixture -- so it is deleted, and Refill names the seasons a
//     re-ingest must revisit to write both of them back.
func (s *Store) Prune(ctx context.Context, dryRun bool) (res PruneResult, err error) {
	if dryRun {
		if err := s.pool.QueryRow(ctx,
			`SELECT count(*) FROM match_players WHERE `+inconsistentStats).
			Scan(&res.StatLines); err != nil {
			return res, fmt.Errorf("count inconsistent stat lines: %w", err)
		}
		if res.Refill, err = collapsedSeasons(ctx, s.pool); err != nil {
			return res, err
		}
		for _, r := range res.Refill {
			res.Collapsed += r.Count
		}
		return res, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return res, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		// A rollback after a successful commit is a harmless no-op.
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			err = errors.Join(err, rbErr)
		}
	}()

	matchIDs, err := clearInconsistentStats(ctx, tx)
	if err != nil {
		return res, err
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

	if res.Refill, err = collapsedSeasons(ctx, tx); err != nil {
		return res, err
	}
	tag, err = tx.Exec(ctx, `DELETE FROM matches m WHERE `+collapsedMatch)
	if err != nil {
		return res, fmt.Errorf("delete collapsed matches: %w", err)
	}
	res.Collapsed = int(tag.RowsAffected())

	if err := tx.Commit(ctx); err != nil {
		return res, fmt.Errorf("commit prune: %w", err)
	}
	return res, nil
}

func clearInconsistentStats(ctx context.Context, tx pgx.Tx) ([]int64, error) {
	rows, err := tx.Query(ctx,
		`UPDATE match_players
		    SET aces = NULL, double_faults = NULL, serve_points = NULL, first_in = NULL,
		        first_won = NULL, second_won = NULL, serve_games = NULL,
		        bp_saved = NULL, bp_faced = NULL
		  WHERE `+inconsistentStats+`
		 RETURNING match_id`)
	if err != nil {
		return nil, fmt.Errorf("clear inconsistent stat lines: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("clear inconsistent stat lines: %w", err)
	}
	return ids, nil
}

func collapsedSeasons(ctx context.Context, q querier) ([]SourceSeason, error) {
	rows, err := q.Query(ctx, `
		SELECT m.source, t.season, count(*)::int
		  FROM matches m
		  JOIN tournaments t ON t.id = m.tournament_id
		 WHERE `+collapsedMatch+`
		 GROUP BY m.source, t.season
		 ORDER BY count(*) DESC, m.source, t.season`)
	if err != nil {
		return nil, fmt.Errorf("find collapsed matches: %w", err)
	}
	defer rows.Close()
	var out []SourceSeason
	for rows.Next() {
		var s SourceSeason
		if err := rows.Scan(&s.Source, &s.Season, &s.Count); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
