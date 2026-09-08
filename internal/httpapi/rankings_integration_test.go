package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// ranking writes one published ranking. points is nil for the decades before
// the tours published them, which is not a week the player scored nothing.
func (f *apiFixture) ranking(playerID int64, date string, rank int32, points *int32) {
	f.t.Helper()
	if _, err := f.tx.Exec(f.ctx,
		`INSERT INTO rankings (player_id, ranking_date, rank, points)
		 VALUES ($1, $2::date, $3, $4)`, playerID, date, rank, points); err != nil {
		f.t.Fatalf("insert ranking: %v", err)
	}
}

func decodeRankings(t *testing.T, res *http.Response) RankingHistory {
	t.Helper()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var history RankingHistory
	if err := json.NewDecoder(res.Body).Decode(&history); err != nil {
		t.Fatalf("decode rankings: %v", err)
	}
	return history
}

func TestRankingHistoryReturnsTheWholeLine(t *testing.T) {
	f := newAPIFixture(t)
	player := f.player("itg-ranked", "Itg Ranked", db.TourAtp)
	points := int32(4200)

	f.ranking(player, "1975-01-06", 42, nil)
	f.ranking(player, "1978-06-05", 3, &points)
	f.ranking(player, "1981-09-07", 11, &points)

	history := decodeRankings(t, f.get("/api/v1/players/itg-ranked/rankings"))
	if len(history.Points) != 3 {
		t.Fatalf("got %d points, want the whole line", len(history.Points))
	}
	if history.From != "1975-01-06" || history.To != "1981-09-07" {
		t.Errorf("range = %s to %s", history.From, history.To)
	}

	// Best is the lowest number, not the last or the highest.
	if history.Best == nil || history.Best.Rank != 3 || history.Best.Date != "1978-06-05" {
		t.Errorf("best = %+v, want rank 3 in 1978", history.Best)
	}

	// Ranking points predate nothing being recorded for them, and a week with
	// no points recorded is not a week worth zero.
	if history.Points[0].Points != nil {
		t.Errorf("1975 reported %v ranking points, want null", *history.Points[0].Points)
	}
	if history.Points[1].Points == nil || *history.Points[1].Points != 4200 {
		t.Errorf("1978 points = %v, want the recorded 4200", history.Points[1].Points)
	}
}

func TestRankingHistoryIsEmptyForAnUnrankedPlayer(t *testing.T) {
	f := newAPIFixture(t)
	f.player("itg-never-ranked", "Itg Never Ranked", db.TourAtp)

	history := decodeRankings(t, f.get("/api/v1/players/itg-never-ranked/rankings"))
	if len(history.Points) != 0 || history.Best != nil {
		t.Errorf("got %+v, want an empty history rather than an error", history)
	}
}

func TestRankingHistoryRejectsBadInput(t *testing.T) {
	f := newAPIFixture(t)
	f.player("itg-bad-rankings", "Itg Bad Rankings", db.TourAtp)

	if res := f.get("/api/v1/players/itg-bad-rankings/rankings?from=lastyear"); res.StatusCode != http.StatusBadRequest {
		t.Errorf("a bad date returned %d, want 400", res.StatusCode)
	}
	if res := f.get("/api/v1/players/itg-no-such/rankings"); res.StatusCode != http.StatusNotFound {
		t.Errorf("an unknown slug returned %d, want 404", res.StatusCode)
	}
}

// Eleven majors and sixty-six titles are two different claims about the same
// career, so the summary counts them separately.
func TestCareerCountsMajorsApartFromTitles(t *testing.T) {
	f := newAPIFixture(t)
	player := f.player("itg-major", "Itg Major", db.TourAtp)
	foil := f.player("itg-major-foil", "Itg Major Foil", db.TourAtp)

	var slam, regular int64
	err := f.tx.QueryRow(f.ctx, `
		INSERT INTO tournaments (source_id, tour, name, level, tier, surface, start_date, season)
		VALUES ('itg-slam', 'atp', 'itg-slam', 'G', 'tour', 'grass', make_date(2019, 7, 1), 2019)
		RETURNING id`).Scan(&slam)
	if err != nil {
		t.Fatal(err)
	}
	regular = f.tournament("itg-regular", db.TierTour, 2019)

	f.match(slam, player, foil, 1, "F", 2019, nil, false)
	f.match(regular, player, foil, 1, "F", 2019, nil, false)
	f.match(regular, player, foil, 2, "SF", 2019, nil, false)

	got := decodeProfile(t, f.get("/api/v1/players/itg-major"))
	if got.Career == nil {
		t.Fatal("no career")
	}
	if got.Career.Titles != 2 {
		t.Errorf("titles = %d, want both finals", got.Career.Titles)
	}
	if got.Career.Majors != 1 {
		t.Errorf("majors = %d, want the one at a Grand Slam", got.Career.Majors)
	}
}
