// Package cache is the read-path cache that sits in front of Postgres.
//
// This data barely changes: it moves when an ingest runs, which is deliberate
// and infrequent. That is what makes it worth caching at all, and it is why
// nothing here expires on a schedule -- the ingest clears the cache when it has
// finished changing things, and the TTL below is a backstop rather than a
// freshness policy.
//
// Every operation tolerates Redis being absent, unreachable or slow. A cache
// that is down costs time and nothing else: the answer to every question
// becomes "not cached", which is the same answer an empty cache gives.
package cache

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// Namespace prefixes every key, so a Redis shared with something else is only
// ever partly ours and Flush cannot reach beyond it.
const Namespace = "deucepoint:v1:"

// DefaultTTL bounds how long a stale entry can outlive the ingest that should
// have cleared it. Freshness comes from Flush; this is what stops a failed
// flush being permanent, and stops keys nobody asks for any more accumulating.
const DefaultTTL = 24 * time.Hour

// opTimeout caps what a slow Redis can add to a request. Beyond it the cache
// reports a miss and the handler does the work, which is slower than a hit and
// faster than waiting.
const opTimeout = 50 * time.Millisecond

// Stats is what the cache has done since the process started.
type Stats struct {
	Enabled bool  `json:"enabled"`
	Hits    int64 `json:"hits"`
	Misses  int64 `json:"misses"`
	// Errors counts operations Redis failed or timed out on. They are also
	// counted as misses, because that is what the caller was told.
	Errors  int64   `json:"errors"`
	HitRate float64 `json:"hit_rate"`
}

// Cache is a Redis-backed byte cache. The zero value is a disabled cache that
// answers every read with a miss, which is what makes the caller's code the
// same whether Redis is configured or not.
type Cache struct {
	client *redis.Client
	ttl    time.Duration
	log    *slog.Logger

	hits, misses, failures atomic.Int64
}

// FromEnv builds the cache from REDIS_URL and CACHE_TTL.
//
// An unset REDIS_URL is not an error: it is a deployment that has chosen not to
// cache, and it must behave exactly like one whose Redis is down.
func FromEnv(log *slog.Logger) (*Cache, error) {
	raw := os.Getenv("REDIS_URL")
	if raw == "" {
		return &Cache{log: log}, nil
	}

	options, err := redis.ParseURL(raw)
	if err != nil {
		return nil, err
	}

	ttl := DefaultTTL
	if v := os.Getenv("CACHE_TTL"); v != "" {
		parsed, err := time.ParseDuration(v)
		if err != nil {
			return nil, err
		}
		ttl = parsed
	}

	// Short dial and read timeouts for the same reason as opTimeout: this is a
	// cache, and waiting on it is worse than missing it.
	options.DialTimeout = opTimeout
	options.ReadTimeout = opTimeout
	options.WriteTimeout = opTimeout
	// Retrying a cache lookup spends the time it was meant to save.
	options.MaxRetries = -1

	return &Cache{client: redis.NewClient(options), ttl: ttl, log: log}, nil
}

// Enabled reports whether a Redis was configured at all.
func (c *Cache) Enabled() bool { return c != nil && c.client != nil }

// Get returns the cached bytes, or false for anything else -- a miss, a broken
// connection, a Redis that is not there.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool) {
	if !c.Enabled() {
		return nil, false
	}

	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()

	value, err := c.client.Get(ctx, Namespace+key).Bytes()
	switch {
	case err == nil:
		c.hits.Add(1)
		return value, true
	case errors.Is(err, redis.Nil):
		c.misses.Add(1)
		return nil, false
	default:
		c.fail("cache read failed", key, err)
		return nil, false
	}
}

// Set stores bytes under key. A failure is logged and dropped: the response is
// already correct, and refusing to answer because the cache would not take a
// copy is the failure mode this whole package exists to avoid.
func (c *Cache) Set(ctx context.Context, key string, value []byte) {
	if !c.Enabled() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()

	if err := c.client.Set(ctx, Namespace+key, value, c.ttl).Err(); err != nil {
		c.fail("cache write failed", key, err)
	}
}

// Flush removes everything this cache put in Redis.
//
// Scanned and deleted by prefix rather than FLUSHDB, because the Redis may not
// be ours alone. This is the invalidation: an ingest calls it when it has
// finished, which is the only moment the answers actually change.
func (c *Cache) Flush(ctx context.Context) (int64, error) {
	if !c.Enabled() {
		return 0, nil
	}

	var removed int64
	var cursor uint64
	for {
		keys, next, err := c.client.Scan(ctx, cursor, Namespace+"*", 500).Result()
		if err != nil {
			return removed, err
		}
		if len(keys) > 0 {
			// Unlink rather than Del: the reclaim happens off the command loop,
			// so a large flush does not stall every reader behind it.
			n, err := c.client.Unlink(ctx, keys...).Result()
			if err != nil {
				return removed, err
			}
			removed += n
		}
		if next == 0 {
			return removed, nil
		}
		cursor = next
	}
}

// Reachable round-trips a PING. Used by the health endpoint to report the
// cache's state, never to decide the service's own.
func (c *Cache) Reachable(ctx context.Context) bool {
	if !c.Enabled() {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	return c.client.Ping(ctx).Err() == nil
}

// Stats reports the hit rate, so the cache's value is measured rather than
// assumed.
func (c *Cache) Stats() Stats {
	if c == nil {
		return Stats{}
	}
	hits, misses := c.hits.Load(), c.misses.Load()
	s := Stats{
		Enabled: c.Enabled(),
		Hits:    hits,
		Misses:  misses,
		Errors:  c.failures.Load(),
	}
	if total := hits + misses; total > 0 {
		s.HitRate = float64(hits) / float64(total)
	}
	return s
}

// Close releases the connection pool.
func (c *Cache) Close() error {
	if !c.Enabled() {
		return nil
	}
	return c.client.Close()
}

// fail records a Redis failure as both an error and a miss, because a miss is
// what the caller was handed.
func (c *Cache) fail(msg, key string, err error) {
	c.failures.Add(1)
	c.misses.Add(1)
	if c.log != nil {
		c.log.Warn(msg, "key", key, "error", err)
	}
}

// TTLFor is the configured lifetime, exposed for the log line that reports it.
func (c *Cache) TTLFor() time.Duration {
	if !c.Enabled() {
		return 0
	}
	return c.ttl
}
