// Package score parses tennis score strings into structured sets.
//
// This feeds every set- and game-level statistic, so it is a real parser
// rather than a regex. It handles retirements, walkovers, defaults,
// pre-tiebreak-era long sets, and the 1970s nine-point tiebreak.
package score
