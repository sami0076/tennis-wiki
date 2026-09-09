-- +goose Up
-- The anchor for ADR-0007's inversion: what a service point is worth in a given
-- context, so the solver has a second constraint and does not answer with a
-- pair that reproduces the right match probability out of two implausible
-- halves.
--
-- It cannot be one constant. The ATP tour wins 62.6% of service points and the
-- WTA tour 55.9%, and clay and grass are four points apart inside either. A WTA
-- clay match anchored on an ATP grass average still comes out with the right
-- match probability -- the inversion would compensate -- but the decomposition
-- underneath it would be describing a different sport.
--
-- Totals rather than rates, for the same reason clutch_baselines stores totals:
-- a lookup that widens to a coarser population has to re-pool the counts, and
-- storing rates throws away what it would need to do that.
CREATE TABLE serve_baselines (
    tour    tour     NOT NULL,
    tier    tier     NOT NULL,
    surface surface  NOT NULL,
    decade  smallint NOT NULL,

    -- Player-sides that carried a serve line, which is the denominator of the
    -- match count rather than of the rate.
    appearances  bigint NOT NULL,
    serve_points bigint NOT NULL,
    serve_won    bigint NOT NULL,

    refreshed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tour, tier, surface, decade)
);

COMMENT ON TABLE serve_baselines IS
    'Serve-point totals by tour, tier, surface and decade. Rebuilt by the ingest refresh step; stale between runs. Matches with no recorded surface are excluded rather than bucketed as a fifth one.';

-- +goose Down
DROP TABLE IF EXISTS serve_baselines;
