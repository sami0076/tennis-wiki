package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/sami0076/tennis-wiki/internal/cache"
	"github.com/sami0076/tennis-wiki/internal/db"
)

// API carries what every handler needs.
type API struct {
	Queries *db.Queries
	Log     *slog.Logger
	Config  Config
	// Cache is optional. A nil one is a disabled one, which is the same thing
	// as a Redis that is down -- and both have to be, or the outage path is
	// code nothing ever runs.
	Cache *cache.Cache
}

// New builds the API. Cross-cutting behaviour lives in middleware so it is
// uniform across handlers rather than remembered in each one.
func New(queries *db.Queries, log *slog.Logger, cfg Config) *API {
	return &API{Queries: queries, Log: log, Config: cfg}
}

// Router returns the versioned handler tree.
func (a *API) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(RequestID)
	r.Use(Logging(a.Log))
	r.Use(Recover)
	r.Use(CORS(a.Config.CORSOrigins))

	// chi's own 404 and 405 return plain text, which would be the only two
	// responses in the API that are not problem documents.
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		NotFound(w, r, "No such endpoint.")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		WriteProblem(w, r, http.StatusMethodNotAllowed, TypeMethodInvalid,
			"That method is not supported on this endpoint.")
	})

	r.Route("/api/v1", func(v1 chi.Router) {
		// Probes are exempt from rate limiting and revalidation: a liveness
		// check that gets a 429 or a 304 reports the wrong thing.
		v1.Get("/health", a.handleHealth)
		v1.Get("/live", a.handleLive)

		v1.Group(func(public chi.Router) {
			public.Use(NewRateLimiter(a.Config.RateLimitPerMin).Middleware(a.Config.TrustProxy))
			public.Use(ETag)
			// Inside ETag, so a cached body is still revalidated and a repeat
			// request can be answered with an empty 304 rather than the bytes.
			public.Use(Cached(a.Cache))
			a.routes(public)
		})

		// Uncached: the hourly ongoing refresh would otherwise have to flush
		// the whole cache, and a few dozen rows cost nothing to read.
		v1.Group(func(fresh chi.Router) {
			fresh.Use(NewRateLimiter(a.Config.RateLimitPerMin).Middleware(a.Config.TrustProxy))
			fresh.Use(ETag)
			fresh.Get("/this-week", a.handleThisWeek)
		})
	})
	return r
}

// routes registers the data endpoints. Kept separate so the middleware stack
// above stays readable as endpoints are added.
func (a *API) routes(r chi.Router) {
	r.Get("/coverage", a.handleCoverage)
	r.Get("/players", a.handlePlayerSearch)
	r.Get("/players/{slug}", a.handlePlayer)
	r.Get("/players/{slug}/matches", a.handlePlayerMatches)
	r.Get("/players/{slug}/ratings", a.handlePlayerRatingSeries)
	r.Get("/players/{slug}/rankings", a.handlePlayerRankingHistory)
	r.Get("/players/{slug}/clutch", a.handlePlayerClutch)
	r.Get("/players/{slug}/percentiles", a.handlePlayerPercentiles)
	r.Get("/players/{slug}/seasons", a.handlePlayerSeasons)
	r.Get("/players/{slug}/highlights", a.handlePlayerHighlights)
	r.Get("/h2h/{slug}/{opponent}", a.handleHeadToHead)
	r.Get("/h2h/{slug}/{opponent}/common", a.handleCommonOpponents)
	r.Get("/charted/{id}", a.handleChartedMatch)
	r.Get("/tournaments", a.handleEvents)
	r.Get("/tournaments/{slug}", a.handleEvent)
	r.Get("/tournaments/{slug}/{season}", a.handleEdition)
	r.Get("/seasons", a.handleSeasons)
	r.Get("/seasons/{year}", a.handleSeasonEvents)
	r.Get("/recent", a.handleRecent)
	r.Get("/leaders/{stat}", a.handleLeaders)
	r.Get("/rankings", a.handleRankings)
	r.Get("/rankings/trajectory", a.handleTrajectories)
	r.Get("/simulate/match", a.handleSimulateMatch)
	r.Get("/simulate/draw", a.handleSimulateDraw)
	r.Get("/simulate/draws", a.handleReplayableDraws)
}

// writeJSON sends a successful response. Errors go through WriteProblem instead.
func writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		LoggerFrom(r.Context()).Error("writing response failed", slog.Any("error", err))
	}
}
