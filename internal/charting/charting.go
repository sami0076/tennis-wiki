// Package charting ingests the Match Charting Project on the terms of
// ADR-0011: as figures attached to matches the database already holds.
//
// The project's files carry names, dates and rounds but no ids, no winner and
// no score, so a charted match is not a match record and is never stored as
// one. It is resolved to a row in matches by tour, date, round and the two
// names, and its per-set figures hang off that row. A charted match that finds
// no row, or more than one, is counted and left; a name is never looked up in
// the players table.
package charting

import (
	"time"

	"github.com/sami0076/tennis-wiki/internal/ingest"
)

// Match is one row of charting-*-matches.csv that read cleanly.
type Match struct {
	// ID is the project's key, e.g. 20260521-M-Roland_Garros-Q3-Jesper_De_Jong-Michael_Zheng.
	ID        string
	Tour      ingest.Tour
	Player1   string
	Player2   string
	PlayedOn  time.Time
	Event     string
	Round     string
	ChartedBy string
}

// Stat is one row of charting-*-stats-Overview.csv: one player's figures for
// one set, or for the match when Set is 0.
type Stat struct {
	MatchID string
	Player  string
	Set     int
	Figures Figures
}

// Figures are the stats-Overview columns, in the file's order.
type Figures struct {
	ServePoints     int16
	Aces            int16
	DoubleFaults    int16
	FirstIn         int16
	FirstWon        int16
	SecondIn        int16
	SecondWon       int16
	BPFaced         int16
	BPSaved         int16
	ReturnPoints    int16
	ReturnPointsWon int16
	Winners         int16
	WinnersFH       int16
	WinnersBH       int16
	Unforced        int16
	UnforcedFH      int16
	UnforcedBH      int16
}
