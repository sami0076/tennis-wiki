package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ProblemContentType is the media type RFC 7807 defines for error bodies.
const ProblemContentType = "application/problem+json"

// Problem is an RFC 7807 problem document. Every error this API returns is one
// of these, so a client needs exactly one error shape.
type Problem struct {
	// Type is a URI reference identifying the problem kind. Clients should
	// switch on this rather than on Title, which is for humans.
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Detail    string `json:"detail,omitempty"`
	Instance  string `json:"instance,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// Problem types. These are URI references, not URLs to fetch: RFC 7807 does not
// require them to resolve, and inventing a documentation host we do not own
// would be worse than a stable relative reference.
const (
	TypeNotFound      = "/problems/not-found"
	TypeBadRequest    = "/problems/bad-request"
	TypeRateLimited   = "/problems/rate-limited"
	TypeInternal      = "/problems/internal"
	TypeUnavailable   = "/problems/unavailable"
	TypeMethodInvalid = "/problems/method-not-allowed"
)

// titles keeps the human-readable half in one place so two handlers cannot
// describe the same condition differently.
var titles = map[string]string{
	TypeNotFound:      "Not found",
	TypeBadRequest:    "Invalid request",
	TypeRateLimited:   "Too many requests",
	TypeInternal:      "Internal server error",
	TypeUnavailable:   "Service unavailable",
	TypeMethodInvalid: "Method not allowed",
}

// WriteProblem writes an RFC 7807 response. detail is shown to the caller, so
// it must describe what they can change, never what went wrong internally.
func WriteProblem(w http.ResponseWriter, r *http.Request, status int, problemType, detail string) {
	title, ok := titles[problemType]
	if !ok {
		title = http.StatusText(status)
	}
	p := Problem{
		Type:      problemType,
		Title:     title,
		Status:    status,
		Detail:    detail,
		Instance:  r.URL.Path,
		RequestID: RequestIDFrom(r.Context()),
	}

	w.Header().Set("Content-Type", ProblemContentType)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(p); err != nil {
		// The status line is already sent, so this can only be logged.
		LoggerFrom(r.Context()).Error("writing problem document failed", slog.Any("error", err))
	}
}

// NotFound writes a 404 problem document.
func NotFound(w http.ResponseWriter, r *http.Request, detail string) {
	WriteProblem(w, r, http.StatusNotFound, TypeNotFound, detail)
}

// BadRequest writes a 400 problem document.
func BadRequest(w http.ResponseWriter, r *http.Request, detail string) {
	WriteProblem(w, r, http.StatusBadRequest, TypeBadRequest, detail)
}

// Internal logs the underlying error and writes a 500 problem document that
// does not leak it.
func Internal(w http.ResponseWriter, r *http.Request, err error) {
	LoggerFrom(r.Context()).Error("request failed", slog.Any("error", err))
	WriteProblem(w, r, http.StatusInternalServerError, TypeInternal,
		"The request could not be completed.")
}
