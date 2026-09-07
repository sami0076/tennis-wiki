// Package rating implements the Elo engine.
//
// Ratings are recomputed from scratch over every match in chronological
// order, never incrementally patched, so a bug fix is always one rerun away
// from correct. Five series are maintained per player -- overall, hard, clay,
// grass and carpet -- mirroring the rating_surface enum.
//
// Chronological is not the same as ordered by date: the source gives nearly
// every tournament a single date for all of its matches, so the round breaks
// the tie and a final is rated after the semi-final that produced its
// finalist.
//
// The pool spans every tier, so K is scaled by a tier weight as well as by
// match importance. See ADR-0003 and ADR-0004.
package rating
