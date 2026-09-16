package events

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sami0076/tennis-wiki/internal/db"
	"github.com/sami0076/tennis-wiki/internal/testdb"
)

type fixture struct {
	pool *pgxpool.Pool
	ctx  context.Context
	t    *testing.T
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	pool, err := db.Open(ctx, db.Config{DSN: testdb.Start(t), MaxConns: 4, MinConns: 1})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE matches, tournaments, events RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return &fixture{pool: pool, ctx: ctx, t: t}
}

func (f *fixture) tournament(tour, sourceID, name, level, tier string, season int) {
	f.t.Helper()
	start := time.Date(season, 6, 1, 0, 0, 0, 0, time.UTC)
	if _, err := f.pool.Exec(f.ctx, `
		INSERT INTO tournaments (source_id, tour, name, level, tier, start_date, season)
		VALUES ($1, $2::tour, $3, $4, $5::tier, $6, $7)`,
		sourceID, tour, name, level, tier, start, season); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) run(overrides *Overrides) (Result, Stats) {
	f.t.Helper()
	res, stats, err := Run(f.ctx, NewStore(f.pool), overrides, slog.Default())
	if err != nil {
		f.t.Fatal(err)
	}
	return res, stats
}

type linked struct {
	slug, name, link string
}

func (f *fixture) linkOf(sourceID string) linked {
	f.t.Helper()
	var l linked
	if err := f.pool.QueryRow(f.ctx, `
		SELECT e.slug, e.name, t.event_link FROM tournaments t JOIN events e ON e.id = t.event_id
		 WHERE t.source_id = $1`, sourceID).Scan(&l.slug, &l.name, &l.link); err != nil {
		f.t.Fatalf("%s: %v", sourceID, err)
	}
	return l
}

func (f *fixture) count(table string) int {
	f.t.Helper()
	var n int
	if err := f.pool.QueryRow(f.ctx, `SELECT count(*) FROM `+table).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

func TestRunWritesEventsAndLinks(t *testing.T) {
	f := newFixture(t)
	f.tournament("atp", "2018-0421", "Canada Masters", "M", "tour", 2018)
	f.tournament("atp", "2019-421", "Canada Masters", "M", "tour", 2019)
	f.tournament("wta", "2016-540", "Wimbledon", "G", "tour", 2016)
	f.tournament("wta", "2022-904", "Wimbledon", "G", "tour", 2022)
	f.tournament("wta", "2015-W-SL-GBR-01A-2015", "Wimbledon", "G", "tour", 2015)
	f.tournament("atp", "2019-M-DC-2019-FLS-A-M-FRA-JPN-01", "Davis Cup FLS A: FRA vs JPN", "D", "tour", 2019)

	overrides := &Overrides{Overrides: []Override{{Tour: "wta", Number: "540", To: "904"}}}
	res, stats := f.run(overrides)

	if stats.Rows != 6 || stats.Events != 3 || stats.Created != 3 {
		t.Errorf("stats %+v", stats)
	}
	if res.ByLink[LinkNumber] != 3 || res.ByLink[LinkOverride] != 1 || res.ByLink[LinkBridged] != 1 || res.ByLink[LinkTeam] != 1 {
		t.Errorf("by link %v", res.ByLink)
	}
	if got := f.linkOf("2018-0421"); got != (linked{"canada-masters-atp", "Canada Masters", "number"}) {
		t.Errorf("Canada 2018: %+v", got)
	}
	if got := f.linkOf("2016-540"); got != (linked{"wimbledon-wta", "Wimbledon", "override"}) {
		t.Errorf("Wimbledon 2016: %+v", got)
	}
	if got := f.linkOf("2015-W-SL-GBR-01A-2015"); got != (linked{"wimbledon-wta", "Wimbledon", "bridged"}) {
		t.Errorf("Wimbledon 2015: %+v", got)
	}
	if got := f.linkOf("2019-M-DC-2019-FLS-A-M-FRA-JPN-01"); got != (linked{"davis-cup-atp", "Davis Cup", "team"}) {
		t.Errorf("Davis Cup tie: %+v", got)
	}

	// Idempotent: nothing created, nothing removed.
	_, again := f.run(overrides)
	if again.Created != 0 || again.Removed != 0 || again.Events != 3 {
		t.Errorf("second run %+v", again)
	}
}

// A slug is minted once. The event whose sanction moves city keeps its URL
// and takes the new name; a name two events share within a tour gets a serial,
// the more recent run taking the bare slug; a name that already carries the
// tour is not suffixed again; an event nothing points at any more goes.
func TestSlugsAreKeptAndSerialled(t *testing.T) {
	f := newFixture(t)
	f.tournament("atp", "2013-0424", "San Jose", "A", "tour", 2013)
	f.tournament("atp", "2001-746", "Sao Paulo CH", "C", "challenger", 2001)
	f.tournament("atp", "2010-6492", "Sao Paulo CH", "C", "challenger", 2010)
	f.tournament("wta", "2025-808", "WTA Finals", "F", "tour", 2025)
	f.run(nil)

	if got := f.linkOf("2013-0424"); got.slug != "san-jose-atp" {
		t.Errorf("San Jose: %+v", got)
	}
	if got := f.linkOf("2010-6492"); got.slug != "sao-paulo-atp" {
		t.Errorf("the more recent Sao Paulo takes the bare slug: %+v", got)
	}
	if got := f.linkOf("2001-746"); got.slug != "sao-paulo-atp-2" {
		t.Errorf("the earlier Sao Paulo takes the serial: %+v", got)
	}
	if got := f.linkOf("2025-808"); got.slug != "wta-finals" {
		t.Errorf("a name carrying the tour is not suffixed again: %+v", got)
	}

	f.tournament("atp", "2022-424", "Dallas", "A", "tour", 2022)
	if _, err := f.pool.Exec(f.ctx, `DELETE FROM tournaments WHERE source_id = '2001-746'`); err != nil {
		t.Fatal(err)
	}
	_, stats := f.run(nil)
	if got := f.linkOf("2022-424"); got != (linked{"san-jose-atp", "Dallas", "number"}) {
		t.Errorf("Dallas keeps San Jose's slug and takes the name: %+v", got)
	}
	if stats.Removed != 1 || f.count("events") != 3 {
		t.Errorf("the emptied Sao Paulo should go: removed %d, events %d", stats.Removed, f.count("events"))
	}
}
