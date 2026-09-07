package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sami0076/tennis-wiki/internal/name"
)

// Store is the database side of reconciliation.
type Store struct{ pool *pgxpool.Pool }

// NewStore wraps a pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// LoadPlayers reads every player of one tour, with the size of their history.
// The larger of a pair becomes canonical, so a merge moves the shorter career
// onto the longer one rather than the other way round.
func (s *Store) LoadPlayers(ctx context.Context, tour string) ([]Player, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, p.source_id, p.slug, p.full_name,
		       coalesce(p.country, ''), p.birth_date,
		       -- The ATP id space has no player table, so for those players the
		       -- only age signal is the one recorded on a match row.
		       -- floor, not round: age is elapsed years, so a player born late in
		       -- the year would otherwise come out a year too young.
		       (SELECT floor(extract(year FROM m.played_on) - mp.age)::int
		          FROM match_players mp JOIN matches m ON m.id = mp.match_id
		         WHERE mp.player_id = p.id AND mp.age IS NOT NULL
		         ORDER BY m.played_on DESC LIMIT 1),
		       (SELECT count(*) FROM match_players mp WHERE mp.player_id = p.id)
		  FROM players p
		 WHERE p.tour = $1::tour`, tour)
	if err != nil {
		return nil, fmt.Errorf("load players: %w", err)
	}
	defer rows.Close()

	var out []Player
	for rows.Next() {
		var p Player
		if err := rows.Scan(&p.ID, &p.SourceID, &p.Slug, &p.FullName,
			&p.Country, &p.BirthDate, &p.BirthYear, &p.Matches); err != nil {
			return nil, fmt.Errorf("load players: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Merge folds the duplicate into the canonical player and records the alias, so
// a re-ingest resolves the duplicate id straight to the canonical row.
//
// Everything happens in one transaction with constraints deferred: matches
// carry a foreign key to match_players naming the winner, and the rows have to
// move before that constraint is checked again.
func (s *Store) Merge(ctx context.Context, m Match) (err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			err = errors.Join(err, rbErr)
		}
	}()

	if _, err := tx.Exec(ctx, `SET CONSTRAINTS ALL DEFERRED`); err != nil {
		return fmt.Errorf("defer constraints: %w", err)
	}

	source, err := sourceOf(ctx, tx, m.Duplicate.ID)
	if err != nil {
		return err
	}

	dup, canonical := m.Duplicate.ID, m.Canonical.ID

	// ON CONFLICT DO NOTHING throughout: if the same match or ranking date
	// arrived from both sources, the canonical row already has it and the
	// duplicate's copy is redundant rather than a conflict to resolve.
	// Both sides move: loser_id is half the match's natural key, so leaving it
	// on the duplicate would strand the row behind a player that no longer
	// exists and break the key on the next ingest.
	if _, err := tx.Exec(ctx, `
		UPDATE matches SET winner_id = $1 WHERE winner_id = $2`, canonical, dup); err != nil {
		return fmt.Errorf("move match winners: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE matches SET loser_id = $1 WHERE loser_id = $2`, canonical, dup); err != nil {
		return fmt.Errorf("move match losers: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO match_players (match_id, player_id, won, seed, entry, rank, rank_points,
		                           age, aces, double_faults, serve_points, first_in, first_won,
		                           second_won, serve_games, bp_saved, bp_faced)
		SELECT match_id, $1, won, seed, entry, rank, rank_points, age, aces, double_faults,
		       serve_points, first_in, first_won, second_won, serve_games, bp_saved, bp_faced
		  FROM match_players WHERE player_id = $2
		    ON CONFLICT (match_id, player_id) DO NOTHING`, canonical, dup); err != nil {
		return fmt.Errorf("move match participation: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM match_players WHERE player_id = $1`, dup); err != nil {
		return fmt.Errorf("clear duplicate participation: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO rankings (player_id, ranking_date, rank, points)
		SELECT $1, ranking_date, rank, points FROM rankings WHERE player_id = $2
		    ON CONFLICT (player_id, ranking_date) DO NOTHING`, canonical, dup); err != nil {
		return fmt.Errorf("move rankings: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM rankings WHERE player_id = $1`, dup); err != nil {
		return fmt.Errorf("clear duplicate rankings: %w", err)
	}

	// Aliases already pointing at the duplicate follow it.
	if _, err := tx.Exec(ctx,
		`UPDATE player_aliases SET player_id = $1 WHERE player_id = $2`, canonical, dup); err != nil {
		return fmt.Errorf("move aliases: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO player_aliases (source, source_id, player_id, confidence)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (source, source_id) DO UPDATE
		   SET player_id = EXCLUDED.player_id, confidence = EXCLUDED.confidence`,
		source, m.Duplicate.SourceID, canonical, m.Confidence); err != nil {
		return fmt.Errorf("record alias: %w", err)
	}

	// The pair is settled, so it is no longer waiting on anyone.
	if _, err := tx.Exec(ctx,
		`DELETE FROM identity_reviews WHERE source_id = $1`, m.Duplicate.SourceID); err != nil {
		return fmt.Errorf("clear review: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM players WHERE id = $1`, dup); err != nil {
		return fmt.Errorf("remove duplicate player: %w", err)
	}

	if _, err := reclaimSlug(ctx, tx, canonical); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit merge: %w", err)
	}
	return nil
}

// reclaimSlug moves a merged player onto the undisambiguated form of its slug
// when the merge has just freed it.
//
// This is the one place in the project a slug moves, and the rule everywhere
// else is that it never does, because a slug is a URL. The exception is narrow:
// the slug being claimed was held by the row deleted moments ago in this same
// transaction, and the alternative is a player permanently parked on
// taylor-fritz-atp while /players/taylor-fritz 404s. Both source ids carry the
// same name, so which row got the suffix at ingest is an accident of write
// order rather than anything a visitor could reason about.
func reclaimSlug(ctx context.Context, tx pgx.Tx, playerID int64) (bool, error) {
	var slug, fullName string
	err := tx.QueryRow(ctx,
		`SELECT slug, full_name FROM players WHERE id = $1`, playerID).Scan(&slug, &fullName)
	if err != nil {
		return false, fmt.Errorf("read the merged player's slug: %w", err)
	}

	base := name.Slug(fullName)
	if base == "" || base == slug {
		return false, nil
	}

	// NOT EXISTS rather than ON CONFLICT: a third player legitimately holding
	// the base slug keeps it, and this player stays where it is.
	tag, err := tx.Exec(ctx, `
		UPDATE players SET slug = $2
		 WHERE id = $1 AND NOT EXISTS (SELECT 1 FROM players WHERE slug = $2)`,
		playerID, base)
	if err != nil {
		return false, fmt.Errorf("reclaim the slug %q: %w", base, err)
	}
	return tag.RowsAffected() == 1, nil
}

// ReclaimFreedSlugs repairs players merged before a merge knew to do it. Every
// merge that ever ran left its survivor on a suffixed slug, and those URLs are
// dead until something moves them; a re-merge cannot, because the pair is gone.
//
// Only players carrying an alias are considered, which is to say only players a
// merge has touched. A suffixed slug on any other player is a live collision,
// not a leftover.
//
// A dry run does the work and rolls it back, so what it reports is what a real
// run would do rather than a second implementation of the same rule.
func (s *Store) ReclaimFreedSlugs(ctx context.Context, tour string, dryRun bool) (n int, err error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id FROM players p
		 WHERE p.tour = $1::tour
		   AND EXISTS (SELECT 1 FROM player_aliases a WHERE a.player_id = p.id)
		 ORDER BY p.id`, tour)
	if err != nil {
		return 0, fmt.Errorf("find merged players: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("find merged players: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("find merged players: %w", err)
	}
	if len(ids) == 0 {
		return 0, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			err = errors.Join(err, rbErr)
		}
	}()

	// One statement per player rather than one over all of them: two merged
	// players can want the same base slug, and running them in sequence lets
	// the second see that the first has taken it.
	for _, id := range ids {
		moved, err := reclaimSlug(ctx, tx, id)
		if err != nil {
			return 0, err
		}
		if moved {
			n++
		}
	}

	if dryRun {
		return n, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit slug reclaim: %w", err)
	}
	return n, nil
}

// Queue records a pair for a human to settle.
func (s *Store) Queue(ctx context.Context, tour string, m Match) error {
	source, err := sourceOf(ctx, s.pool, m.Duplicate.ID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO identity_reviews (source, source_id, tour, candidate, confidence, reason)
		VALUES ($1, $2, $3::tour, $4, $5, $6)
		ON CONFLICT (source, source_id, candidate) DO UPDATE
		   SET confidence = EXCLUDED.confidence, reason = EXCLUDED.reason, seen_at = now()`,
		source, m.Duplicate.SourceID, tour, m.Canonical.ID, m.Confidence, m.Reason)
	if err != nil {
		return fmt.Errorf("queue review: %w", err)
	}
	return nil
}

// AliasedPlayer returns the canonical player id an alias points at.
func (s *Store) AliasedPlayer(ctx context.Context, source, sourceID string) (int64, bool, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`SELECT player_id FROM player_aliases WHERE source = $1 AND source_id = $2`,
		source, sourceID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("look up alias: %w", err)
	}
	return id, true, nil
}

// querier is the subset of pgx both a pool and a transaction satisfy.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// sourceOf reports which ingest source wrote a player's matches, which is the
// key player_aliases is stored under. A player with no matches has no source to
// attribute the alias to.
func sourceOf(ctx context.Context, q querier, playerID int64) (string, error) {
	var source *string
	err := q.QueryRow(ctx, `
		SELECT m.source
		  FROM match_players mp JOIN matches m ON m.id = mp.match_id
		 WHERE mp.player_id = $1
		 GROUP BY m.source
		 ORDER BY count(*) DESC
		 LIMIT 1`, playerID).Scan(&source)
	if errors.Is(err, pgx.ErrNoRows) || source == nil {
		return "unknown", nil
	}
	if err != nil {
		return "", fmt.Errorf("find the source for player %d: %w", playerID, err)
	}
	return *source, nil
}
