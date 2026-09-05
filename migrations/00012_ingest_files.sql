-- +goose Up
-- An ingest re-read every configured file and re-upserted rows it already had,
-- so an interruption cost the whole run: the first full ingest died 900,000
-- rows in, and the retry spent its entire life rewriting those same rows before
-- being killed, making no net progress.
--
-- One row per ingested file, carrying the validator the mirror gave for the
-- content that was read. raw.githubusercontent.com answers If-None-Match with
-- 304 and an empty body, so an unchanged file costs a round trip instead of
-- half a megabyte and a pass over every row in it.
--
-- unit is text because a file is addressed differently per kind: the season for
-- matches, the file's path for reference data, which is not seasonal.
CREATE TABLE ingest_files (
    source       text        NOT NULL,
    unit         text        NOT NULL,
    validator    text        NOT NULL,
    ingested_at  timestamptz NOT NULL DEFAULT now(),
    rows_seen    integer     NOT NULL,
    rows_written integer     NOT NULL,
    PRIMARY KEY (source, unit)
);

-- +goose Down
DROP TABLE IF EXISTS ingest_files;
