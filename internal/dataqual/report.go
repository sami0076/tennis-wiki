package dataqual

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Finding is one check's result.
type Finding struct {
	Name     string   `json:"name"`
	Severity Severity `json:"severity"`
	Count    int      `json:"count"`
	Why      string   `json:"why"`
	Samples  []string `json:"samples,omitempty"`
}

// CoverageRow is one cell of the season/tour/tier matrix.
type CoverageRow struct {
	Season    int    `json:"season"`
	Tour      string `json:"tour"`
	Tier      string `json:"tier"`
	Matches   int    `json:"matches"`
	WithStats int    `json:"with_stats"`
}

// StatPercent returns the share of matches carrying serve statistics.
func (c CoverageRow) StatPercent() float64 {
	if c.Matches == 0 {
		return 0
	}
	return 100 * float64(c.WithStats) / float64(c.Matches)
}

// ProvenanceRow records what one source contributed.
type ProvenanceRow struct {
	Source     string `json:"source"`
	Matches    int    `json:"matches"`
	FirstMatch string `json:"first_match"`
	LastMatch  string `json:"last_match"`
}

// Report is a full data-quality run.
type Report struct {
	GeneratedAt time.Time       `json:"generated_at"`
	Totals      map[string]int  `json:"totals"`
	Findings    []Finding       `json:"findings"`
	Coverage    []CoverageRow   `json:"coverage"`
	Provenance  []ProvenanceRow `json:"provenance"`
}

// tableOrder fixes the reporting order of the counted tables.
var tableOrder = []string{"players", "tournaments", "matches", "match_players", "rankings", "ratings"}

// Failed reports whether any integrity check found something. Anomalies and
// warnings never fail a run; only self-contradiction does.
func (r Report) Failed() bool {
	for _, f := range r.Findings {
		if f.Severity == Integrity && f.Count > 0 {
			return true
		}
	}
	return false
}

// Run executes every check.
func Run(ctx context.Context, pool *pgxpool.Pool) (Report, error) {
	rep := Report{GeneratedAt: time.Now().UTC()}

	totals, err := tableCounts(ctx, pool)
	if err != nil {
		return rep, err
	}
	rep.Totals = totals

	for _, c := range append(append([]Check{}, integrityChecks...), anomalyChecks...) {
		f, err := runCheck(ctx, pool, c)
		if err != nil {
			return rep, err
		}
		rep.Findings = append(rep.Findings, f)
	}

	if rep.Coverage, err = coverage(ctx, pool); err != nil {
		return rep, err
	}
	if rep.Provenance, err = provenance(ctx, pool); err != nil {
		return rep, err
	}
	return rep, nil
}

func runCheck(ctx context.Context, pool *pgxpool.Pool, c Check) (Finding, error) {
	f := Finding{Name: c.Name, Severity: c.Severity, Why: c.Why}
	if err := pool.QueryRow(ctx, c.Query).Scan(&f.Count); err != nil {
		return f, fmt.Errorf("check %s: %w", c.Name, err)
	}
	// Samples only matter when something is wrong, and only for checks that
	// bothered to define one.
	if f.Count > 0 && c.Sample != "" {
		rows, err := pool.Query(ctx, c.Sample)
		if err != nil {
			return f, fmt.Errorf("check %s samples: %w", c.Name, err)
		}
		defer rows.Close()
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				return f, fmt.Errorf("check %s samples: %w", c.Name, err)
			}
			f.Samples = append(f.Samples, s)
		}
		if err := rows.Err(); err != nil {
			return f, fmt.Errorf("check %s samples: %w", c.Name, err)
		}
	}
	return f, nil
}

func tableCounts(ctx context.Context, pool *pgxpool.Pool) (map[string]int, error) {
	out := map[string]int{}
	for _, t := range tableOrder {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM `+t).Scan(&n); err != nil {
			return nil, fmt.Errorf("count %s: %w", t, err)
		}
		out[t] = n
	}
	return out, nil
}

func coverage(ctx context.Context, pool *pgxpool.Pool) ([]CoverageRow, error) {
	rows, err := pool.Query(ctx, coverageQuery)
	if err != nil {
		return nil, fmt.Errorf("coverage: %w", err)
	}
	defer rows.Close()

	var out []CoverageRow
	for rows.Next() {
		var c CoverageRow
		if err := rows.Scan(&c.Season, &c.Tour, &c.Tier, &c.Matches, &c.WithStats); err != nil {
			return nil, fmt.Errorf("coverage: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func provenance(ctx context.Context, pool *pgxpool.Pool) ([]ProvenanceRow, error) {
	rows, err := pool.Query(ctx, provenanceQuery)
	if err != nil {
		return nil, fmt.Errorf("provenance: %w", err)
	}
	defer rows.Close()

	var out []ProvenanceRow
	for rows.Next() {
		var p ProvenanceRow
		if err := rows.Scan(&p.Source, &p.Matches, &p.FirstMatch, &p.LastMatch); err != nil {
			return nil, fmt.Errorf("provenance: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// WriteJSON emits the report for machine consumption.
func (r Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		return fmt.Errorf("encode report: %w", err)
	}
	return nil
}

// errWriter defers error handling to one check at the end rather than after
// each of two dozen writes.
type errWriter struct {
	w   io.Writer
	err error
}

func (e *errWriter) printf(format string, args ...any) {
	if e.err != nil {
		return
	}
	_, e.err = fmt.Fprintf(e.w, format, args...)
}

// WriteText renders the report for a human.
func (r Report) WriteText(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	out := &errWriter{w: tw}

	out.printf("Data quality report\t%s\n\n", r.GeneratedAt.Format(time.RFC3339))

	out.printf("TABLES\n")
	for _, t := range tableOrder {
		out.printf("  %s\t%s\n", t, comma(r.Totals[t]))
	}

	out.printf("\nINTEGRITY\tthese must all be zero\n")
	for _, f := range r.Findings {
		if f.Severity != Integrity {
			continue
		}
		status := "ok"
		if f.Count > 0 {
			status = "FAIL"
		}
		out.printf("  %-32s\t%8s\t%s\n", f.Name, comma(f.Count), status)
		if f.Count > 0 {
			out.printf("  \t\t%s\n", f.Why)
			for _, sample := range f.Samples {
				out.printf("  \t\t  %s\n", sample)
			}
		}
	}

	out.printf("\nANOMALIES\tdescribed, not faulted\n")
	for _, f := range r.Findings {
		if f.Severity == Integrity {
			continue
		}
		marker := " "
		if f.Severity == Warning && f.Count > 0 {
			marker = "!"
		}
		out.printf("%s %-32s\t%8s\t%s\n", marker, f.Name, comma(f.Count), f.Why)
	}

	out.printf("\nCOVERAGE\tmatches, and the share carrying serve statistics\n")
	out.printf("  season\ttour\ttier\tmatches\twith stats\n")
	for _, c := range r.Coverage {
		out.printf("  %d\t%s\t%s\t%s\t%.0f%%\n",
			c.Season, c.Tour, c.Tier, comma(c.Matches), c.StatPercent())
	}

	out.printf("\nPROVENANCE\twhich source wrote what\n")
	for _, p := range r.Provenance {
		out.printf("  %-28s\t%8s\t%s to %s\n", p.Source, comma(p.Matches), p.FirstMatch, p.LastMatch)
	}

	if r.Failed() {
		out.printf("\nRESULT\tFAILED: the database contradicts itself, see INTEGRITY above\n")
	} else {
		out.printf("\nRESULT\tpassed: no integrity violations\n")
	}

	if out.err != nil {
		return fmt.Errorf("write report: %w", out.err)
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flush report: %w", err)
	}
	return nil
}

// comma formats a count with thousands separators.
func comma(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
	}
	for i := pre; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
