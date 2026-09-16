package events

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sami0076/tennis-wiki/internal/name"
)

// Store reads the rows and writes the events.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore wraps a pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Stats is what a run wrote.
type Stats struct {
	Rows    int
	Events  int
	Created int
	Removed int
}

// Load reads every tournaments row the resolver needs.
func (s *Store) Load(ctx context.Context) ([]Row, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tour::text, source_id, name, level, tier::text, season, start_date
		  FROM tournaments ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("load tournaments: %w", err)
	}
	defer rows.Close()

	var out []Row
	for rows.Next() {
		var r Row
		var season int16
		if err := rows.Scan(&r.ID, &r.Tour, &r.SourceID, &r.Name, &r.Level, &r.Tier, &season, &r.StartDate); err != nil {
			return nil, fmt.Errorf("load tournaments: %w", err)
		}
		r.Season = int(season)
		out = append(out, r)
	}
	return out, rows.Err()
}

// Write replaces the events table with the resolved events and points every
// row at its event. Slugs already minted are kept: a URL is a handle, and an
// event whose latest name changed keeps the one it had. One transaction, so a
// reader never sees rows pointing at events that are not there.
func (s *Store) Write(ctx context.Context, events []Event) (stats Stats, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return stats, fmt.Errorf("begin: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			err = errors.Join(err, rbErr)
		}
	}()

	existing, err := loadSlugs(ctx, tx)
	if err != nil {
		return stats, err
	}
	taken := map[string]struct{}{}
	for _, slug := range existing {
		taken[slug] = struct{}{}
	}

	// New events are minted in order of first season, then key, so a serial
	// on a colliding slug is the same on every load.
	order := make([]int, 0, len(events))
	for i := range events {
		if slug, ok := existing[events[i].Tour+"\x00"+events[i].Key]; ok {
			events[i].Slug = slug
			continue
		}
		order = append(order, i)
	}
	sort.SliceStable(order, func(a, b int) bool {
		x, y := events[order[a]], events[order[b]]
		if x.FirstSeason != y.FirstSeason {
			return x.FirstSeason < y.FirstSeason
		}
		return x.Key < y.Key
	})
	for _, i := range order {
		events[i].Slug = mint(events[i], taken)
		taken[events[i].Slug] = struct{}{}
		stats.Created++
	}

	ids := make(map[string]int64, len(events))
	for _, ev := range events {
		var id int64
		err := tx.QueryRow(ctx, `
			INSERT INTO events (tour, slug, name, key, first_season, last_season)
			VALUES ($1::tour, $2, $3, $4, $5, $6)
			ON CONFLICT (tour, key) DO UPDATE
			   SET name = EXCLUDED.name,
			       first_season = EXCLUDED.first_season,
			       last_season = EXCLUDED.last_season
			RETURNING id`,
			ev.Tour, ev.Slug, ev.Name, ev.Key, ev.FirstSeason, ev.LastSeason).Scan(&id)
		if err != nil {
			return stats, fmt.Errorf("write event %s %s: %w", ev.Tour, ev.Key, err)
		}
		ids[ev.Tour+"\x00"+ev.Key] = id
	}
	stats.Events = len(events)

	var rowIDs, eventIDs []int64
	var linkOf []string
	for _, ev := range events {
		id := ids[ev.Tour+"\x00"+ev.Key]
		for _, ed := range ev.Editions {
			rowIDs = append(rowIDs, ed.Row.ID)
			eventIDs = append(eventIDs, id)
			linkOf = append(linkOf, string(ed.Link))
		}
	}
	tag, err := tx.Exec(ctx, `
		UPDATE tournaments t
		   SET event_id = v.event_id, event_link = v.link
		  FROM unnest($1::bigint[], $2::bigint[], $3::text[]) AS v(id, event_id, link)
		 WHERE t.id = v.id`, rowIDs, eventIDs, linkOf)
	if err != nil {
		return stats, fmt.Errorf("link tournaments: %w", err)
	}
	stats.Rows = int(tag.RowsAffected())

	// An event nothing points at any more: the rule or the data changed.
	tag, err = tx.Exec(ctx, `
		DELETE FROM events e
		 WHERE NOT EXISTS (SELECT 1 FROM tournaments t WHERE t.event_id = e.id)`)
	if err != nil {
		return stats, fmt.Errorf("remove empty events: %w", err)
	}
	stats.Removed = int(tag.RowsAffected())

	if err := tx.Commit(ctx); err != nil {
		return stats, fmt.Errorf("commit: %w", err)
	}
	return stats, nil
}

func loadSlugs(ctx context.Context, tx pgx.Tx) (map[string]string, error) {
	rows, err := tx.Query(ctx, `SELECT tour::text, key, slug FROM events`)
	if err != nil {
		return nil, fmt.Errorf("load events: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var tour, key, slug string
		if err := rows.Scan(&tour, &key, &slug); err != nil {
			return nil, fmt.Errorf("load events: %w", err)
		}
		out[tour+"\x00"+key] = slug
	}
	return out, rows.Err()
}

// mint makes the slug for a new event: the name, then the tour, then a serial
// if that is taken. Always the tour, so no tour is the default and the bare
// name stays free for a combined page (ADR-0012).
func mint(ev Event, taken map[string]struct{}) string {
	base := name.Slug(ev.Name)
	if base == "" {
		base = name.Slug(ev.Key)
	}
	base += "-" + ev.Tour
	slug := base
	for n := 2; ; n++ {
		if _, ok := taken[slug]; !ok {
			return slug
		}
		slug = base + "-" + strconv.Itoa(n)
	}
}
