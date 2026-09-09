package testdb

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	"time"
)

// redisImage matches the compose file, so tests and development run the same
// server.
const redisImage = "redis:7.4-alpine"

// One Redis per test binary, for the same reason as the database above.
var (
	redisOnce    sync.Once
	redisURL     string
	redisErr     error
	redisRunning *container
)

// StartRedis returns a URL for an empty Redis.
//
// Setting TEST_REDIS_URL bypasses the container. Without Docker and without
// that variable the test skips, the same way the database tests do: a machine
// with no Docker should report that, not fail.
func StartRedis(t *testing.T) string {
	t.Helper()

	if url := os.Getenv("TEST_REDIS_URL"); url != "" {
		return url
	}

	redisOnce.Do(func() { redisURL, redisErr = startRedisContainer() })
	if redisErr != nil {
		if errors.Is(redisErr, errNoDocker) {
			t.Skipf("Docker is not available, so this integration test cannot run: %v\n"+
				"Start Docker, or set TEST_REDIS_URL to an empty Redis.", redisErr)
		}
		t.Fatalf("start test redis: %v", redisErr)
	}
	return redisURL
}

// StopRedis removes the container if one was started. Called by Run.
func StopRedis() {
	if redisRunning == nil {
		return
	}
	if err := redisRunning.remove(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "testdb: could not remove the redis container: %v\n", err)
	}
	redisRunning = nil
}

func startRedisContainer() (string, error) {
	ctx := context.Background()

	if err := dockerAvailable(ctx); err != nil {
		return "", err
	}
	id, err := docker(ctx, "run", "--detach",
		"--label", label+"=true",
		"--publish", "0:6379",
		redisImage,
		// The same flags the compose file uses: this is a cache, and nothing in
		// it is worth writing to disk.
		"redis-server", "--save", "", "--appendonly", "no",
	)
	if err != nil {
		return "", err
	}
	id = trimID(id)

	port, err := hostPort(ctx, id, "6379/tcp")
	if err != nil {
		return "", fmt.Errorf("%w (container %s)", err, id)
	}
	c := &container{id: id, port: port}
	redisRunning = c

	if err := waitForPong(ctx, port); err != nil {
		return "", errors.Join(err, c.remove(ctx))
	}
	return fmt.Sprintf("redis://127.0.0.1:%s/0", port), nil
}

// waitForPong polls until the server answers PING. Two lines of RESP rather
// than a client dependency in a package every test imports.
func waitForPong(ctx context.Context, port string) error {
	deadline := time.Now().Add(startupTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 2*time.Second)
		if err == nil {
			lastErr = ping(conn)
			if cerr := conn.Close(); cerr != nil && lastErr == nil {
				lastErr = cerr
			}
			if lastErr == nil {
				return nil
			}
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	return fmt.Errorf("redis not ready within %s: %w", startupTimeout, lastErr)
}

func ping(conn net.Conn) error {
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return err
	}
	if _, err := conn.Write([]byte("PING\r\n")); err != nil {
		return err
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return err
	}
	if line != "+PONG\r\n" {
		return fmt.Errorf("redis answered PING with %q", line)
	}
	return nil
}
