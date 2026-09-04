package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// decodeProblem fails the test unless the response is a well-formed RFC 7807
// document with the expected status.
func decodeProblem(t *testing.T, res *http.Response, wantStatus int) Problem {
	t.Helper()
	if res.StatusCode != wantStatus {
		t.Errorf("status = %d, want %d", res.StatusCode, wantStatus)
	}
	if ct := res.Header.Get("Content-Type"); ct != ProblemContentType {
		t.Errorf("content-type = %q, want %q", ct, ProblemContentType)
	}
	var p Problem
	if err := json.NewDecoder(res.Body).Decode(&p); err != nil {
		t.Fatalf("body is not a problem document: %v", err)
	}
	if p.Title == "" || p.Type == "" {
		t.Errorf("problem is missing type or title: %+v", p)
	}
	if p.Status != wantStatus {
		t.Errorf("problem.status = %d, want %d", p.Status, wantStatus)
	}
	return p
}

func TestRequestIDIsAssignedAndEchoed(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFrom(r.Context())
	}))

	t.Run("generated when absent", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if seen == "" {
			t.Error("handler saw no request id in its context")
		}
		if got := rec.Header().Get(RequestIDHeader); got != seen {
			t.Errorf("response header %q, context %q; they must match", got, seen)
		}
	})

	t.Run("an upstream id is kept", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(RequestIDHeader, "from-the-proxy")
		h.ServeHTTP(httptest.NewRecorder(), req)
		if seen != "from-the-proxy" {
			t.Errorf("request id = %q; an upstream id must not be replaced", seen)
		}
	})

	t.Run("an absurd id is replaced", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(RequestIDHeader, string(make([]byte, 200)))
		h.ServeHTTP(httptest.NewRecorder(), req)
		if len(seen) > 64 {
			t.Errorf("request id of length %d was accepted", len(seen))
		}
	})
}

func TestRecoverReturnsAProblemDocument(t *testing.T) {
	h := RequestID(Logging(discardLogger())(Recover(http.HandlerFunc(
		func(http.ResponseWriter, *http.Request) {
			panic("handler exploded")
		}))))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

	p := decodeProblem(t, rec.Result(), http.StatusInternalServerError)
	if p.RequestID == "" {
		t.Error("a 500 must carry the request id, or it cannot be traced to a log line")
	}
	// The panic text must not reach the caller.
	if p.Detail == "handler exploded" {
		t.Error("panic message leaked into the response")
	}
}

func TestCORSAllowsOnlyConfiguredOrigins(t *testing.T) {
	const allowed = "https://tennis.example"
	h := CORS([]string{allowed})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	cases := []struct {
		name      string
		method    string
		origin    string
		wantAllow string
		wantCode  int
	}{
		{"allowed origin", http.MethodGet, allowed, allowed, http.StatusOK},
		{"other origin gets no header", http.MethodGet, "https://evil.example", "", http.StatusOK},
		{"no origin header", http.MethodGet, "", "", http.StatusOK},
		{"preflight", http.MethodOptions, allowed, allowed, http.StatusNoContent},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(c.method, "/api/v1/health", nil)
			if c.origin != "" {
				req.Header.Set("Origin", c.origin)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != c.wantAllow {
				t.Errorf("allow-origin = %q, want %q", got, c.wantAllow)
			}
			if rec.Code != c.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, c.wantCode)
			}
		})
	}
}

func TestETagRevalidation(t *testing.T) {
	body := `{"player":"one"}`
	h := ETag(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, body)
	}))

	first := httptest.NewRecorder()
	h.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/api/v1/players/one", nil))

	tag := first.Header().Get("ETag")
	if tag == "" {
		t.Fatal("no ETag on the first response")
	}
	if first.Body.String() != body {
		t.Errorf("first response body = %q, want %q", first.Body.String(), body)
	}

	t.Run("matching validator gets 304", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/players/one", nil)
		req.Header.Set("If-None-Match", tag)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotModified {
			t.Errorf("status = %d, want 304", rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("304 carried a body of %d bytes", rec.Body.Len())
		}
	})

	t.Run("stale validator gets the body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/players/one", nil)
		req.Header.Set("If-None-Match", `"0000"`)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK || rec.Body.String() != body {
			t.Errorf("stale validator returned %d %q", rec.Code, rec.Body.String())
		}
	})

	t.Run("errors are not tagged", func(t *testing.T) {
		errH := ETag(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, "{}")
		}))
		rec := httptest.NewRecorder()
		errH.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
		if rec.Header().Get("ETag") != "" {
			t.Error("a 404 must not be given a cache validator")
		}
	})
}

func TestRateLimiterRefillsOverTime(t *testing.T) {
	now := time.Now()
	l := NewRateLimiter(60) // one per second, burst of 60
	l.now = func() time.Time { return now }

	for i := 0; i < 60; i++ {
		if ok, _ := l.Allow("1.2.3.4"); !ok {
			t.Fatalf("request %d rejected inside the burst", i+1)
		}
	}

	ok, wait := l.Allow("1.2.3.4")
	if ok {
		t.Fatal("the 61st request in one instant should be rejected")
	}
	if wait <= 0 {
		t.Errorf("Retry-After of %v tells the caller nothing", wait)
	}

	// A different caller has their own allowance.
	if ok, _ := l.Allow("5.6.7.8"); !ok {
		t.Error("one client exhausting its bucket must not affect another")
	}

	now = now.Add(wait)
	if ok, _ := l.Allow("1.2.3.4"); !ok {
		t.Errorf("still rejected after waiting the advertised %v", wait)
	}
}

func TestRateLimitMiddlewareReturns429(t *testing.T) {
	l := NewRateLimiter(1)
	h := RequestID(l.Middleware(false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/players", nil)
	req.RemoteAddr = "9.9.9.9:1234"

	first := httptest.NewRecorder()
	h.ServeHTTP(first, req)
	if first.Code != http.StatusOK {
		t.Fatalf("first request got %d", first.Code)
	}

	second := httptest.NewRecorder()
	h.ServeHTTP(second, req)

	res := second.Result()
	decodeProblem(t, res, http.StatusTooManyRequests)

	retry := res.Header.Get("Retry-After")
	if retry == "" {
		t.Fatal("429 without Retry-After")
	}
	if n, err := strconv.Atoi(retry); err != nil || n < 1 {
		t.Errorf("Retry-After = %q, want a positive number of seconds", retry)
	}
}

// X-Forwarded-For is only honoured where a proxy is known to set it, or the
// limit is bypassable by anyone willing to send the header.
func TestRateLimitIgnoresForwardedHeaderUnlessTrusted(t *testing.T) {
	cases := []struct {
		name       string
		trustProxy bool
		wantSecond int
	}{
		{"untrusted: spoofing does not help", false, http.StatusTooManyRequests},
		{"trusted: the forwarded ip is used", true, http.StatusOK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := NewRateLimiter(1)
			h := RequestID(l.Middleware(c.trustProxy)(http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })))

			for i, forwarded := range []string{"1.1.1.1", "2.2.2.2"} {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/players", nil)
				req.RemoteAddr = "9.9.9.9:1234"
				req.Header.Set("X-Forwarded-For", forwarded)
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)

				if i == 1 && rec.Code != c.wantSecond {
					t.Errorf("second request got %d, want %d", rec.Code, c.wantSecond)
				}
			}
		})
	}
}
