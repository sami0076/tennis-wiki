-- +goose Up
-- The name key now numbers the same name within a season (ADR-0012, amended):
-- the runs it keys are different runs from the ones minted under the old rule,
-- and 278 of the old slugs were minted from a source's own edition number --
-- australia-1-3-atp for the first Australia 1 of the year. The events stage
-- keeps a slug it finds, so the name-keyed events are cleared here and minted
-- again by the stage that follows this migration; the numbered and team
-- events, and every slug still on the calendar, are untouched. Measured on
-- the full load: 505 slugs change, none of an event played since 2012.
UPDATE tournaments
   SET event_id = NULL, event_link = NULL
 WHERE event_id IN (SELECT id FROM events WHERE key LIKE 'name:%');
DELETE FROM events WHERE key LIKE 'name:%';

-- +goose Down
-- Nothing to restore: the events stage rebuilds the table from the rows.
