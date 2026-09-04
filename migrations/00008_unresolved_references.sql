-- +goose Up
-- Ranking history references players by source id. A reference we cannot
-- resolve means the player table is missing someone, which is a fact about the
-- sources worth reporting rather than a row to drop silently.
CREATE TABLE unresolved_references (
    source      text        NOT NULL,
    kind        text        NOT NULL,
    source_id   text        NOT NULL,
    occurrences bigint      NOT NULL DEFAULT 1,
    last_seen   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (source, kind, source_id)
);

-- +goose Down
DROP TABLE IF EXISTS unresolved_references;
