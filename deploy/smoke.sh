#!/usr/bin/env sh
# Smoke check for a deployed API. "It deployed" and "it works" are different
# claims; this is the second one. Exits non-zero on the first request that is
# not a 200 with the shape a page would read.
#
#   deploy/smoke.sh https://api.deucepoint.net
set -eu

base="${1:?usage: smoke.sh https://api.host}"
base="${base%/}/api/v1"

get() {
	code=$(curl -sS -o /tmp/smoke.json -w '%{http_code}' "$base$1")
	if [ "$code" != "200" ]; then
		echo "FAIL $1 -> HTTP $code"
		head -c 300 /tmp/smoke.json
		echo
		exit 1
	fi
	echo "ok   $1"
}

get /health
grep -q '"database":"ok"' /tmp/smoke.json || { echo "FAIL health: database not ok"; exit 1; }

get /coverage
grep -q '"atp"' /tmp/smoke.json || { echo "FAIL coverage: no ATP dates, is the database loaded?"; exit 1; }

# Two names that exist in every load, one per tour.
get "/players?q=federer&tour=atp"
slug=$(grep -o '"slug":"[^"]*"' /tmp/smoke.json | head -1 | cut -d'"' -f4)
[ -n "$slug" ] || { echo "FAIL search: no player for federer"; exit 1; }
get "/players/$slug"
get "/players/$slug/matches"

get "/players?q=williams&tour=wta"
wslug=$(grep -o '"slug":"[^"]*"' /tmp/smoke.json | head -1 | cut -d'"' -f4)
[ -n "$wslug" ] || { echo "FAIL search: no player for williams"; exit 1; }

get "/players?q=nadal&tour=atp"
opp=$(grep -o '"slug":"[^"]*"' /tmp/smoke.json | head -1 | cut -d'"' -f4)
get "/h2h/$slug/$opp"

get "/rankings?tour=atp"
get "/simulate/match?a=$slug&b=$opp&surface=clay"

# Wimbledon 2019 is in every load and is the draw the site opens on: a full
# 128 bracket, so the sheet and the simulator both have to answer for it.
get "/tournaments?q=wimbledon&tour=atp"
event=$(grep -o '"slug":"[^"]*"' /tmp/smoke.json | head -1 | cut -d'"' -f4)
[ -n "$event" ] || { echo "FAIL tournaments: no Wimbledon, has the events stage run?"; exit 1; }
get "/tournaments/$event"
get "/tournaments/$event/2019"
grep -q '"matches":\[{' /tmp/smoke.json || { echo "FAIL edition: no matches on the sheet"; exit 1; }
get "/simulate/draw?event=$event&season=2019&runs=200"

echo "all good: $base"
