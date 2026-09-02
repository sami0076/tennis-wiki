-- +goose Up
CREATE TABLE ingest_runs (
    id           bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source       text        NOT NULL,
    started_at   timestamptz NOT NULL DEFAULT now(),
    finished_at  timestamptz,
    rows_seen    integer,
    rows_written integer,
    error        text
);

CREATE INDEX ingest_runs_started ON ingest_runs (started_at DESC);

-- +goose Down
DROP TABLE IF EXISTS ingest_runs;
