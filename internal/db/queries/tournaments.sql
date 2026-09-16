-- name: ListEvents :many
-- The tournament index. An event's level and tier are those of its latest
-- edition, since both change over a run (a 250 becomes a 500; the WTA renamed
-- every tier in 2009 and again in 2021), and the category folds the two
-- tours' vocabularies into one short list the index groups by.
--
-- Ordered by category then name, keyed on (rank, name, id) for the cursor.
WITH latest AS (
    SELECT DISTINCT ON (t.event_id) t.event_id, t.level, t.tier
      FROM tournaments t
     WHERE t.event_id IS NOT NULL
     ORDER BY t.event_id, t.season DESC, t.start_date DESC, t.id DESC
),
counted AS (
    SELECT event_id, count(*)::int AS editions FROM tournaments GROUP BY event_id
),
shaped AS (
    SELECT e.id, e.tour, e.slug, e.name, e.first_season, e.last_season,
           c.editions, l.level, l.tier,
           CASE
               WHEN l.level = 'D' THEN 'team'
               WHEN l.level = 'G' THEN 'slam'
               WHEN l.level IN ('M', 'PM', '1000') THEN 'masters'
               WHEN l.level = 'F' THEN 'finals'
               WHEN l.level = 'O' THEN 'olympics'
               ELSE l.tier::text
           END AS category,
           CASE
               WHEN l.level = 'G' THEN 1
               WHEN l.level IN ('M', 'PM', '1000') THEN 2
               WHEN l.level = 'F' THEN 3
               WHEN l.level = 'O' THEN 4
               WHEN l.level = 'D' THEN 5
               WHEN l.tier = 'tour' THEN 6
               WHEN l.tier = 'challenger' THEN 7
               ELSE 8
           END AS rank
      FROM events e
      JOIN latest l  ON l.event_id = e.id
      JOIN counted c ON c.event_id = e.id
)
SELECT id, tour::text AS tour, slug, name, first_season, last_season, editions,
       level, tier::text AS tier, category, rank
  FROM shaped
 WHERE (sqlc.narg(tour)::tour IS NULL OR tour = sqlc.narg(tour)::tour)
   AND (sqlc.narg(category)::text IS NULL OR category = sqlc.narg(category)::text)
   AND (sqlc.narg(q)::text IS NULL OR name ILIKE '%' || sqlc.narg(q)::text || '%')
   AND (sqlc.narg(after_rank)::int IS NULL
        OR (rank, name, id) > (sqlc.narg(after_rank)::int, sqlc.narg(after_name)::text, sqlc.narg(after_id)::bigint))
 ORDER BY rank, name, id
 LIMIT @row_limit;

-- name: GetEventBySlug :one
SELECT id, tour::text AS tour, slug, name, key, first_season, last_season
  FROM events
 WHERE slug = @slug;

-- name: ListEventEditions :many
-- Every row of one event, oldest first, with what the sheet writes at the
-- top: who won the final and against whom. A team competition has a row per
-- tie and no final; the handler folds those into one edition per season.
SELECT t.id, t.season, t.name, t.level, t.tier::text AS tier, coalesce(t.surface::text, '')::text AS surface,
       t.draw_size, t.start_date, t.event_link,
       f.score AS final_score,
       w.slug AS champion_slug, w.full_name AS champion_name,
       l.slug AS finalist_slug, l.full_name AS finalist_name,
       (SELECT count(*)::int FROM matches m WHERE m.tournament_id = t.id) AS matches
  FROM tournaments t
  LEFT JOIN LATERAL (
        SELECT m.score, m.winner_id, m.loser_id
          FROM matches m
         WHERE m.tournament_id = t.id AND m.round = 'F'
           AND NOT m.is_qualifying AND NOT m.is_team_event
         ORDER BY m.match_num DESC
         LIMIT 1) f ON true
  LEFT JOIN players w ON w.id = f.winner_id
  LEFT JOIN players l ON l.id = f.loser_id
 WHERE t.event_id = @event_id::bigint
 ORDER BY t.season, t.start_date, t.id;

-- name: ListEditionRows :many
-- The tournaments rows of one edition: one, or one per tie for a team
-- competition.
SELECT t.id, t.season, t.name, t.level, t.tier::text AS tier, coalesce(t.surface::text, '')::text AS surface,
       t.draw_size, t.start_date, t.event_link
  FROM tournaments t
 WHERE t.event_id = @event_id::bigint AND t.season = @season::smallint
 ORDER BY t.start_date, t.id;

-- name: ListEditionMatches :many
-- Every match of an edition in the order the sheet reads: a tie at a time for
-- a team competition, the main draw before qualifying, and match_num within a
-- draw, which is bracket order in every source (simulate.sql relies on it).
-- Both sides come back on one row so a match is one row, not two.
SELECT m.id, m.tournament_id, t.name AS tie,
       m.round, m.match_num, m.is_qualifying, m.is_team_event, m.best_of,
       m.score, m.incomplete, m.minutes, m.played_on,
       w.slug AS winner_slug, w.full_name AS winner_name, w.country AS winner_country,
       wp.seed AS winner_seed, wp.entry AS winner_entry, wp.rank AS winner_rank,
       l.slug AS loser_slug, l.full_name AS loser_name, l.country AS loser_country,
       lp.seed AS loser_seed, lp.entry AS loser_entry, lp.rank AS loser_rank,
       wp.aces AS w_aces, wp.double_faults AS w_double_faults, wp.serve_points AS w_serve_points,
       wp.first_in AS w_first_in, wp.first_won AS w_first_won, wp.second_won AS w_second_won,
       wp.serve_games AS w_serve_games, wp.bp_saved AS w_bp_saved, wp.bp_faced AS w_bp_faced,
       lp.aces AS l_aces, lp.double_faults AS l_double_faults, lp.serve_points AS l_serve_points,
       lp.first_in AS l_first_in, lp.first_won AS l_first_won, lp.second_won AS l_second_won,
       lp.serve_games AS l_serve_games, lp.bp_saved AS l_bp_saved, lp.bp_faced AS l_bp_faced,
       m.has_detailed_stats,
       cm.charting_id
  FROM matches m
  JOIN tournaments t ON t.id = m.tournament_id
  JOIN players w ON w.id = m.winner_id
  JOIN players l ON l.id = m.loser_id
  LEFT JOIN match_players wp ON wp.match_id = m.id AND wp.player_id = m.winner_id
  LEFT JOIN match_players lp ON lp.match_id = m.id AND lp.player_id = m.loser_id
  LEFT JOIN charted_matches cm ON cm.match_id = m.id
 WHERE m.tournament_id = ANY(@tournament_ids::bigint[])
 ORDER BY t.start_date, t.id, m.is_qualifying, m.match_num;

-- name: FindEdition :one
-- What the draw simulator asks for: an edition by the slug and season a URL
-- carries. A team competition has many rows a season and no draw; the first
-- is returned and the simulator declines it as it declines any tie.
SELECT t.id, t.name, t.season, t.tour::text AS tour, t.tier::text AS tier,
       coalesce(t.surface::text, 'unknown')::text AS surface,
       t.level, t.draw_size, t.start_date
  FROM tournaments t
  JOIN events e ON e.id = t.event_id
 WHERE e.slug = @slug AND t.season = @season::smallint
 ORDER BY t.start_date, t.id
 LIMIT 1;
