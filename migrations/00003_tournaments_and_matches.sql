-- +goose Up
CREATE TABLE tournaments (
    id         bigserial PRIMARY KEY,
    source_id  text      NOT NULL,
    tour       tour      NOT NULL,
    name       text      NOT NULL,
    level      text      NOT NULL,   -- G, M, A, D, F, C, S
    tier       tier      NOT NULL,
    surface    surface,
    draw_size  smallint,
    start_date date      NOT NULL,
    season     smallint  NOT NULL,
    UNIQUE (source_id, season, tour)
);

CREATE INDEX tournaments_season ON tournaments (season DESC);
CREATE INDEX tournaments_tier ON tournaments (tier);

CREATE TABLE matches (
    id            bigint    GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tournament_id bigint    NOT NULL REFERENCES tournaments (id) ON DELETE CASCADE,
    match_num     smallint  NOT NULL,
    round         text      NOT NULL,   -- F, SF, QF, R16, R32, R64, R128, RR, BR, Q1-Q3
    best_of       smallint  NOT NULL,
    surface       surface,
    score         text,
    minutes       smallint,
    winner_id     bigint    NOT NULL REFERENCES players (id),
    played_on     date      NOT NULL,
    incomplete    boolean   NOT NULL DEFAULT false,  -- RET, W/O, DEF
    -- Qualifying sits in the same tournament as the main draw; a Challenger
    -- qualifier is still Challenger standard, so this is a flag not a tier.
    is_qualifying boolean   NOT NULL DEFAULT false,
    is_team_event boolean   NOT NULL DEFAULT false,  -- excluded from Elo by default
    -- False for Futures and ITF in every year, and for Challengers before
    -- roughly 2010. Recorded at ingest, never re-inferred from a zero.
    has_detailed_stats boolean NOT NULL DEFAULT false,
    indoor        boolean,
    source        text      NOT NULL,
    UNIQUE (tournament_id, match_num)
);

CREATE INDEX matches_played_on ON matches (played_on DESC);
CREATE INDEX matches_surface_played_on ON matches (surface, played_on DESC);
CREATE INDEX matches_tournament ON matches (tournament_id);
CREATE INDEX matches_winner ON matches (winner_id);

-- +goose Down
DROP TABLE IF EXISTS matches;
DROP TABLE IF EXISTS tournaments;
