-- +goose Up
-- Search ranks by similarity weighted by the best tier a player reached, which
-- needed a lateral aggregate over every candidate's matches. Measured at 160k
-- matches, a query with 86 candidates spent 288ms cold and 93ms warm on that
-- aggregate; reading it from a column costs 20ms and 44ms. The gap grows with
-- the number of candidates, and the full dataset is ten times this size.
--
-- Derived columns rather than a materialised view: a view cannot see uncommitted
-- rows, so no test that rolls back could ever exercise the ranking.
ALTER TABLE players
    ADD COLUMN career_matches integer NOT NULL DEFAULT 0,
    ADD COLUMN best_tier      tier;

COMMENT ON COLUMN players.career_matches IS
    'Derived from match_players by the ingest refresh step; stale between runs.';
COMMENT ON COLUMN players.best_tier IS
    'Best tier reached, derived by the ingest refresh step. NULL until it runs.';

-- +goose Down
ALTER TABLE players DROP COLUMN IF EXISTS best_tier;
ALTER TABLE players DROP COLUMN IF EXISTS career_matches;
