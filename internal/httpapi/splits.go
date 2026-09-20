package httpapi

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/sami0076/tennis-wiki/internal/db"
	"github.com/sami0076/tennis-wiki/internal/score"
)

// PlayerSplits cuts a career by who was beaten and how close it was. Every
// record names the matches it is a record over, because the opponent's
// ranking is known for some matches and not others, and a score can be read
// for nearly all of them but not every one.
type PlayerSplits struct {
	// ByRank is the record against opponents ranked in each band on the day,
	// in order from the top: No. 1, top 5, top 10, top 20, top 50, top 100,
	// outside the top 100, and unranked. The bands nest: a win over No. 1 is
	// in every band down to the top 100.
	ByRank []RankBand `json:"by_rank"`
	// Ranked is how many matches the opponent's ranking is known for: the
	// denominator every band is a share of.
	Ranked int64 `json:"ranked"`
	// Higher and Lower are the record against opponents ranked above and
	// below the player on the day, over the matches where both are known.
	Higher Record `json:"higher"`
	Lower  Record `json:"lower"`
	// FinalSetTiebreaks is the record in matches decided by a tiebreak in the
	// deciding set, a match tiebreak included; Scored is the matches whose
	// score could be read, which is what it is a share of.
	FinalSetTiebreaks Record `json:"final_set_tiebreaks"`
	Scored            int64  `json:"scored"`
}

// RankBand is one band of opponent ranking.
type RankBand struct {
	// Band is the label: "1", "5", "10", "20", "50", "100", "outside", "unranked".
	Band    string `json:"band"`
	Matches int64  `json:"matches"`
	Wins    int64  `json:"wins"`
}

// Record is wins over matches.
type Record struct {
	Matches int64 `json:"matches"`
	Wins    int64 `json:"wins"`
}

// rankBands are the ceilings the record is cut at, top first.
var rankBands = []struct {
	label   string
	ceiling int32
}{
	{"1", 1}, {"5", 5}, {"10", 10}, {"20", 20}, {"50", 50}, {"100", 100},
}

// buildSplits buckets one player's matches by the opponent's ranking and
// finds the ones a final-set tiebreak decided.
func buildSplits(rows []db.ListPlayerOpponentRanksRow) *PlayerSplits {
	if len(rows) == 0 {
		return nil
	}
	s := &PlayerSplits{ByRank: make([]RankBand, 0, len(rankBands)+2)}
	bands := make([]RankBand, len(rankBands))
	for i, b := range rankBands {
		bands[i].Band = b.label
	}
	var outside, unranked RankBand
	outside.Band, unranked.Band = "outside", "unranked"

	for _, row := range rows {
		win := int64(0)
		if row.Won {
			win = 1
		}
		switch {
		case row.OpponentRank == nil:
			unranked.Matches++
			unranked.Wins += win
		default:
			s.Ranked++
			placed := false
			for i, b := range rankBands {
				if *row.OpponentRank <= b.ceiling {
					bands[i].Matches++
					bands[i].Wins += win
					placed = true
				}
			}
			if !placed {
				outside.Matches++
				outside.Wins += win
			}
			if row.OwnRank != nil {
				if *row.OpponentRank < *row.OwnRank {
					s.Higher.Matches++
					s.Higher.Wins += win
				} else if *row.OpponentRank > *row.OwnRank {
					s.Lower.Matches++
					s.Lower.Wins += win
				}
			}
		}
		if row.Score != nil {
			parsed, err := score.Parse(*row.Score)
			if err == nil && !parsed.Incomplete() {
				s.Scored++
				if row.DecidingSet != nil && *row.DecidingSet && lastSetTiebreak(parsed) {
					s.FinalSetTiebreaks.Matches++
					s.FinalSetTiebreaks.Wins += win
				}
			}
		}
	}
	s.ByRank = append(s.ByRank, bands...)
	s.ByRank = append(s.ByRank, outside, unranked)
	return s
}

// lastSetTiebreak reports whether the final set was settled by a tiebreak: a
// set tiebreak, or a match tiebreak played in its place.
func lastSetTiebreak(parsed score.Score) bool {
	if len(parsed.Sets) == 0 {
		return false
	}
	last := parsed.Sets[len(parsed.Sets)-1]
	return last.SuperTiebreak || last.TiebreakSet()
}

// PlayerSeasons is a career a year at a time.
type PlayerSeasons struct {
	Slug    string         `json:"slug"`
	Name    string         `json:"name"`
	Seasons []PlayerSeason `json:"seasons"`
}

// PlayerSeason is one year. Every rate carries the count it is over: a
// season with four recorded matches and sixty played is the common case in
// the 1990s, and the four must not read as the sixty.
type PlayerSeason struct {
	Season  int16 `json:"season"`
	Matches int64 `json:"matches"`
	Wins    int64 `json:"wins"`
	Losses  int64 `json:"losses"`
	Titles  int64 `json:"titles"`
	// Scored is the matches whose score could be read; the set, game and
	// tiebreak rates are over these.
	Scored          int64    `json:"scored"`
	SetsWon         int64    `json:"sets_won"`
	SetsPlayed      int64    `json:"sets_played"`
	GamesWon        int64    `json:"games_won"`
	GamesPlayed     int64    `json:"games_played"`
	TiebreaksWon    int64    `json:"tiebreaks_won"`
	TiebreaksPlayed int64    `json:"tiebreaks_played"`
	SetsPct         *float64 `json:"sets_pct"`
	GamesPct        *float64 `json:"games_pct"`
	TiebreaksPct    *float64 `json:"tiebreaks_pct"`
	// WithServe is the matches carrying the player's own serve line; the
	// four serve rates are over these. WithReturn is the opponents' lines,
	// which the break rate and the dominance ratio also need.
	WithServe  int64    `json:"with_serve"`
	WithReturn int64    `json:"with_return"`
	HoldPct    *float64 `json:"hold_pct"`
	BreakPct   *float64 `json:"break_pct"`
	AcePct     *float64 `json:"ace_pct"`
	DfPct      *float64 `json:"df_pct"`
	Dominance  *float64 `json:"dominance"`
}

func (a *API) handlePlayerSeasons(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	player, err := a.Queries.GetPlayerBySlug(ctx, chi.URLParam(r, "slug"))
	if errors.Is(err, pgx.ErrNoRows) {
		NotFound(w, r, "No player has that slug.")
		return
	}
	if err != nil {
		Internal(w, r, err)
		return
	}
	rows, err := a.Queries.ListPlayerSeasonTotals(ctx, player.ID)
	if err != nil {
		Internal(w, r, err)
		return
	}

	out := PlayerSeasons{Slug: player.Slug, Name: player.FullName, Seasons: make([]PlayerSeason, 0, len(rows))}
	for _, row := range rows {
		s := PlayerSeason{
			Season: row.Season, Matches: row.Matches, Wins: row.Wins, Losses: row.Matches - row.Wins, Titles: row.Titles,
			Scored: row.Scored, SetsWon: row.SetsWon, SetsPlayed: row.SetsPlayed,
			GamesWon: row.GamesWon, GamesPlayed: row.GamesPlayed,
			TiebreaksWon: row.TiebreaksWon, TiebreaksPlayed: row.TiebreaksPlayed,
			WithServe: row.WithServe, WithReturn: row.WithReturn,
		}
		s.SetsPct = share(row.SetsWon, row.SetsPlayed)
		s.GamesPct = share(row.GamesWon, row.GamesPlayed)
		s.TiebreaksPct = share(row.TiebreaksWon, row.TiebreaksPlayed)
		if row.WithServe > 0 {
			s.HoldPct = share(row.ServeGames-(row.BpFaced-row.BpSaved), row.ServeGames)
			s.AcePct = share(row.Aces, row.ServePoints)
			s.DfPct = share(row.DoubleFaults, row.ServePoints)
		}
		if row.WithReturn > 0 {
			s.BreakPct = share(row.OpBpFaced-row.OpBpSaved, row.OpServeGames)
		}
		// Return points won over serve points lost, on the matches that
		// carried both lines; the denominators are the season's totals.
		if row.WithServe > 0 && row.WithReturn > 0 && row.ServePoints > 0 && row.OpServePoints > 0 {
			lost := float64(row.ServePoints-row.FirstWon-row.SecondWon) / float64(row.ServePoints)
			won := float64(row.OpServePoints-row.OpFirstWon-row.OpSecondWon) / float64(row.OpServePoints)
			if lost > 0 {
				ratio := round3(won / lost)
				s.Dominance = &ratio
			}
		}
		out.Seasons = append(out.Seasons, s)
	}
	writeJSON(w, r, http.StatusOK, out)
}

// share is a percentage to one decimal, or nil where there is nothing to
// divide by: a player who faced no break point has not saved 0% of them.
func share(n, d int64) *float64 {
	if d <= 0 {
		return nil
	}
	v := round1(100 * float64(n) / float64(d))
	return &v
}
