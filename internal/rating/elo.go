package rating

import "math"

// Base is where a player who has never been rated starts.
const Base = 1500.0

// Expected is the probability that a player rated a beats one rated b.
func Expected(a, b float64) float64 {
	return 1 / (1 + math.Pow(10, (b-a)/400))
}

// Update returns the two new ratings after a decided match.
//
// The players move by different amounts, because K depends on how many matches
// each of them has played: a debutant beating a veteran gains far more than the
// veteran loses. The pool's total rating is therefore not conserved. That is a
// consequence of the decaying K in spec section 7.2 rather than an oversight --
// conserving it would mean pinning a newcomer's rating to the experience of
// whoever they happened to draw.
func Update(winner, loser, kWinner, kLoser float64) (float64, float64) {
	surprise := 1 - Expected(winner, loser)
	return winner + kWinner*surprise, loser - kLoser*surprise
}
