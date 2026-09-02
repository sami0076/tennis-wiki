-- +goose Up
CREATE TABLE rankings (
    player_id    bigint  NOT NULL REFERENCES players (id) ON DELETE CASCADE,
    ranking_date date    NOT NULL,
    rank         integer NOT NULL,
    points       integer,
    PRIMARY KEY (player_id, ranking_date)
);

CREATE INDEX rankings_date_rank ON rankings (ranking_date DESC, rank);

CREATE TABLE ratings (
    player_id      bigint       NOT NULL REFERENCES players (id) ON DELETE CASCADE,
    as_of          date         NOT NULL,
    surface        rating_surface NOT NULL,
    elo            numeric(7, 2)  NOT NULL,
    matches_played integer        NOT NULL,
    PRIMARY KEY (player_id, as_of, surface)
);

CREATE INDEX ratings_leaderboard ON ratings (surface, as_of DESC, elo DESC);

-- +goose Down
DROP TABLE IF EXISTS ratings;
DROP TABLE IF EXISTS rankings;
