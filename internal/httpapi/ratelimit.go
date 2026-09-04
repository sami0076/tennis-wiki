package httpapi

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// idleEviction is how long a silent client's bucket is kept. Without it the map
// grows for the lifetime of the process, which on a public site is a leak.
const idleEviction = 10 * time.Minute

// bucket is one client's token allowance, refilled continuously rather than
// reset on a boundary: a fixed window lets a caller send 2x the limit across
// the seam between two windows.
type bucket struct {
	tokens float64
	last   time.Time
}

// RateLimiter allows burst requests and then refills at rate tokens per second.
type RateLimiter struct {
	rate  float64
	burst float64
	now   func() time.Time

	mu      sync.Mutex
	buckets map[string]*bucket
	sweepAt time.Time
}

// NewRateLimiter allows perMinute requests a minute per client, with a burst of
// the same size so a page that fires several requests at once still loads.
func NewRateLimiter(perMinute int) *RateLimiter {
	return &RateLimiter{
		rate:    float64(perMinute) / 60,
		burst:   float64(perMinute),
		now:     time.Now,
		buckets: make(map[string]*bucket),
	}
}

// Allow reports whether the client may proceed, and if not, how long to wait.
func (l *RateLimiter) Allow(key string) (bool, time.Duration) {
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep(now)

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}

	b.tokens = math.Min(l.burst, b.tokens+now.Sub(b.last).Seconds()*l.rate)
	b.last = now

	if b.tokens < 1 {
		// Round up: a Retry-After that is a fraction short leaves the caller
		// retrying into another rejection.
		wait := time.Duration(math.Ceil((1-b.tokens)/l.rate)) * time.Second
		return false, wait
	}
	b.tokens--
	return true, 0
}

// sweep drops idle buckets. Called under the lock, at most once a minute, so
// the cost is amortised rather than needing a background goroutine.
func (l *RateLimiter) sweep(now time.Time) {
	if now.Sub(l.sweepAt) < time.Minute {
		return
	}
	l.sweepAt = now
	for key, b := range l.buckets {
		if now.Sub(b.last) > idleEviction {
			delete(l.buckets, key)
		}
	}
}

// Middleware rejects over-limit clients with 429 and a Retry-After.
func (l *RateLimiter) Middleware(trustProxy bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ok, wait := l.Allow(clientIP(r, trustProxy))
			if !ok {
				w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())))
				WriteProblem(w, r, http.StatusTooManyRequests, TypeRateLimited,
					"Rate limit exceeded. Retry in "+strconv.Itoa(int(wait.Seconds()))+"s.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
