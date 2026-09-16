-- name: GetCoverage :many
-- Coverage is queried, never declared. A "data current through" date written
-- down somewhere drifts from the database the first time an ingest changes it;
-- this cannot. See ADR-0006.
SELECT t.tour,
       t.tier,
       count(*)::bigint                                     AS matches,
       min(m.played_on)::date                               AS first_match,
       max(m.played_on)::date                               AS last_match,
       count(*) FILTER (WHERE m.has_detailed_stats)::bigint AS matches_with_stats
  FROM matches m
  JOIN tournaments t ON t.id = m.tournament_id
 GROUP BY t.tour, t.tier
 ORDER BY t.tour, t.tier;

-- name: GetChartedCoverage :many
-- The Match Charting Project's reach, kept apart from the dates above: a
-- charted match is a match the database already had, so it moves nothing in
-- GetCoverage and is its own line here (ADR-0011).
SELECT t.tour,
       count(*)::bigint           AS matches,
       min(c.played_on)::date     AS first_match,
       max(c.played_on)::date     AS last_match,
       count(DISTINCT p.player_id)::bigint AS players
  FROM charted_matches c
  JOIN matches m ON m.id = c.match_id
  JOIN tournaments t ON t.id = m.tournament_id
  LEFT JOIN charted_stats p ON p.match_id = c.match_id AND p.set_no = 0
 GROUP BY t.tour
 ORDER BY t.tour;
