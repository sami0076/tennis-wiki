// Package validate reports whether the rating engine's output stands up.
//
// It exists because the tier weights below tour level -- Challenger 0.80,
// Futures 0.60, qualifying x0.90 -- were a starting point that no data had
// checked. ADR-0004 made them configuration rather than constants precisely so
// this report could try alternatives, so every run states the weights it used.
//
// The whole history is replayed to produce the report. Reading the stored
// ratings would be faster and would only ever describe the weights that wrote
// them, which is the one thing this is meant to question.
package validate

import (
	"context"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sami0076/tennis-wiki/internal/rating"
)

// Config tunes a validation run.
type Config struct {
	Weights rating.Weights
	// IncludeTeamEvents matches the rating engine's own default: excluded.
	IncludeTeamEvents bool
	// MinMatches is how much history each player needs before a prediction
	// counts. A match between two debutants is a coin toss the engine had no
	// way to call, and scoring it measures nothing.
	MinMatches int
	// PromotedAfter is how many lower-tier matches make a first tour-level
	// match a promotion rather than a wildcard.
	PromotedAfter int
	// PromotionWindow is how many tour-level matches after promotion are
	// measured against what the engine expected of them.
	PromotionWindow int
	// SystematicZ is the standard deviations away from expected at which the
	// promotion gap stops being noise. Stated rather than implied, because
	// "systematic" has to mean something checkable.
	SystematicZ float64
}

// Defaults for the thresholds above.
const (
	DefaultMinMatches      = 10
	DefaultPromotedAfter   = 20
	DefaultPromotionWindow = 20
	DefaultSystematicZ     = 3.0
)

func (c Config) withDefaults() Config {
	if (c.Weights == rating.Weights{}) {
		c.Weights = rating.DefaultWeights()
	}
	if c.MinMatches == 0 {
		c.MinMatches = DefaultMinMatches
	}
	if c.PromotedAfter == 0 {
		c.PromotedAfter = DefaultPromotedAfter
	}
	if c.PromotionWindow == 0 {
		c.PromotionWindow = DefaultPromotionWindow
	}
	if c.SystematicZ == 0 {
		c.SystematicZ = DefaultSystematicZ
	}
	return c
}

// TierAccuracy is how often the engine called the winner, at one tier.
type TierAccuracy struct {
	Tier string `json:"tier"`
	// Matches is how many predictions were scored, which is fewer than the
	// matches played: both players need MinMatches of history first.
	Matches  int     `json:"matches"`
	Correct  int     `json:"correct"`
	Accuracy float64 `json:"accuracy"`
}

// Bucket is one band of the calibration table.
type Bucket struct {
	From float64 `json:"from"`
	To   float64 `json:"to"`
	// Predicted is the mean probability the engine gave inside this band.
	Predicted float64 `json:"predicted"`
	// Observed is how often the favourite actually won inside it. Calibration
	// is the two agreeing.
	Observed float64 `json:"observed"`
	Matches  int     `json:"matches"`
}

// TierCalibration is the calibration table for one tier.
type TierCalibration struct {
	Tier    string   `json:"tier"`
	Buckets []Bucket `json:"buckets"`
	// MeanError is the average distance between predicted and observed across
	// the populated bands, weighted by how many matches fell in each.
	MeanError float64 `json:"mean_error"`
}

// MeanReversion reports where the pool sits against the base rating.
type MeanReversion struct {
	Base float64 `json:"base"`
	// Pool is the mean over every rated player. ADR-0004: under one weighted
	// pool this is the number that should sit near the base, and tour players
	// standing well above it is correct rather than a defect.
	Pool        float64 `json:"pool_mean"`
	PoolPlayers int     `json:"pool_players"`
	// Tour is the mean over players who reached tour level, reported beside it
	// for reference rather than as a target.
	Tour        float64 `json:"tour_mean"`
	TourPlayers int     `json:"tour_players"`
}

// PromotionContinuity measures the boundary the tier weights are most visible
// at: a player's first tour-level matches after a career below it.
//
// If the lower tiers are weighted too low, a promoted player arrives underrated
// and beats what the engine expects of them. Too high and the reverse. The sign
// of the gap says which way the weights have erred, which is the whole reason
// this check is worth more than the others.
type PromotionContinuity struct {
	Promotions int     `json:"promotions"`
	Matches    int     `json:"matches"`
	Expected   float64 `json:"expected_wins"`
	Actual     int     `json:"actual_wins"`
	// Z is the gap in standard deviations of the binomial it would be if the
	// engine were right.
	Z float64 `json:"z"`
	// Systematic is |Z| above the configured threshold.
	Systematic bool    `json:"systematic"`
	Threshold  float64 `json:"threshold"`
}

// Report is one validation run.
type Report struct {
	GeneratedAt time.Time           `json:"generated_at"`
	Weights     rating.Weights      `json:"weights"`
	Matches     int                 `json:"matches_replayed"`
	Scored      int                 `json:"predictions_scored"`
	MinMatches  int                 `json:"min_matches"`
	Accuracy    []TierAccuracy      `json:"accuracy"`
	Calibration []TierCalibration   `json:"calibration"`
	Reversion   MeanReversion       `json:"mean_reversion"`
	Promotion   PromotionContinuity `json:"promotion_continuity"`
	// Simulation is null when the run skipped it. It is a separate section
	// because it measures a different thing: not whether the ratings are right,
	// but what the chain built on top of them adds.
	Simulation *SimulationReport `json:"simulation,omitempty"`
	Findings   []Finding         `json:"findings"`
	Took       string            `json:"took"`
}

// Severity says how a finding should be read. The vocabulary matches
// cmd/dataqual so the two reports can be read the same way.
type Severity string

// Severities, in ascending order of urgency.
const (
	// Info is a fact about the ratings, not a defect.
	Info Severity = "info"
	// Warning is worth acting on and does not fail the run. Every finding here
	// is a judgement about weights, and a judgement should not gate CI.
	Warning Severity = "warning"
	// Failure means the run could not measure what it set out to.
	Failure Severity = "failure"
)

// Finding is one conclusion, in words.
type Finding struct {
	Name     string   `json:"name"`
	Severity Severity `json:"severity"`
	Detail   string   `json:"detail"`
}

// Failed reports whether the run should exit non-zero. Only an unmeasurable
// run does: a weighting that looks wrong is a result, not an error.
func (r Report) Failed() bool {
	for _, f := range r.findings() {
		if f.Severity == Failure {
			return true
		}
	}
	return false
}

// tiers fixes the reporting order, strongest first.
var tiers = []rating.Tier{
	rating.TierTour, rating.TierChallenger, rating.TierFutures, rating.TierITF,
}

// Run replays every match and reports on what the engine believed as it went.
func Run(ctx context.Context, pool *pgxpool.Pool, cfg Config) (Report, error) {
	cfg = cfg.withDefaults()
	started := time.Now()

	rep := Report{
		GeneratedAt: started.UTC(),
		Weights:     cfg.Weights,
		MinMatches:  cfg.MinMatches,
	}

	acc := newAccumulator(cfg)
	engine := rating.NewEngine(cfg.Weights, func(rating.Snapshot) error { return nil })
	engine.OnPrediction = acc.observe

	store := rating.NewStore(pool)
	matches, _, err := store.Matches(ctx, cfg.IncludeTeamEvents, engine.Add)
	if err != nil {
		return rep, err
	}
	if err := engine.Close(); err != nil {
		return rep, err
	}

	rep.Matches = matches
	rep.Scored = acc.scored
	rep.Accuracy = acc.accuracy()
	rep.Calibration = acc.calibration()
	rep.Reversion = acc.reversion(engine)
	rep.Promotion = acc.promotion()
	rep.Findings = acc.findings(rep)
	rep.Took = time.Since(started).Round(time.Millisecond).String()
	return rep, nil
}

// bucketCount splits [0.5, 1.0] into bands of five points.
const bucketCount = 10

type tierStats struct {
	matches int
	correct int
	bucketN [bucketCount]int
	bucketP [bucketCount]float64
	bucketW [bucketCount]int
}

type promotionState struct {
	// lower counts matches below tour level, which is what makes a first
	// tour-level match a promotion.
	lower int
	// window counts how many tour-level matches have been measured since.
	window   int
	promoted bool
}

type accumulator struct {
	cfg    Config
	scored int

	byTier map[rating.Tier]*tierStats
	// tourPlayers are everyone who reached tour level, for the reference mean.
	tourPlayers map[int64]struct{}

	promotions map[int64]*promotionState
	promoted   int
	promMatch  int
	promExp    float64
	promVar    float64
	promActual int
}

func newAccumulator(cfg Config) *accumulator {
	return &accumulator{
		cfg:         cfg,
		byTier:      map[rating.Tier]*tierStats{},
		tourPlayers: map[int64]struct{}{},
		promotions:  map[int64]*promotionState{},
	}
}

func (a *accumulator) observe(p rating.Prediction) {
	if p.Tier == rating.TierTour {
		a.tourPlayers[p.WinnerID] = struct{}{}
		a.tourPlayers[p.LoserID] = struct{}{}
	}
	a.trackPromotion(p)

	// Both players need history before the engine can be said to have had an
	// opinion worth scoring.
	if p.WinnerPlayed < a.cfg.MinMatches || p.LoserPlayed < a.cfg.MinMatches {
		return
	}
	a.scored++

	stats, ok := a.byTier[p.Tier]
	if !ok {
		stats = &tierStats{}
		a.byTier[p.Tier] = stats
	}

	probability, won := p.Favourite()
	stats.matches++
	if won {
		stats.correct++
	}

	// [0.5, 0.55) .. [0.95, 1.0]. A probability of exactly 1 belongs in the
	// last band rather than off the end of the table.
	//
	// The epsilon is not decoration: (0.7-0.5)/0.05 is 3.9999999999999996 in
	// binary floating point, which would file a prediction of exactly 70% in
	// the band below the one it names.
	index := int(math.Floor((probability-0.5)/0.05 + 1e-9))
	if index >= bucketCount {
		index = bucketCount - 1
	}
	if index < 0 {
		index = 0
	}
	stats.bucketN[index]++
	stats.bucketP[index] += probability
	if won {
		stats.bucketW[index]++
	}
}

// trackPromotion watches each player for the boundary between a career below
// tour level and their first matches at it.
func (a *accumulator) trackPromotion(p rating.Prediction) {
	for _, side := range []struct {
		id       int64
		expected float64
		won      bool
	}{
		{p.WinnerID, p.WinnerExpected, true},
		{p.LoserID, 1 - p.WinnerExpected, false},
	} {
		state, ok := a.promotions[side.id]
		if !ok {
			state = &promotionState{}
			a.promotions[side.id] = state
		}

		if p.Tier != rating.TierTour {
			if !state.promoted {
				state.lower++
			}
			continue
		}

		// A tour-level match. If enough of a lower-tier career came first,
		// this player has been promoted and the next few results are the
		// evidence about the weighting.
		if !state.promoted {
			if state.lower < a.cfg.PromotedAfter {
				continue
			}
			state.promoted = true
			a.promoted++
		}
		if state.window >= a.cfg.PromotionWindow {
			continue
		}
		state.window++

		a.promMatch++
		a.promExp += side.expected
		a.promVar += side.expected * (1 - side.expected)
		if side.won {
			a.promActual++
		}
	}
}

func (a *accumulator) accuracy() []TierAccuracy {
	out := make([]TierAccuracy, 0, len(tiers))
	for _, tier := range tiers {
		stats, ok := a.byTier[tier]
		if !ok || stats.matches == 0 {
			continue
		}
		out = append(out, TierAccuracy{
			Tier:     string(tier),
			Matches:  stats.matches,
			Correct:  stats.correct,
			Accuracy: 100 * float64(stats.correct) / float64(stats.matches),
		})
	}
	return out
}

func (a *accumulator) calibration() []TierCalibration {
	out := make([]TierCalibration, 0, len(tiers))
	for _, tier := range tiers {
		stats, ok := a.byTier[tier]
		if !ok || stats.matches == 0 {
			continue
		}

		table := TierCalibration{Tier: string(tier)}
		var weighted, total float64
		for i := 0; i < bucketCount; i++ {
			if stats.bucketN[i] == 0 {
				continue
			}
			n := float64(stats.bucketN[i])
			bucket := Bucket{
				From:      0.5 + float64(i)*0.05,
				To:        0.5 + float64(i+1)*0.05,
				Matches:   stats.bucketN[i],
				Predicted: 100 * stats.bucketP[i] / n,
				Observed:  100 * float64(stats.bucketW[i]) / n,
			}
			table.Buckets = append(table.Buckets, bucket)
			weighted += n * math.Abs(bucket.Predicted-bucket.Observed)
			total += n
		}
		if total > 0 {
			table.MeanError = weighted / total
		}
		out = append(out, table)
	}
	return out
}

func (a *accumulator) reversion(engine *rating.Engine) MeanReversion {
	var poolSum, tourSum float64
	var poolN, tourN int

	engine.EachRating(func(player int64, series rating.Series, elo float64, _ int) {
		// One series per player, or the surfaces would weight the mean by how
		// many surfaces someone happened to play on.
		if series != rating.Overall {
			return
		}
		poolSum += elo
		poolN++
		if _, ok := a.tourPlayers[player]; ok {
			tourSum += elo
			tourN++
		}
	})

	rev := MeanReversion{Base: rating.Base, PoolPlayers: poolN, TourPlayers: tourN}
	if poolN > 0 {
		rev.Pool = poolSum / float64(poolN)
	}
	if tourN > 0 {
		rev.Tour = tourSum / float64(tourN)
	}
	return rev
}

func (a *accumulator) promotion() PromotionContinuity {
	p := PromotionContinuity{
		Promotions: a.promoted,
		Matches:    a.promMatch,
		Expected:   a.promExp,
		Actual:     a.promActual,
		Threshold:  a.cfg.SystematicZ,
	}
	if a.promVar > 0 {
		p.Z = (float64(a.promActual) - a.promExp) / math.Sqrt(a.promVar)
	}
	p.Systematic = math.Abs(p.Z) > a.cfg.SystematicZ
	return p
}
