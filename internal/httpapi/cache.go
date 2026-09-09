package httpapi

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/sami0076/tennis-wiki/internal/cache"
)

// Cached answers a GET from Redis when it can, and fills the cache when it
// cannot.
//
// Only a plain 200 is stored. A problem document is cheap to regenerate and
// caching one would keep a transient failure alive long after the cause is
// gone.
//
// Every response says which it was in X-Cache, so the hit rate is visible per
// request as well as in aggregate on the health endpoint.
func Cached(c *cache.Cache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !c.Enabled() || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
				next.ServeHTTP(w, r)
				return
			}

			key := cacheKey(r)
			if body, ok := c.Get(r.Context(), key); ok {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Cache", "hit")
				if _, err := w.Write(body); err != nil {
					LoggerFrom(r.Context()).Warn("writing cached response failed",
						slog.Any("error", err))
				}
				return
			}

			w.Header().Set("X-Cache", "miss")
			rec := &cacheWriter{ResponseWriter: w, body: &bytes.Buffer{}}
			next.ServeHTTP(rec, r)

			if rec.status == 0 || rec.status == http.StatusOK {
				c.Set(r.Context(), key, rec.body.Bytes())
			}
		})
	}
}

// cacheKey identifies a response by what was asked for, not by the URL string.
// Sorting the query means ?tour=atp&surface=clay and ?surface=clay&tour=atp are
// one entry rather than two, and dropping empties means ?tour= is the same
// request as leaving it out -- which is what the handlers already treat it as.
func cacheKey(r *http.Request) string {
	var b strings.Builder
	b.WriteString(r.URL.Path)

	params := r.URL.Query()
	keys := make([]string, 0, len(params))
	for name, values := range params {
		if len(values) > 0 && values[0] != "" {
			keys = append(keys, name)
		}
	}
	if len(keys) == 0 {
		return b.String()
	}

	sort.Strings(keys)
	b.WriteByte('?')
	for i, name := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(url.QueryEscape(name))
		b.WriteByte('=')
		b.WriteString(url.QueryEscape(params.Get(name)))
	}
	return b.String()
}

// cacheWriter passes the response through while keeping a copy to store.
//
// It holds nothing back of its own. This sits inside the ETag middleware, which
// is already buffering the body to hash it, and a second buffer that delayed
// the status would leave ETag with nothing to work from.
type cacheWriter struct {
	http.ResponseWriter
	body   *bytes.Buffer
	status int
}

func (c *cacheWriter) WriteHeader(code int) {
	c.status = code
	c.ResponseWriter.WriteHeader(code)
}

func (c *cacheWriter) Write(b []byte) (int, error) {
	c.body.Write(b)
	return c.ResponseWriter.Write(b)
}
