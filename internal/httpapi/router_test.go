package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// fakeDB stands in for a connection pool so handler tests need no database and
// no network, per the testing rules in the build spec.
type fakeDB struct {
	err   error
	empty bool
}

func (f fakeDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, f.err
}

func (f fakeDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, f.err
}

func (f fakeDB) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	return fakeRow{err: f.err, ready: !f.empty}
}

type fakeRow struct {
	err   error
	ready bool
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) == 1 {
		if p, ok := dest[0].(*bool); ok {
			*p = r.ready
		}
	}
	return nil
}

func testAPI(t *testing.T, dbErr error) http.Handler {
	t.Helper()
	cfg := Config{
		CORSOrigins:     []string{"http://localhost:5173"},
		RateLimitPerMin: 600,
		ShutdownTimeout: time.Second,
	}
	return New(db.New(fakeDB{err: dbErr}), discardLogger(), cfg).Router()
}

func TestHealth(t *testing.T) {
	t.Run("round-trips a query", func(t *testing.T) {
		rec := httptest.NewRecorder()
		testAPI(t, nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var got HealthResponse
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.Status != "ok" || got.Database != "ok" {
			t.Errorf("got %+v", got)
		}
	})

	// A pool that is connected but pointed at an unmigrated database must not
	// report healthy, which is why the check runs a query rather than a ping.
	t.Run("a failing query is unhealthy", func(t *testing.T) {
		rec := httptest.NewRecorder()
		testAPI(t, errors.New("relation \"players\" does not exist")).
			ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))

		p := decodeProblem(t, rec.Result(), http.StatusServiceUnavailable)
		if p.Detail == "" {
			t.Error("an unhealthy response should say what is wrong")
		}
	})
}

// Every error path is a problem document, including the two chi answers by
// default in plain text.
func TestErrorsAreAlwaysProblemDocuments(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{"unknown endpoint", http.MethodGet, "/api/v1/nope", http.StatusNotFound},
		{"unknown version", http.MethodGet, "/api/v2/health", http.StatusNotFound},
		{"root", http.MethodGet, "/", http.StatusNotFound},
		{"wrong method", http.MethodPost, "/api/v1/health", http.StatusMethodNotAllowed},
	}
	handler := testAPI(t, nil)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
			p := decodeProblem(t, rec.Result(), c.want)
			if p.Instance != c.path {
				t.Errorf("problem.instance = %q, want %q", p.Instance, c.path)
			}
		})
	}
}

func TestResponsesCarryARequestID(t *testing.T) {
	rec := httptest.NewRecorder()
	testAPI(t, nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))

	if rec.Header().Get(RequestIDHeader) == "" {
		t.Error("no request id on a successful response")
	}
}

// Health is deliberately outside the rate limiter: a liveness probe that gets a
// 429 reports the process as down when it is fine.
func TestHealthIsNotRateLimited(t *testing.T) {
	cfg := Config{CORSOrigins: []string{"http://localhost"}, RateLimitPerMin: 1}
	handler := New(db.New(fakeDB{}), discardLogger(), cfg).Router()

	for i := 0; i < 5; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("probe %d got %d", i+1, rec.Code)
		}
	}
}

// A rollout cancels the context while requests are still running; those have to
// finish rather than have their connections cut.
func TestServeDrainsInFlightRequests(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	release := make(chan struct{})
	started := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("finished"))
	})

	cfg := Config{ShutdownTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second,
		WriteTimeout: 10 * time.Second, IdleTimeout: time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, cfg, listener, handler, discardLogger()) }()

	type result struct {
		body string
		err  error
	}
	resCh := make(chan result, 1)
	go func() {
		res, err := http.Get("http://" + listener.Addr().String() + "/slow")
		if err != nil {
			resCh <- result{err: err}
			return
		}
		defer res.Body.Close()
		buf := make([]byte, 8)
		n, _ := res.Body.Read(buf)
		resCh <- result{body: string(buf[:n])}
	}()

	<-started
	cancel() // the SIGTERM equivalent, with a request mid-flight
	close(release)

	select {
	case got := <-resCh:
		if got.err != nil {
			t.Errorf("in-flight request failed during shutdown: %v", got.err)
		}
		if got.body != "finished" {
			t.Errorf("body = %q, want the handler to have completed", got.body)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the in-flight request never completed")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("serve returned %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not return after the drain")
	}
}
