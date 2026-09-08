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

-- name: ListHeadToHeadMeetings :many
-- Every match the two have played, oldest first, with both serve lines.
--
-- One query rather than an aggregate per split: even the longest rivalry in the
-- database is a few dozen rows, and counting them in Go keeps the record, the
-- surface split and the tier split from being three chances to disagree.
--
-- Ordered oldest to newest because that is how a rivalry reads.
SELECT m.id,
       m.played_on,
       t.name  AS tournament,
       t.tier,
       t.level,
       t.season,
       m.surface,
       m.round,
       m.is_qualifying,
       m.incomplete,
       m.score,
       m.winner_id,
       a.aces AS a_aces, a.double_faults AS a_double_faults, a.serve_points AS a_serve_points,
       a.first_in AS a_first_in, a.first_won AS a_first_won, a.second_won AS a_second_won,
       a.bp_saved AS a_bp_saved, a.bp_faced AS a_bp_faced,
       b.aces AS b_aces, b.double_faults AS b_double_faults, b.serve_points AS b_serve_points,
       b.first_in AS b_first_in, b.first_won AS b_first_won, b.second_won AS b_second_won,
       b.bp_saved AS b_bp_saved, b.bp_faced AS b_bp_faced
  FROM matches m
  JOIN tournaments t   ON t.id = m.tournament_id
  JOIN match_players a ON a.match_id = m.id AND a.player_id = @player_a
  JOIN match_players b ON b.match_id = m.id AND b.player_id = @player_b
 WHERE (m.winner_id = @player_a AND m.loser_id = @player_b)
    OR (m.winner_id = @player_b AND m.loser_id = @player_a)
 ORDER BY m.played_on, m.id;
