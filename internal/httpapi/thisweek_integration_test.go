package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
)

func decodeThisWeek(t *testing.T, res *http.Response) ThisWeek {
	t.Helper()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if res.Header.Get("X-Cache") != "" {
		t.Errorf("X-Cache = %q, want the endpoint outside the cache", res.Header.Get("X-Cache"))
	}
	var w ThisWeek
	if err := json.NewDecoder(res.Body).Decode(&w); err != nil {
		t.Fatalf("decode this week: %v", err)
	}
	return w
}

func TestThisWeekLinksWhoItKnowsAndSaysWhenItLooked(t *testing.T) {
	f := newAPIFixture(t)

	empty := decodeThisWeek(t, f.get("/api/v1/this-week"))
	if empty.Events == nil || len(empty.Events) != 0 || empty.CheckedAt != nil {
		t.Errorf("before a fetch = %+v, want no events and no time", empty)
	}

	f.player("itg-week-known", "Itg Weekknown", "atp")
	if _, err := f.tx.Exec(f.ctx, `
		UPDATE players SET source_id = 'WK01' WHERE slug = 'itg-week-known';
		INSERT INTO ongoing_matches (file, tour, tourney_source_id, tourney_name, level, match_num,
		                             round, played_on, winner_source_id, winner_name,
		                             loser_source_id, loser_name)
		VALUES ('ongoing_tourneys.csv', 'atp', '2026-1', 'Itg Open', '250', 1, 'R32', current_date,
		        'WK01', 'Itg Weekknown', 'ZZ99', 'Nobody Known');
		INSERT INTO ongoing_files (file, checked_at, changed_at, rows)
		VALUES ('ongoing_tourneys.csv', now(), now(), 1);`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	week := decodeThisWeek(t, f.get("/api/v1/this-week"))
	if len(week.Events) != 1 || week.CheckedAt == nil {
		t.Fatalf("week = %+v, want one event and a checked time", week)
	}
	result := week.Events[0].Latest[0]
	if result.Winner.Slug == nil || *result.Winner.Slug != "itg-week-known" {
		t.Errorf("winner slug = %v, want the player with that source id", result.Winner.Slug)
	}
	if result.Loser.Slug != nil {
		t.Errorf("loser slug = %v, want none for an id the database does not know", *result.Loser.Slug)
	}
}
