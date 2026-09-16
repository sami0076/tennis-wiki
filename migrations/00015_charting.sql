-- +goose Up
-- The Match Charting Project, on the terms of ADR-0011: figures attached to
-- matches the database already holds. It never creates a match or a player,
-- and nothing here is read by the ratings, the simulator or /coverage's dates.
CREATE TABLE charted_matches (
    match_id    bigint PRIMARY KEY REFERENCES matches (id) ON DELETE CASCADE,
    -- The project's own key, e.g. 20240915-M-Davis_Cup-RR-A_B-C_D: provenance
    -- for every figure below, and the way back to the point-by-point file.
    charting_id text   NOT NULL UNIQUE,
    -- The day the match was played. matches.played_on is the event's start.
    played_on   date   NOT NULL,
    charted_by  text,
    source      text   NOT NULL
);

-- One row per player per set, with the match total as set 0. The columns are
-- the project's stats-Overview file; the tour files' serve columns on
-- match_players are a different record of the same match and stay untouched.
CREATE TABLE charted_stats (
    match_id          bigint   NOT NULL REFERENCES charted_matches (match_id) ON DELETE CASCADE,
    player_id         bigint   NOT NULL REFERENCES players (id),
    set_no            smallint NOT NULL,
    serve_points      smallint NOT NULL,
    aces              smallint NOT NULL,
    double_faults     smallint NOT NULL,
    first_in          smallint NOT NULL,
    first_won         smallint NOT NULL,
    second_in         smallint NOT NULL,
    second_won        smallint NOT NULL,
    bp_faced          smallint NOT NULL,
    bp_saved          smallint NOT NULL,
    return_points     smallint NOT NULL,
    return_points_won smallint NOT NULL,
    winners           smallint NOT NULL,
    winners_fh        smallint NOT NULL,
    winners_bh        smallint NOT NULL,
    unforced          smallint NOT NULL,
    unforced_fh       smallint NOT NULL,
    unforced_bh       smallint NOT NULL,
    PRIMARY KEY (match_id, player_id, set_no),
    CONSTRAINT charted_stats_consistent CHECK (
        first_in <= serve_points AND first_won <= first_in
        AND second_won <= second_in AND bp_saved <= bp_faced
        AND return_points_won <= return_points
    )
);

CREATE INDEX charted_stats_player ON charted_stats (player_id);

-- +goose Down
DROP TABLE IF EXISTS charted_stats;
DROP TABLE IF EXISTS charted_matches;
