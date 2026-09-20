package httpapi

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// LeaderStat describes one board: what the value is a share of, and which
// matches it stands on.
type LeaderStat struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	// Family says where the inputs come from: serve (the player's own serve
	// lines), return (the opponents'), points (both), or score (the score
	// string, which nearly every finished match has).
	Family string `json:"family"`
	// Kind is rate for a share of a count, shown as a percentage, and ratio
	// for the dominance ratio, which is a quotient of two rates.
	Kind string `json:"kind"`
	// Sample names what the denominator counts, for the caption.
	Sample string `json:"sample"`
	// Ascending is true where less is better, and the board leads with the least.
	Ascending bool `json:"ascending"`
}

// Leaderboard is one stat, filtered, with its population declared.
type Leaderboard struct {
	Stat    LeaderStat    `json:"stat"`
	Filters LeaderFilters `json:"filters"`
	// Population is what the board is a board of, so the page can say "of
	// 2,140 clay matches in 2019, 1,760 recorded serve statistics".
	Population LeaderPopulation `json:"population"`
	Data       []LeaderRow      `json:"data"`
	Stats      []LeaderStat     `json:"stats"`
	// Player is the row of the player named by ?player=, whether or not they
	// cleared the floor, so a page can say why a name is not on the board:
	// null when they have no figure at all under this filter. Position is 0.
	Player *LeaderRow `json:"player"`
}

// LeaderFilters echoes what was asked, defaults applied.
type LeaderFilters struct {
	Tour       *string `json:"tour"`
	Tier       *string `json:"tier"`
	Surface    *string `json:"surface"`
	Season     *int    `json:"season"`
	MinMatches int     `json:"min_matches"`
	Limit      int     `json:"limit"`
}

// LeaderPopulation counts the matches under the filter, how many of them the
// source recorded statistics for, how many players played any, and how many
// cleared the minimum for this board.
type LeaderPopulation struct {
	Matches   int64 `json:"matches"`
	WithStats int64 `json:"with_stats"`
	Players   int64 `json:"players"`
	Qualified int64 `json:"qualified"`
}

// LeaderRow is one player on the board. Sample is the matches the value was
// computed from, which is never optional; Numerator and Denominator are the
// counts behind a rate, null for the dominance ratio, which has none.
type LeaderRow struct {
	Position    int     `json:"position"`
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Tour        string  `json:"tour"`
	Country     *string `json:"country"`
	Sample      int64   `json:"sample"`
	Matches     int64   `json:"matches"`
	Numerator   *int64  `json:"numerator"`
	Denominator *int64  `json:"denominator"`
	Value       float64 `json:"value"`
}

// leaderStats is the catalogue, in the order the page lists it. Each key is
// a branch of the CASE in ListLeaders, and the two must agree.
var leaderStats = []LeaderStat{
	{Key: "aces", Label: "Aces", Family: "serve", Kind: "rate", Sample: "service points"},
	{Key: "double_faults", Label: "Double faults", Family: "serve", Kind: "rate", Sample: "service points", Ascending: true},
	{Key: "first_serve_in", Label: "First serves in", Family: "serve", Kind: "rate", Sample: "service points"},
	{Key: "first_serve_won", Label: "First-serve points won", Family: "serve", Kind: "rate", Sample: "first serves in"},
	{Key: "second_serve_won", Label: "Second-serve points won", Family: "serve", Kind: "rate", Sample: "second serves"},
	{Key: "serve_points_won", Label: "Service points won", Family: "serve", Kind: "rate", Sample: "service points"},
	{Key: "break_points_saved", Label: "Break points saved", Family: "serve", Kind: "rate", Sample: "break points faced"},
	{Key: "service_games_held", Label: "Service games held", Family: "serve", Kind: "rate", Sample: "service games"},
	{Key: "return_points_won", Label: "Return points won", Family: "return", Kind: "rate", Sample: "return points"},
	{Key: "first_return_won", Label: "First-serve return points won", Family: "return", Kind: "rate", Sample: "first-serve returns"},
	{Key: "second_return_won", Label: "Second-serve return points won", Family: "return", Kind: "rate", Sample: "second-serve returns"},
	{Key: "break_points_won", Label: "Break points converted", Family: "return", Kind: "rate", Sample: "break points"},
	{Key: "return_games_won", Label: "Return games won", Family: "return", Kind: "rate", Sample: "return games"},
	{Key: "points_won", Label: "Total points won", Family: "points", Kind: "rate", Sample: "points"},
	{Key: "dominance", Label: "Dominance ratio", Family: "points", Kind: "ratio", Sample: "matches with both serve lines"},
	{Key: "matches_won", Label: "Matches won", Family: "score", Kind: "rate", Sample: "matches"},
	{Key: "sets_won", Label: "Sets won", Family: "score", Kind: "rate", Sample: "sets"},
	{Key: "games_won", Label: "Games won", Family: "score", Kind: "rate", Sample: "games"},
	{Key: "tiebreaks_won", Label: "Tiebreaks won", Family: "score", Kind: "rate", Sample: "tiebreaks"},
	{Key: "deciding_sets_won", Label: "Deciding sets won", Family: "score", Kind: "rate", Sample: "deciding sets"},
}

// leadersMinMatches is the default floor on a board, measured on the full
// database rather than guessed. At a floor of 5 the all-time first-serve
// board is led by five-match players and a season's by two-match ones; at
// 10 the leaders stand on 113 matches all-time and 15 in a season, and the
// top ten stops changing as the floor rises further. It is also the largest
// floor that leaves a season on one surface with a board: 48 players had
// ten grass matches with statistics in 2019, and none had twenty.
const (
	leadersMinMatches   = 10
	leadersDefaultLimit = 100
	leadersMaxLimit     = 200
)

func (a *API) handleLeaders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := chi.URLParam(r, "stat")
	var stat *LeaderStat
	for i := range leaderStats {
		if leaderStats[i].Key == key {
			stat = &leaderStats[i]
		}
	}
	if stat == nil {
		BadRequest(w, r, "No such stat. The response of any board lists them all under \"stats\".")
		return
	}

	query := r.URL.Query()
	params := db.ListLeadersParams{Stat: key, Ascending: stat.Ascending, MinMatches: leadersMinMatches}
	population := db.CountLeaderPopulationParams{}
	filters := LeaderFilters{MinMatches: leadersMinMatches}

	switch raw := query.Get("tour"); raw {
	case "":
	case string(db.TourAtp), string(db.TourWta):
		tour := db.Tour(raw)
		params.Tour, population.Tour, filters.Tour = &tour, &tour, &raw
	default:
		BadRequest(w, r, "tour must be atp or wta.")
		return
	}
	switch raw := query.Get("tier"); raw {
	case "":
	case string(db.TierTour), string(db.TierChallenger), string(db.TierFutures), string(db.TierItf):
		tier := db.Tier(raw)
		params.Tier, population.Tier, filters.Tier = &tier, &tier, &raw
	default:
		BadRequest(w, r, "tier must be tour, challenger, futures or itf.")
		return
	}
	switch raw := query.Get("surface"); raw {
	case "":
	case string(db.SurfaceHard), string(db.SurfaceClay), string(db.SurfaceGrass), string(db.SurfaceCarpet):
		surface := db.Surface(raw)
		params.Surface, population.Surface, filters.Surface = &surface, &surface, &raw
	default:
		BadRequest(w, r, "surface must be hard, clay, grass or carpet.")
		return
	}
	if raw := query.Get("season"); raw != "" {
		season, err := strconv.Atoi(raw)
		if err != nil || season < firstSeason || season > lastSeason {
			BadRequest(w, r, "season must be a four-digit year.")
			return
		}
		s := int16(season)
		params.Season, population.Season, filters.Season = &s, &s, &season
	}
	if raw := query.Get("min_matches"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 10000 {
			BadRequest(w, r, "min_matches must be between 1 and 10000.")
			return
		}
		params.MinMatches, filters.MinMatches = int64(n), n
	}
	limit, err := Limit(r, leadersDefaultLimit, leadersMaxLimit)
	if err != nil {
		BadRequest(w, r, err.Error())
		return
	}
	params.RowLimit, filters.Limit = int32(limit), limit
	player := query.Get("player")

	rows, err := a.Queries.ListLeaders(ctx, params)
	if err != nil {
		Internal(w, r, err)
		return
	}
	counts, err := a.Queries.CountLeaderPopulation(ctx, population)
	if err != nil {
		Internal(w, r, err)
		return
	}

	out := Leaderboard{
		Stat: *stat, Filters: filters,
		Population: LeaderPopulation{Matches: int64(counts.Matches), WithStats: int64(counts.WithStats), Players: counts.Players},
		Data:       make([]LeaderRow, 0, len(rows)),
		Stats:      leaderStats,
	}
	for i, row := range rows {
		out.Population.Qualified = row.Qualified
		out.Data = append(out.Data, leaderRow(row, i+1, stat))
	}

	if player != "" {
		// The one player's figure under the same filter, floor lowered to
		// one so a sample below it still comes back with its count.
		one := params
		one.Player, one.MinMatches, one.RowLimit = &player, 1, 1
		theirs, err := a.Queries.ListLeaders(ctx, one)
		if err != nil {
			Internal(w, r, err)
			return
		}
		if len(theirs) == 1 {
			row := leaderRow(theirs[0], 0, stat)
			out.Player = &row
		}
	}
	writeJSON(w, r, http.StatusOK, out)
}

func leaderRow(row db.ListLeadersRow, position int, stat *LeaderStat) LeaderRow {
	lr := LeaderRow{
		Position: position, Slug: row.Slug, Name: row.Name, Tour: row.Tour, Country: row.Country,
		Sample: row.Sample, Matches: row.Matches, Value: row.Value,
	}
	if stat.Kind == "rate" {
		n, d := row.Numerator, row.Denominator
		lr.Numerator, lr.Denominator = &n, &d
	}
	return lr
}
