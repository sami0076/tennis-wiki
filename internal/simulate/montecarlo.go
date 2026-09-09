package simulate

import (
	"math"
	"math/rand/v2"
	"runtime"
	"sync"
)

// DefaultRuns is how many times a draw is played.
//
// The interval on a probability narrows as 1/sqrt(n), so the last digit costs a
// hundred times the runs. Ten thousand puts the half-width on a 30% favourite
// at about 0.9 points, which is the precision the design shows and the point
// past which more runs buy a decimal nobody reads. docs/performance.md has the
// measurement this was chosen from.
const DefaultRuns = 10_000

// Odds is what a draw simulation says about one entrant.
type Odds struct {
	Entrant Entrant
	// Title is the probability of winning the event.
	Title float64
	// TitleInterval is the 95% half-width on Title. It travels with the figure
	// rather than beside it, because a sampled probability without its interval
	// is a point estimate pretending to be exact.
	TitleInterval float64
	// Reached is the probability of reaching each round, indexed as the
	// bracket's Rounds are: Reached[0] is winning the first round. A
	// quarter-final probability is the more interesting number for everyone who
	// is not one of the favourites.
	Reached []float64
}

// DrawResult is a whole simulation.
type DrawResult struct {
	Odds []Odds
	Runs int
	Seed uint64
	// Champion is who actually won the event, so the simulation can be read
	// against what happened rather than only admired.
	Champion int64
}

// WinFunc gives the probability that a beats b. The draw simulator asks for
// each pairing once and remembers the answer: a 128 draw has 8,128 possible
// pairings and ten thousand runs would otherwise solve most of them repeatedly.
type WinFunc func(a, b Entrant) float64

// RunDraw plays a bracket to its end, many times.
//
// Monte Carlo rather than closed form because a bracket does not collapse the
// way a match does: who is in the fourth round depends on who won the third,
// and the paths through a 128 draw are past enumerating. So it is sampled, and
// because it is sampled every figure carries an interval.
//
// Deterministic under a seed. Each worker draws from its own stream, seeded
// from the base seed and its index, and the counts are summed rather than
// appended, so the answer does not depend on how the goroutines were scheduled.
func RunDraw(b Bracket, win WinFunc, runs int, seed uint64) DrawResult {
	if runs < 1 {
		runs = DefaultRuns
	}
	size := b.Size()
	rounds := len(b.Rounds)
	if size < 2 || rounds == 0 {
		return DrawResult{Runs: runs, Seed: seed, Champion: b.Champion}
	}

	odds := newPairOdds(b, win)

	workers := runtime.GOMAXPROCS(0)
	if workers > runs {
		workers = runs
	}
	counts := make([][]int, workers) // [worker][entrant*rounds + round]

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		share := runs / workers
		if w < runs%workers {
			share++
		}
		wg.Add(1)
		go func(w, share int) {
			defer wg.Done()
			// Two words of seed, both derived from the base, so one seed fixes
			// every stream and no two workers share one.
			rng := rand.New(rand.NewPCG(seed, uint64(w)+1))
			tally := make([]int, size*rounds)
			field := make([]int, size)
			for i := 0; i < share; i++ {
				playOnce(b, odds, rng, field, tally, rounds)
			}
			counts[w] = tally
		}(w, share)
	}
	wg.Wait()

	total := make([]int, size*rounds)
	for _, tally := range counts {
		for i, n := range tally {
			total[i] += n
		}
	}

	result := DrawResult{Odds: make([]Odds, size), Runs: runs, Seed: seed, Champion: b.Champion}
	for i := range b.Entrants {
		reached := make([]float64, rounds)
		for r := 0; r < rounds; r++ {
			reached[r] = float64(total[i*rounds+r]) / float64(runs)
		}
		title := reached[rounds-1]
		result.Odds[i] = Odds{
			Entrant:       b.Entrants[i],
			Title:         title,
			TitleInterval: interval(title, runs),
			Reached:       reached,
		}
	}
	return result
}

// playOnce plays the bracket through, recording every round each survivor won.
//
// field holds the indices still alive, halving each round, which is why the
// bracket is a flat slice: the winner of positions 2i and 2i+1 lands at i.
func playOnce(b Bracket, odds pairOdds, rng *rand.Rand, field, tally []int, rounds int) {
	for i := range field {
		field[i] = i
	}
	alive := len(field)
	for r := 0; r < rounds; r++ {
		for i := 0; i < alive; i += 2 {
			a, c := field[i], field[i+1]
			winner := c
			if rng.Float64() < odds.of(a, c) {
				winner = a
			}
			field[i/2] = winner
			tally[winner*rounds+r]++
		}
		alive /= 2
	}
}

// interval is the 95% half-width on a proportion from runs samples.
func interval(p float64, runs int) float64 {
	if runs < 2 {
		return 0
	}
	return 1.96 * math.Sqrt(p*(1-p)/float64(runs))
}

// pairOdds remembers the probability for each pairing, computed once.
type pairOdds struct {
	size int
	p    []float64
}

func newPairOdds(b Bracket, win WinFunc) pairOdds {
	size := b.Size()
	o := pairOdds{size: size, p: make([]float64, size*size)}
	for i := 0; i < size; i++ {
		for j := i + 1; j < size; j++ {
			v := win(b.Entrants[i], b.Entrants[j])
			o.p[i*size+j] = v
			// The mirror is stored rather than recomputed, so a WinFunc that is
			// not perfectly symmetric cannot make the bracket disagree with
			// itself depending on which half a player came from.
			o.p[j*size+i] = 1 - v
		}
	}
	return o
}

func (o pairOdds) of(a, b int) float64 { return o.p[a*o.size+b] }
