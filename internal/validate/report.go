package validate

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"text/tabwriter"
)

// The tour-level accuracy band from spec section 7.5. Below tour level the
// target does not apply: ADR-0004 says to expect less where the field is
// deeper and the results noisier, and says so rather than moving the target.
const (
	tourAccuracyFloor   = 68.0
	tourAccuracyCeiling = 72.0
	// calibrationSlack is how far a tier's mean predicted-to-observed gap may
	// run before it is worth reporting. Percentage points.
	calibrationSlack = 3.0
	// reversionSlack is how far the pool mean may sit from the base rating
	// before it is worth a second look, in rating points.
	reversionSlack = 100.0
)

// findings turns the measurements into sentences. Every one of them is a
// judgement about the weights, which is why none of them fails the run: the
// report exists to inform a decision, not to make it.
func (a *accumulator) findings(rep Report) []Finding {
	var out []Finding

	if rep.Scored == 0 {
		return []Finding{{
			Name:     "nothing to validate",
			Severity: Failure,
			Detail: fmt.Sprintf(
				"No prediction had %d matches of history behind both players. Either the "+
					"database is empty or --min-matches is higher than any career in it.",
				rep.MinMatches),
		}}
	}

	for _, t := range rep.Accuracy {
		if t.Tier != "tour" {
			continue
		}
		switch {
		case t.Accuracy < tourAccuracyFloor:
			out = append(out, Finding{
				Name:     "tour-level accuracy below the band",
				Severity: Warning,
				Detail: fmt.Sprintf(
					"%.1f%% against the %.0f-%.0f%% the spec expects at tour level. The engine "+
						"is calling fewer matches than a working Elo should.",
					t.Accuracy, tourAccuracyFloor, tourAccuracyCeiling),
			})
		case t.Accuracy > tourAccuracyCeiling:
			out = append(out, Finding{
				Name:     "tour-level accuracy above the band",
				Severity: Info,
				Detail: fmt.Sprintf(
					"%.1f%%, above the %.0f-%.0f%% band. Worth checking the prediction is not "+
						"reading anything from after the match it is predicting.",
					t.Accuracy, tourAccuracyFloor, tourAccuracyCeiling),
			})
		default:
			out = append(out, Finding{
				Name:     "tour-level accuracy in band",
				Severity: Info,
				Detail: fmt.Sprintf("%.1f%%, inside the expected %.0f-%.0f%%.",
					t.Accuracy, tourAccuracyFloor, tourAccuracyCeiling),
			})
		}
	}

	for _, t := range rep.Calibration {
		if t.MeanError <= calibrationSlack {
			continue
		}
		out = append(out, Finding{
			Name:     "calibration drifts at " + t.Tier,
			Severity: Warning,
			Detail: fmt.Sprintf(
				"predicted and observed differ by %.1f points on average. Of the matches this "+
					"tier called at a given probability, that many fewer or more were won.",
				t.MeanError),
		})
	}

	// Mean reversion is reported rather than judged against a target: under one
	// weighted pool the whole-pool mean is the number that should sit near the
	// base, and tour players standing above it is the design working.
	drift := rep.Reversion.Pool - rep.Reversion.Base
	severity := Info
	if math.Abs(drift) > reversionSlack {
		severity = Warning
	}
	out = append(out, Finding{
		Name:     "pool mean against the base rating",
		Severity: severity,
		Detail: fmt.Sprintf(
			"%.0f against a base of %.0f, %+.0f. Measured over each player once, not over "+
				"each snapshot: averaging the rows would count a player who was rated for "+
				"four hundred weeks four hundred times. Tour players sit at %.0f, above the "+
				"pool, which is what one weighted pool is supposed to look like.",
			rep.Reversion.Pool, rep.Reversion.Base, drift, rep.Reversion.Tour),
	})

	p := rep.Promotion
	switch {
	case p.Matches == 0:
		out = append(out, Finding{
			Name:     "no promotions to measure",
			Severity: Warning,
			Detail: "No player reached tour level after a long enough career below it. " +
				"The most sensitive check on the tier weights had nothing to run on.",
		})
	case p.Systematic && p.Z > 0:
		out = append(out, Finding{
			Name:     "promoted players beat their rating",
			Severity: Warning,
			Detail: fmt.Sprintf(
				"%d won of %d expected across %d first tour-level matches, z=%+.1f. Promoted "+
					"players arrive underrated. Weighting the lower tiers too low looks like "+
					"this, and so does promotion selecting for players who are improving.",
				p.Actual, int(math.Round(p.Expected)), p.Matches, p.Z),
		})
	case p.Systematic:
		out = append(out, Finding{
			Name:     "promoted players fall short of their rating",
			Severity: Warning,
			Detail: fmt.Sprintf(
				"%d won of %d expected across %d first tour-level matches, z=%+.1f. Promoted "+
					"players arrive overrated, which is what weighting the lower tiers too "+
					"high looks like.",
				p.Actual, int(math.Round(p.Expected)), p.Matches, p.Z),
		})
	default:
		out = append(out, Finding{
			Name:     "promotion is continuous",
			Severity: Info,
			Detail: fmt.Sprintf(
				"%d won of %d expected across %d first tour-level matches, z=%+.1f, inside the "+
					"plus or minus %.0f threshold. Nothing says the tier weights are wrong.",
				p.Actual, int(math.Round(p.Expected)), p.Matches, p.Z, p.Threshold),
		})
	}

	return out
}

// WriteJSON emits the report for a machine.
func (r Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// table is a tabwriter that carries its first write error to the end rather
// than asking every row to check one. In practice it writes to a
// strings.Builder and cannot fail, but a discarded error is a discarded error.
type table struct {
	w   *tabwriter.Writer
	err error
}

func newTable(w io.Writer) *table {
	return &table{w: tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)}
}

func (t *table) row(format string, args ...any) {
	if t.err != nil {
		return
	}
	_, t.err = fmt.Fprintf(t.w, format, args...)
}

func (t *table) flush() error {
	if t.err != nil {
		return t.err
	}
	return t.w.Flush()
}

// WriteText emits the report for a person.
func (r Report) WriteText(w io.Writer) error {
	var b strings.Builder

	fmt.Fprintf(&b, "Rating validation\n")
	fmt.Fprintf(&b, "%s, %d matches replayed, %d predictions scored, took %s\n\n",
		r.GeneratedAt.Format("2006-01-02 15:04 UTC"), r.Matches, r.Scored, r.Took)

	fmt.Fprintf(&b, "Weights: slam final %.2f, slam %.2f, finals %.2f, masters %.2f, tour %.2f, "+
		"team %.2f, challenger %.2f, futures %.2f, qualifying x%.2f\n\n",
		r.Weights.GrandSlamFinal, r.Weights.GrandSlam, r.Weights.TourFinals, r.Weights.Masters,
		r.Weights.Tour, r.Weights.TeamEvent, r.Weights.Challenger, r.Weights.Futures,
		r.Weights.Qualifying)

	acc := newTable(&b)
	acc.row("PREDICTIVE ACCURACY\ttier\tmatches\tcorrect\taccuracy\n")
	for _, t := range r.Accuracy {
		acc.row("\t%s\t%d\t%d\t%.1f%%\n", t.Tier, t.Matches, t.Correct, t.Accuracy)
	}
	if err := acc.flush(); err != nil {
		return err
	}
	fmt.Fprintln(&b)

	fmt.Fprintf(&b, "Reported per tier rather than blended: the %.0f-%.0f%% target is a "+
		"tour-level one and a single number would hide where it does not hold.\n\n",
		tourAccuracyFloor, tourAccuracyCeiling)

	for _, t := range r.Calibration {
		fmt.Fprintf(&b, "CALIBRATION, %s\n", t.Tier)
		cal := newTable(&b)
		cal.row("\tband\tmatches\tpredicted\tobserved\tgap\n")
		for _, bucket := range t.Buckets {
			cal.row("\t%.0f-%.0f%%\t%d\t%.1f%%\t%.1f%%\t%+.1f\n",
				bucket.From*100, bucket.To*100, bucket.Matches,
				bucket.Predicted, bucket.Observed, bucket.Observed-bucket.Predicted)
		}
		cal.row("\tmean gap\t\t\t\t%.1f\n", t.MeanError)
		if err := cal.flush(); err != nil {
			return err
		}
		fmt.Fprintln(&b)
	}

	fmt.Fprintf(&b, "MEAN REVERSION\n  pool  %.0f over %d players\n  tour  %.0f over %d players\n"+
		"  base  %.0f\n\n",
		r.Reversion.Pool, r.Reversion.PoolPlayers,
		r.Reversion.Tour, r.Reversion.TourPlayers, r.Reversion.Base)

	p := r.Promotion
	fmt.Fprintf(&b, "PROMOTION CONTINUITY\n  %d promotions, %d matches measured\n"+
		"  expected %.1f wins, actual %d, z=%+.2f (systematic beyond %.0f)\n\n",
		p.Promotions, p.Matches, p.Expected, p.Actual, p.Z, p.Threshold)

	fmt.Fprintln(&b, "FINDINGS")
	for _, f := range r.Findings {
		fmt.Fprintf(&b, "  [%s] %s\n    %s\n", f.Severity, f.Name, wrap(f.Detail, 74, "    "))
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// wrap breaks a detail line so a terminal does not have to.
func wrap(s string, width int, indent string) string {
	var out strings.Builder
	line := 0
	for i, word := range strings.Fields(s) {
		if line+len(word)+1 > width && i > 0 {
			out.WriteString("\n" + indent)
			line = 0
		} else if i > 0 {
			out.WriteString(" ")
			line++
		}
		out.WriteString(word)
		line += len(word)
	}
	return out.String()
}
