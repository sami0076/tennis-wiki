-- +goose Up
-- pg_trgm backs fuzzy player search; it must exist before players is created.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TYPE tour AS ENUM ('atp', 'wta');
CREATE TYPE surface AS ENUM ('hard', 'clay', 'grass', 'carpet');
CREATE TYPE hand AS ENUM ('R', 'L', 'U');

-- Competitive standard, distinct from tourney_level prestige. See ADR-0004.
CREATE TYPE tier AS ENUM ('tour', 'challenger', 'futures', 'itf');

-- Ratings keep an overall series alongside the per-surface ones, and a
-- nullable column cannot sit in a primary key.
CREATE TYPE rating_surface AS ENUM ('overall', 'hard', 'clay', 'grass', 'carpet');

-- +goose Down
DROP TYPE IF EXISTS rating_surface;
DROP TYPE IF EXISTS tier;
DROP TYPE IF EXISTS hand;
DROP TYPE IF EXISTS surface;
DROP TYPE IF EXISTS tour;
DROP EXTENSION IF EXISTS pg_trgm;
