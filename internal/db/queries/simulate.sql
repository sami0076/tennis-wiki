-- name: FindTournament :one
-- Resolve an event by what a URL can carry, rather than by a numeric id nobody
-- can read or guess.
SELECT id, name, season, tour::text AS tour, tier::text AS tier,
       -- The source leaves surface blank for some events. "unknown" is the same
       -- stand-in the career and head-to-head splits use, so one absent surface
       -- does not have two spellings across the API.
       coalesce(surface::text, 'unknown')::text AS surface,
       level, draw_size, start_date
  FROM tournaments
 WHERE tour = @tour::tour AND season = @season::smallint AND lower(name) = lower(@name::text)
 ORDER BY start_date
 LIMIT 1;

-- name: ListDrawMatches :many
-- Every main-draw match of one event, with both players, for reconstructing the
-- bracket.
--
-- Qualifying is a separate draw and team events are not a draw at all -- 341 ATP
-- "tour" events with a stated draw size of 4 are Davis Cup ties, and every one
-- of their matches carries the team flag.
SELECT m.round,
       m.winner_id, wp.slug AS winner_slug, wp.full_name AS winner_name, wmp.seed AS winner_seed,
       m.loser_id,  lp.slug AS loser_slug,  lp.full_name AS loser_name,  lmp.seed AS loser_seed
  FROM matches m
  JOIN players wp ON wp.id = m.winner_id
  JOIN players lp ON lp.id = m.loser_id
  LEFT JOIN match_players wmp ON wmp.match_id = m.id AND wmp.player_id = m.winner_id
  LEFT JOIN match_players lmp ON lmp.match_id = m.id AND lmp.player_id = m.loser_id
 WHERE m.tournament_id = @tournament_id
   AND NOT m.is_qualifying
   AND NOT m.is_team_event
 ORDER BY m.match_num;

-- name: ListSimulatableEvents :many
-- Events whose main draw is a complete power-of-two bracket, which is the only
-- shape that reconstructs. Byes and round-robin groups leave a round with the
-- wrong number of matches and are excluded here rather than failing one at a
-- time in the reconstruction.
SELECT t.id, t.name, t.season, t.tour::text AS tour, t.tier::text AS tier,
       coalesce(t.surface::text, 'unknown')::text AS surface,
       t.start_date, count(*)::bigint AS matches
  FROM tournaments t
  JOIN matches m ON m.tournament_id = t.id
 WHERE NOT m.is_qualifying AND NOT m.is_team_event
   AND (sqlc.narg(tour)::tour IS NULL OR t.tour = sqlc.narg(tour)::tour)
   AND (sqlc.narg(season)::smallint IS NULL OR t.season = sqlc.narg(season)::smallint)
 GROUP BY t.id
HAVING count(*) IN (7, 15, 31, 63, 127)
 ORDER BY t.season DESC, count(*) DESC, t.name
 LIMIT @row_limit;

-- name: GetSimulationPlayer :one
-- Just enough of a player to simulate them: who they are, the level they
-- compete at, and when they last played -- which together choose the serve
-- baseline cell the inversion anchors on.
SELECT p.id, p.slug, p.full_name, p.tour::text AS tour, p.country,
       coalesce(p.best_tier::text, 'tour')::text AS best_tier,
       -- The season decides which decade of the serve baseline anchors the
       -- inversion. Zero is not a season, so it is an unambiguous stand-in for
       -- a player with no matches at all -- who has no rating either, and is
       -- therefore not simulatable for a different reason.
       coalesce((SELECT max(t.season)
                   FROM match_players mp
                   JOIN matches m     ON m.id = mp.match_id
                   JOIN tournaments t ON t.id = m.tournament_id
                  WHERE mp.player_id = p.id), 0)::smallint AS last_season
  FROM players p
 WHERE p.slug = @slug;
