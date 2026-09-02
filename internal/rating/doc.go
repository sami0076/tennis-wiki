// Package rating implements the Elo engine.
//
// Ratings are recomputed from scratch over every match in chronological
// order, never incrementally patched, so a bug fix is always one rerun away
// from correct. Four series are maintained per player: overall, hard, clay,
// and grass.
//
// The pool spans every tier, so K is scaled by a tier weight as well as by
// match importance. See ADR-0003.
package rating
