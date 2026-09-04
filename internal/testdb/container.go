package testdb

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Why not testcontainers-go, which issue #13 names.
//
// It builds on Go 1.23, but its dependency graph — the OpenTelemetry stack and
// current golang.org/x modules — does not: `go mod tidy` raises this module's
// go directive from 1.23.0 to 1.25.0. That floor is a stated constraint, and CI
// builds at it on purpose, so quietly moving it to gain a test helper is the
// wrong trade. Driving the Docker CLI costs about a hundred lines, adds no
// dependency at all, and sidesteps the shared-reaper race that made
// testcontainers fail under `go test ./...` anyway.
//
// Switching back is a `go get` away if the floor ever moves.

const (
	// label marks our containers so a leaked one is easy to find and remove.
	label = "tennis-wiki-test"

	startupTimeout = 90 * time.Second
	password       = "tennis"
)

// container is a running Postgres.
type container struct {
	id   string
	port string
}

// runPostgres starts a Postgres container on a random host port.
func runPostgres(ctx context.Context) (*container, error) {
	if err := dockerAvailable(ctx); err != nil {
		return nil, err
	}

	id, err := docker(ctx, "run", "--detach",
		"--label", label+"=true",
		"--env", "POSTGRES_USER=tennis",
		"--env", "POSTGRES_PASSWORD="+password,
		"--env", "POSTGRES_DB=tennis_test",
		// Nothing here outlives the test run, so durability is wasted work.
		"--tmpfs", "/var/lib/postgresql/data",
		"--publish", "0:5432",
		image,
		"postgres", "-c", "fsync=off", "-c", "synchronous_commit=off",
		"-c", "full_page_writes=off",
	)
	if err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)

	port, err := hostPort(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w (container %s)", err, id)
	}
	return &container{id: id, port: port}, nil
}

// dsn is the connection string for the container.
func (c *container) dsn() string {
	return fmt.Sprintf("postgres://tennis:%s@127.0.0.1:%s/tennis_test?sslmode=disable",
		password, c.port)
}

// remove deletes the container. Called on the way out, and on any failure
// during setup, so a broken run does not leave one behind.
func (c *container) remove(ctx context.Context) error {
	_, err := docker(ctx, "rm", "--force", "--volumes", c.id)
	return err
}

// waitReady polls until the server accepts a query. Postgres logs that it is
// ready once during initialisation and again for real, so waiting on the log is
// a known source of flakiness; connecting is the honest check.
func (c *container) waitReady(ctx context.Context) error {
	deadline := time.Now().Add(startupTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		attempt, cancel := context.WithTimeout(ctx, 2*time.Second)
		conn, err := pgx.Connect(attempt, c.dsn())
		if err == nil {
			err = conn.Ping(attempt)
			closeErr := conn.Close(attempt)
			cancel()
			if err == nil {
				return closeErr
			}
			lastErr = err
		} else {
			cancel()
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	return fmt.Errorf("database not ready within %s: %w", startupTimeout, lastErr)
}

// hostPort reports which host port Docker mapped 5432 to.
func hostPort(ctx context.Context, id string) (string, error) {
	out, err := docker(ctx, "port", id, "5432/tcp")
	if err != nil {
		return "", err
	}
	// Docker prints one line per binding, as "0.0.0.0:49155".
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if i := strings.LastIndex(strings.TrimSpace(line), ":"); i >= 0 {
			if port := strings.TrimSpace(line[i+1:]); port != "" {
				return port, nil
			}
		}
	}
	return "", fmt.Errorf("could not read the mapped port from %q", out)
}

func dockerAvailable(ctx context.Context) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("%w: docker is not on PATH", errNoDocker)
	}
	if _, err := docker(ctx, "version", "--format", "{{.Server.Version}}"); err != nil {
		return fmt.Errorf("%w: %v", errNoDocker, err)
	}
	return nil
}

// docker runs one command and returns its stdout.
func docker(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker %s: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
