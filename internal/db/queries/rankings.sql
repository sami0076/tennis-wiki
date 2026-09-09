-- name: EffectiveEloDate :one
-- The most recent week the ratings actually have, at or before the one asked
-- for. Never today: the coverage gap means those differ, and answering a
-- request for the current rankings with an empty list because today is outside
-- coverage is exactly the failure ADR-0006 exists to avoid.
--
-- The bound is not nullable: a caller who named no date passes the end of time,
-- which reads better than a nullable parameter and is the same question.
--
-- ORDER BY and LIMIT rather than max(): an aggregate always returns a row, and
-- that row is NULL when nothing matched, which is a scan error rather than the
-- no-rows the caller can act on.
--
-- The tour filter belongs here and not only on the leaderboard. WTA data stops
-- four years before ATP data does, and resolving the date across both tours
-- answers a request for the WTA rankings with an ATP week and an empty list.
SELECT r.as_of
  FROM ratings r
  JOIN players p ON p.id = r.player_id
 WHERE r.surface = @surface::rating_surface
   AND r.as_of <= @on_or_before::date
   AND (sqlc.narg(tour)::tour IS NULL OR p.tour = sqlc.narg(tour)::tour)
 ORDER BY r.as_of DESC
 LIMIT 1;


-- name: EffectiveOfficialDate :one
SELECT r.ranking_date
  FROM rankings r
  JOIN players p ON p.id = r.player_id
 WHERE (sqlc.narg(tour)::tour IS NULL OR p.tour = sqlc.narg(tour)::tour)
   AND r.ranking_date <= @on_or_before::date
 ORDER BY r.ranking_date DESC
 LIMIT 1;


-- name: ListEloRankings :many
WITH recent AS (
    SELECT DISTINCT ON (r.player_id)
           r.player_id, r.elo, r.as_of, r.matches_played
      FROM ratings r
     WHERE r.surface = @surface::rating_surface
       AND r.as_of <= @on_date::date
       AND r.as_of > @since::date
     ORDER BY r.player_id, r.as_of DESC
),
ranked AS (
    SELECT c.player_id, c.elo::float8 AS elo, c.matches_played,
           p.slug, p.full_name, p.tour, p.country, p.birth_date,
           rank() OVER (ORDER BY c.elo DESC, c.player_id)::int AS position
      FROM recent c
      JOIN players p ON p.id = c.player_id
     WHERE (sqlc.narg(tour)::tour IS NULL OR p.tour = sqlc.narg(tour)::tour)
)
SELECT k.player_id, k.slug, k.full_name, k.tour, k.country, k.birth_date,
       k.elo, k.matches_played, k.position,
       o.rank AS official_rank, o.points
  FROM ranked k
  LEFT JOIN rankings o
         ON o.player_id = k.player_id AND o.ranking_date = @official_date::date
 WHERE (sqlc.narg(after_elo)::float8 IS NULL
        OR k.elo < sqlc.narg(after_elo)::float8
        OR (k.elo = sqlc.narg(after_elo)::float8
            AND k.player_id > sqlc.narg(after_id)::bigint))
 ORDER BY k.elo DESC, k.player_id
 LIMIT @row_limit;

-- name: ListOfficialRankings :many
-- The tour's own list for one week. Elo comes from CurrentEloAsOf for the page
-- rather than a correlated subquery here: a player the ratings have never seen
-- has no Elo, and sqlc reads a subquery in the select list as never null.
SELECT o.player_id, p.slug, p.full_name, p.tour, p.country, p.birth_date,
       o.rank, o.points
  FROM rankings o
  JOIN players p ON p.id = o.player_id
 WHERE o.ranking_date = @on_date::date
   AND (sqlc.narg(tour)::tour IS NULL OR p.tour = sqlc.narg(tour)::tour)
   AND (sqlc.narg(after_rank)::int IS NULL
        OR o.rank > sqlc.narg(after_rank)::int
        OR (o.rank = sqlc.narg(after_rank)::int
            AND o.player_id > sqlc.narg(after_id)::bigint))
 ORDER BY o.rank, o.player_id
 LIMIT @row_limit;

-- name: CurrentEloAsOf :many
-- The rating each of a page of players held at a date. Sparse table, so it is
-- their last row at or before it.
SELECT DISTINCT ON (r.player_id) r.player_id, r.elo::float8 AS elo
  FROM ratings r
 WHERE r.surface = @surface::rating_surface
   AND r.as_of <= @on_date::date
   AND r.player_id = ANY(@player_ids::bigint[])
 ORDER BY r.player_id, r.as_of DESC;

-- name: PeakEloAsOf :many
SELECT player_id, max(elo)::float8 AS peak
  FROM ratings
 WHERE surface = @surface::rating_surface
   AND as_of <= @on_date::date
   AND player_id = ANY(@player_ids::bigint[])
 GROUP BY player_id;

-- name: ListRatingTrajectories :many
-- The weekly series for a handful of players, which is what a multi-series
-- chart is: one line each, rather than a leaderboard.
--
-- The bounds are two plain dates rather than a date and a width. The caller has
-- to know the window anyway to label the chart, and sqlc miscompiles the
-- arithmetic form here.
SELECT r.player_id, r.as_of AS week, r.elo::float8 AS elo
  FROM ratings r
 WHERE r.surface = @surface::rating_surface
   AND r.player_id = ANY(@player_ids::bigint[])
   AND r.as_of > @from_date::date
   AND r.as_of <= @to_date::date
 ORDER BY r.player_id, r.as_of;
