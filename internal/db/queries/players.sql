-- name: GetPlayerBySlug :one
SELECT id, slug, tour, full_name, first_name, last_name, country, hand,
       height_cm, birth_date, pro_since, wikidata_id
  FROM players
 WHERE slug = @slug;

-- name: SearchPlayers :many
-- Trigram search over 115k players. Similarity alone makes "Alexander" useless,
-- so the score is weighted by the best tier the player has reached: the weights
-- only ever reorder names of comparable similarity. Ordering on the single
-- combined rank keeps the keyset cursor to one value plus the id tiebreak.
WITH scored AS (
    SELECT p.id, p.slug, p.tour, p.full_name, p.country,
           similarity(p.full_name, @query::text)::real AS score,
           p.career_matches::bigint                   AS matches,
           coalesce(p.best_tier::text, '')            AS best_tier
      FROM players p
     WHERE p.full_name % @query::text
       AND (sqlc.narg(tour)::tour IS NULL OR p.tour = sqlc.narg(tour)::tour)
),
ranked AS (
    SELECT s.*,
           (s.score * CASE s.best_tier
               WHEN 'tour'       THEN 1.00
               WHEN 'challenger' THEN 0.85
               WHEN 'futures'    THEN 0.70
               WHEN 'itf'        THEN 0.70
               ELSE 0.60
            END)::real AS rank
      FROM scored s
)
SELECT id, slug, tour, full_name, country, score, matches, best_tier::text AS best_tier, rank
  FROM ranked
 -- Rank descends but id ascends, so the row-value form (rank, id) < (...)
 -- would walk the wrong way through a run of equal ranks and repeat rows.
 WHERE (sqlc.narg(after_rank)::real IS NULL
        OR rank < sqlc.narg(after_rank)::real
        OR (rank = sqlc.narg(after_rank)::real AND id > sqlc.narg(after_id)::bigint))
 ORDER BY rank DESC, id
 LIMIT @row_limit;

-- name: GetPlayerCareerSummary :one
-- Rate statistics come from a separate aggregate over only the matches that
-- recorded them: a NULL sum with stat_matches = 0 means never recorded, which
-- the response must not collapse into a zero.
WITH played AS (
    SELECT mp.won, mp.aces, mp.double_faults, mp.serve_points, mp.first_in,
           mp.first_won, mp.second_won, mp.serve_games, mp.bp_saved, mp.bp_faced,
           m.incomplete, m.round, m.is_qualifying, m.played_on, t.level
      FROM match_players mp
      JOIN matches m     ON m.id = mp.match_id
      JOIN tournaments t ON t.id = m.tournament_id
     WHERE mp.player_id = @player_id
),
counted AS (
    SELECT count(*)::bigint                             AS matches,
           count(*) FILTER (WHERE won)::bigint          AS wins,
           count(*) FILTER (WHERE NOT won)::bigint      AS losses,
           -- Retirements and walkovers count here but are excluded from rates.
           count(*) FILTER (WHERE incomplete)::bigint   AS incomplete_matches,
           count(*) FILTER (WHERE won AND round = 'F' AND NOT is_qualifying)::bigint AS titles,
           -- Majors are titles at level G. Counted separately because 11 majors
           -- and 66 titles are two different claims about the same career.
           count(*) FILTER (WHERE won AND round = 'F' AND NOT is_qualifying
                              AND level = 'G')::bigint AS majors,
           min(played_on)::date                         AS first_match,
           max(played_on)::date                         AS last_match
      FROM played
    -- No matches means no career summary at all. Returning a row of
    -- NULLs instead would not survive the scan.
    HAVING count(*) > 0
),
served AS (
    SELECT count(*)::bigint         AS stat_matches,
           coalesce(sum(aces), 0)::bigint        AS aces,
           coalesce(sum(double_faults), 0)::bigint AS double_faults,
           coalesce(sum(serve_points), 0)::bigint  AS serve_points,
           coalesce(sum(first_in), 0)::bigint      AS first_in,
           coalesce(sum(first_won), 0)::bigint     AS first_won,
           coalesce(sum(second_won), 0)::bigint    AS second_won,
           coalesce(sum(serve_games), 0)::bigint   AS serve_games,
           coalesce(sum(bp_saved), 0)::bigint      AS bp_saved,
           coalesce(sum(bp_faced), 0)::bigint      AS bp_faced
      FROM played
     WHERE serve_points IS NOT NULL AND NOT incomplete
)
SELECT * FROM counted, served;

-- name: GetPlayerSurfaceSplits :many
SELECT m.surface,
       count(*)::bigint                            AS matches,
       count(*) FILTER (WHERE mp.won)::bigint      AS wins,
       count(*) FILTER (WHERE NOT mp.won)::bigint  AS losses
  FROM match_players mp
  JOIN matches m ON m.id = mp.match_id
 WHERE mp.player_id = @player_id
 GROUP BY m.surface
 ORDER BY matches DESC;

-- name: GetPlayerTierSplits :many
-- Which tiers a player competed at, so the API can say a statistic was never
-- recorded for their level rather than implying it was zero.
SELECT t.tier,
       count(*)::bigint                                       AS matches,
       count(*) FILTER (WHERE m.has_detailed_stats)::bigint   AS matches_with_stats
  FROM match_players mp
  JOIN matches m     ON m.id = mp.match_id
  JOIN tournaments t ON t.id = m.tournament_id
 WHERE mp.player_id = @player_id
 GROUP BY t.tier
 ORDER BY matches DESC;

-- name: GetPlayerRatings :many
-- Current and peak per series, in one pass over the player's rows.
--
-- The table is sparse -- a row exists only for a week a series moved -- so
-- "current" is the last row, not a row at any particular date, and there is no
-- date on which every series has one.
--
-- Ordered overall first and then by how much play backs each surface, which is
-- the order the surface strip reads in.
WITH latest AS (
    SELECT DISTINCT ON (r.surface) r.surface, r.elo, r.as_of, r.matches_played
      FROM ratings r
     WHERE r.player_id = @player_id
     ORDER BY r.surface, r.as_of DESC
),
peak AS (
    SELECT DISTINCT ON (r.surface) r.surface, r.elo, r.as_of
      FROM ratings r
     WHERE r.player_id = @player_id
     ORDER BY r.surface, r.elo DESC, r.as_of
)
SELECT l.surface,
       l.matches_played,
       l.elo::float8   AS current_elo,
       l.as_of         AS current_as_of,
       p.elo::float8   AS peak_elo,
       p.as_of         AS peak_as_of
  FROM latest l
  JOIN peak p ON p.surface = l.surface
 ORDER BY (l.surface <> 'overall'), l.matches_played DESC, l.surface;

-- name: ListPlayerRatingSeries :many
-- The whole trajectory for one series. Not paginated: a chart wants the line,
-- and a page of a line is not one.
SELECT as_of, elo::float8 AS elo, matches_played
  FROM ratings
 WHERE player_id = @player_id
   AND surface = @surface::rating_surface
   AND (sqlc.narg(from_date)::date IS NULL OR as_of >= sqlc.narg(from_date)::date)
   AND (sqlc.narg(to_date)::date IS NULL OR as_of <= sqlc.narg(to_date)::date)
 ORDER BY as_of;

-- name: ListPlayerRankingHistory :many
-- The published ATP/WTA ranking over time. Not paginated, for the same reason
-- the rating series is not: a line is not read a page at a time.
SELECT ranking_date, rank, points
  FROM rankings
 WHERE player_id = @player_id
   AND (sqlc.narg(from_date)::date IS NULL OR ranking_date >= sqlc.narg(from_date)::date)
   AND (sqlc.narg(to_date)::date IS NULL OR ranking_date <= sqlc.narg(to_date)::date)
 ORDER BY ranking_date;

-- name: GetPlayerClutch :one
-- Under pressure, measured against the tour over the same levels and years.
--
-- The baseline is weighted by where this player's own opportunities actually
-- fell. A career that spans Futures in the 1990s and tour level in the 2020s is
-- compared with what the tour did across that same spread, rather than against
-- one era it only half belongs to -- which is the judgement the issue said had
-- to be made and stated.
--
-- Both halves are read from columns the ingest derived. Neither the score
-- parsing nor a 1.6 million match aggregate belongs in a request.
WITH played AS (
    SELECT t.tour,
           t.tier,
           ((t.season / 10) * 10)::smallint                   AS decade,
           count(*)                                           AS appearances,
           count(*) FILTER (WHERE m.deciding_set IS NOT NULL) AS scored,
           coalesce(sum(mp.bp_saved), 0)                      AS bp_saved,
           coalesce(sum(mp.bp_faced), 0)                      AS bp_faced,
           coalesce(sum(CASE WHEN mp.won THEN m.tiebreaks_winner
                                         ELSE m.tiebreaks_loser END), 0) AS tiebreaks_won,
           coalesce(sum(m.tiebreaks_winner + m.tiebreaks_loser), 0)      AS tiebreaks_played,
           count(*) FILTER (WHERE m.deciding_set AND mp.won)  AS deciding_sets_won,
           count(*) FILTER (WHERE m.deciding_set)             AS deciding_sets_played
      FROM match_players mp
      JOIN matches m     ON m.id = mp.match_id
      JOIN tournaments t ON t.id = m.tournament_id
     WHERE mp.player_id = @player_id
       AND NOT m.is_team_event
     GROUP BY 1, 2, 3
)
SELECT
    coalesce(sum(p.appearances), 0)::bigint          AS appearances,
    coalesce(sum(p.scored), 0)::bigint               AS scored,
    coalesce(sum(p.bp_saved), 0)::bigint             AS bp_saved,
    coalesce(sum(p.bp_faced), 0)::bigint             AS bp_faced,
    coalesce(sum(p.tiebreaks_won), 0)::bigint        AS tiebreaks_won,
    coalesce(sum(p.tiebreaks_played), 0)::bigint     AS tiebreaks_played,
    coalesce(sum(p.deciding_sets_won), 0)::bigint    AS deciding_sets_won,
    coalesce(sum(p.deciding_sets_played), 0)::bigint AS deciding_sets_played,
    -- Each baseline is the player's own opportunities in a cell against what
    -- the tour did in that cell. A cell where either side had no opportunities
    -- of that kind leaves both the numerator and the denominator, so it cannot
    -- drag the average toward a rate nobody recorded.
    -- Weight of zero is how "no baseline" arrives: a rate over no opportunities
    -- would be a comparison against nothing, and 0.0 would look like one
    -- against something.
    coalesce(sum(p.bp_faced * b.bp_saved::float8 / b.bp_faced)
        FILTER (WHERE b.bp_faced > 0 AND p.bp_faced > 0), 0)::float8      AS baseline_bp_weighted,
    coalesce(sum(p.bp_faced)
        FILTER (WHERE b.bp_faced > 0 AND p.bp_faced > 0), 0)::bigint      AS baseline_bp_weight,
    coalesce(sum(p.tiebreaks_played * b.tiebreaks_won::float8 / b.tiebreaks_played)
        FILTER (WHERE b.tiebreaks_played > 0 AND p.tiebreaks_played > 0), 0)::float8 AS baseline_tiebreaks_weighted,
    coalesce(sum(p.tiebreaks_played)
        FILTER (WHERE b.tiebreaks_played > 0 AND p.tiebreaks_played > 0), 0)::bigint AS baseline_tiebreaks_weight,
    coalesce(sum(p.deciding_sets_played * b.deciding_sets_won::float8 / b.deciding_sets_played)
        FILTER (WHERE b.deciding_sets_played > 0 AND p.deciding_sets_played > 0), 0)::float8 AS baseline_deciding_weighted,
    coalesce(sum(p.deciding_sets_played)
        FILTER (WHERE b.deciding_sets_played > 0 AND p.deciding_sets_played > 0), 0)::bigint AS baseline_deciding_weight,
    -- What the comparison is against, so the response can state it rather than
    -- leaving a reader to assume it.
    coalesce(min(p.decade), 0)::smallint              AS from_decade,
    coalesce(max(p.decade), 0)::smallint              AS to_decade,
    coalesce(sum(b.appearances), 0)::bigint          AS baseline_appearances,
    array_remove(array_agg(DISTINCT p.tier::text), NULL)::text[] AS tiers
  FROM played p
  LEFT JOIN clutch_baselines b
         ON b.tour = p.tour AND b.tier = p.tier AND b.decade = p.decade;

-- name: ListServeBaselines :many
-- Every serve-baseline cell for one tour, for the anchor lookup in
-- internal/simulate.
--
-- The whole tour rather than one cell, because the lookup widens when a cell is
-- thin and a widening query would be four round trips or one query nobody can
-- read. There are at most a few hundred rows per tour.
SELECT tier::text AS tier, surface::text AS surface, decade, serve_points, serve_won
  FROM serve_baselines
 WHERE tour = @tour::tour
 ORDER BY tier, surface, decade;

-- name: ListPlayerOpponentRanks :many
-- Every finished, non-team match of one player with the ranking each side
-- held on the day, for the record by opponent rank, and the score and
-- deciding-set flag for the matches decided by a final-set tiebreak. A few
-- hundred rows for a long career, bucketed in Go.
SELECT mp.won, mp.rank AS own_rank, op.rank AS opponent_rank, m.score, m.deciding_set
  FROM match_players mp
  JOIN matches m ON m.id = mp.match_id
  JOIN match_players op ON op.match_id = m.id AND op.player_id <> mp.player_id
 WHERE mp.player_id = @player_id
   AND NOT m.incomplete AND NOT m.is_team_event;

-- name: ListPlayerSeasonTotals :many
-- One player's year-by-year, summed from player_totals across tier and
-- surface. Totals, so the page computes every rate over its own count of
-- matches: a season with four recorded matches and sixty played must not
-- read as a season of four.
SELECT season,
       sum(matches)::bigint AS matches,
       sum(wins)::bigint AS wins,
       sum(titles)::bigint AS titles,
       sum(with_serve)::bigint AS with_serve,
       sum(aces)::bigint AS aces,
       sum(double_faults)::bigint AS double_faults,
       sum(serve_points)::bigint AS serve_points,
       sum(first_won)::bigint AS first_won,
       sum(second_won)::bigint AS second_won,
       sum(serve_games)::bigint AS serve_games,
       sum(bp_saved)::bigint AS bp_saved,
       sum(bp_faced)::bigint AS bp_faced,
       sum(with_return)::bigint AS with_return,
       sum(op_serve_points)::bigint AS op_serve_points,
       sum(op_first_won)::bigint AS op_first_won,
       sum(op_second_won)::bigint AS op_second_won,
       sum(op_serve_games)::bigint AS op_serve_games,
       sum(op_bp_saved)::bigint AS op_bp_saved,
       sum(op_bp_faced)::bigint AS op_bp_faced,
       sum(scored)::bigint AS scored,
       sum(sets_won)::bigint AS sets_won,
       sum(sets_played)::bigint AS sets_played,
       sum(games_won)::bigint AS games_won,
       sum(games_played)::bigint AS games_played,
       sum(tiebreaks_won)::bigint AS tiebreaks_won,
       sum(tiebreaks_played)::bigint AS tiebreaks_played,
       sum(deciders_won)::bigint AS deciders_won,
       sum(deciders_played)::bigint AS deciders_played
  FROM player_totals
 WHERE player_id = @player_id
 GROUP BY season
 ORDER BY season;

-- name: GetPlayerReturnSummary :one
-- The other half of a career. A return figure is made of the opponent's serve
-- line, never of the player's own, so it is counted over the matches that
-- carried the opponent's line -- which is not always the same set as the ones
-- that carried this player's. Both denominators are reported so neither rate
-- borrows the other's.
--
-- The last three columns are the player's own serve line over the subset where
-- both sides were recorded, because total points won and the dominance ratio
-- are quotients of the two and a match with only one line would bias them.
WITH played AS (
    SELECT mp.serve_points   AS own_points,
           mp.first_won      AS own_first_won,
           mp.second_won     AS own_second_won,
           op.serve_points,
           op.first_in,
           op.first_won,
           op.second_won,
           op.serve_games,
           op.bp_saved,
           op.bp_faced
      FROM match_players mp
      JOIN matches m       ON m.id = mp.match_id
      JOIN match_players op ON op.match_id = m.id AND op.player_id <> mp.player_id
     WHERE mp.player_id = @player_id
       AND NOT m.incomplete
)
SELECT count(*) FILTER (WHERE serve_points IS NOT NULL)::bigint       AS stat_matches,
       coalesce(sum(serve_points), 0)::bigint                         AS serve_points,
       coalesce(sum(first_in), 0)::bigint                             AS first_in,
       coalesce(sum(first_won), 0)::bigint                            AS first_won,
       coalesce(sum(second_won), 0)::bigint                           AS second_won,
       coalesce(sum(serve_games), 0)::bigint                          AS serve_games,
       coalesce(sum(bp_saved), 0)::bigint                             AS bp_saved,
       coalesce(sum(bp_faced), 0)::bigint                             AS bp_faced,
       count(*) FILTER (WHERE serve_points IS NOT NULL
                          AND own_points IS NOT NULL)::bigint         AS both_matches,
       coalesce(sum(own_points) FILTER (WHERE serve_points IS NOT NULL), 0)::bigint     AS own_serve_points,
       coalesce(sum(own_first_won) FILTER (WHERE serve_points IS NOT NULL), 0)::bigint  AS own_first_won,
       coalesce(sum(own_second_won) FILTER (WHERE serve_points IS NOT NULL), 0)::bigint AS own_second_won
  FROM played;

-- name: ListPlayerStreaks :many
-- Gaps and islands over one career: the difference between a row's position in
-- the whole sequence and its position among rows of the same result is constant
-- inside a run and changes at every switch, so grouping on it groups the runs.
--
-- Retirements and walkovers are left out: a run of wins broken by an opponent
-- who never came out has not been broken by a defeat. Team events are out for
-- the same reason Elo leaves them out.
--
-- Up to three labelled rows rather than one row of nullable columns: a career
-- with no defeat has no worst run, and that is a row that is not there rather
-- than a run of length zero.
WITH played AS (
    SELECT m.played_on, m.id, mp.won,
           row_number() OVER (ORDER BY m.played_on, m.id)
         - row_number() OVER (PARTITION BY mp.won ORDER BY m.played_on, m.id) AS run
      FROM match_players mp
      JOIN matches m ON m.id = mp.match_id
     WHERE mp.player_id = @player_id
       AND NOT m.is_team_event AND NOT m.incomplete
),
runs AS (
    SELECT won,
           count(*)::bigint     AS length,
           min(played_on)::date AS from_date,
           max(played_on)::date AS to_date,
           max(id)              AS last_id
      FROM played
     GROUP BY won, run
),
best AS (
    SELECT * FROM runs WHERE won ORDER BY length DESC, to_date DESC LIMIT 1
),
worst AS (
    SELECT * FROM runs WHERE NOT won ORDER BY length DESC, to_date DESC LIMIT 1
),
latest AS (
    SELECT * FROM runs ORDER BY to_date DESC, last_id DESC LIMIT 1
)
SELECT 'best'::text AS kind, won, length, from_date, to_date FROM best
UNION ALL
SELECT 'worst'::text AS kind, won, length, from_date, to_date FROM worst
UNION ALL
SELECT 'current'::text AS kind, won, length, from_date, to_date FROM latest;

-- name: ListPlayerBestWins :many
-- The wins that cost the most to get: every opponent carries the overall Elo
-- they held on the day, read from the last weekly snapshot on or before the
-- match, and the list is ordered on it.
--
-- Rating the opponent as they were rather than as they ended keeps a win over
-- a future champion from being credited with the champion's peak.
SELECT m.played_on,
       t.name        AS tournament,
       e.slug        AS event_slug,
       t.season,
       t.level,
       t.tier,
       m.surface,
       m.round,
       m.score,
       op.slug       AS opponent_slug,
       op.full_name  AS opponent_name,
       op.country    AS opponent_country,
       oe.elo::float8 AS opponent_elo,
       oe.as_of      AS elo_as_of
  FROM match_players mp
  JOIN matches m      ON m.id = mp.match_id
  JOIN tournaments t  ON t.id = m.tournament_id
  LEFT JOIN events e  ON e.id = t.event_id
  JOIN players op     ON op.id = m.loser_id
  JOIN LATERAL (
        SELECT r.elo, r.as_of
          FROM ratings r
         WHERE r.player_id = m.loser_id
           AND r.surface = 'overall'
           AND r.as_of <= m.played_on
         ORDER BY r.as_of DESC
         LIMIT 1) oe ON true
 WHERE mp.player_id = @player_id
   AND mp.won
   AND NOT m.incomplete AND NOT m.is_team_event AND NOT m.is_qualifying
 ORDER BY oe.elo DESC, m.played_on DESC
 LIMIT @row_limit;

-- name: ListPlayerRoundRecord :many
-- How far a career got, round by round. Qualifying is excluded: reaching the
-- second round of qualifying and the second round of a draw are not the same
-- achievement and must not be summed into one row.
SELECT m.round,
       count(*)::bigint                        AS matches,
       count(*) FILTER (WHERE mp.won)::bigint  AS wins
  FROM match_players mp
  JOIN matches m ON m.id = mp.match_id
 WHERE mp.player_id = @player_id
   AND NOT m.is_team_event AND NOT m.is_qualifying AND NOT m.incomplete
 GROUP BY m.round;

-- name: ListPlayerFinalsByCategory :many
-- Titles and finals by what the event was, in the same words the season index
-- uses, so a slam title and a Challenger title are never one number.
SELECT CASE
           WHEN t.level = 'D' OR t.event_link = 'team' THEN 'team'
           WHEN t.level = 'G' THEN 'slam'
           WHEN t.level = 'F' THEN 'finals'
           WHEN t.level = 'O' THEN 'olympics'
           WHEN t.level IN ('M', 'PM', '1000') THEN 'masters'
           ELSE t.tier::text
       END::text                               AS category,
       count(*)::bigint                        AS finals,
       count(*) FILTER (WHERE mp.won)::bigint  AS titles
  FROM match_players mp
  JOIN matches m     ON m.id = mp.match_id
  JOIN tournaments t ON t.id = m.tournament_id
 WHERE mp.player_id = @player_id
   AND m.round = 'F' AND NOT m.is_qualifying
 GROUP BY 1
 ORDER BY titles DESC, finals DESC, category;

-- name: ListPlayerRivals :many
-- Who a career was spent against. Ordered on meetings rather than on the
-- record, because the question a rivalry list answers is who kept turning up.
SELECT op.slug,
       op.full_name         AS name,
       op.country,
       count(*)::bigint     AS matches,
       count(*) FILTER (WHERE mp.won)::bigint AS wins,
       max(m.played_on)::date AS last_played
  FROM match_players mp
  JOIN matches m  ON m.id = mp.match_id
  JOIN players op ON op.id = CASE WHEN mp.won THEN m.loser_id ELSE m.winner_id END
 WHERE mp.player_id = @player_id
   AND NOT m.is_team_event
 GROUP BY op.slug, op.full_name, op.country
 ORDER BY matches DESC, last_played DESC, op.full_name
 LIMIT @row_limit;

-- name: GetPlayerOpponentQuality :one
-- What the schedule was worth: the Elo every opponent held on the day, averaged,
-- and the record against the ones above a bar. A match whose opponent the model
-- had not rated yet is left out of both rather than counted at the base rating.
WITH faced AS (
    SELECT mp.won, oe.elo::float8 AS elo
      FROM match_players mp
      JOIN matches m ON m.id = mp.match_id
      JOIN LATERAL (
            SELECT r.elo
              FROM ratings r
             WHERE r.player_id = CASE WHEN mp.won THEN m.loser_id ELSE m.winner_id END
               AND r.surface = 'overall'
               AND r.as_of <= m.played_on
             ORDER BY r.as_of DESC
             LIMIT 1) oe ON true
     WHERE mp.player_id = @player_id
       AND NOT m.is_team_event AND NOT m.incomplete
)
SELECT count(*)::bigint                                          AS rated_matches,
       coalesce(avg(elo), 0)::float8                             AS average_elo,
       coalesce(max(elo), 0)::float8                             AS highest_elo,
       count(*) FILTER (WHERE elo >= @elite_elo::float8)::bigint AS elite_matches,
       count(*) FILTER (WHERE elo >= @elite_elo::float8 AND won)::bigint AS elite_wins
  FROM faced;
