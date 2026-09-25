-- name: FindTournament :one
-- Resolve an event by what a URL can carry, rather than by a numeric id nobody
-- can read or guess.
SELECT t.id, t.name, t.season, t.tour::text AS tour, t.tier::text AS tier,
       -- The source leaves surface blank for some events. "unknown" is the same
       -- stand-in the career and head-to-head splits use, so one absent surface
       -- does not have two spellings across the API.
       coalesce(t.surface::text, 'unknown')::text AS surface,
       t.level, t.draw_size, t.start_date,
       coalesce(e.slug, '')::text AS slug
  FROM tournaments t
  LEFT JOIN events e ON e.id = t.event_id
 WHERE t.tour = @tour::tour AND t.season = @season::smallint AND lower(t.name) = lower(@name::text)
 ORDER BY t.start_date
 LIMIT 1;

-- name: ListDrawMatches :many
-- Every main-draw match of one event, with both players, for reconstructing the
-- bracket.
--
-- Qualifying is a separate draw and team events are not a draw at all -- 341 ATP
-- "tour" events with a stated draw size of 4 are Davis Cup ties, and every one
-- of their matches carries the team flag.
--
-- A bronze match is played off the semi-final losers, so it hangs beside the
-- tree rather than in it: 306 events carry one, and counting it made every one
-- of them a round too deep.
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
   AND m.round <> 'BR'
 ORDER BY m.match_num;

-- name: ListSimulatableEvents :many
-- Events whose main draw is knockout rounds ending in one final. That is a
-- first cut: byes are found by the reconstruction, and a round the source
-- recorded in part fails there, one event at a time. A round-robin group is
-- excluded here, since no reconstruction reads one.
SELECT t.id, t.name, t.season, t.tour::text AS tour, t.tier::text AS tier,
       coalesce(t.surface::text, 'unknown')::text AS surface,
       t.start_date, count(*)::bigint AS matches
  FROM tournaments t
  JOIN matches m ON m.tournament_id = t.id
 WHERE NOT m.is_qualifying AND NOT m.is_team_event AND m.round <> 'BR'
   AND (sqlc.narg(tour)::tour IS NULL OR t.tour = sqlc.narg(tour)::tour)
   AND (sqlc.narg(season)::smallint IS NULL OR t.season = sqlc.narg(season)::smallint)
 GROUP BY t.id
HAVING bool_and(m.round IN ('R128', 'R64', 'R32', 'R16', 'QF', 'SF', 'F'))
   AND count(*) FILTER (WHERE m.round = 'F') = 1
   AND count(DISTINCT m.round) >= 2
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

-- name: SampleRatedMatches :many
-- Recent completed matches with both players' ratings as they stood at the
-- time, for validating what the simulation chain predicts about them.
--
-- As of the match, never after it. Rating a match with a figure that already
-- knows how it went is the one mistake this whole check exists to avoid.
SELECT m.best_of, m.deciding_set, m.tiebreaks_winner, m.tiebreaks_loser,
       t.tier::text AS tier, m.surface::text AS surface, t.season,
       w.elo::float8 AS winner_elo, l.elo::float8 AS loser_elo
  FROM matches m
  JOIN tournaments t ON t.id = m.tournament_id
  JOIN LATERAL (
        SELECT r.elo FROM ratings r
         WHERE r.player_id = m.winner_id AND r.surface = 'overall' AND r.as_of <= m.played_on
         ORDER BY r.as_of DESC LIMIT 1
       ) w ON true
  JOIN LATERAL (
        SELECT r.elo FROM ratings r
         WHERE r.player_id = m.loser_id AND r.surface = 'overall' AND r.as_of <= m.played_on
         ORDER BY r.as_of DESC LIMIT 1
       ) l ON true
 WHERE m.deciding_set IS NOT NULL
   AND NOT m.is_team_event
   AND m.surface IS NOT NULL
   AND t.tour = @tour::tour
 ORDER BY m.played_on DESC, m.id
 LIMIT @row_limit;

-- name: ListReplayableDraws :many
-- The draws a reader can choose between on the simulator, addressed the way
-- the simulator addresses them: an event slug and a season.
--
-- The HAVING clause is the same first cut ListSimulatableEvents uses -- a
-- knockout ending in exactly one final, no round robin, no team tie -- and it
-- is a first cut for the same reason: byes and a partially recorded round are
-- only found by the reconstruction itself. A draw that passes here can still
-- be declined at simulation time, which the page already has an answer for.
--
-- Joined to events rather than left-joined: a tournament with no event row has
-- no slug, and a draw with no slug is one this list could not address.
WITH eligible AS (
    SELECT e.slug, t.id, t.name, t.season, t.tour, t.tier, t.level, t.surface,
           t.draw_size, t.start_date, count(*) AS matches
      FROM tournaments t
      JOIN events e  ON e.id = t.event_id
      JOIN matches m ON m.tournament_id = t.id
     WHERE NOT m.is_qualifying AND NOT m.is_team_event AND m.round <> 'BR'
       AND (sqlc.narg(tour)::tour IS NULL OR t.tour = sqlc.narg(tour)::tour)
       AND (sqlc.narg(season)::smallint IS NULL OR t.season = sqlc.narg(season)::smallint)
     GROUP BY e.slug, t.id
    HAVING bool_and(m.round IN ('R128', 'R64', 'R32', 'R16', 'QF', 'SF', 'F'))
       AND count(*) FILTER (WHERE m.round = 'F') = 1
       AND count(DISTINCT m.round) >= 2
),
-- One row per address. An event that somehow filed two draws under the same
-- slug and season is one entry in a picker, not two identical ones.
addressed AS (
    SELECT DISTINCT ON (slug, season) *
      FROM eligible
     ORDER BY slug, season, matches DESC, id
)
SELECT slug, name, season, tour::text AS tour, tier::text AS tier, level,
       coalesce(surface::text, 'unknown')::text AS surface,
       draw_size, start_date, matches::bigint AS matches
  FROM addressed
 -- Newest first, and the draws worth replaying at the top of each season: a
 -- Slam before a Masters before everything else.
 ORDER BY season DESC,
          CASE level
              WHEN 'G' THEN 0
              WHEN 'F' THEN 1
              WHEN 'M' THEN 2
              WHEN 'PM' THEN 2
              WHEN '1000' THEN 2
              ELSE 3
          END,
          matches DESC, name
 LIMIT @row_limit;
