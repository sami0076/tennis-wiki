package simulate

import (
	"math"
	"testing"
)

func evenBracket(size int) Bracket {
	entrants := make([]Entrant, size)
	for i := range entrants {
		entrants[i] = Entrant{PlayerID: int64(i + 1), Name: "P"}
	}
	rounds := []string{}
	for n := size; n > 1; n /= 2 {
		rounds = append(rounds, "R")
	}
	return Bracket{Entrants: entrants, Rounds: rounds, Champion: 1}
}

// A coin-flip field: every entrant must have the same title odds, and they must
// sum to one.
func TestEvenFieldIsEven(t *testing.T) {
	t.Parallel()

	b := evenBracket(8)
	coin := func(Entrant, Entrant) float64 { return 0.5 }
	res := RunDraw(b, coin, 20000, 1)

	var sum float64
	for _, o := range res.Odds {
		sum += o.Title
		if math.Abs(o.Title-0.125) > o.TitleInterval*2 {
			t.Errorf("%d has title odds %.4f +-%.4f, want 0.125 in an even field",
				o.Entrant.PlayerID, o.Title, o.TitleInterval)
		}
	}
	if math.Abs(sum-1) > 1e-9 {
		t.Errorf("title odds sum to %.6f, want 1: somebody wins every run", sum)
	}
}

// Every round's probabilities have to sum to the number of survivors, or the
// bracket is losing or inventing players.
func TestEveryRoundKeepsItsSurvivors(t *testing.T) {
	t.Parallel()

	b := evenBracket(16)
	seeded := func(a, c Entrant) float64 {
		return Expected(a.PlayerID, c.PlayerID)
	}
	res := RunDraw(b, seeded, 5000, 7)

	survivors := b.Size() / 2
	for r := range b.Rounds {
		var sum float64
		for _, o := range res.Odds {
			sum += o.Reached[r]
		}
		if math.Abs(sum-float64(survivors)) > 1e-9 {
			t.Errorf("round %d: probabilities sum to %.4f, want %d survivors", r, sum, survivors)
		}
		survivors /= 2
	}
}

// Expected is a stand-in win function: the lower id is the better player, by a
// fixed margin, so the favourite is unambiguous.
func Expected(a, b int64) float64 {
	if a < b {
		return 0.7
	}
	return 0.3
}

// The same seed gives the same answer, and a different one does not. Without
// the first a reported figure cannot be reproduced; without the second the
// simulation is not sampling anything.
func TestSeedDeterminesTheAnswer(t *testing.T) {
	t.Parallel()

	b := evenBracket(8)
	win := func(a, c Entrant) float64 { return Expected(a.PlayerID, c.PlayerID) }

	first := RunDraw(b, win, 3000, 42)
	second := RunDraw(b, win, 3000, 42)
	for i := range first.Odds {
		if first.Odds[i].Title != second.Odds[i].Title {
			t.Fatalf("entrant %d: %.6f then %.6f under the same seed",
				first.Odds[i].Entrant.PlayerID, first.Odds[i].Title, second.Odds[i].Title)
		}
	}

	other := RunDraw(b, win, 3000, 43)
	same := true
	for i := range first.Odds {
		if first.Odds[i].Title != other.Odds[i].Title {
			same = false
			break
		}
	}
	if same {
		t.Error("two different seeds produced identical odds; nothing is being sampled")
	}
}

// The strongest player must come out on top, and the interval must be small
// enough to say so.
func TestTheFavouriteLeads(t *testing.T) {
	t.Parallel()

	b := evenBracket(8)
	win := func(a, c Entrant) float64 { return Expected(a.PlayerID, c.PlayerID) }
	res := RunDraw(b, win, 20000, 3)

	best := res.Odds[0]
	for _, o := range res.Odds {
		if o.Title > best.Title {
			best = o
		}
	}
	if best.Entrant.PlayerID != 1 {
		t.Errorf("entrant %d leads, want the strongest", best.Entrant.PlayerID)
	}
	if best.TitleInterval > 0.02 {
		t.Errorf("interval +-%.4f at 20,000 runs is wider than the design shows", best.TitleInterval)
	}
}

// More runs narrow the interval, at the square-root rate that decides what a
// run count buys.
func TestMoreRunsNarrowTheInterval(t *testing.T) {
	t.Parallel()

	b := evenBracket(8)
	coin := func(Entrant, Entrant) float64 { return 0.5 }

	small := RunDraw(b, coin, 2500, 11).Odds[0].TitleInterval
	large := RunDraw(b, coin, 40000, 11).Odds[0].TitleInterval
	if large >= small {
		t.Fatalf("interval did not narrow: +-%.5f at 2,500 runs, +-%.5f at 40,000", small, large)
	}
	// Sixteen times the runs should be about four times as precise.
	if ratio := small / large; ratio < 3.5 || ratio > 4.5 {
		t.Errorf("interval narrowed by x%.2f over 16x the runs, want about x4", ratio)
	}
}

func TestDegenerateBracket(t *testing.T) {
	t.Parallel()

	res := RunDraw(Bracket{}, func(Entrant, Entrant) float64 { return 0.5 }, 100, 1)
	if len(res.Odds) != 0 {
		t.Errorf("an empty bracket produced %d rows", len(res.Odds))
	}
}

func BenchmarkRunDraw128(b *testing.B) {
	bracket := evenBracket(128)
	win := func(a, c Entrant) float64 { return Expected(a.PlayerID, c.PlayerID) }
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunDraw(bracket, win, DefaultRuns, 1)
	}
}
