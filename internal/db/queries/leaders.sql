-- name: ListLeaders :many
-- A leaderboard over player_totals: the filter is summed per player, the
-- named stat is computed from its own numerator and denominator, and the
-- rows are ranked on it. Every rate names the matches it stands on, in
-- `sample`: the matches that carried this stat's inputs, which for a serve
-- figure is the player's own serve lines, for a return figure the opponents',
-- and for a figure read from the score the matches whose score was read.
--
-- One query with the stat as a CASE rather than a query per stat, so the
-- filter, the ranking and the denominators cannot drift between them. A
-- player whose denominator is zero has no value and is not on the board.
-- Ascending is for the one stat where less is better, double faults.
WITH totals AS (
    SELECT pt.player_id,
           sum(pt.matches)::bigint AS matches,
           sum(pt.wins)::bigint AS wins,
           sum(pt.with_serve)::bigint AS with_serve,
           sum(pt.aces)::bigint AS aces,
           sum(pt.double_faults)::bigint AS double_faults,
           sum(pt.serve_points)::bigint AS serve_points,
           sum(pt.first_in)::bigint AS first_in,
           sum(pt.first_won)::bigint AS first_won,
           sum(pt.second_won)::bigint AS second_won,
           sum(pt.serve_games)::bigint AS serve_games,
           sum(pt.bp_saved)::bigint AS bp_saved,
           sum(pt.bp_faced)::bigint AS bp_faced,
           sum(pt.with_return)::bigint AS with_return,
           sum(pt.op_serve_points)::bigint AS op_serve_points,
           sum(pt.op_first_in)::bigint AS op_first_in,
           sum(pt.op_first_won)::bigint AS op_first_won,
           sum(pt.op_second_won)::bigint AS op_second_won,
           sum(pt.op_serve_games)::bigint AS op_serve_games,
           sum(pt.op_bp_saved)::bigint AS op_bp_saved,
           sum(pt.op_bp_faced)::bigint AS op_bp_faced,
           sum(pt.scored)::bigint AS scored,
           sum(pt.sets_won)::bigint AS sets_won,
           sum(pt.sets_played)::bigint AS sets_played,
           sum(pt.games_won)::bigint AS games_won,
           sum(pt.games_played)::bigint AS games_played,
           sum(pt.tiebreaks_won)::bigint AS tiebreaks_won,
           sum(pt.tiebreaks_played)::bigint AS tiebreaks_played,
           sum(pt.deciders_won)::bigint AS deciders_won,
           sum(pt.deciders_played)::bigint AS deciders_played
      FROM player_totals pt
     WHERE (sqlc.narg(tour)::tour IS NULL OR pt.tour = sqlc.narg(tour)::tour)
       AND (sqlc.narg(tier)::tier IS NULL OR pt.tier = sqlc.narg(tier)::tier)
       AND (sqlc.narg(surface)::surface IS NULL OR pt.surface = sqlc.narg(surface)::surface)
       AND (sqlc.narg(season)::smallint IS NULL OR pt.season = sqlc.narg(season)::smallint)
     GROUP BY pt.player_id
),
valued AS (
    SELECT t.player_id,
           CASE @stat::text
               WHEN 'aces'                THEN t.aces
               WHEN 'double_faults'       THEN t.double_faults
               WHEN 'first_serve_in'      THEN t.first_in
               WHEN 'first_serve_won'     THEN t.first_won
               WHEN 'second_serve_won'    THEN t.second_won
               WHEN 'serve_points_won'    THEN t.first_won + t.second_won
               WHEN 'break_points_saved'  THEN t.bp_saved
               WHEN 'service_games_held'  THEN t.serve_games - (t.bp_faced - t.bp_saved)
               WHEN 'return_points_won'   THEN t.op_serve_points - t.op_first_won - t.op_second_won
               WHEN 'first_return_won'    THEN t.op_first_in - t.op_first_won
               WHEN 'second_return_won'   THEN (t.op_serve_points - t.op_first_in) - t.op_second_won
               WHEN 'break_points_won'    THEN t.op_bp_faced - t.op_bp_saved
               WHEN 'return_games_won'    THEN t.op_bp_faced - t.op_bp_saved
               WHEN 'points_won'          THEN (t.first_won + t.second_won) + (t.op_serve_points - t.op_first_won - t.op_second_won)
               WHEN 'matches_won'         THEN t.wins
               WHEN 'sets_won'            THEN t.sets_won
               WHEN 'games_won'           THEN t.games_won
               WHEN 'tiebreaks_won'       THEN t.tiebreaks_won
               WHEN 'deciding_sets_won'   THEN t.deciders_won
               ELSE 0
           END::bigint AS numerator,
           CASE @stat::text
               WHEN 'aces'                THEN t.serve_points
               WHEN 'double_faults'       THEN t.serve_points
               WHEN 'first_serve_in'      THEN t.serve_points
               WHEN 'first_serve_won'     THEN t.first_in
               WHEN 'second_serve_won'    THEN t.serve_points - t.first_in
               WHEN 'serve_points_won'    THEN t.serve_points
               WHEN 'break_points_saved'  THEN t.bp_faced
               WHEN 'service_games_held'  THEN t.serve_games
               WHEN 'return_points_won'   THEN t.op_serve_points
               WHEN 'first_return_won'    THEN t.op_first_in
               WHEN 'second_return_won'   THEN t.op_serve_points - t.op_first_in
               WHEN 'break_points_won'    THEN t.op_bp_faced
               WHEN 'return_games_won'    THEN t.op_serve_games
               WHEN 'points_won'          THEN t.serve_points + t.op_serve_points
               WHEN 'matches_won'         THEN t.matches
               WHEN 'sets_won'            THEN t.sets_played
               WHEN 'games_won'           THEN t.games_played
               WHEN 'tiebreaks_won'       THEN t.tiebreaks_played
               WHEN 'deciding_sets_won'   THEN t.deciders_played
               ELSE 0
           END::bigint AS denominator,
           -- The dominance ratio is the one figure that is not a share of a
           -- count: return points won over serve points lost.
           CASE WHEN @stat::text = 'dominance'
                     AND t.op_serve_points > 0 AND t.serve_points > 0
                     AND t.serve_points - t.first_won - t.second_won > 0
                THEN ((t.op_serve_points - t.op_first_won - t.op_second_won)::float8 / t.op_serve_points)
                     / ((t.serve_points - t.first_won - t.second_won)::float8 / t.serve_points)
           END::float8 AS ratio,
           CASE @stat::text
               WHEN 'return_points_won'  THEN t.with_return
               WHEN 'first_return_won'   THEN t.with_return
               WHEN 'second_return_won'  THEN t.with_return
               WHEN 'break_points_won'   THEN t.with_return
               WHEN 'return_games_won'   THEN t.with_return
               WHEN 'points_won'         THEN least(t.with_serve, t.with_return)
               WHEN 'dominance'          THEN least(t.with_serve, t.with_return)
               WHEN 'matches_won'        THEN t.matches
               WHEN 'sets_won'           THEN t.scored
               WHEN 'games_won'          THEN t.scored
               WHEN 'tiebreaks_won'      THEN t.scored
               WHEN 'deciding_sets_won'  THEN t.scored
               ELSE t.with_serve
           END::bigint AS sample,
           t.matches
      FROM totals t
),
ranked AS (
    SELECT v.player_id, v.sample, v.matches, v.numerator, v.denominator,
           CASE WHEN @stat::text = 'dominance' THEN v.ratio
                ELSE v.numerator::float8 / nullif(v.denominator, 0)
           END::float8 AS value
      FROM valued v
     WHERE v.sample >= @min_matches::bigint
)
SELECT p.slug, p.full_name AS name, p.tour::text AS tour, p.country,
       r.sample, r.matches, r.numerator, r.denominator, r.value,
       count(*) OVER ()::bigint AS qualified
  FROM ranked r
  JOIN players p ON p.id = r.player_id
 WHERE r.value IS NOT NULL
 ORDER BY CASE WHEN @ascending::boolean THEN r.value END ASC,
          CASE WHEN NOT @ascending::boolean THEN r.value END DESC,
          r.sample DESC, p.slug
 LIMIT @row_limit;

-- name: CountLeaderPopulation :one
-- What the board is a board of: matches under the filter, in matches rather
-- than appearances, and how many of them the source recorded statistics for.
SELECT coalesce(sum(matches), 0)::bigint / 2 AS matches,
       coalesce(sum(with_stats), 0)::bigint / 2 AS with_stats,
       count(DISTINCT player_id)::bigint AS players
  FROM player_totals pt
 WHERE (sqlc.narg(tour)::tour IS NULL OR pt.tour = sqlc.narg(tour)::tour)
   AND (sqlc.narg(tier)::tier IS NULL OR pt.tier = sqlc.narg(tier)::tier)
   AND (sqlc.narg(surface)::surface IS NULL OR pt.surface = sqlc.narg(surface)::surface)
   AND (sqlc.narg(season)::smallint IS NULL OR pt.season = sqlc.narg(season)::smallint);
