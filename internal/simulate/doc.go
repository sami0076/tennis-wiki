// Package simulate implements match and draw simulation.
//
// A single match is solved in closed form rather than by Monte Carlo: the
// state space is small enough to solve exactly, which is both faster and
// more precise. Draw simulation is Monte Carlo and concurrent, since a
// bracket does not collapse the same way.
package simulate
