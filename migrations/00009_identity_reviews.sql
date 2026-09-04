-- +goose Up
-- Ambiguous identity matches wait here for a human. A wrong automatic merge is
-- far more damaging than an unmerged pair, because it is invisible: two careers
-- become one and every total is quietly wrong.
CREATE TABLE identity_reviews (
    source     text          NOT NULL,
    source_id  text          NOT NULL,
    tour       tour          NOT NULL,
    candidate  bigint        NOT NULL REFERENCES players (id) ON DELETE CASCADE,
    confidence numeric(3, 2) NOT NULL,
    reason     text          NOT NULL,
    seen_at    timestamptz   NOT NULL DEFAULT now(),
    PRIMARY KEY (source, source_id, candidate)
);

CREATE INDEX identity_reviews_candidate ON identity_reviews (candidate);

-- +goose Down
DROP TABLE IF EXISTS identity_reviews;
