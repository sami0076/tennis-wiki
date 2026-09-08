package rating

import (
	"fmt"
	"sort"
	"time"
)

// Series names one rating a player carries. It mirrors the rating_surface enum
// created in migration 00001: an overall series alongside one per surface.
type Series string

// The five series. Overall is fed by every match; a surface series only by
// matches played on that surface.
const (
	Overall Series = "overall"
	Hard    Series = "hard"
	Clay    Series = "clay"
	Grass   Series = "grass"
	Carpet  Series = "carpet"
)

// ParseSurface maps the surface enum to the series it feeds. A match whose
// surface the source never recorded feeds the overall series only.
func ParseSurface(s string) (Series, bool) {
	switch Series(s) {
	case Hard, Clay, Grass, Carpet:
		return Series(s), true
	default:
		return "", false
	}
}

// Result is one decided match, as the engine needs to see it.
type Result struct {
	WinnerID int64
	LoserID  int64
	PlayedOn time.Time
	// Surface is empty where the source recorded none.
	Surface Series
	Match   Match
}

// Snapshot is one row of the ratings table: what a series was worth at the end
// of a week the player played in.
type Snapshot struct {
	PlayerID int64
	AsOf     time.Time
	Series   Series
	Elo      float64
	Matches  int
}

type key struct {
	player int64
	series Series
}

type state struct {
	elo     float64
	matches int
}

// Engine replays matches in order, keeping every player's five series and
// emitting a snapshot per series per week that series moved.
//
// Only weeks a player played are snapshotted. Measured on five seasons, the
// sparse form is 651,288 rows against 9,323,720 for the every-player-every-week
// form, and the ratio grows with the span; see docs/performance.md. Reading "as
// of" any date is then a lookup of the last row at or before it.
type Engine struct {
	weights Weights
	emit    func(Snapshot) error

	// OnPrediction, when set, is called with what the engine believed just
	// before each match was rated. Validation reads it; rating a database does
	// not, and leaving it nil costs nothing.
	OnPrediction func(Prediction)

	series  map[key]*state
	touched map[key]struct{}
	week    time.Time
	// Snapshots counts what has been emitted, which is the figure the row-count
	// projection is checked against.
	Snapshots int64
}

// NewEngine returns an engine that hands every snapshot to emit.
func NewEngine(w Weights, emit func(Snapshot) error) *Engine {
	return &Engine{
		weights: w,
		emit:    emit,
		series:  make(map[key]*state),
		touched: make(map[key]struct{}),
	}
}

// Add rates one match. Results must arrive in non-decreasing date order, which
// is what lets a week be closed as soon as the next one opens.
func (e *Engine) Add(r Result) error {
	week := WeekOf(r.PlayedOn)
	if week.Before(e.week) {
		return fmt.Errorf("match on %s arrived after week %s: results must be in order",
			r.PlayedOn.Format(time.DateOnly), e.week.Format(time.DateOnly))
	}
	if week.After(e.week) {
		if err := e.flush(); err != nil {
			return err
		}
		e.week = week
	}

	if e.OnPrediction != nil {
		e.OnPrediction(e.predict(r))
	}

	weight := e.weights.For(r.Match)
	e.rate(r.WinnerID, r.LoserID, Overall, weight)
	if r.Surface != "" {
		e.rate(r.WinnerID, r.LoserID, r.Surface, weight)
	}
	return nil
}

// Close emits the final week.
func (e *Engine) Close() error { return e.flush() }

// Rating reports a player's current rating in one series, which is what the
// tests assert on and what a validation pass reads at the end of a replay.
func (e *Engine) Rating(player int64, s Series) (elo float64, matches int) {
	st, ok := e.series[key{player, s}]
	if !ok {
		return Base, 0
	}
	return st.elo, st.matches
}

// Prediction is what the engine believed before one match was rated, which is
// the only moment the belief is a prediction rather than a memory.
type Prediction struct {
	Tier     Tier
	PlayedOn time.Time
	WinnerID int64
	LoserID  int64
	// WinnerExpected is the probability the overall series gave the player who
	// went on to win. Everything a validation run needs derives from it: the
	// favourite is whichever side is above a half, and the loser's expectation
	// is one minus this.
	WinnerExpected float64
	// Played is how many matches each player had before this one, overall.
	// A prediction over two debutants is not evidence about anything.
	WinnerPlayed int
	LoserPlayed  int
}

// Favourite reports the probability given to the higher-rated player and
// whether that player won.
//
// The tie is broken by id rather than by the result: breaking it in the
// winner's favour would score every even match as a correct prediction.
func (p Prediction) Favourite() (probability float64, won bool) {
	switch {
	case p.WinnerExpected > 0.5:
		return p.WinnerExpected, true
	case p.WinnerExpected < 0.5:
		return 1 - p.WinnerExpected, false
	default:
		return 0.5, p.WinnerID < p.LoserID
	}
}

func (e *Engine) predict(r Result) Prediction {
	w, l := e.stateOf(r.WinnerID, Overall), e.stateOf(r.LoserID, Overall)
	return Prediction{
		Tier:           r.Match.Tier,
		PlayedOn:       r.PlayedOn,
		WinnerID:       r.WinnerID,
		LoserID:        r.LoserID,
		WinnerExpected: Expected(w.elo, l.elo),
		WinnerPlayed:   w.matches,
		LoserPlayed:    l.matches,
	}
}

// EachRating calls fn for every series the replay ended holding, which is what
// a mean over the pool is taken across.
func (e *Engine) EachRating(fn func(player int64, series Series, elo float64, matches int)) {
	for k, st := range e.series {
		fn(k.player, k.series, st.elo, st.matches)
	}
}

// Players counts the players the replay gave a rating to.
func (e *Engine) Players() int {
	n := 0
	for k := range e.series {
		if k.series == Overall {
			n++
		}
	}
	return n
}

func (e *Engine) rate(winner, loser int64, s Series, weight float64) {
	w, l := e.stateOf(winner, s), e.stateOf(loser, s)

	// K is read before either count moves: n is the matches a player had
	// completed *before* this one.
	kw, kl := K(w.matches)*weight, K(l.matches)*weight
	w.elo, l.elo = Update(w.elo, l.elo, kw, kl)
	w.matches++
	l.matches++

	e.touched[key{winner, s}] = struct{}{}
	e.touched[key{loser, s}] = struct{}{}
}

func (e *Engine) stateOf(player int64, s Series) *state {
	k := key{player, s}
	st, ok := e.series[k]
	if !ok {
		st = &state{elo: Base}
		e.series[k] = st
	}
	return st
}

// flush writes one row per series that moved this week. Sorted, so two runs
// over the same data produce the same rows in the same order.
func (e *Engine) flush() error {
	if len(e.touched) == 0 {
		return nil
	}
	keys := make([]key, 0, len(e.touched))
	for k := range e.touched {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].player != keys[j].player {
			return keys[i].player < keys[j].player
		}
		return keys[i].series < keys[j].series
	})

	for _, k := range keys {
		st := e.series[k]
		if err := e.emit(Snapshot{
			PlayerID: k.player,
			AsOf:     e.week,
			Series:   k.series,
			Elo:      st.elo,
			Matches:  st.matches,
		}); err != nil {
			return err
		}
		e.Snapshots++
	}
	clear(e.touched)
	return nil
}

// WeekOf is the Monday of the week a date falls in, matching Postgres's
// date_trunc('week', ...) so the two agree about which week a match sits in.
func WeekOf(t time.Time) time.Time {
	t = t.UTC().Truncate(24 * time.Hour)
	offset := (int(t.Weekday()) + 6) % 7
	return t.AddDate(0, 0, -offset)
}
