package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// API carries what every handler needs.
type API struct {
	Queries *db.Queries
	Log     *slog.Logger
	Config  Config
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

		v1.Group(func(public chi.Router) {
			public.Use(NewRateLimiter(a.Config.RateLimitPerMin).Middleware(a.Config.TrustProxy))
			public.Use(ETag)
			a.routes(public)
		})
	})
	return r
}

// routes registers the data endpoints. Kept separate so the middleware stack
// above stays readable as endpoints are added.
func (a *API) routes(r chi.Router) {
	_ = r // filled in by the player endpoints in #12
}

// writeJSON sends a successful response. Errors go through WriteProblem instead.
func writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		LoggerFrom(r.Context()).Error("writing response failed", slog.Any("error", err))
	}
}
