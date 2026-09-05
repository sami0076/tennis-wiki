package ingest

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store writes ingested rows. Every write is an upsert against a natural key,
// so re-running over the same input changes nothing.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore wraps a pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Result counts what a batch did.
type Result struct {
	Matches int
	Players int
	// Collapsed counts input rows that shared a natural key with an earlier row
	// in the same batch. The sources do contain these, so it is not an error --
	// but leaving it uncounted makes rows seen and rows written disagree for no
	// visible reason.
	Collapsed int
}

// StartRun records the beginning of an ingest and returns its id.
func (s *Store) StartRun(ctx context.Context, source string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO ingest_runs (source) VALUES ($1) RETURNING id`, source).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("start ingest run: %w", err)
	}
	return id, nil
}

// FinishRun closes out a run. A non-nil runErr is stored rather than returned,
// so a failed run leaves a diagnosable record behind.
func (s *Store) FinishRun(ctx context.Context, id int64, seen, written int, runErr error) error {
	var msg *string
	if runErr != nil {
		text := runErr.Error()
		msg = &text
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE ingest_runs
		    SET finished_at = now(), rows_seen = $2, rows_written = $3, error = $4
		  WHERE id = $1`, id, seen, written, msg)
	if err != nil {
		return fmt.Errorf("finish ingest run %d: %w", id, err)
	}
	return nil
}

// WriteBatch persists a batch of rows in one transaction.
//
// Order matters: players and tournaments first, then matches, then
// match_players. The deferred foreign key on matches.winner_id is what allows
// a match to be written before its participants inside the transaction.
func (s *Store) WriteBatch(ctx context.Context, src Source, rows []MatchRow) (res Result, err error) {
	if len(rows) == 0 {
		return Result{}, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		// A rollback after a successful commit is a harmless no-op.
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			err = errors.Join(err, rbErr)
		}
	}()

	playerIDs, err := s.upsertPlayers(ctx, tx, src, rows)
	if err != nil {
		return Result{}, err
	}
	tournamentIDs, err := s.upsertTournaments(ctx, tx, src, rows)
	if err != nil {
		return Result{}, err
	}
	matches, collapsed, err := s.upsertMatches(ctx, tx, src, rows, playerIDs, tournamentIDs)
	if err != nil {
		return Result{}, err
	}
	if err := s.upsertMatchPlayers(ctx, tx, src, rows, playerIDs, tournamentIDs, matches); err != nil {
		return Result{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf("commit batch: %w", err)
	}
	return Result{Matches: len(matches), Players: len(playerIDs), Collapsed: collapsed}, nil
}

// playerKey identifies a player within one source.
type playerKey struct {
	sourceID string
	tour     Tour
}

// tourneyKey identifies a tournament within one source and season.
type tourneyKey struct {
	sourceID string
	season   int
	tour     Tour
}

// matchKey identifies a match by its natural key: the draw it sits in, its
// number within that draw, and the pair who played it. The number alone is not
// unique within a tournament and neither is the draw; see migrations 00007 and
// 00011. The pair is stored low id first, so a correction that swaps winner and
// loser is the same match.
type matchKey struct {
	tournamentID int64
	matchNum     int
	qualifying   bool
	lowPlayerID  int64
	highPlayerID int64
}

func newMatchKey(tournamentID int64, matchNum int, qualifying bool, a, b int64) matchKey {
	if a > b {
		a, b = b, a
	}
	return matchKey{tournamentID, matchNum, qualifying, a, b}
}

func (s *Store) upsertPlayers(ctx context.Context, tx pgx.Tx, src Source, rows []MatchRow) (map[playerKey]int64, error) {
	seen := make(map[playerKey]Player)
	for _, r := range rows {
		for _, p := range []Player{r.Winner, r.Loser} {
			k := playerKey{p.SourceID, src.Tour}
			// Prefer the record carrying the most detail: height and hand are
			// blank in some rows and present in others for the same player.
			if prev, ok := seen[k]; !ok || detail(p) > detail(prev) {
				seen[k] = p
			}
		}
	}

	ids := make(map[playerKey]int64, len(seen))
	for k, p := range seen {
		id, err := s.upsertPlayer(ctx, tx, src.Tour, p)
		if err != nil {
			return nil, err
		}
		ids[k] = id
	}
	return ids, nil
}

// detail scores how complete a player record is.
func detail(p Player) int {
	n := 0
	if p.Hand != "" {
		n++
	}
	if p.HeightCM != nil {
		n++
	}
	if p.Country != "" {
		n++
	}
	if p.Name != "" {
		n++
	}
	return n
}

// upsertPlayer inserts or updates one player, resolving slug collisions by
// trying the stable alternatives from DisambiguateSlug in order.
func (s *Store) upsertPlayer(ctx context.Context, tx pgx.Tx, tour Tour, p Player) (int64, error) {
	// An existing row keeps its slug: changing it would break bookmarked URLs.
	//
	// The alias lookup comes first so a reconciled id resolves to the player it
	// was merged into. Without it every re-ingest would recreate the duplicate
	// that identity reconciliation had just folded away.
	var id int64
	var found *int64
	err := tx.QueryRow(ctx, `
		SELECT coalesce(
		         (SELECT a.player_id FROM player_aliases a
		            JOIN players ap ON ap.id = a.player_id
		           WHERE a.source_id = $1 AND ap.tour = $2 LIMIT 1),
		         (SELECT id FROM players WHERE source_id = $1 AND tour = $2))`,
		p.SourceID, tour).Scan(&found)
	// Neither subquery matched, so this is a player we have not seen.
	if err == nil && found == nil {
		err = pgx.ErrNoRows
	}
	if found != nil {
		id = *found
	}
	if err == nil {
		if _, err := tx.Exec(ctx,
			`UPDATE players
			    SET full_name = $2,
			        first_name = COALESCE(NULLIF($3, ''), first_name),
			        last_name  = COALESCE(NULLIF($4, ''), last_name),
			        country    = COALESCE(NULLIF($5, '')::char(3), country),
			        hand       = COALESCE(NULLIF($6, '')::hand, hand),
			        height_cm  = COALESCE($7, height_cm),
			        birth_date = COALESCE($8, birth_date),
			        wikidata_id = COALESCE(NULLIF($9, ''), wikidata_id)
			  WHERE id = $1`,
			id, p.Name, firstName(p.Name), lastName(p.Name), p.Country, p.Hand, p.HeightCM,
			p.BirthDate, p.WikidataID); err != nil {
			return 0, fmt.Errorf("update player %s: %w", p.SourceID, err)
		}
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("look up player %s: %w", p.SourceID, err)
	}

	base := Slugify(p.Name)
	if base == "" {
		base = "player-" + p.SourceID
	}
	for attempt := 0; attempt < 50; attempt++ {
		slug := DisambiguateSlug(base, tour, attempt)
		// Each attempt runs inside a savepoint: in Postgres a constraint
		// violation aborts the whole transaction, so a failed try has to be
		// rolled back before the next one can run.
		sp, err := tx.Begin(ctx)
		if err != nil {
			return 0, fmt.Errorf("savepoint for player %s: %w", p.SourceID, err)
		}
		err = sp.QueryRow(ctx,
			`INSERT INTO players (source_id, tour, slug, full_name, first_name, last_name,
			                      country, hand, height_cm, birth_date, wikidata_id)
			 VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''),
			         NULLIF($7, '')::char(3), NULLIF($8, '')::hand, $9, $10, NULLIF($11, ''))
			 ON CONFLICT (source_id, tour) DO UPDATE SET full_name = EXCLUDED.full_name
			 RETURNING id`,
			p.SourceID, tour, slug, p.Name, firstName(p.Name), lastName(p.Name),
			p.Country, p.Hand, p.HeightCM, p.BirthDate, p.WikidataID).Scan(&id)
		if err == nil {
			if err := sp.Commit(ctx); err != nil {
				return 0, fmt.Errorf("commit player %s: %w", p.SourceID, err)
			}
			return id, nil
		}
		if rbErr := sp.Rollback(ctx); rbErr != nil {
			return 0, fmt.Errorf("roll back player %s: %w", p.SourceID, rbErr)
		}
		if isUniqueViolation(err, "players_slug_key") {
			continue
		}
		return 0, fmt.Errorf("insert player %s (%s): %w", p.SourceID, p.Name, err)
	}
	return 0, fmt.Errorf("could not find a free slug for %q", p.Name)
}

func (s *Store) upsertTournaments(ctx context.Context, tx pgx.Tx, src Source, rows []MatchRow) (map[tourneyKey]int64, error) {
	seen := make(map[tourneyKey]MatchRow)
	for _, r := range rows {
		k := tourneyKey{r.TourneyID, r.TourneyDate.Year(), src.Tour}
		if _, ok := seen[k]; !ok {
			seen[k] = r
		}
	}

	ids := make(map[tourneyKey]int64, len(seen))
	for k, r := range seen {
		var id int64
		err := tx.QueryRow(ctx,
			`INSERT INTO tournaments (source_id, tour, name, level, tier, surface,
			                          draw_size, start_date, season)
			 VALUES ($1, $2, $3, $4, $5::tier, NULLIF($6, '')::surface, $7, $8, $9)
			 ON CONFLICT (source_id, season, tour) DO UPDATE
			    SET name       = EXCLUDED.name,
			        level      = EXCLUDED.level,
			        tier       = EXCLUDED.tier,
			        surface    = COALESCE(EXCLUDED.surface, tournaments.surface),
			        draw_size  = COALESCE(EXCLUDED.draw_size, tournaments.draw_size),
			        start_date = EXCLUDED.start_date
			 RETURNING id`,
			r.TourneyID, src.Tour, r.TourneyName, r.Level, r.Tier(src.Tier), r.Surface,
			r.DrawSize, r.TourneyDate, k.season).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("upsert tournament %s: %w", r.TourneyID, err)
		}
		ids[k] = id
	}
	return ids, nil
}

func (s *Store) upsertMatches(
	ctx context.Context, tx pgx.Tx, src Source, rows []MatchRow,
	players map[playerKey]int64, tournaments map[tourneyKey]int64,
) (map[matchKey]int64, int, error) {
	ids := make(map[matchKey]int64, len(rows))
	collapsed := 0

	// One round trip per row was the measured bottleneck of a full ingest: at
	// 1.6 million matches the connection spends its life idle in transaction
	// waiting for the next statement. A pipelined batch sends them together.
	batch := &pgx.Batch{}
	keys := make([]matchKey, 0, len(rows))
	labels := make([]string, 0, len(rows))

	for _, r := range rows {
		tid, ok := tournaments[tourneyKey{r.TourneyID, r.TourneyDate.Year(), src.Tour}]
		if !ok {
			return nil, 0, fmt.Errorf("match %s/%d: tournament not written", r.TourneyID, r.MatchNum)
		}
		winnerID, ok := players[playerKey{r.Winner.SourceID, src.Tour}]
		if !ok {
			return nil, 0, fmt.Errorf("match %s/%d: winner not written", r.TourneyID, r.MatchNum)
		}
		loserID, ok := players[playerKey{r.Loser.SourceID, src.Tour}]
		if !ok {
			return nil, 0, fmt.Errorf("match %s/%d: loser not written", r.TourneyID, r.MatchNum)
		}

		parsed := classifyScore(r.Score)
		batch.Queue(
			`INSERT INTO matches (tournament_id, match_num, round, best_of, surface, score,
			                      minutes, winner_id, loser_id, played_on, incomplete,
			                      is_qualifying, is_team_event, has_detailed_stats, indoor, source)
			 VALUES ($1, $2, $3, $4, NULLIF($5, '')::surface, NULLIF($6, ''), $7, $8, $9,
			         $10, $11, $12, $13, $14, $15, $16)
			 ON CONFLICT (tournament_id, match_num, is_qualifying,
			              least(winner_id, loser_id), greatest(winner_id, loser_id)) DO UPDATE
			    SET round              = EXCLUDED.round,
			        best_of            = EXCLUDED.best_of,
			        surface            = COALESCE(EXCLUDED.surface, matches.surface),
			        score              = COALESCE(EXCLUDED.score, matches.score),
			        minutes            = COALESCE(EXCLUDED.minutes, matches.minutes),
			        winner_id          = EXCLUDED.winner_id,
			        loser_id           = EXCLUDED.loser_id,
			        played_on          = EXCLUDED.played_on,
			        incomplete         = EXCLUDED.incomplete,
			        is_qualifying      = EXCLUDED.is_qualifying,
			        is_team_event      = EXCLUDED.is_team_event,
			        has_detailed_stats = EXCLUDED.has_detailed_stats,
			        indoor             = COALESCE(EXCLUDED.indoor, matches.indoor),
			        source             = EXCLUDED.source
			 RETURNING id`,
			tid, r.MatchNum, r.Round, r.BestOf, r.Surface, r.Score, r.Minutes,
			winnerID, loserID, r.TourneyDate, parsed.incomplete, r.IsQualifying(),
			isTeamEvent(r.Level), r.HasDetailedStats(), r.Indoor, src.Name)

		keys = append(keys, newMatchKey(tid, r.MatchNum, r.IsQualifying(), winnerID, loserID))
		labels = append(labels, fmt.Sprintf("%s/%d", r.TourneyID, r.MatchNum))
	}
	if len(keys) == 0 {
		return ids, 0, nil
	}

	results := tx.SendBatch(ctx, batch)
	// Results arrive in the order they were queued, so index correlates a row
	// with the match it came from.
	for i, key := range keys {
		var id int64
		if err := results.QueryRow().Scan(&id); err != nil {
			return nil, 0, errors.Join(
				fmt.Errorf("upsert match %s: %w", labels[i], err), results.Close())
		}
		if _, dup := ids[key]; dup {
			collapsed++
		}
		ids[key] = id
	}
	if err := results.Close(); err != nil {
		return nil, 0, fmt.Errorf("close match batch: %w", err)
	}
	return ids, collapsed, nil
}

func (s *Store) upsertMatchPlayers(
	ctx context.Context, tx pgx.Tx, src Source, rows []MatchRow,
	players map[playerKey]int64, tournaments map[tourneyKey]int64, matches map[matchKey]int64,
) (err error) {
	batch := &pgx.Batch{}
	labels := make([]string, 0, len(rows)*2)
	for _, r := range rows {
		tid, ok := tournaments[tourneyKey{r.TourneyID, r.TourneyDate.Year(), src.Tour}]
		if !ok {
			return fmt.Errorf("match %s/%d: tournament not written", r.TourneyID, r.MatchNum)
		}
		winnerID, wok := players[playerKey{r.Winner.SourceID, src.Tour}]
		loserID, lok := players[playerKey{r.Loser.SourceID, src.Tour}]
		if !wok || !lok {
			return fmt.Errorf("match %s/%d: a player was not written", r.TourneyID, r.MatchNum)
		}
		matchID, ok := matches[newMatchKey(tid, r.MatchNum, r.IsQualifying(), winnerID, loserID)]
		if !ok {
			return fmt.Errorf("match %s/%d: not written", r.TourneyID, r.MatchNum)
		}

		for _, side := range []struct {
			id  int64
			p   Player
			won bool
		}{{winnerID, r.Winner, true}, {loserID, r.Loser, false}} {
			queueMatchPlayer(batch, matchID, side.id, side.won, side.p)
			// Batch results come back by position, so this is the only way to
			// name the row a constraint violation came from. "statement 138"
			// on its own is untraceable in a 1.6 million row ingest.
			labels = append(labels, fmt.Sprintf("%s/%d %s", r.TourneyID, r.MatchNum, side.p.Name))
		}
	}

	results := tx.SendBatch(ctx, batch)
	defer func() {
		if cerr := results.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close match_players batch: %w", cerr)
		}
	}()
	for i := 0; i < batch.Len(); i++ {
		if _, err := results.Exec(); err != nil {
			who := "unknown row"
			if i < len(labels) {
				who = labels[i]
			}
			return fmt.Errorf("write match_players for %s: %w", who, err)
		}
	}
	return nil
}

func queueMatchPlayer(batch *pgx.Batch, matchID, playerID int64, won bool, p Player) {
	var st Stats
	hasStats := p.Stats != nil
	if hasStats {
		st = *p.Stats
	}
	// Every statistic is passed as NULL when the source recorded none. Writing
	// zeros would produce numbers that are wrong but look plausible.
	batch.Queue(
		`INSERT INTO match_players (match_id, player_id, won, seed, entry, rank, rank_points,
		                            age, aces, double_faults, serve_points, first_in,
		                            first_won, second_won, serve_games, bp_saved, bp_faced)
		 VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7, $8,
		         $9, $10, $11, $12, $13, $14, $15, $16, $17)
		 ON CONFLICT (match_id, player_id) DO UPDATE
		    SET won           = EXCLUDED.won,
		        seed          = EXCLUDED.seed,
		        entry         = EXCLUDED.entry,
		        rank          = EXCLUDED.rank,
		        rank_points   = EXCLUDED.rank_points,
		        age           = EXCLUDED.age,
		        aces          = EXCLUDED.aces,
		        double_faults = EXCLUDED.double_faults,
		        serve_points  = EXCLUDED.serve_points,
		        first_in      = EXCLUDED.first_in,
		        first_won     = EXCLUDED.first_won,
		        second_won    = EXCLUDED.second_won,
		        serve_games   = EXCLUDED.serve_games,
		        bp_saved      = EXCLUDED.bp_saved,
		        bp_faced      = EXCLUDED.bp_faced`,
		matchID, playerID, won, p.Seed, p.Entry, p.Rank, p.RankPoints, p.Age,
		nullableInt(hasStats, st.Aces), nullableInt(hasStats, st.DoubleFaults),
		nullableInt(hasStats, st.ServePoints), nullableInt(hasStats, st.FirstIn),
		nullableInt(hasStats, st.FirstWon), nullableInt(hasStats, st.SecondWon),
		nullableInt(hasStats, st.ServeGames), nullableInt(hasStats, st.BPSaved),
		nullableInt(hasStats, st.BPFaced),
	)
}

func nullableInt(present bool, v int) *int {
	if !present {
		return nil
	}
	return &v
}

// TableChecksum returns a stable fingerprint of the ingested tables, used to
// prove that re-running changes nothing.
func (s *Store) TableChecksum(ctx context.Context) (string, error) {
	var sum string
	err := s.pool.QueryRow(ctx, `
		SELECT md5(string_agg(t, '|' ORDER BY t))
		  FROM (
		    SELECT 'p:' || source_id || tour || slug || full_name AS t FROM players
		    UNION ALL
		    SELECT 'm:' || tournament_id || match_num || round || COALESCE(score, '') ||
		           incomplete::text || has_detailed_stats::text || is_qualifying::text FROM matches
		    UNION ALL
		    SELECT 'mp:' || match_id || player_id || won::text ||
		           COALESCE(serve_points::text, 'null') FROM match_players
		  ) rows`).Scan(&sum)
	if err != nil {
		return "", fmt.Errorf("checksum tables: %w", err)
	}
	return sum, nil
}

// Counts returns row counts for the ingested tables.
func (s *Store) Counts(ctx context.Context) (map[string]int, error) {
	out := map[string]int{}
	for _, table := range []string{"players", "tournaments", "matches", "match_players", "rankings"} {
		var n int
		if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&n); err != nil {
			return nil, fmt.Errorf("count %s: %w", table, err)
		}
		out[table] = n
	}
	return out, nil
}
