-- +goose Up
-- An event across seasons, on the terms of ADR-0012: a run of tournaments rows
-- keyed by the tour's number where the id carries a real one, and by name
-- where it does not. Derived by `cmd/ingest -stage events`, never written by
-- hand; the overrides file is the only hand in it.
CREATE TABLE events (
    id           bigserial PRIMARY KEY,
    tour         tour      NOT NULL,
    -- The URL key. Always suffixed with the tour, so no tour is the default
    -- and the bare name stays free for a combined page.
    slug         text      NOT NULL UNIQUE,
    -- The most recent edition's name unless the overrides file pins one.
    name         text      NOT NULL,
    -- What the run is keyed on: number:404, name:tour:wimbledon, team:davis cup.
    key          text      NOT NULL,
    first_season smallint  NOT NULL,
    last_season  smallint  NOT NULL,
    UNIQUE (tour, key)
);

ALTER TABLE tournaments
    -- Null between the match stage, which writes the row, and the events
    -- stage, which keys it. dataqual reports any row still null after a load.
    ADD COLUMN event_id   bigint REFERENCES events (id) ON DELETE SET NULL,
    -- How the row got onto its event: by the tour's number, by an override,
    -- by name alone, by name bridged to a numbered event, or as a team tie.
    -- Provenance the page prints, not a detail it hides.
    ADD COLUMN event_link text
        CHECK (event_link IN ('number', 'override', 'name', 'bridged', 'team'));

CREATE INDEX tournaments_event_season ON tournaments (event_id, season);

-- The season is the year in the id, not the year of the first match: the 1969
-- Perth began on 30 December 1968 and the 1985 Masters was played in January
-- 1986. 371 rows differ. The United Cup, spanning New Year in a single file,
-- had been split into two rows per edition; its matches move to the row the
-- id names and the emptied row goes.
UPDATE matches m
   SET tournament_id = keep.id
  FROM tournaments dup
  JOIN tournaments keep
    ON keep.source_id = dup.source_id AND keep.tour = dup.tour
   AND keep.season = substring(dup.source_id from '^(\d{4})-')::int AND keep.id <> dup.id
 WHERE m.tournament_id = dup.id
   AND NOT EXISTS (SELECT 1 FROM matches k
                    WHERE k.tournament_id = keep.id AND k.match_num = m.match_num
                      AND k.is_qualifying = m.is_qualifying);

DELETE FROM tournaments dup
 WHERE substring(dup.source_id from '^(\d{4})-')::int <> dup.season
   AND EXISTS (SELECT 1 FROM tournaments keep
                WHERE keep.source_id = dup.source_id AND keep.tour = dup.tour
                  AND keep.season = substring(dup.source_id from '^(\d{4})-')::int)
   AND NOT EXISTS (SELECT 1 FROM matches WHERE tournament_id = dup.id);

UPDATE tournaments t
   SET season = substring(source_id from '^(\d{4})-')::int
 WHERE substring(source_id from '^(\d{4})-')::int <> season
   AND NOT EXISTS (SELECT 1 FROM tournaments u
                    WHERE u.source_id = t.source_id AND u.tour = t.tour
                      AND u.season = substring(t.source_id from '^(\d{4})-')::int);

-- +goose Down
DROP INDEX IF EXISTS tournaments_event_season;
ALTER TABLE tournaments DROP COLUMN IF EXISTS event_link, DROP COLUMN IF EXISTS event_id;
DROP TABLE IF EXISTS events;
