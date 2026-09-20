-- name: ListSeasonSummaries :many
-- One row per season and tour: how many events were played and on what, with
-- a team competition's ties counted apart, since a tie is not an event on a
-- calendar. Every season in the table, so the women's tour's years before
-- 1968 are rows with no men's half rather than years the page starts after.
SELECT t.season, t.tour::text AS tour,
       count(*) FILTER (WHERE coalesce(t.event_link, '') <> 'team')::int AS events,
       count(*) FILTER (WHERE t.event_link = 'team')::int AS ties,
       count(*) FILTER (WHERE t.surface = 'hard'   AND coalesce(t.event_link, '') <> 'team')::int AS hard,
       count(*) FILTER (WHERE t.surface = 'clay'   AND coalesce(t.event_link, '') <> 'team')::int AS clay,
       count(*) FILTER (WHERE t.surface = 'grass'  AND coalesce(t.event_link, '') <> 'team')::int AS grass,
       count(*) FILTER (WHERE t.surface = 'carpet' AND coalesce(t.event_link, '') <> 'team')::int AS carpet,
       count(*) FILTER (WHERE t.surface IS NULL    AND coalesce(t.event_link, '') <> 'team')::int AS unknown
  FROM tournaments t
 GROUP BY t.season, t.tour
 ORDER BY t.season DESC, t.tour;

-- name: ListSlamFinals :many
-- Every Grand Slam edition with its final, in calendar order within a
-- season: the four names a season row carries. The finals index makes each
-- one a lookup rather than a pass over the draw.
SELECT t.season, t.tour::text AS tour, e.slug, e.name, t.start_date,
       f.score AS final_score,
       w.slug AS champion_slug, w.full_name AS champion_name,
       l.slug AS finalist_slug, l.full_name AS finalist_name
  FROM tournaments t
  JOIN events e ON e.id = t.event_id
  LEFT JOIN LATERAL (
        SELECT m.score, m.winner_id, m.loser_id
          FROM matches m
         WHERE m.tournament_id = t.id AND m.round = 'F'
           AND NOT m.is_qualifying AND NOT m.is_team_event
         ORDER BY m.match_num DESC
         LIMIT 1) f ON true
  LEFT JOIN players w ON w.id = f.winner_id
  LEFT JOIN players l ON l.id = f.loser_id
 WHERE t.level = 'G'
 ORDER BY t.season DESC, t.tour, t.start_date, t.id;

-- name: ListSeasonEvents :many
-- Every tournament of one season at one tier, either tour or both, in
-- calendar order, with its final and the event it belongs to. The category
-- is the index's, so the year page groups by the same words.
SELECT t.id, t.tour::text AS tour, e.slug AS event_slug, e.name AS event_name,
       t.name, t.level, t.tier::text AS tier, coalesce(t.surface::text, '')::text AS surface,
       t.draw_size, t.start_date, t.event_link,
       CASE
           WHEN t.level = 'D' OR t.event_link = 'team' THEN 'team'
           WHEN t.level = 'G' THEN 'slam'
           WHEN t.level IN ('M', 'PM', '1000') THEN 'masters'
           WHEN t.level = 'F' THEN 'finals'
           WHEN t.level = 'O' THEN 'olympics'
           ELSE t.tier::text
       END AS category,
       f.score AS final_score,
       w.slug AS champion_slug, w.full_name AS champion_name,
       l.slug AS finalist_slug, l.full_name AS finalist_name,
       (SELECT count(*)::int FROM matches m WHERE m.tournament_id = t.id) AS matches
  FROM tournaments t
  LEFT JOIN events e ON e.id = t.event_id
  LEFT JOIN LATERAL (
        SELECT m.score, m.winner_id, m.loser_id
          FROM matches m
         WHERE m.tournament_id = t.id AND m.round = 'F'
           AND NOT m.is_qualifying AND NOT m.is_team_event
         ORDER BY m.match_num DESC
         LIMIT 1) f ON true
  LEFT JOIN players w ON w.id = f.winner_id
  LEFT JOIN players l ON l.id = f.loser_id
 WHERE t.season = @season::smallint
   AND (sqlc.narg(tour)::tour IS NULL OR t.tour = sqlc.narg(tour)::tour)
   AND t.tier = @tier::tier
 ORDER BY t.tour, t.start_date, t.id;
