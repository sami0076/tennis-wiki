-- name: ListPlayerMatches :many
-- The list a player page is mostly made of. Most recent first, keyed on
-- (played_on, id): played_on is a tournament's start date and so is nowhere
-- near unique, and a cursor whose ordering key is not total repeats or skips
-- rows. Both columns descend together, which is what makes the row-value
-- comparison below valid here.
--
-- The opponent comes from matches rather than a second pass over
-- match_players: winner_id and loser_id are both NOT NULL and are half the
-- match's natural key, so whichever is not this player is the other one.
SELECT m.id,
       m.played_on,
       t.name                     AS tournament,
       t.tier,
       t.level,
       t.season,
       m.surface,
       m.round,
       m.is_qualifying,
       m.incomplete,
       m.score,
       m.minutes,
       mp.won,
       op.slug                    AS opponent_slug,
       op.full_name               AS opponent_name,
       mp.aces,
       mp.double_faults,
       mp.serve_points,
       mp.first_in,
       mp.first_won,
       mp.second_won,
       mp.serve_games,
       mp.bp_saved,
       mp.bp_faced
  FROM match_players mp
  JOIN matches m     ON m.id = mp.match_id
  JOIN tournaments t ON t.id = m.tournament_id
  JOIN players op    ON op.id = CASE WHEN mp.won THEN m.loser_id ELSE m.winner_id END
 WHERE mp.player_id = @player_id
   AND (sqlc.narg(surface)::surface IS NULL OR m.surface = sqlc.narg(surface)::surface)
   AND (sqlc.narg(tier)::tier IS NULL OR t.tier = sqlc.narg(tier)::tier)
   AND (sqlc.narg(season)::smallint IS NULL OR t.season = sqlc.narg(season)::smallint)
   AND (sqlc.narg(opponent)::text IS NULL OR op.slug = sqlc.narg(opponent)::text)
   AND (sqlc.narg(after_date)::date IS NULL
        OR (m.played_on, m.id) < (sqlc.narg(after_date)::date, sqlc.narg(after_id)::bigint))
 ORDER BY m.played_on DESC, m.id DESC
 LIMIT @row_limit;
