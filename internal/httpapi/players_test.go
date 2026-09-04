package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/sami0076/tennis-wiki/internal/db"
)

func season(year int) time.Time {
	return time.Date(year, 6, 1, 0, 0, 0, 0, time.UTC)
}

func tierRow(tier db.Tier, matches, withStats int64) db.GetPlayerTierSplitsRow {
	return db.GetPlayerTierSplitsRow{Tier: tier, Matches: matches, MatchesWithStats: withStats}
}

// The three absences the response has to keep apart. Collapsing them into one
// null would lose the ability to explain any of them.
func TestServeAvailability(t *testing.T) {
	cases := []struct {
		name    string
		summary db.GetPlayerCareerSummaryRow
		tiers   []db.GetPlayerTierSplitsRow
		want    string
	}{
		{
			name:    "every eligible match recorded",
			summary: db.GetPlayerCareerSummaryRow{Matches: 10, StatMatches: 10, LastMatch: season(2019)},
			tiers:   []db.GetPlayerTierSplitsRow{tierRow(db.TierTour, 10, 10)},
			want:    AvailabilityRecorded,
		},
		{
			name: "retirements do not make a complete record partial",
			summary: db.GetPlayerCareerSummaryRow{
				Matches: 10, IncompleteMatches: 2, StatMatches: 8, LastMatch: season(2019)},
			tiers: []db.GetPlayerTierSplitsRow{tierRow(db.TierTour, 10, 8)},
			want:  AvailabilityRecorded,
		},
		{
			name: "some matches recorded, some not",
			summary: db.GetPlayerCareerSummaryRow{
				Matches: 10, StatMatches: 6, LastMatch: season(2019)},
			tiers: []db.GetPlayerTierSplitsRow{tierRow(db.TierTour, 10, 6)},
			want:  AvailabilityPartial,
		},
		{
			name:    "futures, where nothing has ever been recorded",
			summary: db.GetPlayerCareerSummaryRow{Matches: 11, StatMatches: 0, LastMatch: season(2019)},
			tiers:   []db.GetPlayerTierSplitsRow{tierRow(db.TierFutures, 11, 0)},
			want:    AvailabilityNeverForTier,
		},
		{
			name:    "mixed futures and itf is still a tier absence",
			summary: db.GetPlayerCareerSummaryRow{Matches: 20, StatMatches: 0, LastMatch: season(2019)},
			tiers: []db.GetPlayerTierSplitsRow{
				tierRow(db.TierFutures, 11, 0), tierRow(db.TierItf, 9, 0)},
			want: AvailabilityNeverForTier,
		},
		{
			name:    "tour level before 1991",
			summary: db.GetPlayerCareerSummaryRow{Matches: 200, StatMatches: 0, LastMatch: season(1969)},
			tiers:   []db.GetPlayerTierSplitsRow{tierRow(db.TierTour, 200, 0)},
			want:    AvailabilityNeverInEra,
		},
		{
			name:    "challenger before it was recorded",
			summary: db.GetPlayerCareerSummaryRow{Matches: 40, StatMatches: 0, LastMatch: season(2004)},
			tiers:   []db.GetPlayerTierSplitsRow{tierRow(db.TierChallenger, 40, 0)},
			want:    AvailabilityNeverInEra,
		},
		{
			name:    "modern tour level with nothing recorded is unexplained",
			summary: db.GetPlayerCareerSummaryRow{Matches: 5, StatMatches: 0, LastMatch: season(2019)},
			tiers:   []db.GetPlayerTierSplitsRow{tierRow(db.TierTour, 5, 0)},
			want:    AvailabilityNotRecorded,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := buildServe(c.summary, c.tiers)
			if got.Availability != c.want {
				t.Errorf("availability = %q, want %q", got.Availability, c.want)
			}
			// Rates exist only where something was recorded. Anywhere else they
			// must be absent rather than a set of zeroes.
			if c.summary.StatMatches == 0 && got.Rates != nil {
				t.Errorf("rates present for %q; nothing was recorded", c.want)
			}
			if c.summary.StatMatches > 0 && got.Rates == nil {
				t.Error("rates missing even though matches recorded them")
			}
		})
	}
}

// A rate with no denominator does not exist. It is not zero, and reporting it
// as zero is the failure this whole response shape exists to avoid.
func TestServeRatesDistinguishZeroFromUndefined(t *testing.T) {
	summary := db.GetPlayerCareerSummaryRow{
		Matches: 2, StatMatches: 2, LastMatch: season(2019),
		Aces: 0, DoubleFaults: 4, ServePoints: 100, FirstIn: 60, FirstWon: 45,
		SecondWon: 20,
		// Never faced a break point in either match.
		BpSaved: 0, BpFaced: 0,
	}
	got := buildServe(summary, []db.GetPlayerTierSplitsRow{tierRow(db.TierTour, 2, 2)})
	if got.Rates == nil {
		t.Fatal("rates should be present")
	}

	// Zero aces across two matches is a real, recorded zero.
	if got.Rates.AcesPerMatch != 0 {
		t.Errorf("aces_per_match = %v, want a recorded 0", got.Rates.AcesPerMatch)
	}
	// Having never faced a break point is not saving 0% of them.
	if got.Rates.BreakPointsSaved != nil {
		t.Errorf("break_points_saved = %v, want null: none were faced", *got.Rates.BreakPointsSaved)
	}
	if got.Rates.FirstServeIn == nil || *got.Rates.FirstServeIn != 60 {
		t.Errorf("first_serve_in = %v, want 60", got.Rates.FirstServeIn)
	}
	if got.Rates.FirstServeWon == nil || *got.Rates.FirstServeWon != 75 {
		t.Errorf("first_serve_won = %v, want 75", got.Rates.FirstServeWon)
	}
	// Second serves are the serve points that missed the first: 100 - 60 = 40.
	if got.Rates.SecondServeWon == nil || *got.Rates.SecondServeWon != 50 {
		t.Errorf("second_serve_won = %v, want 50", got.Rates.SecondServeWon)
	}
}

// Rows predating the ingest's consistency check can still be in a database, and
// one of them made a player page show a second-serve percentage above 100.
func TestServeRatesWithholdImpossiblePercentages(t *testing.T) {
	summary := db.GetPlayerCareerSummaryRow{
		Matches: 1, StatMatches: 1, LastMatch: season(2019),
		// No second serves were played, yet six were won.
		ServePoints: 25, FirstIn: 25, FirstWon: 20, SecondWon: 6,
		BpSaved: 4, BpFaced: 2,
	}
	got := buildServe(summary, []db.GetPlayerTierSplitsRow{tierRow(db.TierTour, 1, 1)})
	if got.Rates == nil {
		t.Fatal("rates should be present")
	}
	if got.Rates.SecondServeWon != nil {
		t.Errorf("second_serve_won = %v, want null", *got.Rates.SecondServeWon)
	}
	if got.Rates.BreakPointsSaved != nil {
		t.Errorf("break_points_saved = %v, want null", *got.Rates.BreakPointsSaved)
	}
	// The rates that are still arithmetically possible stay.
	if got.Rates.FirstServeIn == nil || *got.Rates.FirstServeIn != 100 {
		t.Errorf("first_serve_in = %v, want 100", got.Rates.FirstServeIn)
	}
}

func TestBuildCareerCountsRetirementsInWinLoss(t *testing.T) {
	summary := db.GetPlayerCareerSummaryRow{
		Matches: 10, Wins: 7, Losses: 3, IncompleteMatches: 2, Titles: 1,
		FirstMatch: season(2018), LastMatch: season(2019),
	}
	got := buildCareer(summary, nil, []db.GetPlayerTierSplitsRow{tierRow(db.TierTour, 10, 10)})

	if got.Wins+got.Losses != got.Matches {
		t.Errorf("%d wins and %d losses do not account for %d matches",
			got.Wins, got.Losses, got.Matches)
	}
	if got.IncompleteMatches != 2 {
		t.Errorf("incomplete = %d; retirements must stay visible", got.IncompleteMatches)
	}
	if got.WinPercentage != 70 {
		t.Errorf("win_percentage = %v, want 70", got.WinPercentage)
	}
	if got.FirstMatch != "2018-06-01" || got.LastMatch != "2019-06-01" {
		t.Errorf("dates = %s..%s", got.FirstMatch, got.LastMatch)
	}
}

func TestSearchRejectsMalformedInput(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"no query", "/api/v1/players"},
		{"query too short", "/api/v1/players?q=a"},
		{"query is only punctuation", "/api/v1/players?q=%2B%2B%2B"},
		{"unknown tour", "/api/v1/players?q=federer&tour=itf"},
		{"limit not a number", "/api/v1/players?q=federer&limit=many"},
		{"limit too large", "/api/v1/players?q=federer&limit=1000"},
		{"limit zero", "/api/v1/players?q=federer&limit=0"},
		{"forged cursor", "/api/v1/players?q=federer&cursor=not-a-cursor"},
	}
	handler := testAPI(t, nil)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.path, nil))
			p := decodeProblem(t, rec.Result(), http.StatusBadRequest)
			if p.Detail == "" {
				t.Error("a 400 should say what to change")
			}
		})
	}
}

func TestPlayerNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	testAPI(t, pgx.ErrNoRows).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/players/no-such-player", nil))

	p := decodeProblem(t, rec.Result(), http.StatusNotFound)
	if p.Instance != "/api/v1/players/no-such-player" {
		t.Errorf("problem.instance = %q", p.Instance)
	}
}

// A database failure is a 500 problem document, and the underlying error must
// not travel to the caller.
func TestPlayerQueryFailureIsAProblemDocument(t *testing.T) {
	const secret = "pq: permission denied for relation players"
	rec := httptest.NewRecorder()
	testAPI(t, errors.New(secret)).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/players/anyone", nil))

	p := decodeProblem(t, rec.Result(), http.StatusInternalServerError)
	if p.Detail == secret {
		t.Error("the database error leaked into the response")
	}
}

func TestRound1(t *testing.T) {
	cases := map[float64]float64{
		0: 0, 1.24: 1.2, 1.25: 1.3, 66.14: 66.1, 82.222: 82.2, 100: 100,
	}
	for in, want := range cases {
		if got := round1(in); got != want {
			t.Errorf("round1(%v) = %v, want %v", in, got, want)
		}
	}
}
