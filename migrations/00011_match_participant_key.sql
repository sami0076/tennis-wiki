-- +goose Up
-- match_num is not unique within a draw either. The WTA files carry more than
-- one draw block under a single tourney_id, each numbering its matches from 1:
-- Berkeley Pac Coast 1937 has a 15-match block and a 21-match block, and
-- Wimbledon 1975 qualifying is listed twice with overlapping numbers. Nothing
-- in the source separates them -- name, date, level, surface and draw size are
-- identical across every colliding pair -- so no further column will do it.
--
-- What separates them is who played. Measured over 51,341 source rows from 16
-- files, keying on the tournament, the number and the pair of players leaves
-- three collisions, and all three are byte-identical duplicate rows where
-- collapsing is the right answer. The old key leaves 134, every one of them two
-- different matches sharing a row and accumulating four participants.
--
-- The pair is unordered, so a correction that swaps winner and loser updates
-- the match rather than duplicating it.
ALTER TABLE matches ADD COLUMN loser_id bigint REFERENCES players (id);

UPDATE matches m
   SET loser_id = (SELECT min(mp.player_id) FROM match_players mp
                    WHERE mp.match_id = m.id AND NOT mp.won);

ALTER TABLE matches ALTER COLUMN loser_id SET NOT NULL;

ALTER TABLE matches DROP CONSTRAINT matches_draw_key;
CREATE UNIQUE INDEX matches_draw_key ON matches (
    tournament_id, match_num, is_qualifying,
    least(winner_id, loser_id), greatest(winner_id, loser_id));

-- The mirror of matches_winner_played, and deferred for the same reason: the
-- match row is written before its participants in the same transaction.
ALTER TABLE matches
    ADD CONSTRAINT matches_loser_played
    FOREIGN KEY (id, loser_id) REFERENCES match_players (match_id, player_id)
    DEFERRABLE INITIALLY DEFERRED;

-- +goose Down
ALTER TABLE matches DROP CONSTRAINT IF EXISTS matches_loser_played;
DROP INDEX IF EXISTS matches_draw_key;
ALTER TABLE matches DROP COLUMN IF EXISTS loser_id;
ALTER TABLE matches ADD CONSTRAINT matches_draw_key
    UNIQUE (tournament_id, match_num, is_qualifying);
