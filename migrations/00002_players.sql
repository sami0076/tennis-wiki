-- +goose Up
CREATE TABLE players (
    id          bigserial PRIMARY KEY,
    source_id   text     NOT NULL,
    tour        tour     NOT NULL,
    -- Globally unique because it is the URL key. Collisions across tours are
    -- real at this scale and are resolved when the slug is generated.
    slug        text     NOT NULL UNIQUE,
    full_name   text     NOT NULL,
    first_name  text,
    last_name   text,
    country     char(3),
    hand        hand,
    height_cm   smallint,
    birth_date  date,
    pro_since   smallint,
    -- Stable external identifier, present in the source player tables.
    wikidata_id text,
    UNIQUE (source_id, tour)
);

CREATE INDEX players_full_name_trgm ON players USING gin (full_name gin_trgm_ops);
CREATE INDEX players_tour ON players (tour);

-- One canonical player may be known by several source ids; see issue #7.
CREATE TABLE player_aliases (
    source     text   NOT NULL,
    source_id  text   NOT NULL,
    player_id  bigint NOT NULL REFERENCES players (id) ON DELETE CASCADE,
    confidence numeric(3, 2),
    PRIMARY KEY (source, source_id)
);

CREATE INDEX player_aliases_player ON player_aliases (player_id);

-- +goose Down
DROP TABLE IF EXISTS player_aliases;
DROP TABLE IF EXISTS players;
