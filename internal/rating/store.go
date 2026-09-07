package rating

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sami0076/tennis-wiki/internal/score"
)

// Store is the database side of the rating engine.
type Store struct{ pool *pgxpool.Pool }

// NewStore wraps a pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// matchQuery streams every match with what the engine needs to weight it.
//
// The ordering is the correctness requirement, not a convenience. played_on is
// a tournament's start date for nearly every row in the database, so the round
// has to break the tie or a final is rated before the semi-final that produced
// its finalist. match_num and id follow, to make the order total.
var matchQuery = `
	SELECT m.winner_id, m.loser_id, m.played_on, m.surface, m.score,
	       t.tier, t.level, m.round, m.is_qualifying, m.is_team_event
	  FROM matches m
	  JOIN tournaments t ON t.id = m.tournament_id
	 ORDER BY m.played_on, m.tournament_id, ` + RoundRankSQL("m.round") + `, m.match_num, m.id`

// Excluded counts the matches the replay declined to rate, so the exclusions
// are reported rather than implied.
type Excluded struct {
	// TeamEvents are Davis Cup and the like, excluded unless asked for.
	TeamEvents int
	// Walkovers are matches nobody played. A retirement is rated -- tennis was
	// played and someone won it -- but a walkover would move two ratings on no
	// evidence at all.
	Walkovers int
}

// Matches streams every rated match in order and hands it to fn.
func (s *Store) Matches(ctx context.Context, teamEvents bool, fn func(Result) error) (n int, ex Excluded, err error) {
	rows, err := s.pool.Query(ctx, matchQuery)
	if err != nil {
		return 0, ex, fmt.Errorf("read matches: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			r                     Result
			surface, raw          *string
			tier, level, round    string
			qualifying, teamEvent bool
		)
		if err := rows.Scan(&r.WinnerID, &r.LoserID, &r.PlayedOn, &surface, &raw,
			&tier, &level, &round, &qualifying, &teamEvent); err != nil {
			return n, ex, fmt.Errorf("read matches: %w", err)
		}

		if teamEvent && !teamEvents {
			ex.TeamEvents++
			continue
		}
		if raw != nil {
			// The parser owns what a score string means; matching on it here
			// would be a second, quietly diverging opinion.
			parsed, err := score.Parse(*raw)
			if err == nil && parsed.Outcome == score.Walkover {
				ex.Walkovers++
				continue
			}
		}

		parsedTier, err := ParseTier(tier)
		if err != nil {
			return n, ex, err
		}
		if s, ok := ParseSurface(derefString(surface)); ok {
			r.Surface = s
		}
		r.Match = Match{
			Tier:         parsedTier,
			Level:        Level(level),
			IsFinal:      round == "F",
			IsQualifying: qualifying,
		}

		if err := fn(r); err != nil {
			return n, ex, err
		}
		n++
	}
	return n, ex, rows.Err()
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ratingColumns is the copy target, in order.
var ratingColumns = []string{"player_id", "as_of", "surface", "elo", "matches_played"}

// writer batches snapshots into COPY. One row per statement would be seven
// million round trips, which is the mistake ingest already made once and
// measured; see docs/performance.md.
type writer struct {
	tx    pgx.Tx
	batch int
	buf   [][]any
	rows  int64
}

func (w *writer) add(ctx context.Context, s Snapshot) error {
	w.buf = append(w.buf, []any{s.PlayerID, s.AsOf, string(s.Series), s.Elo, s.Matches})
	if len(w.buf) < w.batch {
		return nil
	}
	return w.flush(ctx)
}

func (w *writer) flush(ctx context.Context) error {
	if len(w.buf) == 0 {
		return nil
	}
	n, err := w.tx.CopyFrom(ctx, pgx.Identifier{"ratings"}, ratingColumns, pgx.CopyFromRows(w.buf))
	if err != nil {
		return fmt.Errorf("write ratings: %w", err)
	}
	w.rows += n
	w.buf = w.buf[:0]
	return nil
}

// Replace empties the ratings table and refills it from the replay fn drives.
//
// One transaction, so a run that dies leaves the previous ratings in place
// rather than a half-recomputed table. The TRUNCATE holds an exclusive lock for
// the length of the run, which is the price of never serving half an answer.
func (s *Store) Replace(ctx context.Context, batch int, fn func(emit func(Snapshot) error) error) (rows int64, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			err = errors.Join(err, rbErr)
		}
	}()

	if _, err := tx.Exec(ctx, `TRUNCATE ratings`); err != nil {
		return 0, fmt.Errorf("empty the ratings table: %w", err)
	}

	w := &writer{tx: tx, batch: batch}
	if err := fn(func(snap Snapshot) error { return w.add(ctx, snap) }); err != nil {
		return 0, err
	}
	if err := w.flush(ctx); err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit ratings: %w", err)
	}
	return w.rows, nil
}
