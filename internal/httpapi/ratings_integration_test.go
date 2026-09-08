package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// rating writes one snapshot. The table is sparse on purpose: a row exists only
// for a week a series moved, so these are written one at a time rather than
// filled in across a range.
func (f *apiFixture) rating(playerID int64, asOf, surface string, elo float64, matches int) {
	f.t.Helper()
	if _, err := f.tx.Exec(f.ctx, `
		INSERT INTO ratings (player_id, as_of, surface, elo, matches_played)
		VALUES ($1, $2::date, $3::rating_surface, $4, $5)`,
		playerID, asOf, surface, elo, matches); err != nil {
		f.t.Fatalf("insert rating: %v", err)
	}
}

func decodeSeries(t *testing.T, res *http.Response) RatingSeries {
	t.Helper()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var series RatingSeries
	if err := json.NewDecoder(res.Body).Decode(&series); err != nil {
		t.Fatalf("decode series: %v", err)
	}
	return series
}

func seriesFor(t *testing.T, profile PlayerProfile, surface string) SeriesRating {
	t.Helper()
	for _, s := range profile.Ratings {
		if s.Surface == surface {
			return s
		}
	}
	t.Fatalf("no %s series in %+v", surface, profile.Ratings)
	return SeriesRating{}
}

func TestProfileCarriesCurrentAndPeakPerSeries(t *testing.T) {
	f := newAPIFixture(t)
	player := f.player("itg-rated", "Itg Rated", db.TourAtp)
	foil := f.player("itg-rated-foil", "Itg Rated Foil", db.TourAtp)
	open := f.tournament("itg-rated-open", db.TierTour, 2019)
	f.match(open, player, foil, 1, "F", 2019, nil, false)

	// The peak is not the last row, which is the whole reason both are served:
	// a career that ended below its best still has a best.
	f.rating(player, "2019-01-07", "overall", 1900, 10)
	f.rating(player, "2020-06-01", "overall", 2210, 40)
	f.rating(player, "2021-03-01", "overall", 1980, 60)
	f.rating(player, "2020-06-01", "clay", 2260, 22)

	got := decodeProfile(t, f.get("/api/v1/players/itg-rated"))
	if len(got.Ratings) != 2 {
		t.Fatalf("got %d series, want overall and clay", len(got.Ratings))
	}
	// Overall leads, because that is the order the surface strip reads in.
	if got.Ratings[0].Surface != "overall" {
		t.Errorf("first series is %q, want overall", got.Ratings[0].Surface)
	}

	overall := seriesFor(t, got, "overall")
	if overall.Current.Elo != 1980 || overall.Current.AsOf != "2021-03-01" {
		t.Errorf("current = %+v, want the last row", overall.Current)
	}
	if overall.Peak.Elo != 2210 || overall.Peak.AsOf != "2020-06-01" {
		t.Errorf("peak = %+v, want the highest row and its date", overall.Peak)
	}
	if overall.Matches != 60 {
		t.Errorf("matches = %d, want the count on the last row", overall.Matches)
	}

	// A surface the player never played is absent, not sitting at 1500.
	for _, s := range got.Ratings {
		if s.Surface == "grass" || s.Surface == "hard" {
			t.Errorf("a series the player never played was reported: %+v", s)
		}
	}
}

// Everyone whose only matches were team events or walkovers is rated by
// nothing. That is a null block, the same way career is null rather than a
// record of zeroes.
func TestProfileRatingsAreNullForAnUnratedPlayer(t *testing.T) {
	f := newAPIFixture(t)
	player := f.player("itg-unrated", "Itg Unrated", db.TourAtp)
	foil := f.player("itg-unrated-foil", "Itg Unrated Foil", db.TourAtp)
	open := f.tournament("itg-unrated-open", db.TierTour, 2019)
	f.match(open, player, foil, 1, "F", 2019, nil, false)

	got := decodeProfile(t, f.get("/api/v1/players/itg-unrated"))
	if got.Ratings != nil {
		t.Errorf("ratings = %+v, want null for a player nothing rated", got.Ratings)
	}
	if got.Career == nil {
		t.Error("an unrated player lost their career record too")
	}
}

// A player with no matches at all still renders: the page is an identity header
// and one empty state, never a 404.
func TestProfileWithNeitherMatchesNorRatings(t *testing.T) {
	f := newAPIFixture(t)
	f.player("itg-nothing", "Itg Nothing", db.TourAtp)

	got := decodeProfile(t, f.get("/api/v1/players/itg-nothing"))
	if got.Career != nil || got.Ratings != nil {
		t.Errorf("career = %+v, ratings = %+v, want both null", got.Career, got.Ratings)
	}
	if got.Name != "Itg Nothing" {
		t.Errorf("name = %q, want the identity header to survive", got.Name)
	}
}

func TestRatingSeriesReturnsTheWholeTrajectory(t *testing.T) {
	f := newAPIFixture(t)
	player := f.player("itg-series", "Itg Series", db.TourAtp)
	for _, r := range []struct {
		date string
		elo  float64
	}{{"2019-01-07", 1800}, {"2019-06-03", 1950}, {"2020-02-10", 2100}} {
		f.rating(player, r.date, "overall", r.elo, 10)
	}
	f.rating(player, "2019-06-03", "clay", 2000, 5)

	series := decodeSeries(t, f.get("/api/v1/players/itg-series/ratings"))
	if series.Surface != "overall" {
		t.Errorf("surface = %q, want overall by default", series.Surface)
	}
	if len(series.Points) != 3 {
		t.Fatalf("got %d points, want the whole overall trajectory", len(series.Points))
	}
	if series.Points[0].AsOf != "2019-01-07" || series.Points[2].Elo != 2100 {
		t.Errorf("points are not in order: %+v", series.Points)
	}
	// The range covered, not the range asked for.
	if series.From != "2019-01-07" || series.To != "2020-02-10" {
		t.Errorf("range = %s to %s, want the weeks that exist", series.From, series.To)
	}

	clay := decodeSeries(t, f.get("/api/v1/players/itg-series/ratings?surface=clay"))
	if len(clay.Points) != 1 {
		t.Errorf("clay returned %d points, want only clay rows", len(clay.Points))
	}
}

func TestRatingSeriesFiltersByDate(t *testing.T) {
	f := newAPIFixture(t)
	player := f.player("itg-window", "Itg Window", db.TourAtp)
	for _, date := range []string{"2018-01-01", "2019-01-07", "2020-01-06", "2021-01-04"} {
		f.rating(player, date, "overall", 1900, 10)
	}

	series := decodeSeries(t,
		f.get("/api/v1/players/itg-window/ratings?from=2019-01-01&to=2020-12-31"))
	if len(series.Points) != 2 {
		t.Fatalf("got %d points, want the two inside the window", len(series.Points))
	}
	if series.From != "2019-01-07" || series.To != "2020-01-06" {
		t.Errorf("range = %s to %s", series.From, series.To)
	}

	// A window with nothing in it is an empty trajectory, not an error: the
	// player simply did not play then.
	empty := decodeSeries(t, f.get("/api/v1/players/itg-window/ratings?from=2025-01-01"))
	if len(empty.Points) != 0 || empty.From != "" {
		t.Errorf("an empty window returned %+v", empty)
	}
}

func TestRatingSeriesRejectsBadInput(t *testing.T) {
	f := newAPIFixture(t)
	f.player("itg-bad-series", "Itg Bad Series", db.TourAtp)

	for _, q := range []string{"?surface=mud", "?surface=indoor", "?from=yesterday", "?to=2020"} {
		res := f.get("/api/v1/players/itg-bad-series/ratings" + q)
		if res.StatusCode != http.StatusBadRequest {
			t.Errorf("%q returned %d, want 400", q, res.StatusCode)
			continue
		}
		if ct := res.Header.Get("Content-Type"); ct != ProblemContentType {
			t.Errorf("%q returned content type %q, want a problem document", q, ct)
		}
	}

	if res := f.get("/api/v1/players/itg-no-such/ratings"); res.StatusCode != http.StatusNotFound {
		t.Errorf("an unknown slug returned %d, want 404", res.StatusCode)
	}
}
