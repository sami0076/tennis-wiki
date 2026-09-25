package httpapi

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/sami0076/tennis-wiki/internal/db"
)

const (
	commonDefaultLimit = 25
	commonMaxLimit     = 100
)

// CommonOpponents is the comparison a short head to head cannot make. Two
// players who have met three times have usually played the same few dozen
// people, and how each did against that shared field says more about the
// matchup than three results do.
//
// Paired the way the rest of the head to head is: index 0 is the first slug in
// the URL, index 1 the second, so /h2h/a/b/common and /h2h/b/a/common are one
// comparison read from opposite ends.
type CommonOpponents struct {
	Players [2]HeadToHeadPlayer `json:"players" tstype:"Pair<HeadToHeadPlayer>"`
	// Totals is each side's record over the whole shared field. It counts only
	// the opponents this response carries, so it moves with the limit; the page
	// says what it is a total of rather than implying it is the career.
	Totals [2]CommonRecord `json:"totals" tstype:"Pair<CommonRecord>"`
	// Opponents is every shared opponent this response carries, most-played
	// first. Shown is how many that is and Total how many there are, so a page
	// can say what it is not showing rather than implying there is no more.
	Opponents []CommonOpponent `json:"opponents"`
	Total     int              `json:"total"`
}

// CommonRecord is one side's win-loss record over the shared field.
type CommonRecord struct {
	Matches int `json:"matches"`
	Wins    int `json:"wins"`
}

// CommonOpponent is one third player and what each side did against them.
type CommonOpponent struct {
	Slug    string  `json:"slug"`
	Name    string  `json:"name"`
	Country *string `json:"country"`
	// Matches and Wins are in Players order. A side that has played this
	// opponent once and won is 1-0, which is a real record over a sample of
	// one; the page is responsible for saying so, not this endpoint.
	Matches [2]int `json:"matches" tstype:"Pair<number>"`
	Wins    [2]int `json:"wins" tstype:"Pair<number>"`
}

func (a *API) handleCommonOpponents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	first, err := a.Queries.GetPlayerBySlug(ctx, chi.URLParam(r, "slug"))
	if errors.Is(err, pgx.ErrNoRows) {
		NotFound(w, r, "No player has that slug.")
		return
	}
	if err != nil {
		Internal(w, r, err)
		return
	}

	second, err := a.Queries.GetPlayerBySlug(ctx, chi.URLParam(r, "opponent"))
	if errors.Is(err, pgx.ErrNoRows) {
		NotFound(w, r, "No player has that slug.")
		return
	}
	if err != nil {
		Internal(w, r, err)
		return
	}

	if first.ID == second.ID {
		BadRequest(w, r, "A player cannot be compared with themselves.")
		return
	}

	limit, err := Limit(r, commonDefaultLimit, commonMaxLimit)
	if err != nil {
		BadRequest(w, r, err.Error())
		return
	}

	rows, err := a.Queries.ListCommonOpponents(ctx, db.ListCommonOpponentsParams{
		PlayerA: first.ID, PlayerB: second.ID, RowLimit: int32(limit),
	})
	if err != nil {
		Internal(w, r, err)
		return
	}

	out := CommonOpponents{
		Players:   [2]HeadToHeadPlayer{headToHeadPlayer(first), headToHeadPlayer(second)},
		Opponents: make([]CommonOpponent, 0, len(rows)),
		Total:     len(rows),
	}
	for _, row := range rows {
		out.Opponents = append(out.Opponents, CommonOpponent{
			Slug:    row.Slug,
			Name:    row.Name,
			Country: row.Country,
			Matches: [2]int{int(row.AMatches), int(row.BMatches)},
			Wins:    [2]int{int(row.AWins), int(row.BWins)},
		})
		out.Totals[0].Matches += int(row.AMatches)
		out.Totals[0].Wins += int(row.AWins)
		out.Totals[1].Matches += int(row.BMatches)
		out.Totals[1].Wins += int(row.BWins)
	}

	// The query is capped at the limit, so a full page might have more behind
	// it. Asking for one more row than needed would be a second shape to
	// maintain; instead the page is told what it has and says only that.
	if len(rows) == limit {
		out.Total = limit
	}

	writeJSON(w, r, http.StatusOK, out)
}
