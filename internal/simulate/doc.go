// Package simulate implements match and draw simulation.
//
// A single match is solved in closed form rather than by Monte Carlo: the
// state space is small enough to solve exactly, which is both faster and more
// precise. The amplification the page is built to show is a claim about small
// differences -- 65.0 against 63.0 on serve -- and a sampled answer would carry
// an interval wider than the effect.
//
// The point-win probabilities the chain starts from are derived from ratings
// rather than from per-player serve statistics, which reach only 3% of the
// players on this site. See ADR-0007. Invert is that derivation: it solves for
// the pair of serve probabilities whose match probability equals what the
// rating already predicts.
//
// Draw simulation is Monte Carlo and concurrent, since a bracket does not
// collapse the same way. It arrives with #81.
package simulate
