package httpapi

import (
	"testing"
	"time"

	"github.com/sami0076/tennis-wiki/internal/db"
)

func weekRow(tour db.Tour, id, name, level, day, round, winner, slug string) db.ListOngoingMatchesRow {
	on, _ := time.Parse(time.DateOnly, day)
	return db.ListOngoingMatchesRow{
		Tour: tour, TourneySourceID: id, TourneyName: name, Level: level,
		Round: round, PlayedOn: on, WinnerName: winner, WinnerSlug: slug, LoserName: "Foil",
	}
}

func TestThisWeekGroupsEventsBiggestFirst(t *testing.T) {
	now, _ := time.Parse(time.DateOnly, "2026-09-27")
	rows := []db.ListOngoingMatchesRow{
		weekRow(db.TourAtp, "2026-1", "Chengdu", "250", "2026-09-24", "R32", "A", "a"),
		weekRow(db.TourAtp, "2026-1", "Chengdu", "250", "2026-09-26", "F", "Champ", "champ"),
		weekRow(db.TourAtp, "2026-1", "Chengdu", "250", "2026-09-26", "SF", "B", ""),
		weekRow(db.TourWta, "2026-2", "Beijing", "1000", "2026-09-21", "QF", "C", "c"),
		weekRow(db.TourAtp, "2026-3", "Old Open", "250", "2026-09-10", "F", "D", "d"),
	}

	week := buildThisWeek(rows, now)

	if len(week.Events) != 2 {
		t.Fatalf("events = %d, want 2 with the stale one dropped", len(week.Events))
	}
	if week.Events[0].Name != "Beijing" {
		t.Errorf("first event = %s, want the 1000 ahead of the 250", week.Events[0].Name)
	}
	chengdu := week.Events[1]
	if chengdu.Round != "F" || chengdu.Matches != 3 {
		t.Errorf("Chengdu round %s over %d matches, want F over 3", chengdu.Round, chengdu.Matches)
	}
	if chengdu.Champion == nil || chengdu.Champion.Slug == nil || *chengdu.Champion.Slug != "champ" {
		t.Errorf("champion = %+v, want the final's winner", chengdu.Champion)
	}
	if chengdu.Latest[0].Round != "F" || chengdu.Latest[1].Round != "SF" {
		t.Errorf("latest = %s, %s; want the final first, then the semi played the same day",
			chengdu.Latest[0].Round, chengdu.Latest[1].Round)
	}
	if chengdu.Latest[1].Winner.Slug != nil {
		t.Errorf("unlinked winner has slug %v, want none", *chengdu.Latest[1].Winner.Slug)
	}
}
