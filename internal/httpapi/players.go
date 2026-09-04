package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/sami0076/tennis-wiki/internal/db"
	"github.com/sami0076/tennis-wiki/internal/name"
)

// Statistics availability. A missing statistic is a fact about the sources, and
// which fact it is decides what the page can say. Collapsing all of these into
// a single null would lose the ability to explain anything.
const (
	// AvailabilityRecorded means every eligible match carried statistics.
	AvailabilityRecorded = "recorded"
	// AvailabilityPartial means some did and some did not.
	AvailabilityPartial = "partial"
	// AvailabilityNeverForTier means no match at this player's level has ever
	// recorded serve statistics — Futures and ITF, in any year.
	AvailabilityNeverForTier = "never_recorded_for_tier"
	// AvailabilityNeverInEra means the matches predate the recording of them:
	// before 1991 anywhere, and before roughly 2010 at Challenger level.
	AvailabilityNeverInEra = "never_recorded_in_era"
	// AvailabilityNotRecorded is the honest fallback: absent, reason unknown.
	AvailabilityNotRecorded = "not_recorded"
)

// firstStatisticalSeason is when the tour began recording serve statistics.
const firstStatisticalSeason = 1991

// challengerStatisticalSeason is roughly when Challenger events followed.
const challengerStatisticalSeason = 2010

// PlayerProfile is the response for a single player.
type PlayerProfile struct {
	Slug      string  `json:"slug"`
	Name      string  `json:"name"`
	Tour      string  `json:"tour"`
	Country   *string `json:"country"`
	Hand      *string `json:"hand"`
	HeightCm  *int16  `json:"height_cm"`
	BirthDate *string `json:"birth_date"`
	ProSince  *int16  `json:"pro_since"`

	// Career is null for a player with no ingested matches, which is not the
	// same as a career of zeroes.
	Career *Career    `json:"career"`
	Serve  ServeStats `json:"serve"`
}

// Career is the win/loss record. Retirements and walkovers are counted here and
// excluded from every rate in Serve.
type Career struct {
	Matches           int64          `json:"matches"`
	Wins              int64          `json:"wins"`
	Losses            int64          `json:"losses"`
	WinPercentage     float64        `json:"win_percentage"`
	Titles            int64          `json:"titles"`
	IncompleteMatches int64          `json:"incomplete_matches"`
	FirstMatch        string         `json:"first_match"`
	LastMatch         string         `json:"last_match"`
	Surfaces          []SurfaceSplit `json:"surfaces"`
	Tiers             []TierSplit    `json:"tiers"`
}

// SurfaceSplit is a win/loss record on one surface.
type SurfaceSplit struct {
	Surface string `json:"surface"`
	Matches int64  `json:"matches"`
	Wins    int64  `json:"wins"`
	Losses  int64  `json:"losses"`
}

// TierSplit reports how many matches at a tier recorded statistics, which is
// what makes "never recorded at this level" a checkable claim.
type TierSplit struct {
	Tier             string `json:"tier"`
	Matches          int64  `json:"matches"`
	MatchesWithStats int64  `json:"matches_with_stats"`
}

// ServeStats carries the serve statistics, or the reason there are none.
type ServeStats struct {
	Availability    string      `json:"availability"`
	MatchesWithData int64       `json:"matches_with_data"`
	Rates           *ServeRates `json:"rates"`
}

// ServeRates are percentages and per-match averages. A nil field is a rate with
// no denominator, which is different from a rate that is zero: a player who
// never faced a break point has not saved 0% of them.
type ServeRates struct {
	AcesPerMatch         float64  `json:"aces_per_match"`
	DoubleFaultsPerMatch float64  `json:"double_faults_per_match"`
	FirstServeIn         *float64 `json:"first_serve_in_percentage"`
	FirstServeWon        *float64 `json:"first_serve_won_percentage"`
	SecondServeWon       *float64 `json:"second_serve_won_percentage"`
	BreakPointsSaved     *float64 `json:"break_points_saved_percentage"`
}

// PlayerSearchResult is one row of the search response.
type PlayerSearchResult struct {
	Slug     string  `json:"slug"`
	Name     string  `json:"name"`
	Tour     string  `json:"tour"`
	Country  *string `json:"country"`
	Matches  int64   `json:"matches"`
	BestTier string  `json:"best_tier,omitempty"`
	Score    float32 `json:"score"`
}

// playerCursor is the keyset position in a search: the rank and id of the last
// row returned. Opaque to clients, so the ordering can change freely.
type playerCursor struct {
	Rank float32 `json:"r"`
	ID   int64   `json:"i"`
}

const (
	searchDefaultLimit = 25
	searchMaxLimit     = 100
	// Below two characters a trigram query matches most of the database and
	// ranks nothing usefully.
	searchMinQuery = 2
)

func (a *API) handlePlayerSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	// Folded to the ASCII the sources actually store, so "Djoković" finds
	// "Novak Djokovic".
	q := name.Normalise(query.Get("q"))
	if len(q) < searchMinQuery {
		BadRequest(w, r, "Provide a search term of at least two characters in q.")
		return
	}

	var tour *db.Tour
	switch raw := query.Get("tour"); raw {
	case "":
	case string(db.TourAtp), string(db.TourWta):
		t := db.Tour(raw)
		tour = &t
	default:
		BadRequest(w, r, "tour must be atp or wta.")
		return
	}

	limit, err := Limit(r, searchDefaultLimit, searchMaxLimit)
	if err != nil {
		BadRequest(w, r, err.Error())
		return
	}

	params := db.SearchPlayersParams{Query: q, Tour: tour, RowLimit: int32(limit)}
	if raw := query.Get("cursor"); raw != "" {
		var c playerCursor
		if err := DecodeCursor(raw, &c); err != nil {
			BadRequest(w, r, "The cursor is not one this API issued.")
			return
		}
		params.AfterRank, params.AfterID = &c.Rank, &c.ID
	}

	rows, err := a.Queries.SearchPlayers(r.Context(), params)
	if err != nil {
		Internal(w, r, err)
		return
	}

	page := Page[PlayerSearchResult]{Data: make([]PlayerSearchResult, 0, len(rows))}
	for _, row := range rows {
		page.Data = append(page.Data, PlayerSearchResult{
			Slug:     row.Slug,
			Name:     row.FullName,
			Tour:     string(row.Tour),
			Country:  row.Country,
			Matches:  row.Matches,
			BestTier: row.BestTier,
			Score:    row.Score,
		})
	}

	// A short page is the last page. Offering a cursor there would send the
	// client back for a guaranteed-empty response.
	if len(rows) == limit {
		last := rows[len(rows)-1]
		cursor, err := EncodeCursor(playerCursor{Rank: last.Rank, ID: last.ID})
		if err != nil {
			Internal(w, r, err)
			return
		}
		page.NextCursor = cursor
	}

	writeJSON(w, r, http.StatusOK, page)
}

func (a *API) handlePlayer(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	ctx := r.Context()

	player, err := a.Queries.GetPlayerBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		NotFound(w, r, "No player has that slug.")
		return
	}
	if err != nil {
		Internal(w, r, err)
		return
	}

	profile := PlayerProfile{
		Slug:      player.Slug,
		Name:      player.FullName,
		Tour:      string(player.Tour),
		Country:   player.Country,
		HeightCm:  player.HeightCm,
		ProSince:  player.ProSince,
		BirthDate: formatDate(player.BirthDate),
		Serve:     ServeStats{Availability: AvailabilityNotRecorded},
	}
	if player.Hand != nil {
		hand := string(*player.Hand)
		profile.Hand = &hand
	}

	summary, err := a.Queries.GetPlayerCareerSummary(ctx, player.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		// The player exists but has no matches. Career stays null rather than
		// reporting a 0-0 record that was never played.
		writeJSON(w, r, http.StatusOK, profile)
		return
	}
	if err != nil {
		Internal(w, r, err)
		return
	}

	surfaces, err := a.Queries.GetPlayerSurfaceSplits(ctx, player.ID)
	if err != nil {
		Internal(w, r, err)
		return
	}
	tiers, err := a.Queries.GetPlayerTierSplits(ctx, player.ID)
	if err != nil {
		Internal(w, r, err)
		return
	}

	profile.Career = buildCareer(summary, surfaces, tiers)
	profile.Serve = buildServe(summary, tiers)
	writeJSON(w, r, http.StatusOK, profile)
}

func buildCareer(s db.GetPlayerCareerSummaryRow,
	surfaces []db.GetPlayerSurfaceSplitsRow, tiers []db.GetPlayerTierSplitsRow) *Career {
	c := &Career{
		Matches:           s.Matches,
		Wins:              s.Wins,
		Losses:            s.Losses,
		Titles:            s.Titles,
		IncompleteMatches: s.IncompleteMatches,
		FirstMatch:        s.FirstMatch.Format(time.DateOnly),
		LastMatch:         s.LastMatch.Format(time.DateOnly),
		Surfaces:          make([]SurfaceSplit, 0, len(surfaces)),
		Tiers:             make([]TierSplit, 0, len(tiers)),
	}
	if s.Matches > 0 {
		c.WinPercentage = round1(100 * float64(s.Wins) / float64(s.Matches))
	}
	for _, row := range surfaces {
		// The source leaves surface blank for some events; "unknown" keeps the
		// matches visible instead of dropping them from the splits.
		surface := "unknown"
		if row.Surface != nil {
			surface = string(*row.Surface)
		}
		c.Surfaces = append(c.Surfaces, SurfaceSplit{
			Surface: surface, Matches: row.Matches, Wins: row.Wins, Losses: row.Losses,
		})
	}
	for _, row := range tiers {
		c.Tiers = append(c.Tiers, TierSplit{
			Tier: string(row.Tier), Matches: row.Matches, MatchesWithStats: row.MatchesWithStats,
		})
	}
	return c
}

func buildServe(s db.GetPlayerCareerSummaryRow, tiers []db.GetPlayerTierSplitsRow) ServeStats {
	serve := ServeStats{MatchesWithData: s.StatMatches}
	if s.StatMatches == 0 {
		serve.Availability = explainMissingStats(s, tiers)
		return serve
	}

	// Retirements are excluded from the rates, so they are not part of what a
	// complete record would have covered.
	eligible := s.Matches - s.IncompleteMatches
	serve.Availability = AvailabilityRecorded
	if s.StatMatches < eligible {
		serve.Availability = AvailabilityPartial
	}

	matches := float64(s.StatMatches)
	serve.Rates = &ServeRates{
		AcesPerMatch:         round1(float64(s.Aces) / matches),
		DoubleFaultsPerMatch: round1(float64(s.DoubleFaults) / matches),
		FirstServeIn:         percent(s.FirstIn, s.ServePoints),
		FirstServeWon:        percent(s.FirstWon, s.FirstIn),
		SecondServeWon:       percent(s.SecondWon, s.ServePoints-s.FirstIn),
		BreakPointsSaved:     percent(s.BpSaved, s.BpFaced),
	}
	return serve
}

// explainMissingStats decides which of the three absences applies, so the page
// can say why rather than showing a wall of zeroes.
func explainMissingStats(s db.GetPlayerCareerSummaryRow, tiers []db.GetPlayerTierSplitsRow) string {
	if len(tiers) == 0 {
		return AvailabilityNotRecorded
	}

	everyTier := func(allowed ...db.Tier) bool {
		for _, t := range tiers {
			if !containsTier(allowed, t.Tier) {
				return false
			}
		}
		return true
	}

	// No Futures or ITF match has ever recorded serve statistics, in any year,
	// so for these players the era is irrelevant.
	if everyTier(db.TierFutures, db.TierItf) {
		return AvailabilityNeverForTier
	}
	if s.LastMatch.Year() < firstStatisticalSeason {
		return AvailabilityNeverInEra
	}
	if everyTier(db.TierChallenger, db.TierFutures, db.TierItf) &&
		s.LastMatch.Year() < challengerStatisticalSeason {
		return AvailabilityNeverInEra
	}
	return AvailabilityNotRecorded
}

func containsTier(tiers []db.Tier, want db.Tier) bool {
	for _, t := range tiers {
		if t == want {
			return true
		}
	}
	return false
}

// percent returns nil when the denominator is zero. That is a rate that does
// not exist, not a rate of zero.
func percent(numerator, denominator int64) *float64 {
	if denominator <= 0 {
		return nil
	}
	// A ratio above 1 means the underlying rows contradict each other. Withhold
	// it rather than serve a percentage above 100, which reads as a real figure.
	if numerator > denominator {
		return nil
	}
	v := round1(100 * float64(numerator) / float64(denominator))
	return &v
}

func round1(v float64) float64 {
	return float64(int64(v*10+0.5)) / 10
}

func formatDate(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.DateOnly)
	return &s
}
