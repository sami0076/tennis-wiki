package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/db"
)

func decodePercentiles(t *testing.T, res *http.Response) Percentiles {
	t.Helper()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var p Percentiles
	if err := json.NewDecoder(res.Body).Decode(&p); err != nil {
		t.Fatalf("decode percentiles: %v", err)
	}
	return p
}

func axisByKey(p Percentiles, key string) PercentileAxis {
	for _, a := range p.Axes {
		if a.Key == key {
			return a
		}
	}
	return PercentileAxis{}
}

// Two players clear the floor and one does not. The one under it is ranked
// against the two, and is not counted in what they are ranked against.
func TestPercentilesRankAgainstTheQualifiedTour(t *testing.T) {
	f := newAPIFixture(t)
	strong := f.player("itg-pct-strong", "Itg Pctstrong", db.TourAtp)
	weak := f.player("itg-pct-weak", "Itg Pctweak", db.TourAtp)
	rookie := f.player("itg-pct-rookie", "Itg Pctrookie", db.TourAtp)
	open := f.tournament("itg-pct-open", db.TierTour, 2019)

	points := int16(80)
	for i := 1; i <= percentileMinMatches; i++ {
		f.match(open, strong, weak, i, "R32", 2019, &points, false)
	}
	f.match(open, rookie, weak, 99, "R32", 2019, &points, false)

	f.rating(strong, "2019-05-02", "overall", 1900, 11)
	f.rating(weak, "2019-05-02", "overall", 1600, 11)
	f.rating(rookie, "2019-05-02", "overall", 1700, 1)
	f.rating(strong, "2019-05-02", "hard", 2000, 40)

	p := decodePercentiles(t, f.get("/api/v1/players/itg-pct-strong/percentiles"))
	if p.From == nil || *p.From != "2018-05-03" || p.To == nil || *p.To != "2019-05-02" {
		t.Errorf("window = %v to %v, want the year ending at the last match", p.From, p.To)
	}
	if p.Population != 2 || !p.Qualified || p.Matches != int64(percentileMinMatches) {
		t.Errorf("population %d, qualified %v, matches %d; want 2, true, %d",
			p.Population, p.Qualified, p.Matches, percentileMinMatches)
	}
	// 0.75 of 2000 and 0.25 of 1900, as the simulator blends it.
	hard := axisByKey(p, "hard")
	if hard.Value == nil || *hard.Value != 1975 || hard.Percentile == nil || *hard.Percentile != 75 {
		t.Errorf("hard = %v at %v, want 1975 at the 75th", hard.Value, hard.Percentile)
	}
	if big := axisByKey(p, "big"); big.Percentile != nil {
		t.Errorf("big matches = %v, want absent without a Slam or a top-ten opponent", *big.Percentile)
	}

	rookieP := decodePercentiles(t, f.get("/api/v1/players/itg-pct-rookie/percentiles"))
	if rookieP.Qualified || rookieP.Population != 2 {
		t.Errorf("rookie qualified %v in a population of %d, want false in 2",
			rookieP.Qualified, rookieP.Population)
	}
	if hard := axisByKey(rookieP, "hard"); hard.Percentile == nil || *hard.Percentile != 50 {
		t.Errorf("rookie hard = %v, want the 50th, between the two", hard.Percentile)
	}
}

func TestPercentilesWithoutTourMatchesHaveNoAxes(t *testing.T) {
	f := newAPIFixture(t)
	player := f.player("itg-pct-futures", "Itg Pctfutures", db.TourAtp)
	foil := f.player("itg-pct-futures-foil", "Itg Pctfuturesfoil", db.TourAtp)
	event := f.tournament("itg-pct-futures-event", db.TierFutures, 2019)
	f.match(event, player, foil, 1, "R32", 2019, nil, false)

	p := decodePercentiles(t, f.get("/api/v1/players/itg-pct-futures/percentiles"))
	if p.From != nil || p.To != nil || len(p.Axes) != 0 {
		t.Errorf("window %v to %v with %d axes, want none", p.From, p.To, len(p.Axes))
	}
}

func TestPercentilesUnknownPlayerIs404(t *testing.T) {
	f := newAPIFixture(t)
	if res := f.get("/api/v1/players/itg-nobody/percentiles"); res.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", res.StatusCode)
	}
}
