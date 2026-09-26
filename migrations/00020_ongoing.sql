-- +goose Up
-- This week's results so far, from the source's ongoing files. Provisional and
-- kept apart from matches on purpose: ingest only ever upserts, so a row the
-- source later corrects or drops would outlive it there, and nothing that
-- rates, ranks or aggregates should see a draw that is still being played.
-- Each fetch replaces a file's rows wholesale; the weekly load brings the
-- finished event into matches from the season file.
CREATE TABLE ongoing_matches (
    file              text     NOT NULL,
    tour              tour     NOT NULL,
    tourney_source_id text     NOT NULL,
    tourney_name      text     NOT NULL,
    level             text     NOT NULL,
    surface           surface,
    indoor            boolean,
    match_num         integer  NOT NULL,
    round             text     NOT NULL,
    -- The ongoing files date each match by the day it was played, where the
    -- season files use the week the event started.
    played_on         date     NOT NULL,
    score             text,
    winner_source_id  text     NOT NULL,
    winner_name       text     NOT NULL,
    winner_seed       smallint,
    loser_source_id   text     NOT NULL,
    loser_name        text     NOT NULL,
    loser_seed        smallint,
    PRIMARY KEY (file, tourney_source_id, match_num)
);

-- When each file was last asked for and last changed, so the page can say how
-- fresh "so far" is rather than implying it is live.
CREATE TABLE ongoing_files (
    file       text        PRIMARY KEY,
    etag       text,
    checked_at timestamptz NOT NULL,
    changed_at timestamptz NOT NULL,
    rows       integer     NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS ongoing_files;
DROP TABLE IF EXISTS ongoing_matches;
