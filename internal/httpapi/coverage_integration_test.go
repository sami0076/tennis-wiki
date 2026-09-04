package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// The disclosure ADR-0006 promises is only worth anything if it comes from the
// data. These assert the endpoint reports what was actually ingested.
func TestCoverageReportsWhatWasIngested(t *testing.T) {
	f := newAPIFixture(t)
	pts := int16(100)

	a := f.player("cov-a", "Cov Aaa", db.TourAtp)
	b := f.player("cov-b", "Cov Bbb", db.TourAtp)
	w1 := f.player("cov-w1", "Cov Www", db.TourWta)
	w2 := f.player("cov-w2", "Cov Wxx", db.TourWta)

	tour := f.tournament("cov-tour", db.TierTour, 2019)
	futures := f.tournament("cov-futures", db.TierFutures, 2015)
	wtaTour := f.tourTournament("cov-wta", db.TierTour, 2024, db.TourWta)

	f.match(tour, a, b, 1, "F", 2019, &pts, false)
	f.match(futures, a, b, 2, "F", 2015, nil, false) // futures records nothing
	f.match(wtaTour, w1, w2, 3, "F", 2024, &pts, false)

	res := f.get("/api/v1/coverage")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	var got CoverageResponse
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	byKey := map[string]CoverageEntry{}
	for _, e := range got.Tiers {
		byKey[e.Tour+"/"+e.Tier] = e
	}

	atpFutures, ok := byKey["atp/futures"]
	if !ok {
		t.Fatalf("no atp/futures entry in %+v", got.Tiers)
	}
	if atpFutures.MatchesWithStats != 0 || atpFutures.StatsPercentage != 0 {
		t.Errorf("futures reports %d matches with stats; nothing has ever been recorded there",
			atpFutures.MatchesWithStats)
	}

	atpTour, ok := byKey["atp/tour"]
	if !ok {
		t.Fatal("no atp/tour entry")
	}
	if atpTour.StatsPercentage != 100 {
		t.Errorf("atp/tour stats = %v%%, want 100", atpTour.StatsPercentage)
	}
	if atpTour.LastMatch != "2019-05-02" {
		t.Errorf("atp/tour last match = %q", atpTour.LastMatch)
	}

	// current_through is the most recent match per tour, which is the honest
	// answer to how up to date the site is.
	if got.CurrentThrough["atp"] != "2019-05-02" {
		t.Errorf("atp current_through = %q, want the latest atp match",
			got.CurrentThrough["atp"])
	}
	if got.CurrentThrough["wta"] != "2024-05-02" {
		t.Errorf("wta current_through = %q, want the latest wta match",
			got.CurrentThrough["wta"])
	}
	// The tours run out at different dates, and the endpoint must not flatten
	// that into one number: the gap between them is the thing being disclosed.
	if got.CurrentThrough["atp"] == got.CurrentThrough["wta"] {
		t.Error("both tours report the same currency; they are reported separately for a reason")
	}
}

func TestCoverageIsRevalidatable(t *testing.T) {
	f := newAPIFixture(t)

	first := f.get("/api/v1/coverage")
	tag := first.Header.Get("ETag")
	if tag == "" {
		t.Fatal("no ETag on the coverage response")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/coverage", nil)
	req.Header.Set("If-None-Match", tag)
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotModified {
		t.Errorf("status = %d, want 304", rec.Code)
	}
}
