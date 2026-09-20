-- +goose Up
-- The final of a draw is what a season row, an event row and a year's list
-- all read, and each read it through the tournament's whole match list.
-- 64,000 finals among 1.65 million matches: a partial index, so that a page
-- of finals is a page of index hits rather than a scan per row.
CREATE INDEX matches_finals ON matches (tournament_id) WHERE round = 'F' AND NOT is_qualifying;

-- +goose Down
DROP INDEX IF EXISTS matches_finals;
