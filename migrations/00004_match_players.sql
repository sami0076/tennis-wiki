-- +goose Up
-- Source rows are winner/loser oriented; this is the symmetric form. Deriving
-- "did this player win" from a join is trivial, the reverse is a permanent tax.
CREATE TABLE match_players (
    match_id      bigint   NOT NULL REFERENCES matches (id) ON DELETE CASCADE,
    player_id     bigint   NOT NULL REFERENCES players (id),
    won           boolean  NOT NULL,
    seed          smallint,
    entry         text,    -- WC, Q, LL, PR
    rank          integer,
    rank_points   integer,
    age           numeric(4, 1),
    -- NULL where the source recorded nothing. Never 0: a zeroed ace count for
    -- a 1970s match is wrong but plausible-looking, the worst failure we have.
    aces          smallint,
    double_faults smallint,
    serve_points  smallint,
    first_in      smallint,
    first_won     smallint,
    second_won    smallint,
    serve_games   smallint,
    bp_saved      smallint,
    bp_faced      smallint,
    PRIMARY KEY (match_id, player_id),
    CONSTRAINT match_players_stats_consistent CHECK (
        (serve_points IS NULL AND first_in IS NULL AND first_won IS NULL)
        OR (serve_points IS NOT NULL AND first_in <= serve_points AND first_won <= first_in)
    )
);

CREATE INDEX match_players_player ON match_players (player_id);
CREATE INDEX match_players_player_won ON match_players (player_id, won);
CREATE UNIQUE INDEX match_players_identity ON match_players (match_id, player_id, won);

-- matches.winner_id duplicates match_players.won for query convenience. This
-- guarantees the winner actually played; deferred because the match row is
-- written before its participants in the same transaction.
ALTER TABLE matches
    ADD CONSTRAINT matches_winner_played
    FOREIGN KEY (id, winner_id) REFERENCES match_players (match_id, player_id)
    DEFERRABLE INITIALLY DEFERRED;

-- +goose Down
ALTER TABLE matches DROP CONSTRAINT IF EXISTS matches_winner_played;
DROP TABLE IF EXISTS match_players;
