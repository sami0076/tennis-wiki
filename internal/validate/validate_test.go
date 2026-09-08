package validate

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/sami0076/tennis-wiki/internal/rating"
)

func prediction(tier rating.Tier, winner, loser int64, expected float64, played int) rating.Prediction {
	return rating.Prediction{
		Tier:           tier,
		PlayedOn:       time.Date(2020, 1, 6, 0, 0, 0, 0, time.UTC),
		WinnerID:       winner,
		LoserID:        loser,
		WinnerExpected: expected,
		WinnerPlayed:   played,
		LoserPlayed:    played,
	}
}

func newTestAccumulator(cfg Config) *accumulator {
	return newAccumulator(cfg.withDefaults())
}

// close compares percentages, which are sums of binary fractions and land a
// few parts in 10^14 away from the round number they are.
func close(got, want float64) bool { return math.Abs(got-want) < 1e-9 }

// A prediction is only worth scoring once the engine has seen enough of both
// players to have had an opinion.
func TestPredictionsWithoutHistoryAreNotScored(t *testing.T) {
	a := newTestAccumulator(Config{MinMatches: 10})

	a.observe(prediction(rating.TierTour, 1, 2, 0.8, 3))
	if a.scored != 0 {
		t.Errorf("scored %d predictions over debutants, want none", a.scored)
	}

	a.observe(prediction(rating.TierTour, 1, 2, 0.8, 10))
	if a.scored != 1 {
		t.Errorf("scored %d, want the one with history behind it", a.scored)
	}
}

func TestAccuracyIsPerTier(t *testing.T) {
	a := newTestAccumulator(Config{})

	// Tour: three favourites win, one loses.
	for i := 0; i < 3; i++ {
		a.observe(prediction(rating.TierTour, 1, 2, 0.7, 20))
	}
	a.observe(prediction(rating.TierTour, 2, 1, 0.3, 20))
	// Futures: the favourite loses both.
	a.observe(prediction(rating.TierFutures, 3, 4, 0.2, 20))
	a.observe(prediction(rating.TierFutures, 3, 4, 0.2, 20))

	got := a.accuracy()
	if len(got) != 2 {
		t.Fatalf("got %d tiers, want tour and futures separately", len(got))
	}
	if got[0].Tier != "tour" || got[0].Accuracy != 75 {
		t.Errorf("tour = %+v, want 75%%", got[0])
	}
	if got[1].Tier != "futures" || got[1].Accuracy != 0 {
		t.Errorf("futures = %+v, want 0%%", got[1])
	}
}

// Breaking a tie in the winner's favour would score every even match as a
// correct call and quietly inflate the headline number.
func TestAnEvenMatchIsNotScoredAsACorrectCall(t *testing.T) {
	a := newTestAccumulator(Config{})
	// Player 2 wins, but player 1 has the lower id and so is the favourite.
	a.observe(prediction(rating.TierTour, 2, 1, 0.5, 20))

	got := a.accuracy()
	if len(got) != 1 || got[0].Correct != 0 {
		t.Errorf("got %+v, want the tie broken against the result", got)
	}
}

func TestCalibrationBucketsByPredictedProbability(t *testing.T) {
	a := newTestAccumulator(Config{})
	// Ten matches the engine called at 70%, seven of which the favourite won.
	for i := 0; i < 7; i++ {
		a.observe(prediction(rating.TierTour, 1, 2, 0.7, 20))
	}
	for i := 0; i < 3; i++ {
		a.observe(prediction(rating.TierTour, 2, 1, 0.3, 20))
	}

	tables := a.calibration()
	if len(tables) != 1 {
		t.Fatalf("got %d tables, want one for tour", len(tables))
	}
	var band *Bucket
	for i := range tables[0].Buckets {
		if tables[0].Buckets[i].From == 0.7 {
			band = &tables[0].Buckets[i]
		}
	}
	if band == nil {
		t.Fatalf("no 70%% band in %+v", tables[0].Buckets)
	}
	if band.Matches != 10 || !close(band.Predicted, 70) || !close(band.Observed, 70) {
		t.Errorf("band = %+v, want ten matches predicted and observed at 70", *band)
	}
	if !close(tables[0].MeanError, 0) {
		t.Errorf("mean error = %v, want a perfectly calibrated set to read zero", tables[0].MeanError)
	}
}

// A probability of exactly 1 belongs in the last band, not off the end of it.
func TestCalibrationKeepsACertaintyInTheTable(t *testing.T) {
	a := newTestAccumulator(Config{})
	a.observe(prediction(rating.TierTour, 1, 2, 1, 20))

	tables := a.calibration()
	total := 0
	for _, b := range tables[0].Buckets {
		total += b.Matches
	}
	if total != 1 {
		t.Errorf("the table holds %d matches, want the one that was played", total)
	}
}

func TestPromotionNeedsALowerTierCareerFirst(t *testing.T) {
	a := newTestAccumulator(Config{PromotedAfter: 5, PromotionWindow: 2})

	// A player who starts at tour level has not been promoted to it.
	for i := 0; i < 4; i++ {
		a.observe(prediction(rating.TierTour, 1, 2, 0.5, 20))
	}
	if a.promoted != 0 {
		t.Errorf("%d promotions, want none for players who began at tour level", a.promoted)
	}

	// Player 7 plays a Challenger career, then arrives.
	for i := 0; i < 5; i++ {
		a.observe(prediction(rating.TierChallenger, 7, 8, 0.5, 20))
	}
	a.observe(prediction(rating.TierTour, 7, 9, 0.4, 20))
	if a.promoted != 1 {
		t.Fatalf("%d promotions, want the one player who came up", a.promoted)
	}
	if a.promMatch != 1 {
		t.Errorf("measured %d matches, want the first after promotion", a.promMatch)
	}
}

// The window is what stops a promoted player's whole later career counting as
// evidence about the boundary they crossed years earlier.
func TestPromotionStopsAfterTheWindow(t *testing.T) {
	a := newTestAccumulator(Config{PromotedAfter: 2, PromotionWindow: 3})
	for i := 0; i < 2; i++ {
		a.observe(prediction(rating.TierFutures, 7, 8, 0.5, 20))
	}
	for i := 0; i < 10; i++ {
		a.observe(prediction(rating.TierTour, 7, 9, 0.5, 20))
	}
	if a.promMatch != 3 {
		t.Errorf("measured %d matches, want the window of 3", a.promMatch)
	}
}

// The sign is the whole message: a surplus means promoted players arrive
// underrated, which is what weighting the lower tiers too low looks like.
func TestPromotionSurplusReadsAsUnderrated(t *testing.T) {
	a := newTestAccumulator(Config{PromotedAfter: 1, PromotionWindow: 40, SystematicZ: 2})
	a.observe(prediction(rating.TierChallenger, 7, 8, 0.5, 20))
	// Forty tour matches the engine gave them a 30% chance of, all won.
	for i := 0; i < 40; i++ {
		a.observe(prediction(rating.TierTour, 7, int64(100+i), 0.3, 20))
	}

	p := a.promotion()
	if p.Actual != 40 {
		t.Fatalf("actual wins = %d, want 40", p.Actual)
	}
	if p.Z <= 0 || !p.Systematic {
		t.Errorf("z = %.2f systematic = %v, want a systematic surplus", p.Z, p.Systematic)
	}

	finding := findingsFor(t, a, p)
	if !strings.Contains(finding, "underrated") {
		t.Errorf("finding = %q, want it to name the direction", finding)
	}
}

func TestPromotionDeficitReadsAsOverrated(t *testing.T) {
	a := newTestAccumulator(Config{PromotedAfter: 1, PromotionWindow: 40, SystematicZ: 2})
	a.observe(prediction(rating.TierChallenger, 7, 8, 0.5, 20))
	for i := 0; i < 40; i++ {
		// The promoted player loses every one of them, at 70% each time.
		a.observe(prediction(rating.TierTour, int64(100+i), 7, 0.3, 20))
	}

	p := a.promotion()
	if p.Z >= 0 || !p.Systematic {
		t.Errorf("z = %.2f, want a systematic deficit", p.Z)
	}
	if finding := findingsFor(t, a, p); !strings.Contains(finding, "overrated") {
		t.Errorf("finding = %q, want it to name the direction", finding)
	}
}

func findingsFor(t *testing.T, a *accumulator, p PromotionContinuity) string {
	t.Helper()
	rep := Report{Scored: a.scored, Promotion: p, MinMatches: a.cfg.MinMatches}
	var b strings.Builder
	for _, f := range a.findings(rep) {
		b.WriteString(f.Name + ": " + f.Detail + "\n")
	}
	return b.String()
}

// A run that measured nothing is the one thing that should exit non-zero.
func TestAnUnmeasurableRunFails(t *testing.T) {
	a := newTestAccumulator(Config{})
	rep := Report{Scored: 0, MinMatches: 10}
	rep.Findings = a.findings(rep)

	if !rep.Failed() {
		t.Error("a run that scored nothing did not fail")
	}
}

// Everything else is a judgement about weights, and a judgement should not gate
// anything.
func TestAWrongLookingWeightingDoesNotFail(t *testing.T) {
	a := newTestAccumulator(Config{PromotedAfter: 1, PromotionWindow: 40, SystematicZ: 2})
	a.observe(prediction(rating.TierChallenger, 7, 8, 0.5, 20))
	for i := 0; i < 40; i++ {
		a.observe(prediction(rating.TierTour, 7, int64(100+i), 0.3, 20))
	}

	rep := Report{
		Scored:    a.scored,
		Accuracy:  a.accuracy(),
		Promotion: a.promotion(),
	}
	rep.Findings = a.findings(rep)

	if rep.Failed() {
		t.Error("a systematic promotion gap failed the run; it is a result, not an error")
	}
}

func TestReportRendersBothWays(t *testing.T) {
	a := newTestAccumulator(Config{})
	for i := 0; i < 20; i++ {
		a.observe(prediction(rating.TierTour, 1, 2, 0.7, 20))
	}
	rep := Report{
		Matches:     20,
		Scored:      a.scored,
		Accuracy:    a.accuracy(),
		Calibration: a.calibration(),
		Reversion:   MeanReversion{Base: rating.Base, Pool: 1684, Tour: 1990},
		Promotion:   a.promotion(),
		Weights:     rating.DefaultWeights(),
	}
	rep.Findings = a.findings(rep)

	var text strings.Builder
	if err := rep.WriteText(&text); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	for _, want := range []string{"PREDICTIVE ACCURACY", "CALIBRATION", "MEAN REVERSION",
		"PROMOTION CONTINUITY", "FINDINGS", "challenger 0.80"} {
		if !strings.Contains(text.String(), want) {
			t.Errorf("the report does not mention %q", want)
		}
	}

	var jsonOut strings.Builder
	if err := rep.WriteJSON(&jsonOut); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	// The weights are in the output because a report that does not say what it
	// was measuring cannot be compared with another one.
	if !strings.Contains(jsonOut.String(), `"challenger": 0.8`) {
		t.Errorf("the JSON does not carry the weights it used")
	}
}

func TestMeanErrorIsWeightedByBandSize(t *testing.T) {
	a := newTestAccumulator(Config{})
	// One badly called match in a band of its own, and a hundred well-called
	// ones in another.
	a.observe(prediction(rating.TierTour, 2, 1, 0.05, 20))
	for i := 0; i < 70; i++ {
		a.observe(prediction(rating.TierTour, 1, 2, 0.7, 20))
	}
	for i := 0; i < 30; i++ {
		a.observe(prediction(rating.TierTour, 2, 1, 0.3, 20))
	}

	tables := a.calibration()
	// If the mean were taken across bands rather than matches, the one stray
	// band would count for half the table instead of one match in a hundred.
	if tables[0].MeanError > 2 {
		t.Errorf("mean error = %.1f, want the big well-called band to dominate", tables[0].MeanError)
	}
	if math.IsNaN(tables[0].MeanError) {
		t.Error("mean error is not a number")
	}
}
