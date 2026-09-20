-- +goose Up
-- Sets and games won live in the score string, the way tiebreaks and the
-- deciding set do (00013), and are derived the same way: once, at ingest, by
-- the Go parser, never per request. NULL on the same terms as the clutch
-- columns: an unreadable score or a match that did not finish.
ALTER TABLE matches
    ADD COLUMN sets_winner  smallint,
    ADD COLUMN sets_loser   smallint,
    ADD COLUMN games_winner smallint,
    ADD COLUMN games_loser  smallint;

COMMENT ON COLUMN matches.sets_winner IS
    'Sets won by the match winner, a match tiebreak played in place of a final set counted as one. NULL where the score could not be read or the match did not finish.';
COMMENT ON COLUMN matches.games_winner IS
    'Games won by the match winner; a match tiebreak is not games. NULL on the same terms as sets_winner.';

-- The backfill's pending index now keys on the newest derived column, so a
-- database that already carries the clutch columns derives the new ones on
-- its next refresh without a forced re-parse of every row.
DROP INDEX IF EXISTS matches_clutch_pending;
CREATE INDEX matches_derived_pending ON matches (id)
    WHERE sets_winner IS NULL AND NOT incomplete AND score IS NOT NULL;

-- A leaderboard is an aggregate over a population, and the population is the
-- whole database: summing 3.3 million appearances per request took eleven
-- seconds on the full load. This is that sum, cut by everything a leaderboard
-- or a year-by-year row can be filtered on, rebuilt by the ingest refresh step
-- in the same family as clutch_baselines and serve_baselines.
--
-- Totals, never rates, so every rate is computed from its own denominators at
-- read time and a row can say how many matches it stands on. Each figure has
-- its own count of matches that carried it: serve lines are on 17% of rows,
-- a readable score on nearly all, and a rate over one must not borrow the
-- other's denominator.
CREATE TABLE player_totals (
    player_id bigint   NOT NULL REFERENCES players (id) ON DELETE CASCADE,
    tour      tour     NOT NULL,
    season    smallint NOT NULL,
    tier      tier     NOT NULL,
    -- NULL is a surface the file did not record, kept as its own row rather
    -- than folded into any of the four.
    surface   surface,

    -- Finished, non-team matches: the unit every other count is a subset of.
    matches bigint NOT NULL,
    wins    bigint NOT NULL,
    -- Titles are finals won in the main draw.
    titles  bigint NOT NULL,
    -- Matches the source recorded serve statistics for, a fact of the match
    -- rather than of either side, so a population can be counted in matches.
    with_stats bigint NOT NULL,

    -- The player's own serve lines, and how many matches carried one.
    with_serve    bigint NOT NULL,
    aces          bigint NOT NULL,
    double_faults bigint NOT NULL,
    serve_points  bigint NOT NULL,
    first_in      bigint NOT NULL,
    first_won     bigint NOT NULL,
    second_won    bigint NOT NULL,
    serve_games   bigint NOT NULL,
    bp_saved      bigint NOT NULL,
    bp_faced      bigint NOT NULL,

    -- The opponents' serve lines, which is what a return figure is made of.
    with_return     bigint NOT NULL,
    op_serve_points bigint NOT NULL,
    op_first_in     bigint NOT NULL,
    op_first_won    bigint NOT NULL,
    op_second_won   bigint NOT NULL,
    op_serve_games  bigint NOT NULL,
    op_bp_saved     bigint NOT NULL,
    op_bp_faced     bigint NOT NULL,

    -- From the score: matches whose score the parser read, and what it said.
    scored           bigint NOT NULL,
    sets_won         bigint NOT NULL,
    sets_played      bigint NOT NULL,
    games_won        bigint NOT NULL,
    games_played     bigint NOT NULL,
    tiebreaks_won    bigint NOT NULL,
    tiebreaks_played bigint NOT NULL,
    deciders_won     bigint NOT NULL,
    deciders_played  bigint NOT NULL
);

COMMENT ON TABLE player_totals IS
    'Per player, season, tier and surface: every total a leaderboard or a year-by-year row is a rate over. Rebuilt by the ingest refresh step; stale between runs.';

-- The player page reads one player's rows; a leaderboard reads a season's
-- or a tier's, or all of them.
CREATE INDEX player_totals_player ON player_totals (player_id, season);
CREATE INDEX player_totals_season ON player_totals (season, tier);

-- +goose Down
DROP TABLE IF EXISTS player_totals;
DROP INDEX IF EXISTS matches_derived_pending;
CREATE INDEX matches_clutch_pending ON matches (id)
    WHERE deciding_set IS NULL AND NOT incomplete AND score IS NOT NULL;
ALTER TABLE matches
    DROP COLUMN IF EXISTS games_loser,
    DROP COLUMN IF EXISTS games_winner,
    DROP COLUMN IF EXISTS sets_loser,
    DROP COLUMN IF EXISTS sets_winner;
