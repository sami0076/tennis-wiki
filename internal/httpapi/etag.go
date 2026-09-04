package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
)

// ETag adds revalidation to safe requests. Historical match data barely changes,
// so most repeat requests can be answered with an empty 304 rather than the
// body again.
//
// The body is buffered to hash it. That is affordable because every response
// here is a bounded JSON document, and it must not be applied to anything
// streamed.
func ETag(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}

		buf := &etagWriter{ResponseWriter: w, body: &bytes.Buffer{}}
		next.ServeHTTP(buf, r)

		// Only a plain 200 is safely cacheable this way. Anything else, and
		// the buffered body is passed straight through.
		if buf.status != 0 && buf.status != http.StatusOK {
			buf.flush(r)
			return
		}

		sum := sha256.Sum256(buf.body.Bytes())
		tag := `"` + hex.EncodeToString(sum[:16]) + `"`
		w.Header().Set("ETag", tag)

		if matches(r.Header.Get("If-None-Match"), tag) {
			// RFC 9110: a 304 carries no body and no Content-Length.
			w.Header().Del("Content-Length")
			w.WriteHeader(http.StatusNotModified)
			return
		}
		buf.flush(r)
	})
}

// matches reports whether an If-None-Match header covers tag. The weak-validator
// prefix is ignored: these tags are only ever compared to themselves.
func matches(header, tag string) bool {
	if header == "" {
		return false
	}
	if strings.TrimSpace(header) == "*" {
		return true
	}
	for _, candidate := range strings.Split(header, ",") {
		if strings.TrimPrefix(strings.TrimSpace(candidate), "W/") == tag {
			return true
		}
	}
	return false
}

// etagWriter holds the response until its hash is known.
type etagWriter struct {
	http.ResponseWriter
	body    *bytes.Buffer
	status  int
	flushed bool
}

func (e *etagWriter) WriteHeader(code int) { e.status = code }

func (e *etagWriter) Write(b []byte) (int, error) { return e.body.Write(b) }

func (e *etagWriter) flush(r *http.Request) {
	if e.flushed {
		return
	}
	e.flushed = true
	if e.status == 0 {
		e.status = http.StatusOK
	}
	e.ResponseWriter.WriteHeader(e.status)
	if _, err := e.ResponseWriter.Write(e.body.Bytes()); err != nil {
		// The status line is already gone; the client hung up mid-write.
		LoggerFrom(r.Context()).Warn("writing buffered response failed", slog.Any("error", err))
	}
}
