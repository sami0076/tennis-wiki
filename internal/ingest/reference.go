package ingest

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RefKind distinguishes the two kinds of reference file. Neither is seasonal,
// which is why they do not go through Source.
type RefKind string

const (
	// RefPlayers is a player table: one row per player, with biography.
	RefPlayers RefKind = "players"
	// RefRankings is ranking history, split across decade files.
	RefRankings RefKind = "rankings"
)

// RefSource is a player table or ranking history file family.
type RefSource struct {
	Name    string  `json:"name"`
	Tour    Tour    `json:"tour"`
	Kind    RefKind `json:"kind"`
	BaseURL string  `json:"base_url"`
	// Path is a template; ranking paths contain {decade}.
	Path string `json:"path"`
	// Decades expands {decade}, e.g. "70s", "20s", "current". Empty for a
	// player table, which is a single file.
	Decades     []string `json:"decades,omitempty"`
	Attribution string   `json:"attribution"`
}

// RelPaths expands the path template into every file this source covers.
func (r RefSource) RelPaths() []string {
	if len(r.Decades) == 0 {
		return []string{r.Path}
	}
	out := make([]string, 0, len(r.Decades))
	for _, d := range r.Decades {
		out = append(out, strings.ReplaceAll(r.Path, "{decade}", d))
	}
	return out
}

func (r RefSource) validate() error {
	switch {
	case r.Name == "":
		return fmt.Errorf("reference source with no name")
	case r.Tour != TourATP && r.Tour != TourWTA:
		return fmt.Errorf("reference source %q: unknown tour %q", r.Name, r.Tour)
	case r.Kind != RefPlayers && r.Kind != RefRankings:
		return fmt.Errorf("reference source %q: unknown kind %q", r.Name, r.Kind)
	case r.BaseURL == "" || r.Path == "":
		return fmt.Errorf("reference source %q: base_url and path are required", r.Name)
	case r.Kind == RefRankings && len(r.Decades) == 0:
		return fmt.Errorf("reference source %q: ranking sources need decades", r.Name)
	case r.Attribution == "":
		return fmt.Errorf("reference source %q: attribution is required", r.Name)
	}
	return nil
}

// PlayerBio is one row of a player table.
type PlayerBio struct {
	SourceID   string
	FirstName  string
	LastName   string
	Hand       string
	BirthDate  *time.Time
	Country    string
	HeightCm   *int16
	WikidataID string
}

// FullName is the canonical display name. The player table is where it lives;
// match files repeat an abbreviated form.
func (p PlayerBio) FullName() string {
	return strings.TrimSpace(strings.TrimSpace(p.FirstName) + " " + strings.TrimSpace(p.LastName))
}

// RankingEntry is one row of ranking history.
type RankingEntry struct {
	Date     time.Time
	Rank     int32
	SourceID string
	Points   *int32
}

// parseCompactDate reads the YYYYMMDD form the sources use.
//
// It has to reject more than it accepts: the player tables carry 19000000 and
// similar placeholders for "born, date unknown", which time.Parse would refuse
// anyway, and a few real-looking dates with a zero month or day.
func parseCompactDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if len(s) != 8 {
		return time.Time{}, false
	}
	year, err := strconv.Atoi(s[0:4])
	if err != nil {
		return time.Time{}, false
	}
	month, err := strconv.Atoi(s[4:6])
	if err != nil || month < 1 || month > 12 {
		return time.Time{}, false
	}
	day, err := strconv.Atoi(s[6:8])
	if err != nil || day < 1 || day > 31 {
		return time.Time{}, false
	}
	if year < 1850 || year > time.Now().Year()+1 {
		return time.Time{}, false
	}
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	// Rejects 31 February and friends, which round forward silently.
	if t.Day() != day || int(t.Month()) != month {
		return time.Time{}, false
	}
	return t, true
}
