package ingest

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/sami0076/tennis-wiki/internal/score"
)

// classified is what the score string tells us about how a match ended.
type classified struct {
	incomplete bool
	sets       []score.Set
	// The clutch columns, nil where the score could not be read or the match
	// did not finish. Never zero for those: no tiebreaks and no readable score
	// are different facts, and the second one must not average as the first.
	tiebreaksWinner, tiebreaksLoser *int16
	decidingSet                     *bool
}

// classifyScore parses the score, tolerating anything the parser cannot read.
// An unreadable score is a data-quality fact for cmd/dataqual to report, not a
// reason to fail the row.
func classifyScore(raw string, bestOf int) classified {
	s, err := score.Parse(raw)
	if err != nil {
		return classified{incomplete: true}
	}

	out := classified{incomplete: s.Incomplete(), sets: s.Sets}
	if out.incomplete {
		return out
	}

	winner, loser := s.Tiebreaks()
	decider := s.WentToDecider(bestOf)
	w, l := int16(winner), int16(loser)
	out.tiebreaksWinner, out.tiebreaksLoser, out.decidingSet = &w, &l, &decider
	return out
}

// teamEventLevels are the source's codes for Davis Cup and Billie Jean King
// Cup. These are ingested but flagged, and excluded from Elo by default.
var teamEventLevels = map[string]struct{}{
	"D": {}, // Davis Cup
	"T": {}, // team events in some WTA files
}

func isTeamEvent(level string) bool {
	_, ok := teamEventLevels[strings.ToUpper(strings.TrimSpace(level))]
	return ok
}

// firstName and lastName split a display name. The source writes "First Last"
// with no reliable delimiter for multi-part surnames, so the split is a best
// effort that the full player table in #18 later corrects.
func firstName(full string) string {
	parts := strings.Fields(full)
	if len(parts) < 2 {
		return ""
	}
	return parts[0]
}

func lastName(full string) string {
	parts := strings.Fields(full)
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[1:], " ")
}

// isUniqueViolation reports whether err is a unique constraint violation on the
// named constraint.
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	const uniqueViolation = "23505"
	return pgErr.Code == uniqueViolation &&
		(constraint == "" || pgErr.ConstraintName == constraint)
}
