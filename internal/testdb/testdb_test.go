package testdb

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpSection(t *testing.T) {
	const body = "-- +goose Up\nCREATE TABLE a (id int);\n\n-- +goose Down\nDROP TABLE a;\n"
	got, err := upSection(body)
	if err != nil {
		t.Fatalf("upSection: %v", err)
	}
	if !strings.Contains(got, "CREATE TABLE") {
		t.Errorf("Up section = %q", got)
	}
	// Applying a Down section during setup would drop the schema it just built.
	if strings.Contains(got, "DROP TABLE") {
		t.Error("the Down section leaked into the Up section")
	}
}

func TestUpSectionRejectsUnusableFiles(t *testing.T) {
	cases := map[string]string{
		"no markers":     "CREATE TABLE a (id int);",
		"empty up":       "-- +goose Up\n\n-- +goose Down\nDROP TABLE a;",
		"only down":      "-- +goose Down\nDROP TABLE a;",
		"nothing at all": "",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := upSection(body); err == nil {
				t.Errorf("accepted %q", body)
			}
		})
	}
}

// Every committed migration has to parse, or the harness builds a partial
// schema and the failure surfaces as a confusing query error.
func TestEveryMigrationHasAnUpSection(t *testing.T) {
	dir, err := migrationsDir()
	if err != nil {
		t.Fatalf("migrationsDir: %v", err)
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no migrations found in %s", dir)
	}
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := upSection(string(body)); err != nil {
			t.Errorf("%s: %v", filepath.Base(f), err)
		}
	}
}

func TestDatabaseName(t *testing.T) {
	cases := map[string]string{
		"postgres://u:p@localhost:5433/tennis_test?sslmode=disable": "tennis_test",
		"postgres://u:p@localhost:5433/tennis":                      "tennis",
		"postgres://localhost/":                                     "",
	}
	for dsn, want := range cases {
		if got := databaseName(dsn); got != want {
			t.Errorf("databaseName(%q) = %q, want %q", dsn, got, want)
		}
	}

	// What matters for the guard is that nothing unparseable is mistaken for a
	// disposable database, not what exactly comes back.
	for _, dsn := range []string{"not a url at all", "", "://"} {
		if strings.Contains(databaseName(dsn), "test") {
			t.Errorf("databaseName(%q) looks like a test database", dsn)
		}
	}
}

// A missing Docker skips; anything else fails. The distinction is a sentinel
// rather than string matching, so it cannot drift.
func TestNoDockerIsDistinguishable(t *testing.T) {
	wrapped := fmt.Errorf("%w: docker is not on PATH", errNoDocker)
	if !errors.Is(wrapped, errNoDocker) {
		t.Error("a wrapped errNoDocker should still be recognised, or the test fails instead of skipping")
	}
	if errors.Is(errors.New("apply 00003.sql: syntax error"), errNoDocker) {
		t.Error("a migration failure would skip instead of failing")
	}
}
