-- +goose Up
-- match_num is only unique within a draw, not within a tournament. Wimbledon
-- 2019 has a main-draw match 100 (Barty beat Zheng) and a qualifying match 100
-- (Gauff beat Bolsova); the source keeps them in separate files. Keyed on
-- (tournament_id, match_num) alone they collapse into one row that accumulates
-- four participants. 1,369 tournaments in a single season carry both draws.
ALTER TABLE matches DROP CONSTRAINT matches_tournament_id_match_num_key;
ALTER TABLE matches ADD CONSTRAINT matches_draw_key
    UNIQUE (tournament_id, match_num, is_qualifying);

-- +goose Down
ALTER TABLE matches DROP CONSTRAINT matches_draw_key;
ALTER TABLE matches ADD CONSTRAINT matches_tournament_id_match_num_key
    UNIQUE (tournament_id, match_num);
