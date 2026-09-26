package ongoing

import (
	"context"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/sami0076/tennis-wiki/internal/db"
	"github.com/sami0076/tennis-wiki/internal/ingest"
	"github.com/sami0076/tennis-wiki/internal/testdb"
)

func TestMain(m *testing.M) { testdb.Run(m) }

const header = "tourney_id,tourney_name,surface,draw_size,tourney_level,indoor,tourney_date,match_num," +
	"winner_id,winner_seed,winner_entry,winner_name,winner_hand,winner_ht,winner_ioc,winner_age," +
	"winner_rank,winner_rank_points,loser_id,loser_seed,loser_entry,loser_name,loser_hand,loser_ht," +
	"loser_ioc,loser_age,loser_rank,loser_rank_points,score,best_of,round,minutes," +
	"w_ace,w_df,w_svpt,w_1stIn,w_1stWon,w_2ndWon,w_SvGms,w_bpSaved,w_bpFaced," +
	"l_ace,l_df,l_svpt,l_1stIn,l_1stWon,l_2ndWon,l_SvGms,l_bpSaved,l_bpFaced"

func row(tourney, name, level, date string, num int, winner, loser, round string) string {
	return strings.Join([]string{
		tourney, name, "Hard", "32", level, "O", date, strconv.Itoa(num),
		winner, "1", "", "Winner " + winner, "R", "", "", "",
		"", "", loser, "", "", "Loser " + loser, "R", "", "",
		"", "", "", "6-4 6-4", "3", round, "",
		"", "", "", "", "", "", "", "", "",
		"", "", "", "", "", "", "", "", "",
	}, ",")
}

// fake answers with a fixed body, or 304 when the validator matches.
type fake struct {
	body, etag string
}

func (f fake) OpenPathIfChanged(_ context.Context, _, _, validator string) (io.ReadCloser, string, error) {
	if validator != "" && validator == f.etag {
		return nil, validator, ingest.ErrUnchanged
	}
	return io.NopCloser(strings.NewReader(f.body)), f.etag, nil
}

func TestRefreshReplacesTheWeekAndKeepsOnlyTheMainDraw(t *testing.T) {
	ctx := context.Background()
	pool, err := db.Open(ctx, db.Config{DSN: testdb.Start(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, "TRUNCATE ongoing_matches, ongoing_files"); err != nil {
		t.Fatal(err)
	}
	file := Files[0]

	first := strings.Join([]string{
		header,
		row("2026-4713", "Hangzhou", "250", "20260924", 1, "A1", "B1", "R32"),
		row("2026-4713", "Hangzhou", "250", "20260925", 2, "A2", "B2", "R16"),
		row("2026-4713", "Hangzhou", "250", "20260922", 3, "A3", "B3", "Q1"),
		row("2026-9999", "Davis Cup", "D", "20260925", 1, "A4", "B4", "RR"),
	}, "\n")
	res, err := Refresh(ctx, pool, fake{body: first, etag: `"v1"`}, "", file)
	if err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if !res.Changed || res.Rows != 2 {
		t.Errorf("first refresh = %+v, want 2 main-draw tour rows", res)
	}

	rows, err := db.New(pool).ListOngoingMatches(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].PlayedOn.Format("2006-01-02") != "2026-09-25" {
		t.Fatalf("rows = %+v, want two, newest first by the day played", rows)
	}

	// The source dropped one match and added one: the week is what the file
	// says now, not the union of every fetch.
	second := strings.Join([]string{
		header,
		row("2026-4713", "Hangzhou", "250", "20260925", 2, "A2", "B2", "R16"),
		row("2026-4713", "Hangzhou", "250", "20260926", 4, "A5", "B5", "QF"),
	}, "\n")
	if _, err := Refresh(ctx, pool, fake{body: second, etag: `"v2"`}, "", file); err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	rows, _ = db.New(pool).ListOngoingMatches(ctx)
	if len(rows) != 2 || rows[0].Round != "QF" {
		t.Fatalf("rows after replace = %+v, want R16 and QF only", rows)
	}

	// Unchanged: nothing written, and the file still says when it last moved.
	res, err = Refresh(ctx, pool, fake{body: second, etag: `"v2"`}, "", file)
	if err != nil || res.Changed {
		t.Fatalf("unchanged refresh = %+v, %v; want no change", res, err)
	}
	files, err := db.New(pool).ListOngoingFiles(ctx)
	if err != nil || len(files) != 1 || files[0].Rows != 2 {
		t.Fatalf("files = %+v, %v; want one file of 2 rows", files, err)
	}
	if !files[0].CheckedAt.Time.After(files[0].ChangedAt.Time) {
		t.Errorf("checked %v, changed %v; want the check after the change",
			files[0].CheckedAt.Time, files[0].ChangedAt.Time)
	}
}
