package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/sami0076/tennis-wiki/internal/db"
)

const (
	drawsDefaultLimit = 120
	drawsMaxLimit     = 400
)

// ReplayableDraws is the list a reader picks a draw from.
//
// It exists because the simulator could only ever replay the draw named in the
// URL, and nothing on the page told anyone a URL was involved. Landing on
// /simulator meant the featured draw forever.
type ReplayableDraws struct {
	Filters DrawFilters      `json:"filters"`
	Data    []ReplayableDraw `json:"data"`
}

// DrawFilters echoes the cut, defaults applied.
type DrawFilters struct {
	Tour   *string `json:"tour"`
	Season *int    `json:"season"`
	Limit  int     `json:"limit"`
}

// ReplayableDraw is one draw, addressed the way the simulator addresses one.
type ReplayableDraw struct {
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Season int16  `json:"season"`
	Tour   string `json:"tour"`
	Tier   string `json:"tier"`
	Level  string `json:"level"`
	// Surface is "unknown" where the source recorded none, the same stand-in
	// the rest of the API uses rather than a second spelling for absent.
	Surface string `json:"surface"`
	// DrawSize is the source's own figure and is null where it recorded none.
	// Matches is always known, and is what the list falls back to describing.
	DrawSize  *int16 `json:"draw_size"`
	Matches   int64  `json:"matches"`
	StartDate string `json:"start_date"`
}

// handleReplayableDraws lists the draws the simulator can be pointed at.
//
// The list is the same first cut the query documents: a knockout ending in one
// final. A draw here can still be declined at simulation time by the
// reconstruction, which finds byes and partially recorded rounds one event at
// a time, so this promises an address worth trying rather than a guarantee.
func (a *API) handleReplayableDraws(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	params := db.ListReplayableDrawsParams{}
	filters := DrawFilters{}

	switch raw := query.Get("tour"); raw {
	case "":
	case string(db.TourAtp), string(db.TourWta):
		tour := db.Tour(raw)
		params.Tour = &tour
		filters.Tour = &raw
	default:
		BadRequest(w, r, "tour must be atp or wta.")
		return
	}

	if raw := query.Get("season"); raw != "" {
		season, err := strconv.Atoi(raw)
		if err != nil || season < firstSeason || season > lastSeason {
			BadRequest(w, r, "season must be a four-digit year.")
			return
		}
		year := int16(season)
		params.Season = &year
		filters.Season = &season
	}

	limit, err := Limit(r, drawsDefaultLimit, drawsMaxLimit)
	if err != nil {
		BadRequest(w, r, err.Error())
		return
	}
	params.RowLimit = int32(limit)
	filters.Limit = limit

	rows, err := a.Queries.ListReplayableDraws(ctx, params)
	if err != nil {
		Internal(w, r, err)
		return
	}

	out := ReplayableDraws{Filters: filters, Data: make([]ReplayableDraw, 0, len(rows))}
	for _, row := range rows {
		out.Data = append(out.Data, ReplayableDraw{
			Slug:      row.Slug,
			Name:      row.Name,
			Season:    row.Season,
			Tour:      row.Tour,
			Tier:      row.Tier,
			Level:     row.Level,
			Surface:   row.Surface,
			DrawSize:  row.DrawSize,
			Matches:   row.Matches,
			StartDate: row.StartDate.Format(time.DateOnly),
		})
	}

	writeJSON(w, r, http.StatusOK, out)
}
