package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// PlayerMatch is one match in a player's history, from that player's side.
type PlayerMatch struct {
	Date       string     `json:"date"`
	Tournament string     `json:"tournament"`
	Tier       string     `json:"tier"`
	Level      string     `json:"level"`
	Season     int16      `json:"season"`
	Round      string     `json:"round"`
	Qualifying bool       `json:"qualifying"`
	Surface    *string    `json:"surface"`
	Opponent   Opponent   `json:"opponent"`
	Won        bool       `json:"won"`
	Score      *string    `json:"score"`
	Incomplete bool       `json:"incomplete"`
	Minutes    *int16     `json:"minutes"`
	Serve      MatchServe `json:"serve"`
}

// Opponent is the other player, named well enough to link to.
type Opponent struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// MatchServe is this player's serve line for one match. Every count is a
// pointer: the source recorded nothing for 83% of the matches in the database,
// and a zero would read as a player who served and achieved nothing.
type MatchServe struct {
	// Availability is why the counts are absent when they are, using the same
	// vocabulary as the profile endpoint.
	Availability     string `json:"availability"`
	Aces             *int16 `json:"aces"`
	DoubleFaults     *int16 `json:"double_faults"`
	ServePoints      *int16 `json:"serve_points"`
	FirstIn          *int16 `json:"first_in"`
	FirstWon         *int16 `json:"first_won"`
	SecondWon        *int16 `json:"second_won"`
	ServeGames       *int16 `json:"serve_games"`
	BreakPointsSaved *int16 `json:"break_points_saved"`
	BreakPointsFaced *int16 `json:"break_points_faced"`
}

// matchCursor is the keyset position in a history: the date and id of the last
// row returned. Both order descending, so one row-value comparison walks the
// whole list without repeating or skipping.
type matchCursor struct {
	Date string `json:"d"`
	ID   int64  `json:"i"`
}

const (
	matchesDefaultLimit = 25
	matchesMaxLimit     = 100
)

func (a *API) handlePlayerMatches(w http.ResponseWriter, r *http.Request) {
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

	params := db.ListPlayerMatchesParams{PlayerID: player.ID}
	if err := applyMatchFilters(r, &params); err != nil {
		BadRequest(w, r, err.Error())
		return
	}

	limit, err := Limit(r, matchesDefaultLimit, matchesMaxLimit)
	if err != nil {
		BadRequest(w, r, err.Error())
		return
	}
	params.RowLimit = int32(limit)

	if raw := r.URL.Query().Get("cursor"); raw != "" {
		var c matchCursor
		if err := DecodeCursor(raw, &c); err != nil {
			BadRequest(w, r, "The cursor is not one this API issued.")
			return
		}
		after, err := time.Parse(time.DateOnly, c.Date)
		if err != nil {
			BadRequest(w, r, "The cursor is not one this API issued.")
			return
		}
		params.AfterDate, params.AfterID = &after, &c.ID
	}

	rows, err := a.Queries.ListPlayerMatches(ctx, params)
	if err != nil {
		Internal(w, r, err)
		return
	}

	page := Page[PlayerMatch]{Data: make([]PlayerMatch, 0, len(rows))}
	for _, row := range rows {
		page.Data = append(page.Data, buildPlayerMatch(row))
	}

	// A short page is the last page. Offering a cursor there would send the
	// client back for a guaranteed-empty response.
	if len(rows) == limit {
		last := rows[len(rows)-1]
		cursor, err := EncodeCursor(matchCursor{
			Date: last.PlayedOn.Format(time.DateOnly), ID: last.ID,
		})
		if err != nil {
			Internal(w, r, err)
			return
		}
		page.NextCursor = cursor
	}

	writeJSON(w, r, http.StatusOK, page)
}

// applyMatchFilters reads the filters off the query string. They compose: each
// one narrows what the others left.
func applyMatchFilters(r *http.Request, params *db.ListPlayerMatchesParams) error {
	query := r.URL.Query()

	switch raw := query.Get("surface"); raw {
	case "":
	case string(db.SurfaceHard), string(db.SurfaceClay),
		string(db.SurfaceGrass), string(db.SurfaceCarpet):
		s := db.Surface(raw)
		params.Surface = &s
	default:
		return errors.New("surface must be hard, clay, grass or carpet")
	}

	switch raw := query.Get("tier"); raw {
	case "":
	case string(db.TierTour), string(db.TierChallenger),
		string(db.TierFutures), string(db.TierItf):
		t := db.Tier(raw)
		params.Tier = &t
	default:
		return errors.New("tier must be tour, challenger, futures or itf")
	}

	if raw := query.Get("season"); raw != "" {
		year, err := strconv.Atoi(raw)
		if err != nil {
			return errors.New("season must be a four-digit year")
		}
		// The database starts in 1922. Anything outside the range is a typo
		// rather than a query, and saying so beats an empty page.
		if year < firstSeason || year > lastSeason {
			return errors.New("season must be a year between " +
				strconv.Itoa(firstSeason) + " and " + strconv.Itoa(lastSeason))
		}
		season := int16(year)
		params.Season = &season
	}

	// An opponent this player never faced is an empty page, not an error: the
	// slug may be a real player, and which pairs have met is #44's question.
	if raw := query.Get("opponent"); raw != "" {
		params.Opponent = &raw
	}
	return nil
}

// The span the sources cover. Wider than the data on both sides on purpose --
// this is a sanity check on the input, not a claim about coverage.
const (
	firstSeason = 1900
	lastSeason  = 2100
)

func buildPlayerMatch(row db.ListPlayerMatchesRow) PlayerMatch {
	m := PlayerMatch{
		Date:       row.PlayedOn.Format(time.DateOnly),
		Tournament: row.Tournament,
		Tier:       string(row.Tier),
		Level:      row.Level,
		Season:     row.Season,
		Round:      row.Round,
		Qualifying: row.IsQualifying,
		Won:        row.Won,
		Score:      row.Score,
		Incomplete: row.Incomplete,
		Minutes:    row.Minutes,
		Opponent:   Opponent{Slug: row.OpponentSlug, Name: row.OpponentName},
		Serve:      buildMatchServe(row),
	}
	if row.Surface != nil {
		surface := string(*row.Surface)
		m.Surface = &surface
	}
	return m
}

func buildMatchServe(row db.ListPlayerMatchesRow) MatchServe {
	// serve_points is the column the ingest check hangs off: present means the
	// row carries a stat line, absent means it never did.
	if row.ServePoints == nil {
		return MatchServe{Availability: explainMatchStats(row.Tier, int(row.Season))}
	}
	return MatchServe{
		Availability:     AvailabilityRecorded,
		Aces:             row.Aces,
		DoubleFaults:     row.DoubleFaults,
		ServePoints:      row.ServePoints,
		FirstIn:          row.FirstIn,
		FirstWon:         row.FirstWon,
		SecondWon:        row.SecondWon,
		ServeGames:       row.ServeGames,
		BreakPointsSaved: row.BpSaved,
		BreakPointsFaced: row.BpFaced,
	}
}

// explainMatchStats says why one match carries no serve line. The profile
// answers the same question over a whole career; this answers it per row, so a
// page can label the gap in a list rather than only in the summary.
func explainMatchStats(tier db.Tier, season int) string {
	// No Futures or ITF match has ever recorded serve statistics, in any year,
	// so for these the era is irrelevant.
	if tier == db.TierFutures || tier == db.TierItf {
		return AvailabilityNeverForTier
	}
	if season < firstStatisticalSeason {
		return AvailabilityNeverInEra
	}
	if tier == db.TierChallenger && season < challengerStatisticalSeason {
		return AvailabilityNeverInEra
	}
	return AvailabilityNotRecorded
}
