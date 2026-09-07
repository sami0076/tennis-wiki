package rating

import (
	"strings"
	"testing"
)

func TestRoundRankOrdersADraw(t *testing.T) {
	order := []string{"Q1", "Q3", "R128", "R32", "QF", "SF", "F"}
	for i := 1; i < len(order); i++ {
		if RoundRank(order[i-1]) >= RoundRank(order[i]) {
			t.Errorf("%s does not sort before %s", order[i-1], order[i])
		}
	}
	// The challenge round was played after the final, not before it.
	if RoundRank("CR") <= RoundRank("F") {
		t.Error("the challenge round sorts before the final")
	}
	if RoundRank("NOPE") != len(rounds) {
		t.Errorf("an unknown round ranked %d, want last", RoundRank("NOPE"))
	}
}

// The database sorts by this expression and the engine assumes that order, so
// the two have to come from the same list.
func TestRoundRankSQLCoversEveryRound(t *testing.T) {
	sql := RoundRankSQL("m.round")
	if !strings.HasPrefix(sql, "CASE m.round WHEN 'Q1' THEN 0") {
		t.Errorf("unexpected expression: %s", sql)
	}
	for i, r := range rounds {
		if !strings.Contains(sql, "WHEN '"+r+"' THEN "+itoa(i)) {
			t.Errorf("%s is missing from the expression", r)
		}
	}
	if !strings.HasSuffix(sql, "ELSE "+itoa(len(rounds))+" END") {
		t.Errorf("unknown rounds are not ranked last: %s", sql)
	}
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
