-- name: GetOngoingFile :one
SELECT file, etag, checked_at, changed_at, rows FROM ongoing_files WHERE file = @file;

-- name: DeleteOngoingMatches :exec
DELETE FROM ongoing_matches WHERE file = @file;

-- name: InsertOngoingMatch :exec
INSERT INTO ongoing_matches (file, tour, tourney_source_id, tourney_name, level, surface, indoor,
                             match_num, round, played_on, score,
                             winner_source_id, winner_name, winner_seed,
                             loser_source_id, loser_name, loser_seed)
VALUES (@file, @tour, @tourney_source_id, @tourney_name, @level, sqlc.narg(surface), sqlc.narg(indoor),
        @match_num, @round, @played_on, sqlc.narg(score),
        @winner_source_id, @winner_name, sqlc.narg(winner_seed),
        @loser_source_id, @loser_name, sqlc.narg(loser_seed));

-- name: SaveOngoingFile :exec
-- changed is false for a 304: the file was asked for and had not moved.
INSERT INTO ongoing_files (file, etag, checked_at, changed_at, rows)
VALUES (@file, sqlc.narg(etag), now(), now(), @rows)
ON CONFLICT (file) DO UPDATE
   SET checked_at = now(),
       etag       = CASE WHEN @changed::boolean THEN EXCLUDED.etag ELSE ongoing_files.etag END,
       changed_at = CASE WHEN @changed::boolean THEN now() ELSE ongoing_files.changed_at END,
       rows       = CASE WHEN @changed::boolean THEN EXCLUDED.rows ELSE ongoing_files.rows END;

-- name: ListOngoingMatches :many
-- Every provisional result, with a player's slug where the database knows the
-- source id: directly for a player first seen in these files, through an alias
-- for one the older files named differently.
SELECT o.tour, o.tourney_source_id, o.tourney_name, o.level, o.surface, o.indoor,
       o.match_num, o.round, o.played_on, o.score,
       o.winner_name, o.winner_seed, coalesce(w.slug, '') AS winner_slug,
       o.loser_name, o.loser_seed, coalesce(l.slug, '') AS loser_slug
  FROM ongoing_matches o
  LEFT JOIN LATERAL (
        SELECT p.slug FROM players p
         WHERE (p.source_id = o.winner_source_id AND p.tour = o.tour)
            OR p.id = (SELECT a.player_id FROM player_aliases a
                        WHERE a.source_id = o.winner_source_id AND a.source LIKE 'tml-%'
                        LIMIT 1)
         LIMIT 1) w ON true
  LEFT JOIN LATERAL (
        SELECT p.slug FROM players p
         WHERE (p.source_id = o.loser_source_id AND p.tour = o.tour)
            OR p.id = (SELECT a.player_id FROM player_aliases a
                        WHERE a.source_id = o.loser_source_id AND a.source LIKE 'tml-%'
                        LIMIT 1)
         LIMIT 1) l ON true
 ORDER BY o.tour, o.tourney_name, o.played_on DESC, o.match_num DESC;

-- name: ListOngoingFiles :many
SELECT file, checked_at, changed_at, rows FROM ongoing_files ORDER BY file;
