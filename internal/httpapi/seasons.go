package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// SeasonsResponse is the calendar: one row per year, both tours on it.
type SeasonsResponse struct {
	Data []SeasonRow `json:"data"`
	// CurrentThrough is the last match per tour, the date the current
	// season is complete to. A row marked partial is partial to here.
	CurrentThrough map[string]string `json:"current_through"`
}

// SeasonRow is one year. A tour that played nothing that year is null: the
// women's tour reaches back to 1923 and the men's files start in 1968.
type SeasonRow struct {
	Season int16       `json:"season"`
	ATP    *SeasonTour `json:"atp"`
	WTA    *SeasonTour `json:"wta"`
}

// SeasonTour is one tour's year: the calendar's size and shape, and the
// four names that summarise it.
type SeasonTour struct {
	Events   int           `json:"events"`
	Ties     int           `json:"ties"`
	Surfaces SurfaceCounts `json:"surfaces"`
	Slams    []SeasonSlam  `json:"slams"`
	// Partial is true when this is the season the tour's coverage ends in,
	// so the row is a year in progress rather than a small one.
	Partial bool `json:"partial"`
}

// SurfaceCounts is how many events a year's calendar held on each surface.
// Unknown is events whose surface the file did not record, kept apart
// rather than folded into any of the four.
type SurfaceCounts struct {
	Hard    int `json:"hard"`
	Clay    int `json:"clay"`
	Grass   int `json:"grass"`
	Carpet  int `json:"carpet"`
	Unknown int `json:"unknown"`
}

// SeasonSlam is one Grand Slam edition and its final.
type SeasonSlam struct {
	Slug       string    `json:"slug"`
	Name       string    `json:"name"`
	StartDate  string    `json:"start_date"`
	Champion   *Opponent `json:"champion"`
	Finalist   *Opponent `json:"finalist"`
	FinalScore *string   `json:"final_score"`
}

// SeasonEventsResponse is one year's calendar at one tier.
type SeasonEventsResponse struct {
	Season int16   `json:"season"`
	Tour   *string `json:"tour"`
	Tier   string  `json:"tier"`
	// Partial names the tours whose coverage ends inside this season, with
	// the date each is complete to.
	Partial map[string]string `json:"partial"`
	Events  []SeasonEvent     `json:"events"`
}

// SeasonEvent is one tournament of the year. A team competition's ties are
// folded into one row with their count.
type SeasonEvent struct {
	// Slug is the event's, and null for a row the events stage has not keyed.
	Slug       *string   `json:"slug"`
	Name       string    `json:"name"`
	Tour       string    `json:"tour"`
	Category   string    `json:"category"`
	Level      string    `json:"level"`
	Tier       string    `json:"tier"`
	Surface    *string   `json:"surface"`
	DrawSize   *int16    `json:"draw_size"`
	StartDate  string    `json:"start_date"`
	Champion   *Opponent `json:"champion"`
	Finalist   *Opponent `json:"finalist"`
	FinalScore *string   `json:"final_score"`
	Matches    int       `json:"matches"`
	Ties       int       `json:"ties"`
}

func (a *API) handleSeasons(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	summaries, err := a.Queries.ListSeasonSummaries(ctx)
	if err != nil {
		Internal(w, r, err)
		return
	}
	slams, err := a.Queries.ListSlamFinals(ctx)
	if err != nil {
		Internal(w, r, err)
		return
	}
	through, err := a.currentThrough(r)
	if err != nil {
		Internal(w, r, err)
		return
	}

	slamsBy := map[int16]map[string][]SeasonSlam{}
	for _, s := range slams {
		if slamsBy[s.Season] == nil {
			slamsBy[s.Season] = map[string][]SeasonSlam{}
		}
		slam := SeasonSlam{Slug: s.Slug, Name: s.Name, StartDate: s.StartDate.Format(time.DateOnly), FinalScore: s.FinalScore}
		if s.ChampionSlug != nil && s.ChampionName != nil {
			slam.Champion = &Opponent{Slug: *s.ChampionSlug, Name: *s.ChampionName}
		}
		if s.FinalistSlug != nil && s.FinalistName != nil {
			slam.Finalist = &Opponent{Slug: *s.FinalistSlug, Name: *s.FinalistName}
		}
		slamsBy[s.Season][s.Tour] = append(slamsBy[s.Season][s.Tour], slam)
	}

	out := SeasonsResponse{Data: []SeasonRow{}, CurrentThrough: through}
	for _, s := range summaries {
		tour := &SeasonTour{
			Events: int(s.Events), Ties: int(s.Ties),
			Surfaces: SurfaceCounts{Hard: int(s.Hard), Clay: int(s.Clay), Grass: int(s.Grass), Carpet: int(s.Carpet), Unknown: int(s.Unknown)},
			Slams:    []SeasonSlam{},
			Partial:  partialSeason(through[s.Tour], s.Season),
		}
		if list := slamsBy[s.Season][s.Tour]; list != nil {
			tour.Slams = list
		}
		if n := len(out.Data); n == 0 || out.Data[n-1].Season != s.Season {
			out.Data = append(out.Data, SeasonRow{Season: s.Season})
		}
		row := &out.Data[len(out.Data)-1]
		if s.Tour == string(db.TourAtp) {
			row.ATP = tour
		} else {
			row.WTA = tour
		}
	}
	writeJSON(w, r, http.StatusOK, out)
}

func (a *API) handleSeasonEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	season, err := strconv.Atoi(chi.URLParam(r, "year"))
	if err != nil || season < firstSeason || season > lastSeason {
		BadRequest(w, r, "The year must be a four-digit season, for example 2019.")
		return
	}
	query := r.URL.Query()
	params := db.ListSeasonEventsParams{Season: int16(season), Tier: db.TierTour}
	var tourName *string
	switch raw := query.Get("tour"); raw {
	case "":
	case string(db.TourAtp), string(db.TourWta):
		tour := db.Tour(raw)
		params.Tour = &tour
		tourName = &raw
	default:
		BadRequest(w, r, "tour must be atp or wta.")
		return
	}
	switch raw := query.Get("tier"); raw {
	case "":
	case string(db.TierTour), string(db.TierChallenger), string(db.TierFutures), string(db.TierItf):
		params.Tier = db.Tier(raw)
	default:
		BadRequest(w, r, "tier must be tour, challenger, futures or itf.")
		return
	}

	rows, err := a.Queries.ListSeasonEvents(ctx, params)
	if err != nil {
		Internal(w, r, err)
		return
	}
	through, err := a.currentThrough(r)
	if err != nil {
		Internal(w, r, err)
		return
	}

	out := SeasonEventsResponse{
		Season: int16(season), Tour: tourName, Tier: string(params.Tier),
		Partial: map[string]string{}, Events: make([]SeasonEvent, 0, len(rows)),
	}
	for tour, last := range through {
		if partialSeason(last, int16(season)) {
			out.Partial[tour] = last
		}
	}
	// A team competition's ties, one row each in the file, fold into one row
	// per competition, the way the event page folds them into a season.
	ties := map[string]int{}
	for _, row := range rows {
		if link(row.EventLink) == "team" && row.EventSlug != nil {
			if at, ok := ties[*row.EventSlug]; ok {
				out.Events[at].Ties++
				out.Events[at].Matches += int(row.Matches)
				continue
			}
			ties[*row.EventSlug] = len(out.Events)
		}
		ev := SeasonEvent{
			Slug: row.EventSlug, Name: row.Name, Tour: row.Tour, Category: row.Category,
			Level: row.Level, Tier: row.Tier, Surface: nonEmpty(row.Surface), DrawSize: row.DrawSize,
			StartDate: row.StartDate.Format(time.DateOnly), FinalScore: row.FinalScore, Matches: int(row.Matches),
		}
		if row.ChampionSlug != nil && row.ChampionName != nil {
			ev.Champion = &Opponent{Slug: *row.ChampionSlug, Name: *row.ChampionName}
		}
		if row.FinalistSlug != nil && row.FinalistName != nil {
			ev.Finalist = &Opponent{Slug: *row.FinalistSlug, Name: *row.FinalistName}
		}
		if link(row.EventLink) == "team" {
			ev.Ties = 1
			if row.EventName != nil {
				ev.Name = *row.EventName
			}
		}
		out.Events = append(out.Events, ev)
	}
	writeJSON(w, r, http.StatusOK, out)
}

// currentThrough is the last match per tour, the same figure /coverage
// reports, so a season row and the coverage page cannot disagree. A tour with
// no match at all is left out.
func (a *API) currentThrough(r *http.Request) (map[string]string, error) {
	rows, err := a.Queries.GetCurrentThrough(r.Context())
	if err != nil {
		return nil, err
	}
	through := map[string]string{}
	for _, row := range rows {
		if !row.LastMatch.IsZero() {
			through[row.Tour] = row.LastMatch.Format(time.DateOnly)
		}
	}
	return through, nil
}

// partialSeason: a season is in progress when the tour's last match falls
// inside it. Every earlier season is complete as far as the file goes.
func partialSeason(through string, season int16) bool {
	return len(through) >= 4 && through[:4] == strconv.Itoa(int(season))
}
