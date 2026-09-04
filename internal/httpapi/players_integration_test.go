package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/sami0076/tennis-wiki/internal/db"
	"github.com/sami0076/tennis-wiki/internal/testdb"
)

// apiFixture runs the whole stack — router, middleware, generated queries —
// against a real database inside a transaction that is rolled back.
type apiFixture struct {
	handler http.Handler
	tx      pgx.Tx
	ctx     context.Context
	t       *testing.T
}

func newAPIFixture(t *testing.T) *apiFixture {
	t.Helper()
	dsn := testdb.Start(t)
	ctx := context.Background()
	pool, err := db.Open(ctx, db.Config{DSN: dsn, MaxConns: 2, MinConns: 1})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	cfg := Config{CORSOrigins: []string{"http://localhost:5173"}, RateLimitPerMin: 600}
	return &apiFixture{
		handler: New(db.New(tx), discardLogger(), cfg).Router(),
		tx:      tx, ctx: ctx, t: t,
	}
}

func (f *apiFixture) get(path string) *http.Response {
	f.t.Helper()
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec.Result()
}

func (f *apiFixture) player(slug, fullName string, tour db.Tour) int64 {
	f.t.Helper()
	var id int64
	err := f.tx.QueryRow(f.ctx,
		`INSERT INTO players (source_id, tour, slug, full_name, country, hand)
		 VALUES ($1, $2, $1, $3, 'SRB', 'R') RETURNING id`,
		slug, tour, fullName).Scan(&id)
	if err != nil {
		f.t.Fatalf("insert player %s: %v", slug, err)
	}
	return id
}

func (f *apiFixture) tournament(sourceID string, tier db.Tier, season int) int64 {
	return f.tourTournament(sourceID, tier, season, db.TourAtp)
}

func (f *apiFixture) tourTournament(sourceID string, tier db.Tier, season int, tour db.Tour) int64 {
	f.t.Helper()
	var id int64
	err := f.tx.QueryRow(f.ctx,
		`INSERT INTO tournaments (source_id, tour, name, level, tier, surface, start_date, season)
		 VALUES ($1, $4, $1, 'A', $2, 'clay', make_date($3, 5, 1), $3) RETURNING id`,
		sourceID, tier, season, tour).Scan(&id)
	if err != nil {
		f.t.Fatalf("insert tournament: %v", err)
	}
	return id
}

// match writes one match. servePoints nil means the era or tier recorded nothing.
func (f *apiFixture) match(tournamentID, winnerID, loserID int64, num int, round string,
	season int, servePoints *int16, incomplete bool) {
	f.t.Helper()
	var matchID int64
	err := f.tx.QueryRow(f.ctx,
		`INSERT INTO matches (tournament_id, match_num, round, best_of, surface, winner_id,
		                      played_on, incomplete, has_detailed_stats, source)
		 VALUES ($1, $2, $3, 3, 'clay', $4, make_date($5, 5, 2), $6, $7, 'test')
		 RETURNING id`,
		tournamentID, num, round, winnerID, season, incomplete, servePoints != nil).Scan(&matchID)
	if err != nil {
		f.t.Fatalf("insert match: %v", err)
	}
	for _, p := range []struct {
		id  int64
		won bool
	}{{winnerID, true}, {loserID, false}} {
		// first_in and first_won are held below serve_points by a check
		// constraint, so they are derived rather than reused.
		var firstIn, firstWon *int16
		if servePoints != nil {
			in := *servePoints * 6 / 10
			won := in * 3 / 4
			firstIn, firstWon = &in, &won
		}
		if _, err := f.tx.Exec(f.ctx,
			`INSERT INTO match_players (match_id, player_id, won, aces, double_faults,
			                            serve_points, first_in, first_won, second_won,
			                            bp_saved, bp_faced)
			 VALUES ($1, $2, $3, 0, 2, $4, $5, $6, 10, 0, 0)`,
			matchID, p.id, p.won, servePoints, firstIn, firstWon); err != nil {
			f.t.Fatalf("insert match_player: %v", err)
		}
	}
}

func decodeProfile(t *testing.T, res *http.Response) PlayerProfile {
	t.Helper()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var p PlayerProfile
	if err := json.NewDecoder(res.Body).Decode(&p); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	return p
}

// The Phase 1 completion criterion, for both tours.
func TestPlayerProfileReturnsRealData(t *testing.T) {
	f := newAPIFixture(t)
	pts := int16(100)

	atp := f.player("itg-atp-player", "Itg Atpplayer", db.TourAtp)
	wta := f.player("itg-wta-player", "Itg Wtaplayer", db.TourWta)
	foil := f.player("itg-foil", "Itg Foil", db.TourAtp)
	open := f.tournament("itg-open", db.TierTour, 2019)

	f.match(open, atp, foil, 1, "SF", 2019, &pts, false)
	f.match(open, atp, foil, 2, "F", 2019, &pts, false)
	f.match(open, wta, foil, 3, "F", 2019, &pts, false)

	for _, c := range []struct {
		slug  string
		tour  string
		wins  int64
		title int64
	}{
		{"itg-atp-player", "atp", 2, 1},
		{"itg-wta-player", "wta", 1, 1},
	} {
		t.Run(c.slug, func(t *testing.T) {
			got := decodeProfile(t, f.get("/api/v1/players/"+c.slug))
			if got.Tour != c.tour {
				t.Errorf("tour = %q, want %q", got.Tour, c.tour)
			}
			if got.Career == nil {
				t.Fatal("career is null for a player with matches")
			}
			if got.Career.Wins != c.wins || got.Career.Titles != c.title {
				t.Errorf("wins=%d titles=%d, want %d/%d",
					got.Career.Wins, got.Career.Titles, c.wins, c.title)
			}
			if got.Serve.Availability != AvailabilityRecorded || got.Serve.Rates == nil {
				t.Errorf("serve = %+v, want recorded rates", got.Serve)
			}
			if len(got.Career.Surfaces) == 0 || got.Career.Surfaces[0].Surface != "clay" {
				t.Errorf("surfaces = %+v, want a clay split", got.Career.Surfaces)
			}
		})
	}
}

// A pre-1991 player has matches and no statistics. The response must say so
// rather than return zeroes the page would render as real numbers.
func TestPreStatisticalEraPlayerHasAbsentNotZeroStats(t *testing.T) {
	f := newAPIFixture(t)
	laver := f.player("itg-laver", "Itg Laver", db.TourAtp)
	foil := f.player("itg-rosewall", "Itg Rosewall", db.TourAtp)
	old := f.tournament("itg-old-open", db.TierTour, 1969)

	f.match(old, laver, foil, 1, "F", 1969, nil, false)

	got := decodeProfile(t, f.get("/api/v1/players/itg-laver"))
	if got.Career == nil || got.Career.Wins != 1 {
		t.Fatalf("career = %+v, want a recorded win", got.Career)
	}
	if got.Serve.Availability != AvailabilityNeverInEra {
		t.Errorf("availability = %q, want %q", got.Serve.Availability, AvailabilityNeverInEra)
	}
	if got.Serve.Rates != nil {
		t.Errorf("rates = %+v, want null: nothing was recorded in 1969", got.Serve.Rates)
	}
	if got.Serve.MatchesWithData != 0 {
		t.Errorf("matches_with_data = %d, want 0", got.Serve.MatchesWithData)
	}
}

// The majority case at full depth: a Futures player whose level has never
// recorded statistics in any year.
func TestFuturesPlayerReportsATierAbsence(t *testing.T) {
	f := newAPIFixture(t)
	player := f.player("itg-journeyman", "Itg Journeyman", db.TourAtp)
	foil := f.player("itg-journeyman-foil", "Itg Journeyfoil", db.TourAtp)
	small := f.tournament("itg-futures", db.TierFutures, 2019)

	f.match(small, player, foil, 1, "R32", 2019, nil, false)

	got := decodeProfile(t, f.get("/api/v1/players/itg-journeyman"))
	if got.Serve.Availability != AvailabilityNeverForTier {
		t.Errorf("availability = %q, want %q", got.Serve.Availability, AvailabilityNeverForTier)
	}
	if len(got.Career.Tiers) != 1 || got.Career.Tiers[0].Tier != "futures" {
		t.Errorf("tiers = %+v, want one futures row", got.Career.Tiers)
	}
	if got.Career.Tiers[0].MatchesWithStats != 0 {
		t.Error("a futures match must not claim recorded statistics")
	}
}

// Retirements count in the win/loss record but are excluded from the rates.
func TestRetirementsCountedButNotAveraged(t *testing.T) {
	f := newAPIFixture(t)
	pts := int16(100)
	player := f.player("itg-retiree", "Itg Retiree", db.TourAtp)
	foil := f.player("itg-retiree-foil", "Itg Retirefoil", db.TourAtp)
	open := f.tournament("itg-ret-open", db.TierTour, 2019)

	f.match(open, player, foil, 1, "R16", 2019, &pts, false)
	f.match(open, player, foil, 2, "QF", 2019, &pts, true) // retirement

	got := decodeProfile(t, f.get("/api/v1/players/itg-retiree"))
	if got.Career.Wins != 2 {
		t.Errorf("wins = %d, want 2: a retirement is still a win", got.Career.Wins)
	}
	if got.Career.IncompleteMatches != 1 {
		t.Errorf("incomplete = %d, want 1 and visible", got.Career.IncompleteMatches)
	}
	if got.Serve.MatchesWithData != 1 {
		t.Errorf("matches_with_data = %d, want 1: the retirement is excluded",
			got.Serve.MatchesWithData)
	}
	if got.Serve.Availability != AvailabilityRecorded {
		t.Errorf("availability = %q; a retirement should not make the record partial",
			got.Serve.Availability)
	}
}

func TestPlayerWithNoMatchesHasNullCareer(t *testing.T) {
	f := newAPIFixture(t)
	f.player("itg-unplayed", "Itg Unplayed", db.TourWta)

	got := decodeProfile(t, f.get("/api/v1/players/itg-unplayed"))
	if got.Career != nil {
		t.Errorf("career = %+v, want null rather than a 0-0 record", got.Career)
	}
	if got.Name != "Itg Unplayed" {
		t.Errorf("name = %q", got.Name)
	}
}

// Diacritics and misspellings both have to find the stored ASCII name.
func TestSearchHandlesDiacriticsAndMisspellings(t *testing.T) {
	f := newAPIFixture(t)
	f.player("itg-zzokovic", "Zzokovic Novak", db.TourAtp)

	for _, query := range []string{"zzokovic", "Zzokovič", "ZZOKOVIC", "zzokovi"} {
		t.Run(query, func(t *testing.T) {
			res := f.get("/api/v1/players?q=" + url.QueryEscape(query))
			if res.StatusCode != http.StatusOK {
				t.Fatalf("status = %d", res.StatusCode)
			}
			var page Page[PlayerSearchResult]
			if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if len(page.Data) == 0 {
				t.Fatalf("%q found nothing", query)
			}
			if page.Data[0].Slug != "itg-zzokovic" {
				t.Errorf("first result = %q, want itg-zzokovic", page.Data[0].Slug)
			}
		})
	}
}

func TestSearchPagesWithACursor(t *testing.T) {
	f := newAPIFixture(t)
	for _, suffix := range []string{"aa", "bb", "cc"} {
		f.player("itg-yyandom-"+suffix, "Yyandom "+suffix, db.TourAtp)
	}

	res := f.get("/api/v1/players?q=yyandom&limit=2")
	var first Page[PlayerSearchResult]
	if err := json.NewDecoder(res.Body).Decode(&first); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(first.Data) != 2 || first.NextCursor == "" {
		t.Fatalf("first page = %d rows, cursor %q", len(first.Data), first.NextCursor)
	}

	res = f.get("/api/v1/players?q=yyandom&limit=2&cursor=" + first.NextCursor)
	var second Page[PlayerSearchResult]
	if err := json.NewDecoder(res.Body).Decode(&second); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, row := range second.Data {
		for _, seen := range first.Data {
			if row.Slug == seen.Slug {
				t.Errorf("%s appears on both pages", row.Slug)
			}
		}
	}
	// The last page must not offer a cursor, or a client loops forever.
	if second.NextCursor != "" {
		t.Errorf("last page offered cursor %q", second.NextCursor)
	}
}

func TestSearchFiltersByTourEndToEnd(t *testing.T) {
	f := newAPIFixture(t)
	f.player("itg-xxfilter-atp", "Xxfilter Man", db.TourAtp)
	f.player("itg-xxfilter-wta", "Xxfilter Woman", db.TourWta)

	res := f.get("/api/v1/players?q=xxfilter&tour=wta")
	var page Page[PlayerSearchResult]
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(page.Data) != 1 || page.Data[0].Tour != "wta" {
		t.Errorf("tour filter returned %+v", page.Data)
	}
}

// A profile is revalidatable, which is most of the point of caching a dataset
// that barely changes.
func TestProfileIsRevalidatable(t *testing.T) {
	f := newAPIFixture(t)
	f.player("itg-etag", "Itg Etag", db.TourAtp)

	first := f.get("/api/v1/players/itg-etag")
	tag := first.Header.Get("ETag")
	if tag == "" {
		t.Fatal("no ETag on a player profile")
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/players/itg-etag", nil)
	req.Header.Set("If-None-Match", tag)
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotModified {
		t.Errorf("status = %d, want 304", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("304 carried %d bytes", rec.Body.Len())
	}
}
