package ingest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// WritePlayers upserts a batch of player biographies, returning how many rows
// were written.
//
// The player tables are the canonical source for names, so this is also where a
// player first gets a slug. An existing player keeps the slug it already has:
// changing it would break every URL pointing at that page.
func (s *Store) WritePlayers(ctx context.Context, tour Tour, bios []PlayerBio) (n int, err error) {
	if len(bios) == 0 {
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

	for _, bio := range bios {
		p := Player{
			SourceID:   bio.SourceID,
			Name:       bio.FullName(),
			Hand:       bio.Hand,
			Country:    bio.Country,
			BirthDate:  bio.BirthDate,
			WikidataID: bio.WikidataID,
		}
		if bio.HeightCm != nil {
			h := int(*bio.HeightCm)
			p.HeightCM = &h
		}
		if p.Name == "" {
			// The sources carry rows with no name at all. Skipping them would
			// orphan any ranking that references one, so they get a stable
			// placeholder instead and show up in the data-quality report.
			p.Name = "Unknown Player " + bio.SourceID
		}
		if _, err := s.upsertPlayer(ctx, tx, tour, p); err != nil {
			return n, err
		}
		n++
	}

	if err := tx.Commit(ctx); err != nil {
		return n, fmt.Errorf("commit players: %w", err)
	}
	return n, nil
}

// PlayerIDsBySource loads the source id to player id map for one tour.
//
// Held in memory deliberately: ranking history is millions of rows and this is
// about 115,000 entries, so resolving each row with a query would dominate the
// run. The rankings themselves still stream.
func (s *Store) PlayerIDsBySource(ctx context.Context, tour Tour) (map[string]int64, error) {
	rows, err := s.pool.Query(ctx, `SELECT source_id, id FROM players WHERE tour = $1`, tour)
	if err != nil {
		return nil, fmt.Errorf("load player ids: %w", err)
	}
	defer rows.Close()

	out := make(map[string]int64, 1024)
	for rows.Next() {
		var sourceID string
		var id int64
		if err := rows.Scan(&sourceID, &id); err != nil {
			return nil, fmt.Errorf("load player ids: %w", err)
		}
		out[sourceID] = id
	}
	return out, rows.Err()
}

// rankingWrite is one resolved ranking row.
type rankingWrite struct {
	playerID int64
	date     time.Time
	rank     int32
	points   *int32
}

// WriteRankings writes a batch of already-resolved rankings.
//
// COPY into a temporary table and then a single upsert: ranking history runs to
// millions of rows, and a statement per row would take hours. DISTINCT ON is
// required rather than tidy — Postgres refuses an ON CONFLICT that would touch
// the same row twice in one statement, and the sources do repeat a player on a
// date.
func (s *Store) WriteRankings(ctx context.Context, batch []rankingWrite) (n int64, err error) {
	if len(batch) == 0 {
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

	if _, err := tx.Exec(ctx, `
		CREATE TEMP TABLE rankings_in (
			player_id bigint, ranking_date date, rank integer, points integer
		) ON COMMIT DROP`); err != nil {
		return 0, fmt.Errorf("create staging table: %w", err)
	}

	_, err = tx.CopyFrom(ctx, pgx.Identifier{"rankings_in"},
		[]string{"player_id", "ranking_date", "rank", "points"},
		pgx.CopyFromSlice(len(batch), func(i int) ([]any, error) {
			r := batch[i]
			return []any{r.playerID, r.date, r.rank, r.points}, nil
		}))
	if err != nil {
		return 0, fmt.Errorf("copy rankings: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		INSERT INTO rankings (player_id, ranking_date, rank, points)
		SELECT DISTINCT ON (player_id, ranking_date) player_id, ranking_date, rank, points
		  FROM rankings_in
		 ORDER BY player_id, ranking_date, rank
		    ON CONFLICT (player_id, ranking_date) DO UPDATE
		   SET rank = EXCLUDED.rank, points = EXCLUDED.points`)
	if err != nil {
		return 0, fmt.Errorf("upsert rankings: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit rankings: %w", err)
	}
	return tag.RowsAffected(), nil
}

// RecordUnresolved persists references that could not be resolved, so the
// data-quality report can show them instead of them vanishing into a log line.
func (s *Store) RecordUnresolved(ctx context.Context, source, kind string, counts map[string]int) (err error) {
	if len(counts) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for id, n := range counts {
		batch.Queue(`
			INSERT INTO unresolved_references (source, kind, source_id, occurrences)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (source, kind, source_id) DO UPDATE
			   SET occurrences = EXCLUDED.occurrences, last_seen = now()`,
			source, kind, id, n)
	}
	results := s.pool.SendBatch(ctx, batch)
	defer func() {
		if cerr := results.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	for range counts {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("record unresolved reference: %w", err)
		}
	}
	return nil
}

// RefreshProminence recomputes the columns search ranks by.
//
// One pass over match_players rather than an aggregate per candidate at query
// time. Only rows whose values actually changed are written, so a re-run over
// unchanged data does no work.
func (s *Store) RefreshProminence(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE players p
		   SET career_matches = s.matches,
		       best_tier      = s.best_tier
		  FROM (
		        SELECT mp.player_id,
		               count(*)::int AS matches,
		               -- Enum order is tour, challenger, futures, itf.
		               min(t.tier)   AS best_tier
		          FROM match_players mp
		          JOIN matches m     ON m.id = mp.match_id
		          JOIN tournaments t ON t.id = m.tournament_id
		         GROUP BY mp.player_id
		       ) s
		 WHERE p.id = s.player_id
		   AND (p.career_matches IS DISTINCT FROM s.matches
		     OR p.best_tier      IS DISTINCT FROM s.best_tier)`)
	if err != nil {
		return fmt.Errorf("refresh player prominence: %w", err)
	}
	return nil
}
