// Package testdb gives an integration test a migrated, empty Postgres.
//
// The queries in this project lean on window functions, trigram indexes, enum
// ordering and deferred foreign keys. A mock would confirm the string we wrote,
// not the behaviour we depend on, so the repository tests run against a real
// server.
package testdb

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
)

// errNoDocker marks the one failure that should skip rather than fail.
var errNoDocker = errors.New("docker is not available")

// image matches the compose file, so tests and development run the same server.
const image = "postgres:16.10-alpine"

// One container per test binary. Go runs each package's tests in its own
// process, so this is a container per package: enough isolation that packages
// no longer contend over a shared database, without the minutes a container
// per test would cost.
var (
	once    sync.Once
	dsn     string
	err     error
	running *container
)

// Run is the package's TestMain: it runs the tests, then stops the container.
//
//	func TestMain(m *testing.M) { testdb.Run(m) }
//
// Needed because the container is shared across a package's tests, so there is
// no single test that can own its cleanup.
func Run(m *testing.M) {
	code := m.Run()
	Stop()
	os.Exit(code)
}

// Stop removes the container if one was started.
func Stop() {
	if running == nil {
		return
	}
	if err := running.remove(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "testdb: could not remove the container: %v\n", err)
	}
	running = nil
}

// Start returns a DSN for a migrated, empty database.
//
// Setting TEST_DATABASE_URL bypasses the container and uses that server
// instead. Without Docker and without that variable, the test skips with an
// explanation rather than failing confusingly.
func Start(t *testing.T) string {
	t.Helper()

	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		// The override is the only way to aim these tests at a database
		// somebody cares about, and some of them truncate every table. This
		// has already destroyed a development database once.
		if name := databaseName(url); !strings.Contains(name, "test") {
			t.Fatalf("refusing to run: TEST_DATABASE_URL points at %q, which is not a test "+
				"database. These tests truncate tables. Unset it to use a throwaway "+
				"container, or name the database *test*.", name)
		}
		return url
	}

	once.Do(func() { dsn, err = startContainer() })
	if err != nil {
		if errors.Is(err, errNoDocker) {
			t.Skipf("Docker is not available, so this integration test cannot run: %v\n"+
				"Start Docker, or set TEST_DATABASE_URL to a migrated database.", err)
		}
		t.Fatalf("start test database: %v", err)
	}
	return dsn
}

// databaseName extracts the database from a connection string.
func databaseName(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(u.Path, "/")
}

func startContainer() (string, error) {
	// Deliberately not a test's context: the container outlives any single
	// test, and TestMain removes it.
	ctx := context.Background()

	c, err := runPostgres(ctx)
	if err != nil {
		return "", err
	}
	running = c

	if err := c.waitReady(ctx); err != nil {
		return "", errors.Join(err, c.remove(ctx))
	}
	if err := migrate(ctx, c.dsn()); err != nil {
		return "", errors.Join(err, c.remove(ctx))
	}
	return c.dsn(), nil
}

// migrate applies every migration's Up section in order.
//
// goose is not used here: it needs a newer Go than this module's floor, and
// importing it would raise the floor for everyone to run tests. CI applies the
// migrations with real goose, so the two are checked against each other there.
func migrate(ctx context.Context, url string) error {
	dir, err := migrationsDir()
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no migrations found in %s", dir)
	}
	sort.Strings(files)

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer func() {
		if cerr := conn.Close(ctx); cerr != nil {
			fmt.Fprintf(os.Stderr, "testdb: closing the migration connection: %v\n", cerr)
		}
	}()

	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}
		up, err := upSection(string(body))
		if err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(f), err)
		}
		if _, err := conn.Exec(ctx, up); err != nil {
			return fmt.Errorf("apply %s: %w", filepath.Base(f), err)
		}
	}
	return nil
}

// upSection extracts the statements between the goose Up and Down markers.
func upSection(body string) (string, error) {
	const upMarker, downMarker = "-- +goose Up", "-- +goose Down"
	start := strings.Index(body, upMarker)
	if start < 0 {
		return "", errors.New("no goose Up marker")
	}
	rest := body[start+len(upMarker):]
	if end := strings.Index(rest, downMarker); end >= 0 {
		rest = rest[:end]
	}
	if strings.TrimSpace(rest) == "" {
		return "", errors.New("empty Up section")
	}
	return rest, nil
}

// migrationsDir walks up from the working directory to the module root. Tests
// run in their own package directory, so the path cannot be relative.
func migrationsDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "migrations"), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("could not find the module root from the test's working directory")
}
