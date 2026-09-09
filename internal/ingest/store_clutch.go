package ingest

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/sami0076/tennis-wiki/internal/score"
)

// clutchBatch is how many matches are parsed and written back at a time. Large
// enough that the round trips disappear, small enough that the arrays sent to
// Postgres stay ordinary.
const clutchBatch = 5000

// RefreshClutch derives the clutch columns for matches that do not have them
// yet, then rebuilds the baselines from every match that does.
//
// Both halves are here rather than at query time for the same reason: neither
// can be a per-request query. The columns need the Go score parser over 1.6
// million strings, and the baselines need one pass over every appearance.
//
// force re-derives every match rather than only the ones with nothing yet,
// which is what a change to the derivation itself needs.
//
// Returns how many matches were derived, which is the number worth logging: on
// every run after the first it is zero.
func (s *Store) RefreshClutch(ctx context.Context, force bool) (int64, error) {
	derived, err := s.deriveClutchColumns(ctx, force)
	if err != nil {
		return 0, err
	}
	if err := s.rebuildClutchBaselines(ctx); err != nil {
		return derived, err
	}
	return derived, nil
}

// deriveClutchColumns fills tiebreaks_winner, tiebreaks_loser and deciding_set
// for matches the ingest has not already written them for.
//
// Paged by id rather than by "still NULL", because a score the parser cannot
// read stays NULL: selecting on the condition it fails to clear would be an
// infinite loop over exactly those rows.
func (s *Store) deriveClutchColumns(ctx context.Context, force bool) (int64, error) {
	var after, total int64

	// A row already derived is skipped, unless the derivation itself changed.
	pending := "deciding_set IS NULL AND "
	if force {
		pending = ""
	}
	query := `
		SELECT id, score, best_of
		  FROM matches
		 WHERE ` + pending + `NOT incomplete AND score IS NOT NULL
		   AND id > $1
		 ORDER BY id
		 LIMIT $2`

	for {
		ids := make([]int64, 0, clutchBatch)
		winners := make([]int16, 0, clutchBatch)
		losers := make([]int16, 0, clutchBatch)
		deciders := make([]bool, 0, clutchBatch)
		last := after

		rows, err := s.pool.Query(ctx, query, after, clutchBatch)
		if err != nil {
			return total, fmt.Errorf("read matches to derive: %w", err)
		}

		read := 0
		for rows.Next() {
			var id int64
			var raw string
			var bestOf int
			if err := rows.Scan(&id, &raw, &bestOf); err != nil {
				rows.Close()
				return total, fmt.Errorf("scan match to derive: %w", err)
			}
			read++
			last = id

			parsed, err := score.Parse(raw)
			// An unreadable score is a data-quality fact for cmd/dataqual, not
			// a reason to write a zero that would average as a real one.
			if err != nil || parsed.Incomplete() {
				continue
			}
			winner, loser := parsed.Tiebreaks()
			ids = append(ids, id)
			winners = append(winners, int16(winner))
			losers = append(losers, int16(loser))
			deciders = append(deciders, parsed.WentToDecider(bestOf))
		}
		if err := rows.Err(); err != nil {
			return total, fmt.Errorf("read matches to derive: %w", err)
		}
		rows.Close()

		if len(ids) > 0 {
			tag, err := s.pool.Exec(ctx, `
				UPDATE matches m
				   SET tiebreaks_winner = u.winner,
				       tiebreaks_loser  = u.loser,
				       deciding_set     = u.decider
				  FROM (
				        SELECT unnest($1::bigint[])   AS id,
				               unnest($2::smallint[]) AS winner,
				               unnest($3::smallint[]) AS loser,
				               unnest($4::boolean[])  AS decider
				       ) u
				 WHERE m.id = u.id`, ids, winners, losers, deciders)
			if err != nil {
				return total, fmt.Errorf("write derived clutch columns: %w", err)
			}
			total += tag.RowsAffected()
		}

		if read < clutchBatch {
			return total, nil
		}
		after = last
	}
}

// rebuildClutchBaselines recomputes what "vs tour average" is measured against.
//
// Counted per appearance -- both sides of every match -- because that is the
// unit a player's own figures are counted in, and a baseline in a different
// unit is not a baseline. Team events are out, the same exclusion the rating
// engine makes: a Davis Cup rubber is not the same competition.
func (s *Store) rebuildClutchBaselines(ctx context.Context) (err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin baselines: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			err = errors.Join(err, rbErr)
		}
	}()

	// Replaced wholesale rather than upserted: a cell that loses its last match
	// has to disappear, and there are about a hundred rows in the table.
	if _, err := tx.Exec(ctx, `DELETE FROM clutch_baselines`); err != nil {
		return fmt.Errorf("clear baselines: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO clutch_baselines
		       (tour, tier, decade, appearances, bp_saved, bp_faced,
		        tiebreaks_won, tiebreaks_played, deciding_sets_won, deciding_sets_played)
		SELECT t.tour,
		       t.tier,
		       ((t.season / 10) * 10)::smallint,
		       count(*),
		       coalesce(sum(mp.bp_saved), 0),
		       coalesce(sum(mp.bp_faced), 0),
		       coalesce(sum(CASE WHEN mp.won THEN m.tiebreaks_winner
		                                     ELSE m.tiebreaks_loser END), 0),
		       coalesce(sum(m.tiebreaks_winner + m.tiebreaks_loser), 0),
		       count(*) FILTER (WHERE m.deciding_set AND mp.won),
		       count(*) FILTER (WHERE m.deciding_set)
		  FROM match_players mp
		  JOIN matches m     ON m.id = mp.match_id
		  JOIN tournaments t ON t.id = m.tournament_id
		 WHERE NOT m.is_team_event
		 GROUP BY 1, 2, 3`); err != nil {
		return fmt.Errorf("rebuild baselines: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit baselines: %w", err)
	}
	return nil
}
