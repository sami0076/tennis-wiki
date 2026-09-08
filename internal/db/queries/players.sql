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
