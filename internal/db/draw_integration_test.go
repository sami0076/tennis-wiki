package db

import (
	"testing"

	"github.com/sami0076/tennis-wiki/internal/rating"
	"github.com/sami0076/tennis-wiki/internal/simulate"
)

// An eight-player draw, written the way the source records one: three rounds,
// seven matches, and no draw positions anywhere.
func TestDrawMatchesReconstructABracket(t *testing.T) {
	h := newHarness(t)
	event := h.tournament("itg-draw-open", TierTour, 2019)

	ids := map[string]int64{}
	for _, slug := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		ids[slug] = h.player("itg-draw-"+slug, "Itg Draw"+slug, TourAtp)
	}

	// a beats b, c beats d, e beats f, g beats h; a beats c, e beats g; a beats e.
	type played struct {
		winner, loser string
		round         string
	}
	for i, m := range []played{
		{"a", "b", "QF"}, {"c", "d", "QF"}, {"e", "f", "QF"}, {"g", "h", "QF"},
		{"a", "c", "SF"}, {"e", "g", "SF"},
		{"a", "e", "F"},
	} {
		h.match(event, ids[m.winner], ids[m.loser], i+1, m.round, nil, false)
	}

	rows, err := h.ListDrawMatches(h.ctx, event)
	if err != nil {
		t.Fatalf("ListDrawMatches: %v", err)
	}
	if len(rows) != 7 {
		t.Fatalf("got %d matches, want the seven that were played", len(rows))
	}

	matches := make([]simulate.BracketMatch, 0, len(rows))
	entrants := map[int64]simulate.Entrant{}
	for _, r := range rows {
		matches = append(matches, simulate.BracketMatch{
			Round:    r.Round,
			RoundIdx: rating.RoundRank(r.Round),
			WinnerID: r.WinnerID,
			LoserID:  r.LoserID,
		})
		entrants[r.WinnerID] = simulate.Entrant{PlayerID: r.WinnerID, Slug: r.WinnerSlug, Name: r.WinnerName}
		entrants[r.LoserID] = simulate.Entrant{PlayerID: r.LoserID, Slug: r.LoserSlug, Name: r.LoserName}
	}

	bracket, err := simulate.BuildBracket(matches, entrants)
	if err != nil {
		t.Fatalf("BuildBracket: %v", err)
	}
	if bracket.Size() != 8 {
		t.Errorf("bracket size = %d, want 8", bracket.Size())
	}
	if bracket.Champion != ids["a"] {
		t.Errorf("champion = %d, want a", bracket.Champion)
	}
	if len(bracket.Rounds) != 3 || bracket.Rounds[0] != "QF" || bracket.Rounds[2] != "F" {
		t.Errorf("rounds = %v, want QF, SF, F", bracket.Rounds)
	}

	// The pairs that opened the event have to be adjacent, or the tree the
	// simulation plays forward is not the draw that was played.
	for i := 0; i < 8; i += 2 {
		left, right := bracket.Entrants[i].PlayerID, bracket.Entrants[i+1].PlayerID
		if !metInTheFirstRound(rows, left, right) {
			t.Errorf("entrants %d and %d are adjacent but never played each other", left, right)
		}
	}
}

func metInTheFirstRound(rows []ListDrawMatchesRow, a, b int64) bool {
	for _, r := range rows {
		if r.Round != "QF" {
			continue
		}
		if (r.WinnerID == a && r.LoserID == b) || (r.WinnerID == b && r.LoserID == a) {
			return true
		}
	}
	return false
}

// Qualifying is a separate draw and team events are not a draw at all, so
// neither reaches the reconstruction.
func TestDrawMatchesExcludeQualifyingAndTeamEvents(t *testing.T) {
	h := newHarness(t)
	event := h.tournament("itg-draw-mixed", TierTour, 2019)
	a := h.player("itg-mixed-a", "Itg Mixeda", TourAtp)
	b := h.player("itg-mixed-b", "Itg Mixedb", TourAtp)

	h.match(event, a, b, 1, "F", nil, false)
	if _, err := h.tx.Exec(h.ctx,
		`INSERT INTO matches (tournament_id, match_num, round, best_of, surface, winner_id,
		                      loser_id, played_on, source, is_qualifying)
		 VALUES ($1, 2, 'Q1', 3, 'hard', $2, $3, make_date(2019, 6, 1), 'test', true)`,
		event, a, b); err != nil {
		t.Fatalf("insert qualifying: %v", err)
	}

	rows, err := h.ListDrawMatches(h.ctx, event)
	if err != nil {
		t.Fatalf("ListDrawMatches: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("got %d matches, want only the main draw", len(rows))
	}
}
