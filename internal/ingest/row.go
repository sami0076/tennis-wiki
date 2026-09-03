package ingest

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Stats are one player's serve statistics for one match. A nil *Stats means the
// source recorded none, which is the case before roughly 1991 and at Futures
// and ITF level in every year. It must never be represented as zeros: a zeroed
// ace count is wrong but plausible-looking, the worst failure this project has.
type Stats struct {
	Aces         int
	DoubleFaults int
	ServePoints  int
	FirstIn      int
	FirstWon     int
	SecondWon    int
	ServeGames   int
	BPSaved      int
	BPFaced      int
}

// Player is one side of a source match row.
type Player struct {
	SourceID   string
	Name       string
	Hand       string
	Country    string
	HeightCM   *int
	Age        *float64
	Seed       *int
	Entry      string
	Rank       *int
	RankPoints *int
	Stats      *Stats
}

// MatchRow is one source row, normalised across schema profiles. It stays
// winner/loser oriented because the source is; the split into the symmetric
// match_players form happens when it is written.
type MatchRow struct {
	TourneyID   string
	TourneyName string
	Surface     string
	Level       string
	DrawSize    *int
	TourneyDate time.Time
	MatchNum    int
	Round       string
	BestOf      int
	Score       string
	Minutes     *int
	Indoor      *bool
	Winner      Player
	Loser       Player
}

// HasDetailedStats reports whether both players' statistics were recorded.
func (m MatchRow) HasDetailedStats() bool {
	return m.Winner.Stats != nil && m.Loser.Stats != nil
}

// qualifyingRound matches Q1, Q2, Q3 and so on. It must not match QF, which is
// the quarterfinal of a main draw.
var qualifyingRound = regexp.MustCompile(`^Q\d+$`)

// IsQualifying reports whether the match belongs to a qualifying draw. The
// source bundles qualifying into the same file and the same tournament as the
// main draw, so this is derived from the round rather than the filename.
func (m MatchRow) IsQualifying() bool {
	return qualifyingRound.MatchString(strings.ToUpper(m.Round))
}

// parseRow reads one CSV record.
func parseRow(c *columns, rec []string) (MatchRow, error) {
	date, err := parseTourneyDate(c.get(rec, "tourney_date"))
	if err != nil {
		return MatchRow{}, err
	}
	matchNum, err := strconv.Atoi(c.get(rec, "match_num"))
	if err != nil {
		return MatchRow{}, fmt.Errorf("match_num %q: %w", c.get(rec, "match_num"), err)
	}

	m := MatchRow{
		TourneyID:   c.get(rec, "tourney_id"),
		TourneyName: c.get(rec, "tourney_name"),
		Surface:     normaliseSurface(c.get(rec, "surface")),
		Level:       c.get(rec, "tourney_level"),
		DrawSize:    optInt(c.get(rec, "draw_size")),
		TourneyDate: date,
		MatchNum:    matchNum,
		Round:       c.get(rec, "round"),
		BestOf:      atoiOr(c.get(rec, "best_of"), 3),
		Score:       c.get(rec, "score"),
		Minutes:     optInt(c.get(rec, "minutes")),
		Indoor:      parseIndoor(c, rec),
		Winner:      parsePlayer(c, rec, "winner", c.profile.winnerStat),
		Loser:       parsePlayer(c, rec, "loser", c.profile.loserStat),
	}

	if m.TourneyID == "" {
		return MatchRow{}, fmt.Errorf("row has no tourney_id")
	}
	if m.Winner.SourceID == "" || m.Loser.SourceID == "" {
		return MatchRow{}, fmt.Errorf("row %s/%d has a missing player id", m.TourneyID, m.MatchNum)
	}
	return m, nil
}

func parsePlayer(c *columns, rec []string, side, statPrefix string) Player {
	return Player{
		SourceID:   c.get(rec, side+"_id"),
		Name:       c.get(rec, side+"_name"),
		Hand:       normaliseHand(c.get(rec, side+"_hand")),
		Country:    strings.ToUpper(c.get(rec, side+"_ioc")),
		HeightCM:   optInt(c.get(rec, side+"_ht")),
		Age:        optFloat(c.get(rec, side+"_age")),
		Seed:       optInt(c.get(rec, side+"_seed")),
		Entry:      strings.ToUpper(c.get(rec, side+"_entry")),
		Rank:       optInt(c.get(rec, side+"_rank")),
		RankPoints: optInt(c.get(rec, side+"_rank_points")),
		Stats:      parseStats(c, rec, statPrefix),
	}
}

// parseStats returns nil unless the source actually recorded statistics. It
// keys off serve points: a row with no serve points has no usable stat line,
// whatever the other columns contain.
func parseStats(c *columns, rec []string, prefix string) *Stats {
	svpt := optInt(c.get(rec, prefix+"svpt"))
	if svpt == nil {
		return nil
	}
	return &Stats{
		Aces:         atoiOr(c.get(rec, prefix+"ace"), 0),
		DoubleFaults: atoiOr(c.get(rec, prefix+"df"), 0),
		ServePoints:  *svpt,
		FirstIn:      atoiOr(c.get(rec, prefix+"1stIn"), 0),
		FirstWon:     atoiOr(c.get(rec, prefix+"1stWon"), 0),
		SecondWon:    atoiOr(c.get(rec, prefix+"2ndWon"), 0),
		ServeGames:   atoiOr(c.get(rec, prefix+"SvGms"), 0),
		BPSaved:      atoiOr(c.get(rec, prefix+"bpSaved"), 0),
		BPFaced:      atoiOr(c.get(rec, prefix+"bpFaced"), 0),
	}
}

func parseIndoor(c *columns, rec []string) *bool {
	if !c.profile.hasIndoor || !c.has("indoor") {
		return nil
	}
	switch strings.ToUpper(c.get(rec, "indoor")) {
	case "I":
		t := true
		return &t
	case "O":
		f := false
		return &f
	default:
		return nil
	}
}

// parseTourneyDate reads the YYYYMMDD form the sources use.
func parseTourneyDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("row has no tourney_date")
	}
	t, err := time.Parse("20060102", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("tourney_date %q: %w", s, err)
	}
	return t, nil
}

// normaliseSurface lowercases the source's capitalised values. An empty or
// "None" surface stays empty: it is stored as NULL rather than guessed.
func normaliseSurface(s string) string {
	switch strings.ToLower(s) {
	case "hard", "clay", "grass", "carpet":
		return strings.ToLower(s)
	default:
		return ""
	}
}

func normaliseHand(s string) string {
	switch strings.ToUpper(s) {
	case "R", "L":
		return strings.ToUpper(s)
	case "U", "A":
		return "U"
	default:
		return ""
	}
}

// optInt returns nil for anything that is not a number, so an unrecorded value
// stays unrecorded rather than becoming zero.
func optInt(s string) *int {
	if s == "" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		// Heights and ranks occasionally arrive as "183.0".
		f, ferr := strconv.ParseFloat(s, 64)
		if ferr != nil {
			return nil
		}
		n = int(f)
	}
	return &n
}

func optFloat(s string) *float64 {
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &f
}

func atoiOr(s string, fallback int) int {
	if n := optInt(s); n != nil {
		return *n
	}
	return fallback
}
