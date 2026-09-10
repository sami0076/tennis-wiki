package validate

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sami0076/tennis-wiki/internal/db"
	"github.com/sami0076/tennis-wiki/internal/rating"
	"github.com/sami0076/tennis-wiki/internal/simulate"
)

// What the simulator can and cannot be validated for.
//
// ADR-0007 derives the point probabilities by solving for the pair whose match
// probability equals rating.Expected, so the chain's match-level answer is the
// rating's by construction. Checking it against results would be checking the
// rating engine, which the accuracy and calibration sections above already do,
// and reporting agreement as a finding would be reporting an identity.
//
// What the chain adds is everything below match level, and that is what these
// checks are for: how often a match should reach a deciding set, and how well a
// bracket played forward picks the player who actually won it.

// SimulationConfig bounds the two samples. Both default to something that runs
// in seconds, because a check nobody runs is not a check.
type SimulationConfig struct {
	Tour string
	// Matches sampled for the deciding-set check, most recent first.
	Matches int
	// Events sampled for the draw check.
	Events int
	Runs   int
	Seed   uint64
}

func (c SimulationConfig) withDefaults() SimulationConfig {
	if c.Tour == "" {
		c.Tour = "atp"
	}
	if c.Matches <= 0 {
		c.Matches = 20000
	}
	if c.Events <= 0 {
		c.Events = 200
	}
	if c.Runs <= 0 {
		c.Runs = 2000
	}
	if c.Seed == 0 {
		c.Seed = 1
	}
	return c
}

// DecidingSetBucket compares how often matches should have gone the distance
// with how often they did.
type DecidingSetBucket struct {
	// From and To bound the predicted probability of reaching a deciding set.
	From     float64 `json:"from"`
	To       float64 `json:"to"`
	Matches  int     `json:"matches"`
	Expected float64 `json:"expected"`
	Observed float64 `json:"observed"`
}

// DrawScore is how well the draw simulation picked champions.
//
// Brier is the mean squared error on the champion's own probability, which is
// the standard score for a probabilistic forecast: lower is better, and it is
// only meaningful against a baseline.
type DrawScore struct {
	Events int `json:"events"`
	// Brier is the simulation's score and BrierUniform what a model that knew
	// nothing but the size of the draw would get.
	Brier        float64 `json:"brier"`
	BrierUniform float64 `json:"brier_uniform"`
	// TopPick is how often the simulation's favourite actually won.
	TopPick float64 `json:"top_pick"`
	// MeanChampionOdds is the average probability given to the player who won.
	MeanChampionOdds float64 `json:"mean_champion_odds"`
}

// SimulationReport is the whole simulation section.
type SimulationReport struct {
	Tour         string              `json:"tour"`
	Sampled      int                 `json:"matches_sampled"`
	DecidingSets []DecidingSetBucket `json:"deciding_sets"`
	Draws        DrawScore           `json:"draws"`
	Findings     []Finding           `json:"findings"`
	Took         string              `json:"took"`
}

// RunSimulation measures what the chain adds over the rating it came from.
func RunSimulation(
	ctx context.Context, pool *pgxpool.Pool, cfg SimulationConfig,
) (SimulationReport, error) {
	cfg = cfg.withDefaults()
	started := time.Now()
	q := db.New(pool)

	rep := SimulationReport{Tour: cfg.Tour}

	cells, err := serveCells(ctx, q, cfg.Tour)
	if err != nil {
		return rep, err
	}
	if len(cells) == 0 {
		rep.Findings = append(rep.Findings, Finding{
			Name: "serve baseline", Severity: Failure,
			Detail: "No serve baselines exist, so no anchor can be found and nothing " +
				"below match level can be predicted. Run the ingest refresh step.",
		})
		rep.Took = time.Since(started).Round(time.Millisecond).String()
		return rep, nil
	}

	if err := rep.checkDecidingSets(ctx, q, cfg, cells); err != nil {
		return rep, err
	}
	if err := rep.checkDraws(ctx, q, cfg, cells); err != nil {
		return rep, err
	}

	rep.Findings = append(rep.Findings, Finding{
		Name: "match level", Severity: Info,
		Detail: "Match-level calibration is not reported here and cannot be. The chain " +
			"is solved so its match probability equals the rating's, so agreement is an " +
			"identity rather than a result. The accuracy and calibration sections above " +
			"are the check on that number.",
	})
	rep.Took = time.Since(started).Round(time.Millisecond).String()
	return rep, nil
}

// checkDecidingSets is the first thing the chain predicts that the rating alone
// does not: how often two players of a given gap should go the distance.
func (r *SimulationReport) checkDecidingSets(
	ctx context.Context, q *db.Queries, cfg SimulationConfig, cells []simulate.Baseline,
) error {
	rows, err := q.SampleRatedMatches(ctx, db.SampleRatedMatchesParams{
		Tour: db.Tour(cfg.Tour), RowLimit: int32(cfg.Matches),
	})
	if err != nil {
		return fmt.Errorf("sample rated matches: %w", err)
	}
	r.Sampled = len(rows)
	if len(rows) == 0 {
		return nil
	}

	const buckets = 5
	type cell struct {
		n              int
		expected, went float64
	}
	tally := make([]cell, buckets)
	var lo, hi = 1.0, 0.0

	for _, m := range rows {
		if m.DecidingSet == nil {
			continue
		}
		format := simulate.Format{BestOf: int(m.BestOf)}
		anchor, scope, _ := simulate.Anchor(
			cells, m.Tier, m.Surface, (int(m.Season)/10)*10)
		if scope == simulate.ScopeNone {
			continue
		}

		target := rating.Expected(m.WinnerElo, m.LoserElo)
		pA, pB, _ := simulate.Invert(target, anchor, format)
		set := simulate.Solve(pA, pB, format).Set[0]
		p := decidingSetProbability(set, format.BestOf)

		i := int(p * buckets)
		if i >= buckets {
			i = buckets - 1
		}
		tally[i].n++
		tally[i].expected += p
		if *m.DecidingSet {
			tally[i].went++
		}
		lo, hi = math.Min(lo, p), math.Max(hi, p)
	}

	for i, c := range tally {
		if c.n == 0 {
			continue
		}
		r.DecidingSets = append(r.DecidingSets, DecidingSetBucket{
			From:     float64(i) / buckets,
			To:       float64(i+1) / buckets,
			Matches:  c.n,
			Expected: c.expected / float64(c.n),
			Observed: c.went / float64(c.n),
		})
	}

	// One number for the whole sample, so the finding can say something.
	var n int
	var expected, observed float64
	for _, b := range r.DecidingSets {
		n += b.Matches
		expected += b.Expected * float64(b.Matches)
		observed += b.Observed * float64(b.Matches)
	}
	if n == 0 {
		return nil
	}
	expected, observed = expected/float64(n), observed/float64(n)

	gap := observed - expected
	severity := Info
	if math.Abs(gap) > 0.05 {
		severity = Warning
	}
	r.Findings = append(r.Findings, Finding{
		Name: "deciding sets", Severity: severity,
		Detail: fmt.Sprintf(
			"Over %d matches the chain expected %.1f%% to reach a deciding set and %.1f%% "+
				"did, a gap of %+.1f points. This is the first thing the chain claims that "+
				"the rating alone does not, so it is the first thing worth checking.",
			n, expected*100, observed*100, gap*100),
	})
	return nil
}

// decidingSetProbability is the chance a match reaches its last set, given the
// chance of winning any one of them.
func decidingSetProbability(set float64, bestOf int) float64 {
	lose := 1 - set
	if bestOf == 5 {
		// Two sets each after four.
		return 6 * set * set * lose * lose
	}
	return 2 * set * lose
}

// checkDraws plays historical draws forward and scores them against the player
// who actually won.
//
// This is the check the whole "simulate the past" decision bought: a
// forward-looking draw simulator could never be scored at all.
func (r *SimulationReport) checkDraws(
	ctx context.Context, q *db.Queries, cfg SimulationConfig, cells []simulate.Baseline,
) error {
	tour := db.Tour(cfg.Tour)
	events, err := q.ListSimulatableEvents(ctx, db.ListSimulatableEventsParams{
		Tour: &tour, RowLimit: int32(cfg.Events),
	})
	if err != nil {
		return fmt.Errorf("list events: %w", err)
	}

	var scored int
	var brier, uniform, championOdds, topPicks float64

	for _, e := range events {
		bracket, elos, err := reconstruct(ctx, q, e)
		if err != nil || bracket.Size() == 0 {
			continue
		}

		anchor, scope, _ := simulate.Anchor(cells, e.Tier, e.Surface, (int(e.Season)/10)*10)
		format := simulate.Format{BestOf: 3}
		if e.Tier == "tour" && bracket.Size() >= 128 {
			format.BestOf = 5
		}

		win := func(x, y simulate.Entrant) float64 {
			ex, okx := elos[x.PlayerID]
			ey, oky := elos[y.PlayerID]
			if !okx || !oky {
				return 0.5
			}
			target := rating.Expected(ex, ey)
			if scope == simulate.ScopeNone {
				return target
			}
			pA, pB, _ := simulate.Invert(target, anchor, format)
			return simulate.Solve(pA, pB, format).Match[0]
		}

		result := simulate.RunDraw(bracket, win, cfg.Runs, cfg.Seed)
		if len(result.Odds) == 0 {
			continue
		}

		var given, best float64
		var favourite int64
		for _, o := range result.Odds {
			if o.Entrant.PlayerID == bracket.Champion {
				given = o.Title
			}
			if o.Title > best {
				best, favourite = o.Title, o.Entrant.PlayerID
			}
		}

		scored++
		championOdds += given
		// Brier over the field: one squared error per entrant, of which exactly
		// one had an outcome of 1.
		for _, o := range result.Odds {
			actual := 0.0
			if o.Entrant.PlayerID == bracket.Champion {
				actual = 1
			}
			brier += (o.Title - actual) * (o.Title - actual)
		}
		even := 1 / float64(bracket.Size())
		uniform += (1-even)*(1-even) + float64(bracket.Size()-1)*even*even
		if favourite == bracket.Champion {
			topPicks++
		}
	}

	if scored == 0 {
		r.Findings = append(r.Findings, Finding{
			Name: "draw simulation", Severity: Warning,
			Detail: "No draw could be reconstructed and rated, so the draw simulation is " +
				"unscored. Ratings may not cover the sampled events.",
		})
		return nil
	}

	r.Draws = DrawScore{
		Events:           scored,
		Brier:            brier / float64(scored),
		BrierUniform:     uniform / float64(scored),
		TopPick:          topPicks / float64(scored),
		MeanChampionOdds: championOdds / float64(scored),
	}

	severity := Info
	if r.Draws.Brier >= r.Draws.BrierUniform {
		severity = Warning
	}
	r.Findings = append(r.Findings, Finding{
		Name: "draw simulation", Severity: severity,
		Detail: fmt.Sprintf(
			"Over %d draws the simulation scored %.4f against %.4f for a model that knew "+
				"only the size of the field. It gave the eventual champion %.1f%% on "+
				"average and named them its favourite %.1f%% of the time.",
			scored, r.Draws.Brier, r.Draws.BrierUniform,
			r.Draws.MeanChampionOdds*100, r.Draws.TopPick*100),
	})
	return nil
}

// reconstruct rebuilds one event's bracket and the ratings its entrants held
// the week it began.
func reconstruct(ctx context.Context, q *db.Queries, e db.ListSimulatableEventsRow) (
	simulate.Bracket, map[int64]float64, error,
) {
	rows, err := q.ListDrawMatches(ctx, e.ID)
	if err != nil {
		return simulate.Bracket{}, nil, err
	}
	matches := make([]simulate.BracketMatch, 0, len(rows))
	entrants := map[int64]simulate.Entrant{}
	for _, m := range rows {
		matches = append(matches, simulate.BracketMatch{
			Round: m.Round, RoundIdx: rating.RoundRank(m.Round),
			WinnerID: m.WinnerID, LoserID: m.LoserID,
		})
		entrants[m.WinnerID] = simulate.Entrant{PlayerID: m.WinnerID, Name: m.WinnerName}
		entrants[m.LoserID] = simulate.Entrant{PlayerID: m.LoserID, Name: m.LoserName}
	}

	bracket, err := simulate.BuildBracket(matches, entrants)
	if err != nil {
		return simulate.Bracket{}, nil, err
	}

	ids := make([]int64, 0, bracket.Size())
	for _, en := range bracket.Entrants {
		ids = append(ids, en.PlayerID)
	}
	ratings, err := q.CurrentEloAsOf(ctx, db.CurrentEloAsOfParams{
		Surface: db.RatingSurfaceOverall, OnDate: e.StartDate, PlayerIds: ids,
	})
	if err != nil {
		return simulate.Bracket{}, nil, err
	}
	elos := make(map[int64]float64, len(ratings))
	for _, r := range ratings {
		elos[r.PlayerID] = r.Elo
	}
	return bracket, elos, nil
}

func serveCells(ctx context.Context, q *db.Queries, tour string) ([]simulate.Baseline, error) {
	rows, err := q.ListServeBaselines(ctx, db.Tour(tour))
	if err != nil {
		return nil, err
	}
	cells := make([]simulate.Baseline, 0, len(rows))
	for _, r := range rows {
		cells = append(cells, simulate.Baseline{
			Tier: r.Tier, Surface: r.Surface, Decade: int(r.Decade),
			ServePoints: r.ServePoints, ServeWon: r.ServeWon,
		})
	}
	return cells, nil
}
