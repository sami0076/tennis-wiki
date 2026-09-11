package httpapi

import (
	"net/http"

	"github.com/sami0076/tennis-wiki/internal/cache"
)

// HealthResponse reports whether the process can actually serve requests.
type HealthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
	// Cache is reported, never judged. A cache that is down costs time, not
	// correctness, and a readiness probe that failed on it would take a healthy
	// process out of rotation for a slower one.
	Cache CacheHealth `json:"cache"`
}

// CacheHealth is the read cache's state and what it has been worth so far.
type CacheHealth struct {
	cache.Stats
	// Reachable is false when Redis is configured and not answering, which is
	// exactly when the hit rate below stops moving.
	Reachable bool `json:"reachable"`
}

// handleHealth round-trips a query rather than reporting on the socket. A pool
// that is connected but pointed at an unmigrated database is not healthy, and
// only a real query tells them apart.
func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	if _, err := a.Queries.Health(r.Context()); err != nil {
		LoggerFrom(r.Context()).Error("health check query failed", "error", err)
		WriteProblem(w, r, http.StatusServiceUnavailable, TypeUnavailable,
			"The database is not reachable.")
		return
	}
	writeJSON(w, r, http.StatusOK, HealthResponse{
		Status:   "ok",
		Database: "ok",
		Cache: CacheHealth{
			Stats:     a.Cache.Stats(),
			Reachable: a.Cache.Reachable(r.Context()),
		},
	})
}

// LiveResponse says only that the process is running and serving.
type LiveResponse struct {
	Status string `json:"status"`
}

// handleLive deliberately touches nothing. Liveness answers "should this
// process be killed", and the only honest answer from a stateless API is that
// it is up: a Postgres hiccup wired to liveness restarts every pod in the
// deployment at the moment the database can least afford the reconnections.
// Readiness is where a dependency belongs, and handleHealth is that.
func (a *API) handleLive(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, LiveResponse{Status: "ok"})
}
