package dataqual

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/db"
	"github.com/sami0076/tennis-wiki/internal/testdb"
)

func TestComma(t *testing.T) {
	cases := map[int]string{
		0: "0", 7: "7", 42: "42", 999: "999",
		1000: "1,000", 12345: "12,345", 123456: "123,456",
		1234567: "1,234,567", 193737: "193,737",
	}
	for in, want := range cases {
		if got := comma(in); got != want {
			t.Errorf("comma(%d) = %q, want %q", in, got, want)
		}
	}
}

// Only integrity findings fail a run. Coverage gaps and anomalies are facts
// about the sources, not defects, and must never break CI.
func TestOnlyIntegrityFails(t *testing.T) {
	clean := Report{Findings: []Finding{
		{Name: "a", Severity: Integrity, Count: 0},
		{Name: "b", Severity: Warning, Count: 500},
		{Name: "c", Severity: Info, Count: 100000},
	}}
	if clean.Failed() {
		t.Error("warnings and info should not fail the report")
	}

	broken := clean
	broken.Findings = append(broken.Findings, Finding{Name: "d", Severity: Integrity, Count: 1})
	if !broken.Failed() {
		t.Error("a non-zero integrity finding must fail the report")
	}
}

func TestStatPercent(t *testing.T) {
	cases := []struct {
		row  CoverageRow
		want float64
	}{
		{CoverageRow{Matches: 100, WithStats: 99}, 99},
		{CoverageRow{Matches: 0, WithStats: 0}, 0},
		{CoverageRow{Matches: 4, WithStats: 1}, 25},
	}
	for _, c := range cases {
		if got := c.row.StatPercent(); got != c.want {
			t.Errorf("StatPercent(%+v) = %v, want %v", c.row, got, c.want)
		}
	}
}

func TestWriteJSONIsValid(t *testing.T) {
	r := Report{
		Totals:     map[string]int{"matches": 10},
		Findings:   []Finding{{Name: "x", Severity: Integrity, Count: 0, Why: "because"}},
		Coverage:   []CoverageRow{{Season: 2024, Tour: "atp", Tier: "tour", Matches: 10, WithStats: 9}},
		Provenance: []ProvenanceRow{{Source: "s", Matches: 10, FirstMatch: "2024-01-01", LastMatch: "2024-12-31"}},
	}
	var buf bytes.Buffer
	if err := r.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	var back Report
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if back.Totals["matches"] != 10 || len(back.Coverage) != 1 {
		t.Errorf("round trip lost data: %+v", back)
	}
}

func TestWriteTextMentionsEverySection(t *testing.T) {
	r := Report{
		Totals:   map[string]int{"matches": 1},
		Findings: []Finding{{Name: "check_one", Severity: Integrity, Count: 0}},
		Coverage: []CoverageRow{{Season: 2024, Tour: "atp", Tier: "tour", Matches: 1}},
	}
	var buf bytes.Buffer
	if err := r.WriteText(&buf); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"TABLES", "INTEGRITY", "ANOMALIES", "COVERAGE", "PROVENANCE", "passed"} {
		if !strings.Contains(out, want) {
			t.Errorf("report is missing the %s section", want)
		}
	}
}

// A failing report must say so in the text output, not only in the exit code.
func TestWriteTextReportsFailure(t *testing.T) {
	r := Report{Findings: []Finding{{
		Name: "matches_without_two_players", Severity: Integrity, Count: 3,
		Why: "explanation", Samples: []string{"Wimbledon 2019 match 100"},
	}}}
	var buf bytes.Buffer
	if err := r.WriteText(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"FAIL", "FAILED", "explanation", "Wimbledon 2019 match 100"} {
		if !strings.Contains(out, want) {
			t.Errorf("failure output is missing %q", want)
		}
	}
}

// Every check must have a name and an explanation: a bare number in a report
// nobody can interpret is worse than no report.
func TestChecksAreDocumented(t *testing.T) {
	all := append(append([]Check{}, integrityChecks...), anomalyChecks...)
	if len(all) < 15 {
		t.Errorf("only %d checks defined", len(all))
	}
	seen := map[string]bool{}
	for _, c := range all {
		if c.Name == "" || c.Why == "" || c.Query == "" {
			t.Errorf("check %+v is missing a name, explanation or query", c)
		}
		if seen[c.Name] {
			t.Errorf("duplicate check name %q", c.Name)
		}
		seen[c.Name] = true
	}
}

// The queries must actually run against the real schema. A typo here is only
// discoverable against a database.
func TestChecksRunAgainstRealSchema(t *testing.T) {
	dsn := testdb.Start(t)
	ctx := context.Background()
	pool, err := db.Open(ctx, db.Config{DSN: dsn, MaxConns: 4, MinConns: 1})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	report, err := Run(ctx, pool)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Findings) != len(integrityChecks)+len(anomalyChecks) {
		t.Errorf("got %d findings, expected one per check", len(report.Findings))
	}
	for _, f := range report.Findings {
		if f.Count < 0 {
			t.Errorf("check %s returned a negative count", f.Name)
		}
	}

	// The ingested database must not contradict itself.
	for _, f := range report.Findings {
		if f.Severity == Integrity && f.Count > 0 {
			t.Errorf("integrity check %s failed with %d rows: %v", f.Name, f.Count, f.Samples)
		}
	}
}
