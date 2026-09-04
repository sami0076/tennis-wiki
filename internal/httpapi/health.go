package httpapi

import (
	"net/http"
)

// HealthResponse reports whether the process can actually serve requests.
type HealthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
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
	writeJSON(w, r, http.StatusOK, HealthResponse{Status: "ok", Database: "ok"})
}
