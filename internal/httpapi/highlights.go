package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// eliteElo is the bar a "big win" is measured against. The rating scale starts
// every player at 1500 and the top of the men's tour sits near 2200 in a strong
// era, so 2000 is roughly the top ten of the week and comfortably above the
// noise of a single upset. It is a constant rather than a query parameter
// because a bar a caller can move is a bar nothing can be compared across, and
// the response states it so the page never has to guess what it meant.
const eliteElo = 2000.0

// highlightLimit caps the three lists. A best-wins list is read, not scanned;
// past a dozen rows it stops being a highlight and starts being the match log,
// which the player page already has in full.
const (
	bestWinsLimit = 10
	rivalsLimit   = 10
)

// roundOrder is the draw read from its widest round inwards, which is the order
// a career funnels through and the order the page draws. Qualifying rounds are
// excluded from the query, so they are not here either. RR is last because a
// round robin is not a rung of the same ladder.
var roundOrder = []string{"R128", "R64", "R32", "R16", "QF", "SF", "F", "RR", "BR"}

// PlayerHighlights is a career read as what happened rather than as a rate:
// the runs, the wins that cost the most, what was won and where a career kept
// stopping. Everything here is derived from matches the model rated, so a
// player it never rated has lists that are empty rather than zeroed.
type PlayerHighlights struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
	// Streaks holds at most three rows, keyed best, worst and current. A career
	// that has never lost has no worst row; that is an absent row, not a zero.
	Streaks  []Streak        `json:"streaks"`
	BestWins []BestWin       `json:"best_wins"`
	Finals   []FinalsRecord  `json:"finals"`
	Rounds   []RoundRecord   `json:"rounds"`
	Rivals   []Rival         `json:"rivals"`
	Schedule ScheduleQuality `json:"schedule"`
}

// Streak is one unbroken run of the same result. Retirements and walkovers
// break nothing: they are left out of the sequence entirely.
type Streak struct {
	// Kind is best, worst or current. Current can also be the best one.
	Kind   string `json:"kind"`
	Won    bool   `json:"won"`
	Length int64  `json:"length"`
	From   string `json:"from"`
	To     string `json:"to"`
}

// BestWin is one win, carrying the rating the opponent actually held that week
// rather than the one they ended their career on.
type BestWin struct {
	Date        string      `json:"date"`
	Tournament  string      `json:"tournament"`
	EventSlug   *string     `json:"event_slug"`
	Season      int16       `json:"season"`
	Level       string      `json:"level"`
	Tier        string      `json:"tier"`
	Surface     *string     `json:"surface"`
	Round       string      `json:"round"`
	Score       *string     `json:"score"`
	Opponent    NamedPlayer `json:"opponent"`
	OpponentElo float64     `json:"opponent_elo"`
	// EloAsOf is the week the rating was read from, which is on or before the
	// match rather than the day of it: the model rates weekly.
	EloAsOf string `json:"elo_as_of"`
}

// NamedPlayer is enough of another player to link to them and fly their flag.
// Distinct from Opponent in the match log, which is the same two fields
// without the country and is part of a response shape that does not carry one.
type NamedPlayer struct {
	Slug    string  `json:"slug"`
	Name    string  `json:"name"`
	Country *string `json:"country"`
}

// FinalsRecord is titles and finals at one kind of event. Category uses the
// same words the season index does, so a slam title and a Challenger title are
// never summed into one number.
type FinalsRecord struct {
	Category string `json:"category"`
	Titles   int64  `json:"titles"`
	Finals   int64  `json:"finals"`
}

// RoundRecord is the record in one round of a main draw.
type RoundRecord struct {
	Round   string `json:"round"`
	Matches int64  `json:"matches"`
	Wins    int64  `json:"wins"`
}

// Rival is an opponent a career kept running into, with the record against
// them and when they last met.
type Rival struct {
	// Spelled out rather than embedding NamedPlayer: Go inlines an embedded
	// struct into the JSON and the type generator does not, so embedding here
	// would put a field in the TypeScript that the API never sends.
	Slug       string  `json:"slug"`
	Name       string  `json:"name"`
	Country    *string `json:"country"`
	Matches    int64   `json:"matches"`
	Wins       int64   `json:"wins"`
	LastPlayed string  `json:"last_played"`
}

// ScheduleQuality is what a career was played against. RatedMatches is the
// denominator and is smaller than the career: an opponent the model had not
// rated yet contributes to neither the average nor the record.
type ScheduleQuality struct {
	RatedMatches int64 `json:"rated_matches"`
	// AverageElo and HighestElo are null when nothing was rated. A career
	// averaging zero Elo is not a thing that can happen.
	AverageElo *float64 `json:"average_elo"`
	HighestElo *float64 `json:"highest_elo"`
	// EliteElo is the bar EliteMatches and EliteWins are counted above, stated
	// so the page can name it instead of hardcoding it a second time.
	EliteElo     float64 `json:"elite_elo"`
	EliteMatches int64   `json:"elite_matches"`
	EliteWins    int64   `json:"elite_wins"`
}

func (a *API) handlePlayerHighlights(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slug := chi.URLParam(r, "slug")

	player, err := a.Queries.GetPlayerBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		NotFound(w, r, "No player has that slug.")
		return
	}
	if err != nil {
		Internal(w, r, err)
		return
	}

	out := PlayerHighlights{
		Slug:     player.Slug,
		Name:     player.FullName,
		Streaks:  []Streak{},
		BestWins: []BestWin{},
		Finals:   []FinalsRecord{},
		Rounds:   []RoundRecord{},
		Rivals:   []Rival{},
		Schedule: ScheduleQuality{EliteElo: eliteElo},
	}

	streaks, err := a.Queries.ListPlayerStreaks(ctx, player.ID)
	if err != nil {
		Internal(w, r, err)
		return
	}
	for _, row := range streaks {
		out.Streaks = append(out.Streaks, Streak{
			Kind:   row.Kind,
			Won:    row.Won,
			Length: row.Length,
			From:   row.FromDate.Format(time.DateOnly),
			To:     row.ToDate.Format(time.DateOnly),
		})
	}

	wins, err := a.Queries.ListPlayerBestWins(ctx, db.ListPlayerBestWinsParams{
		PlayerID: player.ID, RowLimit: bestWinsLimit,
	})
	if err != nil {
		Internal(w, r, err)
		return
	}
	for _, row := range wins {
		out.BestWins = append(out.BestWins, BestWin{
			Date:       row.PlayedOn.Format(time.DateOnly),
			Tournament: row.Tournament,
			EventSlug:  row.EventSlug,
			Season:     row.Season,
			Level:      row.Level,
			Tier:       string(row.Tier),
			Surface:    surfaceName(row.Surface),
			Round:      row.Round,
			Score:      row.Score,
			Opponent: NamedPlayer{
				Slug: row.OpponentSlug, Name: row.OpponentName, Country: row.OpponentCountry,
			},
			OpponentElo: round1(row.OpponentElo),
			EloAsOf:     row.EloAsOf.Format(time.DateOnly),
		})
	}

	finals, err := a.Queries.ListPlayerFinalsByCategory(ctx, player.ID)
	if err != nil {
		Internal(w, r, err)
		return
	}
	for _, row := range finals {
		out.Finals = append(out.Finals, FinalsRecord{
			Category: row.Category, Titles: row.Titles, Finals: row.Finals,
		})
	}

	rounds, err := a.Queries.ListPlayerRoundRecord(ctx, player.ID)
	if err != nil {
		Internal(w, r, err)
		return
	}
	out.Rounds = orderRounds(rounds)

	rivals, err := a.Queries.ListPlayerRivals(ctx, db.ListPlayerRivalsParams{
		PlayerID: player.ID, RowLimit: rivalsLimit,
	})
	if err != nil {
		Internal(w, r, err)
		return
	}
	for _, row := range rivals {
		out.Rivals = append(out.Rivals, Rival{
			Slug:       row.Slug,
			Name:       row.Name,
			Country:    row.Country,
			Matches:    row.Matches,
			Wins:       row.Wins,
			LastPlayed: row.LastPlayed.Format(time.DateOnly),
		})
	}

	quality, err := a.Queries.GetPlayerOpponentQuality(ctx, db.GetPlayerOpponentQualityParams{
		PlayerID: player.ID, EliteElo: eliteElo,
	})
	if err != nil {
		Internal(w, r, err)
		return
	}
	out.Schedule.RatedMatches = quality.RatedMatches
	out.Schedule.EliteMatches = quality.EliteMatches
	out.Schedule.EliteWins = quality.EliteWins
	if quality.RatedMatches > 0 {
		average, highest := round1(quality.AverageElo), round1(quality.HighestElo)
		out.Schedule.AverageElo, out.Schedule.HighestElo = &average, &highest
	}

	writeJSON(w, r, http.StatusOK, out)
}

// orderRounds puts the rounds in draw order and drops the ones never played,
// so a page can read the list straight across without deciding what R16 means
// for a career that never reached one.
func orderRounds(rows []db.ListPlayerRoundRecordRow) []RoundRecord {
	byRound := make(map[string]db.ListPlayerRoundRecordRow, len(rows))
	for _, row := range rows {
		byRound[row.Round] = row
	}
	out := make([]RoundRecord, 0, len(rows))
	for _, round := range roundOrder {
		row, ok := byRound[round]
		if !ok {
			continue
		}
		delete(byRound, round)
		out = append(out, RoundRecord{Round: round, Matches: row.Matches, Wins: row.Wins})
	}
	// A round the source spells in a way this list does not know about is
	// appended rather than dropped: an unplaceable round is still played tennis.
	for _, row := range rows {
		if _, ok := byRound[row.Round]; !ok {
			continue
		}
		delete(byRound, row.Round)
		out = append(out, RoundRecord{Round: row.Round, Matches: row.Matches, Wins: row.Wins})
	}
	return out
}

// surfaceName turns the database enum into the string the API speaks, keeping
// an unrecorded surface as null rather than inventing a fifth one.
func surfaceName(s *db.Surface) *string {
	if s == nil {
		return nil
	}
	name := string(*s)
	return &name
}
