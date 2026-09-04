package db

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// harness owns one rolled-back transaction. Nothing these tests write is ever
// committed, so they are safe against any migrated database and need no
// truncation.
type harness struct {
	*Queries
	tx  pgx.Tx
	ctx context.Context
	t   *testing.T
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx := context.Background()
	pool, err := Open(ctx, Config{DSN: dsn, MaxConns: 2, MinConns: 1})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	return &harness{Queries: New(tx), tx: tx, ctx: ctx, t: t}
}

func (h *harness) player(slug, name string, tour Tour) int64 {
	h.t.Helper()
	var id int64
	err := h.tx.QueryRow(h.ctx,
		`INSERT INTO players (source_id, tour, slug, full_name, country)
		 VALUES ($1, $2, $3, $4, 'SRB') RETURNING id`,
		slug, tour, slug, name).Scan(&id)
	if err != nil {
		h.t.Fatalf("insert player %s: %v", slug, err)
	}
	return id
}

func (h *harness) tournament(name string, tier Tier, season int) int64 {
	h.t.Helper()
	var id int64
	err := h.tx.QueryRow(h.ctx,
		`INSERT INTO tournaments (source_id, tour, name, level, tier, surface, start_date, season)
		 VALUES ($1, 'atp', $2, 'A', $3, 'hard', make_date($4, 1, 1), $4) RETURNING id`,
		name, name, tier, season).Scan(&id)
	if err != nil {
		h.t.Fatalf("insert tournament: %v", err)
	}
	return id
}

// match writes a match and both participants. A nil servePoints marks the era
// where nothing was recorded, which must stay distinct from a recorded zero.
func (h *harness) match(tournamentID, winnerID, loserID int64, num int, round string,
	servePoints *int16, incomplete bool) {
	h.t.Helper()
	var matchID int64
	err := h.tx.QueryRow(h.ctx,
		`INSERT INTO matches (tournament_id, match_num, round, best_of, surface, winner_id,
		                      played_on, incomplete, has_detailed_stats, source)
		 VALUES ($1, $2, $3, 3, 'hard', $4, make_date(2019, 6, 1), $5, $6, 'test')
		 RETURNING id`,
		tournamentID, num, round, winnerID, incomplete, servePoints != nil).Scan(&matchID)
	if err != nil {
		h.t.Fatalf("insert match: %v", err)
	}
	for _, p := range []struct {
		id  int64
		won bool
	}{{winnerID, true}, {loserID, false}} {
		if _, err := h.tx.Exec(h.ctx,
			`INSERT INTO match_players (match_id, player_id, won, aces, serve_points, first_in, first_won)
			 VALUES ($1, $2, $3, $4, $4, $4, $4)`,
			matchID, p.id, p.won, servePoints); err != nil {
			h.t.Fatalf("insert match_player: %v", err)
		}
	}
}

func TestHealthTouchesARealTable(t *testing.T) {
	h := newHarness(t)
	if _, err := h.Health(h.ctx); err != nil {
		t.Fatalf("Health: %v", err)
	}
}

func TestGetPlayerBySlug(t *testing.T) {
	h := newHarness(t)
	h.player("test-djokovic", "Novak Djokovic", TourAtp)

	got, err := h.GetPlayerBySlug(h.ctx, "test-djokovic")
	if err != nil {
		t.Fatalf("GetPlayerBySlug: %v", err)
	}
	if got.FullName != "Novak Djokovic" || got.Tour != TourAtp {
		t.Errorf("got %+v", got)
	}
	// Absent biography fields must arrive as nil, not as zero values.
	if got.Hand != nil || got.BirthDate != nil {
		t.Errorf("absent biography should be nil, got hand=%v birth=%v", got.Hand, got.BirthDate)
	}

	if _, err := h.GetPlayerBySlug(h.ctx, "no-such-player"); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("unknown slug returned %v, want pgx.ErrNoRows", err)
	}
}

func TestCareerSummaryCountsWinsAndTitles(t *testing.T) {
	h := newHarness(t)
	winner := h.player("test-winner", "Test Winner", TourAtp)
	loser := h.player("test-loser", "Test Loser", TourAtp)
	open := h.tournament("Test Open", TierTour, 2019)

	pts := int16(70)
	h.match(open, winner, loser, 1, "SF", &pts, false)
	h.match(open, winner, loser, 2, "F", &pts, false)
	// A retirement counts in win/loss but must not reach the rate statistics.
	h.match(open, winner, loser, 3, "R16", &pts, true)

	got, err := h.GetPlayerCareerSummary(h.ctx, winner)
	if err != nil {
		t.Fatalf("GetPlayerCareerSummary: %v", err)
	}
	if got.Matches != 3 || got.Wins != 3 || got.Losses != 0 {
		t.Errorf("matches=%d wins=%d losses=%d, want 3/3/0", got.Matches, got.Wins, got.Losses)
	}
	if got.Titles != 1 {
		t.Errorf("titles = %d, want 1: only the final", got.Titles)
	}
	if got.IncompleteMatches != 1 {
		t.Errorf("incomplete = %d, want 1", got.IncompleteMatches)
	}
	if got.StatMatches != 2 {
		t.Errorf("stat_matches = %d, want 2: the retirement is excluded from rates", got.StatMatches)
	}
	if got.ServePoints != 140 {
		t.Errorf("serve_points = %d, want 140 from the two complete matches", got.ServePoints)
	}
	if got.FirstMatch.IsZero() || got.LastMatch.After(time.Now()) {
		t.Errorf("bad date range %v..%v", got.FirstMatch, got.LastMatch)
	}
}

// The pre-1991 and Futures case: matches exist, statistics never did. This must
// report zero recorded matches rather than fail to scan or imply real zeroes.
func TestCareerSummaryWithNoRecordedStatistics(t *testing.T) {
	h := newHarness(t)
	winner := h.player("test-laver", "Rod Laver", TourAtp)
	loser := h.player("test-rosewall", "Ken Rosewall", TourAtp)
	old := h.tournament("Old Open", TierFutures, 1969)

	h.match(old, winner, loser, 1, "F", nil, false)

	got, err := h.GetPlayerCareerSummary(h.ctx, winner)
	if err != nil {
		t.Fatalf("GetPlayerCareerSummary: %v", err)
	}
	if got.Matches != 1 || got.Wins != 1 {
		t.Errorf("matches=%d wins=%d, want 1/1", got.Matches, got.Wins)
	}
	if got.StatMatches != 0 {
		t.Fatalf("stat_matches = %d, want 0: nothing was ever recorded", got.StatMatches)
	}
	if got.Aces != 0 || got.ServePoints != 0 {
		t.Errorf("sums over no rows should be zero, got aces=%d serve=%d", got.Aces, got.ServePoints)
	}

	splits, err := h.GetPlayerTierSplits(h.ctx, winner)
	if err != nil {
		t.Fatalf("GetPlayerTierSplits: %v", err)
	}
	if len(splits) != 1 || splits[0].Tier != TierFutures || splits[0].MatchesWithStats != 0 {
		t.Errorf("tier splits = %+v, want one futures row with no recorded stats", splits)
	}
}

// A player with no matches has no career, which is a different answer from a
// career of zeroes.
func TestCareerSummaryOfAPlayerWithNoMatches(t *testing.T) {
	h := newHarness(t)
	id := h.player("test-unplayed", "Never Played", TourWta)

	if _, err := h.GetPlayerCareerSummary(h.ctx, id); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("got %v, want pgx.ErrNoRows", err)
	}
}

func TestSurfaceSplits(t *testing.T) {
	h := newHarness(t)
	winner := h.player("test-surface", "Surface Player", TourAtp)
	loser := h.player("test-surface-opp", "Surface Opponent", TourAtp)
	open := h.tournament("Surface Open", TierTour, 2019)
	pts := int16(70)
	h.match(open, winner, loser, 1, "F", &pts, false)

	splits, err := h.GetPlayerSurfaceSplits(h.ctx, winner)
	if err != nil {
		t.Fatalf("GetPlayerSurfaceSplits: %v", err)
	}
	if len(splits) != 1 || splits[0].Surface == nil || *splits[0].Surface != SurfaceHard {
		t.Fatalf("splits = %+v, want one hard-court row", splits)
	}
	if splits[0].Wins != 1 || splits[0].Losses != 0 {
		t.Errorf("hard court %d-%d, want 1-0", splits[0].Wins, splits[0].Losses)
	}
}

// Similarity alone cannot separate two players with the same surname, so the
// best tier a player reached breaks the tie.
func TestSearchRanksByTierWithinEqualSimilarity(t *testing.T) {
	h := newHarness(t)
	big := h.player("test-zzverev-alexander", "Zzverev Alexander", TourAtp)
	small := h.player("test-zzverev-alexandra", "Zzverev Alexandra", TourWta)
	// Separate opponents keep each player's tier set disjoint; sharing a match
	// would give both the same best tier and defeat the point of the test.
	bigOpp := h.player("test-big-opponent", "Big Opponent", TourAtp)
	smallOpp := h.player("test-small-opponent", "Small Opponent", TourWta)

	tour := h.tournament("Big Open", TierTour, 2019)
	futures := h.tournament("Small Open", TierFutures, 2019)
	pts := int16(70)
	h.match(tour, big, bigOpp, 1, "F", &pts, false)
	h.match(futures, small, smallOpp, 2, "F", &pts, false)

	rows, err := h.SearchPlayers(h.ctx, SearchPlayersParams{Query: "zzverev", RowLimit: 10})
	if err != nil {
		t.Fatalf("SearchPlayers: %v", err)
	}
	if len(rows) < 2 {
		t.Fatalf("got %d rows, want both players", len(rows))
	}
	if rows[0].ID != big {
		t.Errorf("first result is %q (%s); the tour-level player should rank first",
			rows[0].FullName, rows[0].BestTier)
	}
	if rows[0].Rank <= rows[1].Rank {
		t.Errorf("ranks not descending: %v then %v", rows[0].Rank, rows[1].Rank)
	}
	if rows[0].Matches == 0 {
		t.Error("match count should be populated from the lateral join")
	}
}

func TestSearchFiltersByTour(t *testing.T) {
	h := newHarness(t)
	h.player("test-qqunique-atp", "Qqunique Player", TourAtp)
	h.player("test-qqunique-wta", "Qqunique Person", TourWta)

	wta := TourWta
	rows, err := h.SearchPlayers(h.ctx, SearchPlayersParams{Query: "qqunique", RowLimit: 10, Tour: &wta})
	if err != nil {
		t.Fatalf("SearchPlayers: %v", err)
	}
	if len(rows) != 1 || rows[0].Tour != TourWta {
		t.Errorf("tour filter returned %d rows: %+v", len(rows), rows)
	}
}

// Equal ranks are the case a row-value cursor gets wrong, so page through three
// players who all score identically.
func TestSearchCursorPaginates(t *testing.T) {
	h := newHarness(t)
	for i, name := range []string{"Wwilson Aaa", "Wwilson Bbb", "Wwilson Ccc"} {
		h.player("test-wwilson-"+string(rune('a'+i)), name, TourAtp)
	}

	first, err := h.SearchPlayers(h.ctx, SearchPlayersParams{Query: "wwilson", RowLimit: 2})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if len(first) != 2 {
		t.Fatalf("first page has %d rows, want 2", len(first))
	}

	last := first[len(first)-1]
	second, err := h.SearchPlayers(h.ctx, SearchPlayersParams{
		Query: "wwilson", RowLimit: 2, AfterRank: &last.Rank, AfterID: &last.ID,
	})
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if len(second) != 1 {
		t.Errorf("second page has %d rows, want the remaining 1", len(second))
	}
	for _, r := range second {
		for _, seen := range first {
			if r.ID == seen.ID {
				t.Errorf("player %d appears on both pages", r.ID)
			}
		}
	}
}
