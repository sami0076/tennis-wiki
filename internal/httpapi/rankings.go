package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// Ranking types.
const (
	// RankingElo is this project's own rating.
	RankingElo = "elo"
	// RankingOfficial is the list the tour publishes.
	RankingOfficial = "official"
)

// activeWindow is how stale a rating may be and still appear in a leaderboard.
//
// Without it a ranking is a list of the retired: Navratilova holds 2498 from
// 2005 and is not in anybody's current standings. A year is long enough to keep
// an injured player and short enough to drop a departed one.
const activeWindow = 52 * 7 * 24 * time.Hour

// endOfTime stands in for "no date given", which is the same question as "as of
// the last week there is".
var endOfTime = time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)

// RankingRow is one player in a ranking.
type RankingRow struct {
	// Position is where they stand in this list, which is not their official
	// rank and is not the same thing under the two types.
	Position int     `json:"position"`
	Slug     string  `json:"slug"`
	Name     string  `json:"name"`
	Tour     string  `json:"tour"`
	Country  *string `json:"country"`
	// Elo and OfficialRank are both here under both types, because the
	// comparison between them is the point of the page.
	Elo          *float64 `json:"elo"`
	PeakElo      *float64 `json:"peak_elo"`
	OfficialRank *int32   `json:"official_rank"`
	Points       *int32   `json:"points"`
	Matches      *int32   `json:"matches"`
	// Age at the effective date, null where no birth date was recorded.
	Age *int `json:"age"`
	// Delta is how many places above or below their official rank the model
	// puts them. Null under the official type, where this list's position is
	// the official rank and the comparison would be with itself.
	Delta *int `json:"delta"`
	// BestSurface is the surface series they are currently highest on, and
	// BestSurfaceElo the raw rating there: the figure the player page's strip
	// shows, not the blend the simulator uses. Both null for a player with no
	// surface rated inside the active window, never 1500.
	BestSurface    *string  `json:"best_surface"`
	BestSurfaceElo *float64 `json:"best_surface_elo"`
}

// RankingPage is a page of a ranking, and the date it is a ranking as of.
type RankingPage struct {
	Type    string  `json:"type"`
	Surface string  `json:"surface,omitempty"`
	Tour    *string `json:"tour"`
	// AsOf is the week actually used. Always the latest that exists at or
	// before what was asked for, never today.
	AsOf string `json:"as_of"`
	// Requested is echoed only when it differed from AsOf, so a caller can see
	// that their date was moved and by how much.
	Requested  string       `json:"requested,omitempty"`
	Data       []RankingRow `json:"data"`
	NextCursor string       `json:"next_cursor,omitempty"`
}

// eloCursor is the keyset position in an Elo ranking.
type eloCursor struct {
	Elo float64 `json:"e"`
	ID  int64   `json:"i"`
}

// officialCursor is the keyset position in a published ranking.
type officialCursor struct {
	Rank int32 `json:"r"`
	ID   int64 `json:"i"`
}

const (
	rankingsDefaultLimit = 25
	rankingsMaxLimit     = 200
)

func (a *API) handleRankings(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	kind := query.Get("type")
	if kind == "" {
		kind = RankingElo
	}
	if kind != RankingElo && kind != RankingOfficial {
		BadRequest(w, r, "type must be elo or official.")
		return
	}

	surface := db.RatingSurfaceOverall
	if raw := query.Get("surface"); raw != "" {
		if kind == RankingOfficial {
			BadRequest(w, r, "surface applies to the elo type only: the tours publish one list.")
			return
		}
		parsed, ok := parseRatingSurface(raw)
		if !ok {
			BadRequest(w, r, "surface must be overall, hard, clay, grass or carpet.")
			return
		}
		surface = parsed
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

	requested := endOfTime
	requestedRaw := query.Get("date")
	if requestedRaw != "" {
		parsed, err := time.Parse(time.DateOnly, requestedRaw)
		if err != nil {
			BadRequest(w, r, "date must be a date, as YYYY-MM-DD.")
			return
		}
		requested = parsed
	}

	limit, err := Limit(r, rankingsDefaultLimit, rankingsMaxLimit)
	if err != nil {
		BadRequest(w, r, err.Error())
		return
	}

	page := RankingPage{Type: kind, Tour: tourName(tour), Data: []RankingRow{}}
	if kind == RankingElo {
		page.Surface = string(surface)
	}
	if requestedRaw != "" {
		page.Requested = requestedRaw
	}

	raw := query.Get("cursor")
	if kind == RankingElo {
		var after *eloCursor
		if raw != "" {
			var c eloCursor
			if err := DecodeCursor(raw, &c); err != nil {
				BadRequest(w, r, "The cursor is not one this API issued.")
				return
			}
			after = &c
		}
		a.eloRankings(w, r, &page, surface, tour, requested, limit, after)
		return
	}

	var after *officialCursor
	if raw != "" {
		var c officialCursor
		if err := DecodeCursor(raw, &c); err != nil {
			BadRequest(w, r, "The cursor is not one this API issued.")
			return
		}
		after = &c
	}
	a.officialRankings(w, r, &page, surface, tour, requested, limit, after)
}

func tourName(t *db.Tour) *string {
	if t == nil {
		return nil
	}
	name := string(*t)
	return &name
}

// emptyAsOf answers a date outside coverage: an empty page that states the date
// it looked at, rather than a 404 that implies the endpoint is missing.
func emptyAsOf(w http.ResponseWriter, r *http.Request, page *RankingPage, requested time.Time) {
	if requested != endOfTime {
		page.AsOf = requested.Format(time.DateOnly)
	}
	writeJSON(w, r, http.StatusOK, *page)
}

func (a *API) eloRankings(
	w http.ResponseWriter, r *http.Request, page *RankingPage,
	surface db.RatingSurface, tour *db.Tour, requested time.Time, limit int, after *eloCursor,
) {
	ctx := r.Context()

	asOf, err := a.Queries.EffectiveEloDate(ctx, db.EffectiveEloDateParams{
		Surface: surface, OnOrBefore: requested, Tour: tour,
	})
	if err != nil || asOf.IsZero() {
		if err != nil && !isNoRows(err) {
			Internal(w, r, err)
			return
		}
		emptyAsOf(w, r, page, requested)
		return
	}
	page.AsOf = asOf.Format(time.DateOnly)

	// The published list has its own dates, so it is resolved separately rather
	// than assumed to share a Monday with the ratings.
	officialDate, err := a.Queries.EffectiveOfficialDate(ctx, db.EffectiveOfficialDateParams{
		Tour: tour, OnOrBefore: asOf,
	})
	if err != nil && !isNoRows(err) {
		Internal(w, r, err)
		return
	}

	params := db.ListEloRankingsParams{
		Surface:      surface,
		OnDate:       asOf,
		Since:        asOf.Add(-activeWindow),
		Tour:         tour,
		OfficialDate: officialDate,
		RowLimit:     int32(limit),
	}
	if after != nil {
		params.AfterElo, params.AfterID = &after.Elo, &after.ID
	}

	rows, err := a.Queries.ListEloRankings(ctx, params)
	if err != nil {
		Internal(w, r, err)
		return
	}

	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.PlayerID)
	}
	peaks, err := a.peaks(r, surface, asOf, ids)
	if err != nil {
		Internal(w, r, err)
		return
	}
	best, err := a.bestSurfaces(r, asOf, ids)
	if err != nil {
		Internal(w, r, err)
		return
	}

	for _, row := range rows {
		out := RankingRow{
			Position:     int(row.Position),
			Slug:         row.Slug,
			Name:         row.FullName,
			Tour:         string(row.Tour),
			Country:      row.Country,
			OfficialRank: row.OfficialRank,
			Points:       row.Points,
			Age:          ageAt(row.BirthDate, asOf),
		}
		elo := round1(row.Elo)
		out.Elo = &elo
		matches := row.MatchesPlayed
		out.Matches = &matches
		if peak, ok := peaks[row.PlayerID]; ok {
			value := round1(peak)
			out.PeakElo = &value
		}
		out.BestSurface, out.BestSurfaceElo = best[row.PlayerID].fields()
		// Positive means the model puts them higher than the tour does, which
		// is the direction the caption on the design reads.
		if row.OfficialRank != nil {
			delta := int(*row.OfficialRank) - int(row.Position)
			out.Delta = &delta
		}
		page.Data = append(page.Data, out)
	}

	if len(rows) == limit {
		last := rows[len(rows)-1]
		cursor, err := EncodeCursor(eloCursor{Elo: last.Elo, ID: last.PlayerID})
		if err != nil {
			Internal(w, r, err)
			return
		}
		page.NextCursor = cursor
	}

	writeJSON(w, r, http.StatusOK, *page)
}

func (a *API) officialRankings(
	w http.ResponseWriter, r *http.Request, page *RankingPage,
	surface db.RatingSurface, tour *db.Tour, requested time.Time, limit int,
	after *officialCursor,
) {
	ctx := r.Context()

	asOf, err := a.Queries.EffectiveOfficialDate(ctx, db.EffectiveOfficialDateParams{
		Tour: tour, OnOrBefore: requested,
	})
	if err != nil || asOf.IsZero() {
		if err != nil && !isNoRows(err) {
			Internal(w, r, err)
			return
		}
		emptyAsOf(w, r, page, requested)
		return
	}
	page.AsOf = asOf.Format(time.DateOnly)

	params := db.ListOfficialRankingsParams{OnDate: asOf, Tour: tour, RowLimit: int32(limit)}
	if after != nil {
		params.AfterRank, params.AfterID = &after.Rank, &after.ID
	}

	rows, err := a.Queries.ListOfficialRankings(ctx, params)
	if err != nil {
		Internal(w, r, err)
		return
	}

	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.PlayerID)
	}
	peaks, err := a.peaks(r, surface, asOf, ids)
	if err != nil {
		Internal(w, r, err)
		return
	}
	current, err := a.currentElo(r, surface, asOf, ids)
	if err != nil {
		Internal(w, r, err)
		return
	}
	best, err := a.bestSurfaces(r, asOf, ids)
	if err != nil {
		Internal(w, r, err)
		return
	}

	for _, row := range rows {
		rank := row.Rank
		out := RankingRow{
			Position:     int(row.Rank),
			Slug:         row.Slug,
			Name:         row.FullName,
			Tour:         string(row.Tour),
			Country:      row.Country,
			OfficialRank: &rank,
			Points:       row.Points,
			Age:          ageAt(row.BirthDate, asOf),
		}
		// A player the ratings have never seen has no Elo, which is not an Elo
		// of zero and not one of 1500 either.
		if elo, ok := current[row.PlayerID]; ok {
			value := round1(elo)
			out.Elo = &value
		}
		if peak, ok := peaks[row.PlayerID]; ok {
			value := round1(peak)
			out.PeakElo = &value
		}
		out.BestSurface, out.BestSurfaceElo = best[row.PlayerID].fields()
		page.Data = append(page.Data, out)
	}

	if len(rows) == limit {
		last := rows[len(rows)-1]
		cursor, err := EncodeCursor(officialCursor{Rank: last.Rank, ID: last.PlayerID})
		if err != nil {
			Internal(w, r, err)
			return
		}
		page.NextCursor = cursor
	}

	writeJSON(w, r, http.StatusOK, *page)
}

// peaks reads the peak rating for one page of players.
func (a *API) peaks(
	r *http.Request, surface db.RatingSurface, asOf time.Time, ids []int64,
) (map[int64]float64, error) {
	out := map[int64]float64{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := a.Queries.PeakEloAsOf(r.Context(), db.PeakEloAsOfParams{
		Surface: surface, OnDate: asOf, PlayerIds: ids,
	})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.PlayerID] = row.Peak
	}
	return out, nil
}

// bestSurface is the surface a player is currently highest on, and the rating.
type bestSurface struct {
	surface string
	elo     float64
}

// fields is the pair as the row carries it: both null when there is no entry,
// which is what the zero value of a map lookup is.
func (b *bestSurface) fields() (*string, *float64) {
	if b == nil {
		return nil, nil
	}
	elo := round1(b.elo)
	return &b.surface, &elo
}

// bestSurfaces reads the best surface of one page of players, inside the same
// window the leaderboard uses.
func (a *API) bestSurfaces(r *http.Request, asOf time.Time, ids []int64) (map[int64]*bestSurface, error) {
	out := map[int64]*bestSurface{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := a.Queries.BestSurfaceEloAsOf(r.Context(), db.BestSurfaceEloAsOfParams{
		OnDate: asOf, Since: asOf.Add(-activeWindow), PlayerIds: ids,
	})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.PlayerID] = &bestSurface{surface: string(row.Surface), elo: row.Elo}
	}
	return out, nil
}

// currentElo reads the rating one page of players held at a date.
func (a *API) currentElo(
	r *http.Request, surface db.RatingSurface, asOf time.Time, ids []int64,
) (map[int64]float64, error) {
	out := map[int64]float64{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := a.Queries.CurrentEloAsOf(r.Context(), db.CurrentEloAsOfParams{
		Surface: surface, OnDate: asOf, PlayerIds: ids,
	})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.PlayerID] = row.Elo
	}
	return out, nil
}

// ageAt is whole years at a date, which is how an age is quoted.
func ageAt(birth *time.Time, on time.Time) *int {
	if birth == nil {
		return nil
	}
	age := on.Year() - birth.Year()
	if on.YearDay() < birth.YearDay() {
		age--
	}
	if age < 0 || age > 120 {
		return nil
	}
	return &age
}

func parsePositiveInt(raw string, def, max int) (int, error) {
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > max {
		return 0, errBadCount
	}
	return n, nil
}
