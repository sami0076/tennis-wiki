package httpapi

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/sami0076/tennis-wiki/internal/db"
)

// EventSummary is one row of the tournament index.
type EventSummary struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
	Tour string `json:"tour"`
	// Category folds both tours' level vocabularies into the list the index
	// groups by: slam, masters, finals, olympics, team, tour, challenger,
	// futures, itf. Level and tier are the latest edition's own.
	Category    string `json:"category"`
	Level       string `json:"level"`
	Tier        string `json:"tier"`
	FirstSeason int16  `json:"first_season"`
	LastSeason  int16  `json:"last_season"`
	Editions    int    `json:"editions"`
}

// Event is a tournament across seasons, on the terms of ADR-0012.
type Event struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
	Tour string `json:"tour"`
	// Keyed says what the run is identified by: "number" (the tour's own),
	// "name", or "team" (a competition). Number is the tour's number when
	// that is the key, null otherwise.
	Keyed       string    `json:"keyed"`
	Number      *string   `json:"number"`
	FirstSeason int16     `json:"first_season"`
	LastSeason  int16     `json:"last_season"`
	Names       []NameRun `json:"names"`
	// Provenance counts the editions by how they were placed on this event.
	// It is printed, not hidden: an edition here by the tour's number and one
	// here by name are two different claims.
	Provenance Provenance     `json:"provenance"`
	Editions   []EventEdition `json:"editions"`
}

// NameRun is one stretch of seasons under one name.
type NameRun struct {
	Name        string `json:"name"`
	FirstSeason int16  `json:"first_season"`
	LastSeason  int16  `json:"last_season"`
}

// Provenance is how many editions arrived by each link (ADR-0012).
type Provenance struct {
	Number   int `json:"number"`
	Override int `json:"override"`
	Bridged  int `json:"bridged"`
	Name     int `json:"name"`
	Team     int `json:"team"`
}

// EventEdition is one season on an event page: what the sheet writes at the
// top. A team competition has one per season, holding its ties and no final.
type EventEdition struct {
	Season    int16   `json:"season"`
	Name      string  `json:"name"`
	Level     string  `json:"level"`
	Tier      string  `json:"tier"`
	Surface   *string `json:"surface"`
	DrawSize  *int16  `json:"draw_size"`
	StartDate string  `json:"start_date"`
	// Link is how this edition got onto the event: number, override,
	// bridged, name or team.
	Link       string    `json:"link"`
	Champion   *Opponent `json:"champion"`
	Finalist   *Opponent `json:"finalist"`
	FinalScore *string   `json:"final_score"`
	Matches    int       `json:"matches"`
	Ties       int       `json:"ties"`
}

// EventRef names an event well enough to link to.
type EventRef struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
	Tour string `json:"tour"`
}

// Edition is one season of an event as a draw sheet.
type Edition struct {
	Event      EventRef  `json:"event"`
	Season     int16     `json:"season"`
	Name       string    `json:"name"`
	Level      string    `json:"level"`
	Tier       string    `json:"tier"`
	Surface    *string   `json:"surface"`
	DrawSize   *int16    `json:"draw_size"`
	StartDate  string    `json:"start_date"`
	Link       string    `json:"link"`
	Champion   *Opponent `json:"champion"`
	Finalist   *Opponent `json:"finalist"`
	FinalScore *string   `json:"final_score"`
	// Serve says once for the edition whether its matches carry serve lines,
	// rather than fifty-six times in the margin.
	Serve   EditionServe   `json:"serve"`
	Seeds   []SeedLine     `json:"seeds"`
	Matches []EditionMatch `json:"matches"`
}

// EditionServe is the edition's serve-statistics coverage.
type EditionServe struct {
	// Availability is recorded when every match carries a line, partial when
	// some do, and otherwise says why none does, in the profile's vocabulary.
	Availability string `json:"availability"`
	MatchesWith  int    `json:"matches_with"`
	Matches      int    `json:"matches"`
}

// SeedLine is one seed and where they went out: the round of their last
// main-draw loss, or W for the champion.
type SeedLine struct {
	Seed   int16    `json:"seed"`
	Player Opponent `json:"player"`
	Exit   string   `json:"exit"`
}

// EditionMatch is one match on the sheet. Players are winner first.
type EditionMatch struct {
	Round      string `json:"round"`
	MatchNum   int16  `json:"match_num"`
	Qualifying bool   `json:"qualifying"`
	BestOf     int16  `json:"best_of"`
	// Tie names the tie a team competition's match belongs to; null otherwise.
	Tie        *string        `json:"tie"`
	Players    [2]EditionSide `json:"players"`
	Score      *string        `json:"score"`
	Incomplete bool           `json:"incomplete"`
	Minutes    *int16         `json:"minutes"`
	// Serve is each side's line, in Players order; null where the match has
	// none, and the edition's Serve says why.
	Serve      [2]*MatchServe `json:"serve"`
	ChartingID *string        `json:"charting_id"`
}

// EditionSide is one player as the sheet writes them: name, seed in
// brackets, entry and ranking in the margin.
type EditionSide struct {
	Slug    string  `json:"slug"`
	Name    string  `json:"name"`
	Country *string `json:"country"`
	Seed    *int16  `json:"seed"`
	Entry   *string `json:"entry"`
	Rank    *int32  `json:"rank"`
}

// eventCursor is the keyset position in the index: the last row's ordering
// key, all three of which ascend.
type eventCursor struct {
	Rank int32  `json:"r"`
	Name string `json:"n"`
	ID   int64  `json:"i"`
}

const (
	eventsDefaultLimit = 100
	eventsMaxLimit     = 500
)

var eventCategories = map[string]bool{
	"slam": true, "masters": true, "finals": true, "olympics": true, "team": true,
	"tour": true, "challenger": true, "futures": true, "itf": true,
}

func (a *API) handleEvents(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	params := db.ListEventsParams{}

	switch raw := query.Get("tour"); raw {
	case "":
	case string(db.TourAtp), string(db.TourWta):
		tour := db.Tour(raw)
		params.Tour = &tour
	default:
		BadRequest(w, r, "tour must be atp or wta.")
		return
	}
	if raw := query.Get("level"); raw != "" {
		if !eventCategories[raw] {
			BadRequest(w, r, "level must be slam, masters, finals, olympics, team, tour, challenger, futures or itf.")
			return
		}
		params.Category = &raw
	}
	if raw := strings.TrimSpace(query.Get("q")); raw != "" {
		params.Q = &raw
	}
	limit, err := Limit(r, eventsDefaultLimit, eventsMaxLimit)
	if err != nil {
		BadRequest(w, r, err.Error())
		return
	}
	params.RowLimit = int32(limit)
	if raw := query.Get("cursor"); raw != "" {
		var c eventCursor
		if err := DecodeCursor(raw, &c); err != nil {
			BadRequest(w, r, "The cursor is not one this API issued.")
			return
		}
		params.AfterRank, params.AfterName, params.AfterID = &c.Rank, &c.Name, &c.ID
	}

	rows, err := a.Queries.ListEvents(r.Context(), params)
	if err != nil {
		Internal(w, r, err)
		return
	}
	page := Page[EventSummary]{Data: make([]EventSummary, 0, len(rows))}
	for _, row := range rows {
		page.Data = append(page.Data, EventSummary{
			Slug: row.Slug, Name: row.Name, Tour: row.Tour, Category: row.Category,
			Level: row.Level, Tier: row.Tier,
			FirstSeason: row.FirstSeason, LastSeason: row.LastSeason, Editions: int(row.Editions),
		})
	}
	if len(rows) == limit {
		last := rows[len(rows)-1]
		cursor, err := EncodeCursor(eventCursor{Rank: last.Rank, Name: last.Name, ID: last.ID})
		if err != nil {
			Internal(w, r, err)
			return
		}
		page.NextCursor = cursor
	}
	writeJSON(w, r, http.StatusOK, page)
}

func (a *API) handleEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	event, err := a.Queries.GetEventBySlug(ctx, chi.URLParam(r, "slug"))
	if errors.Is(err, pgx.ErrNoRows) {
		NotFound(w, r, "No tournament has that slug.")
		return
	}
	if err != nil {
		Internal(w, r, err)
		return
	}
	rows, err := a.Queries.ListEventEditions(ctx, event.ID)
	if err != nil {
		Internal(w, r, err)
		return
	}

	out := Event{
		Slug: event.Slug, Name: event.Name, Tour: event.Tour,
		FirstSeason: event.FirstSeason, LastSeason: event.LastSeason,
		Names: []NameRun{}, Editions: []EventEdition{},
	}
	out.Keyed, out.Number = keyedBy(event.Key)

	for _, row := range rows {
		switch link(row.EventLink) {
		case "number":
			out.Provenance.Number++
		case "override":
			out.Provenance.Override++
		case "bridged":
			out.Provenance.Bridged++
		case "team":
			out.Provenance.Team++
		default:
			out.Provenance.Name++
		}

		// A team competition's season is its ties, folded into one edition.
		if link(row.EventLink) == "team" && len(out.Editions) > 0 &&
			out.Editions[len(out.Editions)-1].Season == row.Season {
			last := &out.Editions[len(out.Editions)-1]
			last.Ties++
			last.Matches += int(row.Matches)
			continue
		}
		ed := EventEdition{
			Season: row.Season, Name: row.Name, Level: row.Level, Tier: row.Tier,
			Surface: nonEmpty(row.Surface), DrawSize: row.DrawSize,
			StartDate: row.StartDate.Format(time.DateOnly), Link: link(row.EventLink),
			FinalScore: row.FinalScore, Matches: int(row.Matches),
		}
		if row.ChampionSlug != nil && row.ChampionName != nil {
			ed.Champion = &Opponent{Slug: *row.ChampionSlug, Name: *row.ChampionName}
		}
		if row.FinalistSlug != nil && row.FinalistName != nil {
			ed.Finalist = &Opponent{Slug: *row.FinalistSlug, Name: *row.FinalistName}
		}
		if ed.Link == "team" {
			ed.Name = event.Name
			ed.Ties = 1
		}
		out.Editions = append(out.Editions, ed)
	}
	out.Names = nameRuns(out.Editions)

	writeJSON(w, r, http.StatusOK, out)
}

func (a *API) handleEdition(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	season, err := strconv.Atoi(chi.URLParam(r, "season"))
	if err != nil || season < firstSeason || season > lastSeason {
		BadRequest(w, r, "season must be a four-digit year.")
		return
	}
	event, err := a.Queries.GetEventBySlug(ctx, chi.URLParam(r, "slug"))
	if errors.Is(err, pgx.ErrNoRows) {
		NotFound(w, r, "No tournament has that slug.")
		return
	}
	if err != nil {
		Internal(w, r, err)
		return
	}
	rows, err := a.Queries.ListEditionRows(ctx, db.ListEditionRowsParams{EventID: event.ID, Season: int16(season)})
	if err != nil {
		Internal(w, r, err)
		return
	}
	if len(rows) == 0 {
		NotFound(w, r, "That tournament was not played that season.")
		return
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	matches, err := a.Queries.ListEditionMatches(ctx, ids)
	if err != nil {
		Internal(w, r, err)
		return
	}

	first := rows[0]
	out := Edition{
		Event:  EventRef{Slug: event.Slug, Name: event.Name, Tour: event.Tour},
		Season: first.Season, Name: first.Name, Level: first.Level, Tier: first.Tier,
		Surface: nonEmpty(first.Surface), DrawSize: first.DrawSize,
		StartDate: first.StartDate.Format(time.DateOnly), Link: link(first.EventLink),
		Seeds: []SeedLine{}, Matches: make([]EditionMatch, 0, len(matches)),
	}
	team := out.Link == "team"
	if team {
		out.Name = event.Name
	}

	tier := db.Tier(first.Tier)
	for _, m := range matches {
		em := EditionMatch{
			Round: m.Round, MatchNum: m.MatchNum, Qualifying: m.IsQualifying, BestOf: m.BestOf,
			Score: m.Score, Incomplete: m.Incomplete, Minutes: m.Minutes, ChartingID: m.ChartingID,
			Players: [2]EditionSide{
				{Slug: m.WinnerSlug, Name: m.WinnerName, Country: m.WinnerCountry,
					Seed: m.WinnerSeed, Entry: m.WinnerEntry, Rank: m.WinnerRank},
				{Slug: m.LoserSlug, Name: m.LoserName, Country: m.LoserCountry,
					Seed: m.LoserSeed, Entry: m.LoserEntry, Rank: m.LoserRank},
			},
		}
		if team {
			tie := m.Tie
			em.Tie = &tie
		}
		if m.WServePoints != nil {
			em.Serve[0] = &MatchServe{
				Availability: AvailabilityRecorded, Aces: m.WAces, DoubleFaults: m.WDoubleFaults,
				ServePoints: m.WServePoints, FirstIn: m.WFirstIn, FirstWon: m.WFirstWon,
				SecondWon: m.WSecondWon, ServeGames: m.WServeGames,
				BreakPointsSaved: m.WBpSaved, BreakPointsFaced: m.WBpFaced,
			}
			out.Serve.MatchesWith++
		}
		if m.LServePoints != nil {
			em.Serve[1] = &MatchServe{
				Availability: AvailabilityRecorded, Aces: m.LAces, DoubleFaults: m.LDoubleFaults,
				ServePoints: m.LServePoints, FirstIn: m.LFirstIn, FirstWon: m.LFirstWon,
				SecondWon: m.LSecondWon, ServeGames: m.LServeGames,
				BreakPointsSaved: m.LBpSaved, BreakPointsFaced: m.LBpFaced,
			}
		}
		if m.Round == "F" && !m.IsQualifying && !m.IsTeamEvent {
			out.Champion = &Opponent{Slug: m.WinnerSlug, Name: m.WinnerName}
			out.Finalist = &Opponent{Slug: m.LoserSlug, Name: m.LoserName}
			out.FinalScore = m.Score
		}
		out.Matches = append(out.Matches, em)
	}
	out.Serve.Matches = len(matches)
	switch {
	case len(matches) == 0:
		out.Serve.Availability = AvailabilityNotRecorded
	case out.Serve.MatchesWith == len(matches):
		out.Serve.Availability = AvailabilityRecorded
	case out.Serve.MatchesWith > 0:
		out.Serve.Availability = AvailabilityPartial
	default:
		out.Serve.Availability = explainMatchStats(tier, season)
	}
	if !team {
		out.Seeds = seedLines(matches, out.Champion)
	}

	writeJSON(w, r, http.StatusOK, out)
}

// keyedBy reads an event key: number:404, name:tour:wimbledon, team:davis-cup.
func keyedBy(key string) (string, *string) {
	kind, rest, _ := strings.Cut(key, ":")
	if kind == "number" {
		return kind, &rest
	}
	return kind, nil
}

func link(l *string) string {
	if l == nil {
		return "name"
	}
	return *l
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// nameRuns folds consecutive editions under one name into a run, so the page
// can say "Bari 1989, Genova 1990-93, St. Pölten 1994-2005, Kitzbühel 2009-".
func nameRuns(editions []EventEdition) []NameRun {
	runs := []NameRun{}
	for _, ed := range editions {
		if n := len(runs); n > 0 && runs[n-1].Name == ed.Name {
			runs[n-1].LastSeason = ed.Season
			continue
		}
		runs = append(runs, NameRun{Name: ed.Name, FirstSeason: ed.Season, LastSeason: ed.Season})
	}
	return runs
}

// seedLines lists the main-draw seeds and where each went out. A seed's exit
// is the round of their loss; the champion's is W. Seeds are read off the
// matches, so a seed who lost a first-round match is on the list too.
func seedLines(matches []db.ListEditionMatchesRow, champion *Opponent) []SeedLine {
	type seen struct {
		line SeedLine
		lost bool
	}
	bySlug := map[string]*seen{}
	for _, m := range matches {
		if m.IsQualifying || m.IsTeamEvent {
			continue
		}
		sides := []struct {
			slug, name string
			seed       *int16
			won        bool
		}{
			{m.WinnerSlug, m.WinnerName, m.WinnerSeed, true},
			{m.LoserSlug, m.LoserName, m.LoserSeed, false},
		}
		for _, s := range sides {
			if s.seed == nil {
				continue
			}
			entry, ok := bySlug[s.slug]
			if !ok {
				entry = &seen{line: SeedLine{Seed: *s.seed, Player: Opponent{Slug: s.slug, Name: s.name}}}
				bySlug[s.slug] = entry
			}
			if !s.won && !entry.lost {
				entry.line.Exit = m.Round
				entry.lost = true
			}
		}
	}
	out := make([]SeedLine, 0, len(bySlug))
	for _, s := range bySlug {
		if !s.lost {
			if champion != nil && champion.Slug == s.line.Player.Slug {
				s.line.Exit = "W"
			} else {
				// Seeded, never lost, never won the final: the draw is
				// incomplete in the source, and the margin says so.
				s.line.Exit = "n/r"
			}
		}
		out = append(out, s.line)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Seed != out[j].Seed {
			return out[i].Seed < out[j].Seed
		}
		return out[i].Player.Slug < out[j].Player.Slug
	})
	return out
}
