package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/cache"
	"github.com/sami0076/tennis-wiki/internal/db"
	"github.com/sami0076/tennis-wiki/internal/testdb"
)

// withCache rebuilds the fixture's handler with a cache in front of it, from a
// REDIS_URL the test controls.
func (f *apiFixture) withCache(url string) *cache.Cache {
	f.t.Helper()
	f.t.Setenv("REDIS_URL", url)

	c, err := cache.FromEnv(discardLogger())
	if err != nil {
		f.t.Fatalf("build cache: %v", err)
	}
	f.t.Cleanup(func() { _ = c.Close() })

	cfg := Config{CORSOrigins: []string{"http://localhost:5173"}, RateLimitPerMin: 600}
	api := New(db.New(f.tx), discardLogger(), cfg)
	api.Cache = c
	f.handler = api.Router()
	return c
}

func decodeHealth(t *testing.T, res *http.Response) HealthResponse {
	t.Helper()
	var h HealthResponse
	if err := json.NewDecoder(res.Body).Decode(&h); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	return h
}

func TestCacheServesTheSecondRequest(t *testing.T) {
	url := testdb.StartRedis(t)
	f := newAPIFixture(t)
	c := f.withCache(url)
	if _, err := c.Flush(context.Background()); err != nil {
		t.Fatalf("clear the cache before the test: %v", err)
	}

	pts := int16(100)
	player := f.player("itg-cached", "Itg Cached", db.TourAtp)
	foil := f.player("itg-cached-foil", "Itg Cachedfoil", db.TourAtp)
	open := f.tournament("itg-cached-open", db.TierTour, 2019)
	f.match(open, player, foil, 1, "F", 2019, &pts, false)

	first := f.get("/api/v1/players/itg-cached")
	if got := first.Header.Get("X-Cache"); got != "miss" {
		t.Errorf("first request X-Cache = %q, want miss", got)
	}
	before := decodeProfile(t, first)

	second := f.get("/api/v1/players/itg-cached")
	if got := second.Header.Get("X-Cache"); got != "hit" {
		t.Fatalf("second request X-Cache = %q, want hit", got)
	}
	after := decodeProfile(t, second)

	if before.Name != after.Name || before.Career.Matches != after.Career.Matches {
		t.Errorf("cached response differs: %+v then %+v", before.Career, after.Career)
	}

	// The order of the query string is not part of what was asked for.
	f.get("/api/v1/players/itg-cached/matches?surface=clay&season=2019")
	mirrored := f.get("/api/v1/players/itg-cached/matches?season=2019&surface=clay")
	if got := mirrored.Header.Get("X-Cache"); got != "hit" {
		t.Errorf("reordered query X-Cache = %q, want hit: it is the same request", got)
	}

	if stats := c.Stats(); stats.Hits < 2 || stats.HitRate <= 0 {
		t.Errorf("stats = %+v, want the hits to be counted", stats)
	}
}

// The invalidation. Nothing here expires on a timer, so the ingest clearing the
// cache is the only thing that makes a changed answer visible.
func TestFlushMakesTheNextRequestFresh(t *testing.T) {
	url := testdb.StartRedis(t)
	f := newAPIFixture(t)
	c := f.withCache(url)
	if _, err := c.Flush(context.Background()); err != nil {
		t.Fatalf("clear the cache before the test: %v", err)
	}

	pts := int16(100)
	player := f.player("itg-flushed", "Itg Flushed", db.TourAtp)
	foil := f.player("itg-flushed-foil", "Itg Flushedfoil", db.TourAtp)
	open := f.tournament("itg-flushed-open", db.TierTour, 2019)
	f.match(open, player, foil, 1, "SF", 2019, &pts, false)

	if got := decodeProfile(t, f.get("/api/v1/players/itg-flushed")); got.Career.Matches != 1 {
		t.Fatalf("first read saw %d matches, want 1", got.Career.Matches)
	}

	// A second match, as an ingest would write one.
	f.match(open, player, foil, 2, "F", 2019, &pts, false)

	stale := decodeProfile(t, f.get("/api/v1/players/itg-flushed"))
	if stale.Career.Matches != 1 {
		t.Errorf("cached read saw %d matches; the cache is not caching", stale.Career.Matches)
	}

	if _, err := c.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	fresh := f.get("/api/v1/players/itg-flushed")
	if got := fresh.Header.Get("X-Cache"); got != "miss" {
		t.Errorf("X-Cache after a flush = %q, want miss", got)
	}
	if got := decodeProfile(t, fresh); got.Career.Matches != 2 {
		t.Errorf("read after a flush saw %d matches, want 2", got.Career.Matches)
	}
}

// The acceptance criterion this whole package is shaped around: Redis being
// down costs time and nothing else.
func TestRedisDownIsSlowerAndNothingElse(t *testing.T) {
	f := newAPIFixture(t)
	// A port nothing is listening on. Closed rather than firewalled, so the
	// failure is immediate and the test does not spend its timeout.
	f.withCache("redis://127.0.0.1:1/0")

	pts := int16(100)
	player := f.player("itg-nocache", "Itg Nocache", db.TourAtp)
	foil := f.player("itg-nocache-foil", "Itg Nocachefoil", db.TourAtp)
	open := f.tournament("itg-nocache-open", db.TierTour, 2019)
	f.match(open, player, foil, 1, "F", 2019, &pts, false)

	for i := 0; i < 2; i++ {
		res := f.get("/api/v1/players/itg-nocache")
		if res.StatusCode != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200 with the cache down", i, res.StatusCode)
		}
		if got := decodeProfile(t, res); got.Career.Matches != 1 {
			t.Errorf("request %d returned %d matches, want the real answer", i, got.Career.Matches)
		}
	}

	// Readiness is about this process and its database. A cache that is down is
	// reported, not judged.
	health := f.get("/api/v1/health")
	if health.StatusCode != http.StatusOK {
		t.Fatalf("health = %d with the cache down, want 200", health.StatusCode)
	}
	reported := decodeHealth(t, health)
	if reported.Status != "ok" || reported.Database != "ok" {
		t.Errorf("health reported %+v, want ok", reported)
	}
	if reported.Cache.Reachable {
		t.Error("health says the cache is reachable, and it is not")
	}
	if !reported.Cache.Enabled || reported.Cache.Errors == 0 {
		t.Errorf("cache health = %+v, want an enabled cache reporting its failures",
			reported.Cache)
	}
}

func TestCacheIsOptional(t *testing.T) {
	if os.Getenv("REDIS_URL") != "" {
		t.Setenv("REDIS_URL", "")
	}
	c, err := cache.FromEnv(discardLogger())
	if err != nil {
		t.Fatalf("build cache without a URL: %v", err)
	}
	if c.Enabled() {
		t.Error("a cache with no REDIS_URL reports itself enabled")
	}
	if _, ok := c.Get(context.Background(), "anything"); ok {
		t.Error("a disabled cache answered a read")
	}
	// Every write path has to be a no-op rather than a panic.
	c.Set(context.Background(), "anything", []byte("x"))
	if n, err := c.Flush(context.Background()); err != nil || n != 0 {
		t.Errorf("flushing a disabled cache removed %d keys, error %v", n, err)
	}
}
