package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// historyMatch writes one match with everything the history endpoint reads:
// its own date, surface, round and score, rather than the season-shaped
// defaults the profile tests are happy with.
func (f *apiFixture) historyMatch(tournamentID, winner, loser int64, num int,
	round, surface, date, score string, servePoints *int16) {
	f.t.Helper()

	var nullableSurface *string
	if surface != "" {
		nullableSurface = &surface
	}

	var matchID int64
	err := f.tx.QueryRow(f.ctx, `
		INSERT INTO matches (tournament_id, match_num, round, best_of, surface, winner_id,
		                     loser_id, played_on, score, incomplete, is_qualifying,
		                     has_detailed_stats, source)
		VALUES ($1, $2, $3, 3, $4::surface, $5, $6, $7::date, $8, $9, $3 LIKE 'Q%', $10, 'test')
		RETURNING id`,
		tournamentID, num, round, nullableSurface, winner, loser, date, score,
		score == "6-4 RET", servePoints != nil).Scan(&matchID)
	if err != nil {
		f.t.Fatalf("insert match: %v", err)
	}

	for _, p := range []struct {
		id  int64
		won bool
	}{{winner, true}, {loser, false}} {
		// first_in and first_won sit below serve_points by a check constraint,
		// so they are derived rather than reused.
		var firstIn, firstWon *int16
		if servePoints != nil {
			in := *servePoints * 6 / 10
			won := in * 3 / 4
			firstIn, firstWon = &in, &won
		}
		if _, err := f.tx.Exec(f.ctx, `
			INSERT INTO match_players (match_id, player_id, won, aces, double_faults,
			                           serve_points, first_in, first_won, second_won,
			                           bp_saved, bp_faced)
			VALUES ($1, $2, $3, 5, 2, $4, $5, $6, 10, 3, 4)`,
			matchID, p.id, p.won, servePoints, firstIn, firstWon); err != nil {
			f.t.Fatalf("insert match_player: %v", err)
		}
	}
}

func decodeMatches(t *testing.T, res *http.Response) Page[PlayerMatch] {
	t.Helper()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var page Page[PlayerMatch]
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatalf("decode matches: %v", err)
	}
	return page
}

func TestPlayerMatchesReturnsHistoryMostRecentFirst(t *testing.T) {
	f := newAPIFixture(t)
	pts := int16(80)

	player := f.player("itg-history", "Itg History", db.TourAtp)
	rival := f.player("itg-rival", "Itg Rival", db.TourAtp)
	open := f.tournament("itg-history-open", db.TierTour, 2019)

	f.historyMatch(open, player, rival, 1, "SF", "clay", "2019-05-02", "6-4 6-4", &pts)
	f.historyMatch(open, rival, player, 2, "F", "hard", "2021-06-02", "7-5 6-3", &pts)

	page := decodeMatches(t, f.get("/api/v1/players/itg-history/matches"))
	if len(page.Data) != 2 {
		t.Fatalf("got %d matches, want 2", len(page.Data))
	}
	if page.NextCursor != "" {
		t.Error("a short page offered a cursor")
	}

	first, second := page.Data[0], page.Data[1]
	if first.Date != "2021-06-02" || second.Date != "2019-05-02" {
		t.Errorf("order = %s then %s, want most recent first", first.Date, second.Date)
	}
	// The row is written from this player's side: they lost the later one.
	if first.Won || !second.Won {
		t.Errorf("results = %v then %v, want a loss then a win", first.Won, second.Won)
	}
	if first.Opponent.Slug != "itg-rival" || first.Opponent.Name != "Itg Rival" {
		t.Errorf("opponent = %+v, want the other player", first.Opponent)
	}
	if first.Surface == nil || *first.Surface != "hard" {
		t.Errorf("surface = %v, want hard", first.Surface)
	}
	if first.Score == nil || *first.Score != "7-5 6-3" {
		t.Errorf("score = %v, want the recorded one", first.Score)
	}
	if first.Round != "F" || first.Tier != "tour" || first.Season != 2019 {
		t.Errorf("round/tier/season = %s/%s/%d", first.Round, first.Tier, first.Season)
	}
	if first.Serve.Availability != AvailabilityRecorded || first.Serve.Aces == nil {
		t.Errorf("serve = %+v, want the recorded line", first.Serve)
	}
}

// Paging is the point of the cursor: the same date on every row is the case
// OFFSET and a non-total ordering key both get wrong.
func TestPlayerMatchesPagingNeverRepeatsOrSkips(t *testing.T) {
	f := newAPIFixture(t)
	player := f.player("itg-paged", "Itg Paged", db.TourAtp)
	rival := f.player("itg-paged-rival", "Itg Paged Rival", db.TourAtp)
	open := f.tournament("itg-paged-open", db.TierTour, 2019)

	// Ten matches, every one of them on the same day, so played_on alone cannot
	// separate them.
	const total = 10
	for i := 0; i < total; i++ {
		f.historyMatch(open, player, rival, i+1, "R32", "clay", "2019-05-02", "6-4 6-4", nil)
	}

	seen := map[string]int{}
	path := "/api/v1/players/itg-paged/matches?limit=3"
	for pages := 0; ; pages++ {
		if pages > total {
			t.Fatal("paging did not terminate")
		}
		page := decodeMatches(t, f.get(path))
		for _, m := range page.Data {
			seen[m.Date+" "+m.Round+" "+m.Opponent.Slug+" "+m.Tournament]++
		}
		if page.NextCursor == "" {
			break
		}
		path = "/api/v1/players/itg-paged/matches?limit=3&cursor=" + page.NextCursor
	}

	// Every row is distinguishable only by identity, so count what came back.
	var got int
	for _, n := range seen {
		got += n
	}
	if got != total {
		t.Errorf("paging returned %d rows, want %d with no repeats or gaps", got, total)
	}
}

func TestPlayerMatchesFiltersCompose(t *testing.T) {
	f := newAPIFixture(t)
	player := f.player("itg-filtered", "Itg Filtered", db.TourAtp)
	rival := f.player("itg-filter-rival", "Itg Filter Rival", db.TourAtp)
	other := f.player("itg-filter-other", "Itg Filter Other", db.TourAtp)
	tour := f.tournament("itg-filter-tour", db.TierTour, 2019)
	chall := f.tournament("itg-filter-chall", db.TierChallenger, 2021)

	f.historyMatch(tour, player, rival, 1, "F", "clay", "2019-05-02", "6-4 6-4", nil)
	f.historyMatch(tour, player, other, 2, "SF", "hard", "2019-05-02", "6-4 6-4", nil)
	f.historyMatch(chall, player, rival, 1, "F", "clay", "2021-06-02", "6-4 6-4", nil)

	for _, c := range []struct {
		query string
		want  int
	}{
		{"", 3},
		{"?surface=clay", 2},
		{"?tier=challenger", 1},
		{"?season=2019", 2},
		{"?opponent=itg-filter-rival", 2},
		// Composing narrows further rather than replacing.
		{"?surface=clay&season=2019", 1},
		{"?surface=clay&opponent=itg-filter-rival&tier=tour", 1},
		{"?surface=grass", 0},
		// A real player this one never faced is an empty page, not an error.
		{"?opponent=itg-nobody", 0},
	} {
		page := decodeMatches(t, f.get("/api/v1/players/itg-filtered/matches"+c.query))
		if len(page.Data) != c.want {
			t.Errorf("%q returned %d matches, want %d", c.query, len(page.Data), c.want)
		}
	}
}

func TestPlayerMatchesRejectsBadInput(t *testing.T) {
	f := newAPIFixture(t)
	f.player("itg-bad-input", "Itg Bad Input", db.TourAtp)

	for _, q := range []string{
		"?surface=mud",
		"?tier=elite",
		"?season=recently",
		"?season=12",
		"?limit=0",
		"?limit=1000",
		"?cursor=not-a-cursor",
	} {
		res := f.get("/api/v1/players/itg-bad-input/matches" + q)
		if res.StatusCode != http.StatusBadRequest {
			t.Errorf("%q returned %d, want 400", q, res.StatusCode)
			continue
		}
		if ct := res.Header.Get("Content-Type"); ct != ProblemContentType {
			t.Errorf("%q returned content type %q, want a problem document", q, ct)
		}
	}
}

func TestPlayerMatchesUnknownSlugIsNotFound(t *testing.T) {
	f := newAPIFixture(t)
	res := f.get("/api/v1/players/itg-no-such-player/matches")
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != ProblemContentType {
		t.Errorf("content type = %q, want a problem document", ct)
	}
}

// The standing rule, per row this time: absent is not zero, and the row says
// which kind of absent it is.
func TestPlayerMatchesStatisticsAreAbsentNotZero(t *testing.T) {
	f := newAPIFixture(t)
	pts := int16(90)

	player := f.player("itg-absent", "Itg Absent", db.TourAtp)
	rival := f.player("itg-absent-rival", "Itg Absent Rival", db.TourAtp)
	old := f.tournament("itg-absent-1969", db.TierTour, 1969)
	futures := f.tournament("itg-absent-futures", db.TierFutures, 2019)
	modern := f.tournament("itg-absent-modern", db.TierTour, 2019)

	f.historyMatch(old, player, rival, 1, "F", "grass", "1969-07-05", "6-4 6-4", nil)
	f.historyMatch(futures, player, rival, 1, "F", "hard", "2019-03-02", "6-4 6-4", nil)
	f.historyMatch(modern, player, rival, 1, "F", "hard", "2019-05-02", "6-4 6-4", &pts)

	page := decodeMatches(t, f.get("/api/v1/players/itg-absent/matches"))
	if len(page.Data) != 3 {
		t.Fatalf("got %d matches, want 3", len(page.Data))
	}

	byDate := map[string]MatchServe{}
	for _, m := range page.Data {
		byDate[m.Date] = m.Serve
	}
	for date, want := range map[string]string{
		"1969-07-05": AvailabilityNeverInEra,
		"2019-03-02": AvailabilityNeverForTier,
		"2019-05-02": AvailabilityRecorded,
	} {
		got := byDate[date]
		if got.Availability != want {
			t.Errorf("%s: availability = %q, want %q", date, got.Availability, want)
		}
		if want == AvailabilityRecorded {
			if got.ServePoints == nil || *got.ServePoints != pts {
				t.Errorf("%s: serve points = %v, want %d", date, got.ServePoints, pts)
			}
			continue
		}
		// Not a zero anywhere: the counts are absent.
		if got.ServePoints != nil || got.Aces != nil || got.BreakPointsFaced != nil {
			t.Errorf("%s: counts present on a match that recorded none: %+v", date, got)
		}
	}
}

// A match the source left without a surface still belongs in the history.
func TestPlayerMatchesKeepARowWithNoSurface(t *testing.T) {
	f := newAPIFixture(t)
	player := f.player("itg-nosurface", "Itg Nosurface", db.TourAtp)
	rival := f.player("itg-nosurface-rival", "Itg Nosurface Rival", db.TourAtp)
	open := f.tournament("itg-nosurface-open", db.TierTour, 1935)

	f.historyMatch(open, player, rival, 1, "F", "", "1935-05-02", "6-4 6-4", nil)

	page := decodeMatches(t, f.get("/api/v1/players/itg-nosurface/matches"))
	if len(page.Data) != 1 {
		t.Fatalf("got %d matches, want the one that was played", len(page.Data))
	}
	if page.Data[0].Surface != nil {
		t.Errorf("surface = %v, want null rather than a guess", *page.Data[0].Surface)
	}
}
