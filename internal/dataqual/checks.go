package dataqual

// Severity says how a finding should be read.
type Severity string

// Severities, in ascending order of urgency.
const (
	// Info is a fact about the data, not a defect. Coverage gaps live here:
	// they are real and disclosed, not bugs.
	Info Severity = "info"
	// Warning is worth a look but does not fail the report.
	Warning Severity = "warning"
	// Integrity means the database contradicts itself. These fail the run,
	// because every one of them silently corrupts a downstream statistic.
	Integrity Severity = "integrity"
)

// Check is one query whose row count is the finding.
type Check struct {
	Name     string
	Severity Severity
	// Why explains what a non-zero count means, so the report is readable by
	// someone who did not write the query.
	Why   string
	Query string
	// Sample optionally returns a few offending rows to make a failure
	// diagnosable rather than merely alarming.
	Sample string
}

// integrityChecks must all be zero. Each corresponds to a way the database can
// disagree with itself; the first two exist because both actually happened.
var integrityChecks = []Check{
	{
		Name:     "matches_without_two_players",
		Severity: Integrity,
		Why: "Every match has exactly two participants. More means two source rows " +
			"collapsed onto one natural key; fewer means a player faced themselves.",
		Query: `SELECT count(*) FROM (
		          SELECT match_id FROM match_players GROUP BY match_id HAVING count(*) <> 2
		        ) x`,
		Sample: `SELECT t.name || ' ' || t.season || ' match ' || m.match_num ||
		                ' (' || m.round || ', qualifying=' || m.is_qualifying || '): ' ||
		                count(*) || ' participants'
		           FROM matches m
		           JOIN tournaments t ON t.id = m.tournament_id
		           JOIN match_players mp ON mp.match_id = m.id
		          GROUP BY m.id, t.name, t.season, m.match_num, m.round, m.is_qualifying
		         HAVING count(*) <> 2 LIMIT 5`,
	},
	{
		Name:     "winner_did_not_win",
		Severity: Integrity,
		Why: "matches.winner_id is denormalised from match_players.won. If they " +
			"disagree, every win-loss record built from either is wrong.",
		Query: `SELECT count(*) FROM matches m
		         WHERE NOT EXISTS (
		           SELECT 1 FROM match_players mp
		            WHERE mp.match_id = m.id AND mp.player_id = m.winner_id AND mp.won
		         )`,
		Sample: `SELECT t.name || ' ' || t.season || ' match ' || m.match_num
		           FROM matches m JOIN tournaments t ON t.id = m.tournament_id
		          WHERE NOT EXISTS (
		            SELECT 1 FROM match_players mp
		             WHERE mp.match_id = m.id AND mp.player_id = m.winner_id AND mp.won
		          ) LIMIT 5`,
	},
	{
		Name:     "matches_without_one_winner",
		Severity: Integrity,
		Why:      "A match has exactly one winner. Two or zero breaks every aggregate.",
		Query: `SELECT count(*) FROM (
		          SELECT match_id FROM match_players
		           GROUP BY match_id HAVING count(*) FILTER (WHERE won) <> 1
		        ) x`,
	},
	{
		Name:     "stats_claimed_but_absent",
		Severity: Integrity,
		Why: "has_detailed_stats is recorded at ingest and must agree with the " +
			"rows. A mismatch means rate statistics silently use the wrong denominator.",
		Query: `SELECT count(*) FROM matches m
		         WHERE m.has_detailed_stats
		           AND EXISTS (
		             SELECT 1 FROM match_players mp
		              WHERE mp.match_id = m.id AND mp.serve_points IS NULL
		           )`,
	},
	{
		Name:     "impossible_serve_numbers",
		Severity: Integrity,
		Why:      "Points won on serve cannot exceed points served.",
		Query: `SELECT count(*) FROM match_players
		         WHERE serve_points IS NOT NULL
		           AND (first_in > serve_points
		             OR first_won > first_in
		             OR COALESCE(bp_saved, 0) > COALESCE(bp_faced, 0))`,
	},
	{
		Name:     "rankings_without_player",
		Severity: Integrity,
		Why:      "A ranking row referring to no player means the player ingest dropped rows.",
		Query: `SELECT count(*) FROM rankings r
		         WHERE NOT EXISTS (SELECT 1 FROM players p WHERE p.id = r.player_id)`,
	},
	{
		Name:     "duplicate_slugs",
		Severity: Integrity,
		Why:      "Slugs are URL keys and must be unique.",
		Query:    `SELECT count(*) FROM (SELECT slug FROM players GROUP BY slug HAVING count(*) > 1) x`,
	},
}

// anomalyChecks describe the data rather than fault it. These are the section
// 6.2 handling rules, reported so the handling is visible rather than assumed.
var anomalyChecks = []Check{
	{
		Name:     "matches_without_surface",
		Severity: Info,
		Why:      "Surface was empty or None in the source and is stored NULL rather than guessed.",
		Query:    `SELECT count(*) FROM matches WHERE surface IS NULL`,
	},
	{
		Name:     "incomplete_matches",
		Severity: Info,
		Why:      "Retirements, walkovers and defaults. Counted in win-loss, excluded from stat aggregates.",
		Query:    `SELECT count(*) FROM matches WHERE incomplete`,
	},
	{
		Name:     "team_event_matches",
		Severity: Info,
		Why:      "Davis Cup and Billie Jean King Cup. Ingested and flagged, excluded from Elo by default.",
		Query:    `SELECT count(*) FROM matches WHERE is_team_event`,
	},
	{
		Name:     "qualifying_matches",
		Severity: Info,
		Why:      "Qualifying draws, derived from the round rather than the source filename.",
		Query:    `SELECT count(*) FROM matches WHERE is_qualifying`,
	},
	{
		Name:     "matches_without_stats",
		Severity: Info,
		Why: "No serve statistics recorded. Expected before roughly 1991, at Futures " +
			"and ITF level always, and at Challenger level before about 2010.",
		Query: `SELECT count(*) FROM matches WHERE NOT has_detailed_stats`,
	},
	{
		Name:     "matches_without_score",
		Severity: Warning,
		Why:      "No score string at all, so no set or game detail can be derived.",
		Query:    `SELECT count(*) FROM matches WHERE score IS NULL OR score = ''`,
	},
	{
		Name:     "implausible_age",
		Severity: Warning,
		Why:      "Ages outside 12-60 suggest a bad date of birth in the source.",
		Query:    `SELECT count(*) FROM match_players WHERE age IS NOT NULL AND (age < 12 OR age > 60)`,
	},
	{
		Name:     "implausible_duration",
		Severity: Warning,
		Why:      "Matches under 15 minutes or over 8 hours are almost always data errors.",
		Query:    `SELECT count(*) FROM matches WHERE minutes IS NOT NULL AND (minutes < 15 OR minutes > 480)`,
	},
	{
		Name:     "players_without_country",
		Severity: Info,
		Why:      "Country is absent for some lower-tier players.",
		Query:    `SELECT count(*) FROM players WHERE country IS NULL`,
	},
	{
		Name:     "zero_serve_points",
		Severity: Info,
		Why: "Genuinely zero, not missing: a player who retired before serving. " +
			"NULL means unrecorded, zero means recorded as zero.",
		Query: `SELECT count(*) FROM match_players WHERE serve_points = 0`,
	},
}

// coverageQuery is the season/tour/tier matrix. The gaps it shows are the ones
// disclosed in DATA_LICENSE.md, and they must be visible here rather than
// something a reader has to already know.
const coverageQuery = `
	SELECT t.season,
	       t.tour::text,
	       t.tier::text,
	       count(*) AS matches,
	       count(*) FILTER (WHERE m.has_detailed_stats) AS with_stats
	  FROM matches m
	  JOIN tournaments t ON t.id = m.tournament_id
	 GROUP BY t.season, t.tour, t.tier
	 ORDER BY t.season, t.tour, t.tier`

// provenanceQuery shows which source wrote what, so a mirror swap is auditable.
const provenanceQuery = `
	SELECT source, count(*) AS matches,
	       min(played_on)::text AS first_match,
	       max(played_on)::text AS last_match
	  FROM matches
	 GROUP BY source
	 ORDER BY count(*) DESC`
