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
