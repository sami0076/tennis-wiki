package cache_test

import (
	"context"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/cache"
	"github.com/sami0076/tennis-wiki/internal/testdb"
)

// FlushFromEnv is what a batch command calls when it has finished changing the
// data, so what matters is that it reads the same environment the API does and
// leaves nothing of ours behind.
func TestFlushFromEnvClearsWhatTheAPICached(t *testing.T) {
	url := testdb.StartRedis(t)
	t.Setenv("REDIS_URL", url)
	ctx := context.Background()

	c, err := cache.FromEnv(nil)
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	c.Set(ctx, "rankings:atp", []byte("yesterday"))
	if _, ok := c.Get(ctx, "rankings:atp"); !ok {
		t.Fatal("the key was not cached, so the flush would prove nothing")
	}

	cache.FlushFromEnv(ctx, nil)

	if _, ok := c.Get(ctx, "rankings:atp"); ok {
		t.Error("the key survived the flush")
	}
}

// A command with no Redis configured still calls this, and it has to be a
// no-op rather than a failure.
func TestFlushFromEnvWithoutRedis(t *testing.T) {
	t.Setenv("REDIS_URL", "")
	cache.FlushFromEnv(context.Background(), nil)
}
