-- +goose Up
-- Tiebreaks won and deciding sets won live in the score string and nowhere
-- else. Parsing 1.6 million score strings per request is not a query, so they
-- are derived once, in the same family as the prominence columns in 00010.
ALTER TABLE matches
    ADD COLUMN tiebreaks_winner smallint,
    ADD COLUMN tiebreaks_loser  smallint,
    ADD COLUMN deciding_set     boolean;

COMMENT ON COLUMN matches.tiebreaks_winner IS
    'Set tiebreaks won by the match winner, excluding a match tiebreak played in place of a final set. NULL where the score could not be read or the match did not finish: no tiebreaks and no readable score are different facts.';
COMMENT ON COLUMN matches.tiebreaks_loser IS
    'Set tiebreaks won by the match loser. NULL on the same terms as tiebreaks_winner.';
COMMENT ON COLUMN matches.deciding_set IS
    'True when a finished match reached its deciding set. NULL when it did not finish: somebody advanced from a third-set retirement, but nobody won that set, and every rate here leaves incomplete matches out.';

-- The backfill reads only what it has not derived yet, and this is what keeps
-- that from being a scan of every match on every ingest. Rows that stay NULL
-- because their score cannot be parsed are the only ones left behind.
CREATE INDEX matches_clutch_pending ON matches (id)
    WHERE deciding_set IS NULL AND NOT incomplete AND score IS NOT NULL;

-- A baseline over 1.6 million matches is not a per-request query either.
--
-- One row per tour, tier and decade -- about a hundred in all -- because a
-- Futures player measured against a tour baseline is being told something true
-- and useless, and serve statistics from 1995 describe a different game from
-- 2025's. A player who spans both is compared against their own spread of
-- levels and years, which is why this stores totals rather than rates: a
-- weighted average of rates needs the denominators, and storing rates throws
-- them away.
CREATE TABLE clutch_baselines (
    tour   tour     NOT NULL,
    tier   tier     NOT NULL,
    decade smallint NOT NULL,

    -- Player-sides, two per match: the same unit every figure below is counted
    -- in, and the same unit a player's own figures are counted in. Counting a
    -- match once would make the baseline and the player disagree on what a
    -- denominator is.
    appearances          bigint NOT NULL,
    bp_saved             bigint NOT NULL,
    bp_faced             bigint NOT NULL,
    -- Won is exactly half of played for both of these, because every tiebreak
    -- in the cell is counted from both sides of the same match. It is stored
    -- rather than assumed so the page can state a measured number, and so
    -- anything that breaks the symmetry shows up instead of hiding.
    tiebreaks_won        bigint NOT NULL,
    tiebreaks_played     bigint NOT NULL,
    deciding_sets_won    bigint NOT NULL,
    deciding_sets_played bigint NOT NULL,

    refreshed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tour, tier, decade)
);

COMMENT ON TABLE clutch_baselines IS
    'What "vs tour average" is measured against. Rebuilt by the ingest refresh step; stale between runs.';

-- +goose Down
DROP TABLE IF EXISTS clutch_baselines;
DROP INDEX IF EXISTS matches_clutch_pending;
ALTER TABLE matches
    DROP COLUMN IF EXISTS deciding_set,
    DROP COLUMN IF EXISTS tiebreaks_loser,
    DROP COLUMN IF EXISTS tiebreaks_winner;
