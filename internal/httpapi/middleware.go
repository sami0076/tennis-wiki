package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"
)

type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota
	ctxKeyLogger
)

// RequestIDHeader is both read and written: an upstream proxy that already
// assigned one should not have it replaced, or the two logs cannot be joined.
const RequestIDHeader = "X-Request-Id"

// RequestIDFrom returns the id assigned to this request, or "" outside one.
func RequestIDFrom(ctx context.Context) string {
	id, ok := ctx.Value(ctxKeyRequestID).(string)
	if !ok {
		return ""
	}
	return id
}

// LoggerFrom returns the request-scoped logger, falling back to the default so
// callers never have to nil-check.
func LoggerFrom(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKeyLogger).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// RequestID assigns an id, echoes it back, and puts it in the context so every
// log line and every problem document can carry it.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" || len(id) > 64 {
			id = newRequestID()
		}
		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKeyRequestID, id)))
	})
}

func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand does not fail in practice; a timestamp still correlates
		// a log line with a response, which is all this is for.
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b[:])
}

// statusRecorder captures what was written so the log line can report it.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

// Logging emits one structured line per request and puts a logger already
// tagged with the request id into the context.
func Logging(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			log := base.With(slog.String("request_id", RequestIDFrom(r.Context())))
			ctx := context.WithValue(r.Context(), ctxKeyLogger, log)

			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r.WithContext(ctx))

			if rec.status == 0 {
				rec.status = http.StatusOK
			}
			log.LogAttrs(ctx, levelFor(rec.status), "request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Int("bytes", rec.bytes),
				slog.Duration("duration", time.Since(started)),
			)
		})
	}
}

// levelFor keeps client mistakes out of the error log; only our own failures
// are worth waking someone for.
func levelFor(status int) slog.Level {
	switch {
	case status >= 500:
		return slog.LevelError
	case status >= 400:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}

// Recover turns a panic into a problem document. Without it the connection is
// dropped with no body, which a client cannot distinguish from a network fault.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			// A panic after a partial write cannot be turned into a clean
			// response; dropping the connection at least signals the failure.
			if rec == http.ErrAbortHandler {
				panic(rec)
			}
			LoggerFrom(r.Context()).Error("panic serving request",
				slog.Any("panic", rec),
				slog.String("stack", string(debug.Stack())),
			)
			WriteProblem(w, r, http.StatusInternalServerError, TypeInternal,
				"The request could not be completed.")
		}()
		next.ServeHTTP(w, r)
	})
}

// CORS allows exactly the configured origins. A public read-only API is still
// no reason to reflect arbitrary origins back.
func CORS(origins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && slices.Contains(origins, origin) {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Content-Type, "+RequestIDHeader)
				h.Set("Access-Control-Max-Age", "86400")
				// The response varies by Origin, so a shared cache must not
				// serve one origin's response to another.
				h.Add("Vary", "Origin")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP identifies the caller for rate limiting. X-Forwarded-For is trivially
// spoofed, so it is only read when the deployment says a proxy sets it.
func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			if first, _, ok := strings.Cut(fwd, ","); ok {
				return strings.TrimSpace(first)
			}
			return strings.TrimSpace(fwd)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
