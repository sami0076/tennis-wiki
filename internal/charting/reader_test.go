package charting

import (
	"os"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/ingest"
)

func open(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// Real rows. Among them: the Davis Cup tie whose names were left out, which
// shifts its date into the "Pl 1 hand" column and appears once broken and
// once whole; and a Bucharest semi-final with the umpire in "Best of", which
// is a column this reader does not use and so does not reject over.
func TestReadMatchesRejectsShiftedRowsAndKeepsTheWholeOne(t *testing.T) {
	matches, stats, err := ReadMatches(ingest.TourATP, open(t, "charting-m-matches.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if stats.Seen != 5 || stats.Kept != 4 || stats.Rejected != 1 {
		t.Errorf("stats = %+v, want 5 seen, 4 kept, 1 rejected", stats)
	}

	byID := map[string]Match{}
	for _, m := range matches {
		byID[m.ID] = m
	}
	davis := byID["20240915-M-Davis_Cup_World_Group-RR-Botic_Van_De_Zandschulp-Matteo_Berrettini"]
	if davis.Player1 != "Botic Van De Zandschulp" || davis.Player2 != "Matteo Berrettini" {
		t.Errorf("Davis Cup row read as %+v; want the whole row's names", davis)
	}
	if davis.PlayedOn.Format("2006-01-02") != "2024-09-15" || davis.Round != "RR" {
		t.Errorf("Davis Cup row: date %s round %s", davis.PlayedOn, davis.Round)
	}

	wimbledon := byID["19800705-M-Wimbledon-F-John_Mcenroe-Bjorn_Borg"]
	if wimbledon.Event != "Wimbledon" || wimbledon.Round != "F" || wimbledon.Tour != ingest.TourATP {
		t.Errorf("Wimbledon 1980 read as %+v", wimbledon)
	}
	if _, ok := byID["20260404-M-Bucharest-SF-Botic_Van_De_Zandschulp-Mariano_Navone"]; !ok {
		t.Error("the row with an umpire in Best of should still be read; that column is not used")
	}
}

func TestReadStatsTotalIsSetZero(t *testing.T) {
	stats, rs, err := ReadStats(open(t, "charting-m-stats-Overview.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if rs.Rejected != 0 || rs.Kept != rs.Seen || rs.Seen == 0 {
		t.Errorf("stats = %+v, want every row kept", rs)
	}

	var total, sets int
	for _, s := range stats {
		if s.MatchID != "20260521-M-Roland_Garros-Q3-Jesper_De_Jong-Michael_Zheng" {
			continue
		}
		if s.Set == 0 {
			total++
			if s.Player == "Jesper De Jong" && (s.Figures.ServePoints != 80 || s.Figures.Aces != 6 || s.Figures.UnforcedBH != 6) {
				t.Errorf("De Jong's total read as %+v", s.Figures)
			}
		} else {
			sets++
		}
	}
	if total != 2 {
		t.Errorf("%d total rows for the match, want one per player", total)
	}
	if sets == 0 {
		t.Error("no set rows read")
	}
}

func TestReadStatsRejectsAFigureThatIsNotOne(t *testing.T) {
	const csv = "match_id,player,set,serve_pts,aces,dfs,first_in,first_won,second_in,second_won,bk_pts,bp_saved,return_pts,return_pts_won,winners,winners_fh,winners_bh,unforced,unforced_fh,unforced_bh\n" +
		"x,A,Total,80,6,4,48,33,32,13,11,8,61,18,25,13,5,25,15,6\n" +
		"x,B,Total,80,six,4,48,33,32,13,11,8,61,18,25,13,5,25,15,6\n" +
		"x,B,7,80,6,4,48,33,32,13,11,8,61,18,25,13,5,25,15,6\n"
	stats, rs, err := ReadStats(stringReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 || rs.Rejected != 2 {
		t.Errorf("kept %d rejected %d, want 1 and 2", len(stats), rs.Rejected)
	}
}

func TestReadMatchesNamesTheMissingColumn(t *testing.T) {
	_, _, err := ReadMatches(ingest.TourWTA, stringReader("match_id,Player 1,Date\n"))
	if err == nil || err.Error() != `column "Player 2" not in header` {
		t.Errorf("err = %v, want the missing column named", err)
	}
}
