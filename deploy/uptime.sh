#!/usr/bin/env sh
# Is the site up? Two requests, retried once so a blip is not a page.
#
#   deploy/uptime.sh https://api.deucepoint.net https://deucepoint.net
set -eu

api="${1:?usage: uptime.sh https://api.host https://site.host}"
site="${2:?usage: uptime.sh https://api.host https://site.host}"

# get URL PATTERN: 200 and the body contains PATTERN, with one retry.
get() {
	for attempt in 1 2; do
		code=$(curl -sS -o /tmp/uptime.body -w '%{http_code}' --max-time 20 "$1" || echo 000)
		if [ "$code" = "200" ] && grep -q "$2" /tmp/uptime.body; then
			echo "ok   $1"
			return 0
		fi
		[ "$attempt" = 1 ] && sleep 30
	done
	echo "FAIL $1 -> HTTP $code"
	head -c 300 /tmp/uptime.body
	echo
	exit 1
}

get "$api/api/v1/health" '"database":"ok"'
get "$site/" '<div id="root">'
