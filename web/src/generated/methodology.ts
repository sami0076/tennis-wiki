// Generated from docs/methodology.md and docs/validation.json by scripts/methodology.mjs.
// Do not edit: run `npm run methodology` instead. CI regenerates this file and
// fails if it differs, the same way it does for the API types.

export interface MethodologyHeading {
  id: string
  text: string
  level: number
}

/** The figures the checks produced, so the page cannot quote a different set. */
export interface MethodologyChecks {
  generatedAt: string
  matchesReplayed: number
  tourAccuracy: number
  worstCalibration: number
  promotions: number
  promotionMatches: number
  promotionExpected: number
  promotionActual: number
  decidingSampled: number
  decidingExpected: number
  decidingObserved: number
  events: number
  brier: number
  brierUniform: number
  topPick: number
  championOdds: number
}

export const source = "docs/methodology.md"
export const checks: MethodologyChecks = {
  "generatedAt": "2026-09-10T15:17:28.626691Z",
  "matchesReplayed": 1585442,
  "tourAccuracy": 69.94282457296546,
  "worstCalibration": 1.6261937913224715,
  "promotions": 7054,
  "promotionMatches": 81161,
  "promotionExpected": 31341.53217597121,
  "promotionActual": 33152,
  "decidingSampled": 30000,
  "decidingExpected": 44.284665119238454,
  "decidingObserved": 34.14333333333333,
  "events": 296,
  "brier": 0.8550295743243183,
  "brierUniform": 0.9696209881756757,
  "topPick": 27.364864864864863,
  "championOdds": 14.373817567567585
}
export const headings: ReadonlyArray<MethodologyHeading> = [
  {
    "id": "methodology",
    "text": "Methodology",
    "level": 1
  },
  {
    "id": "what-the-data-is",
    "text": "What the data is",
    "level": 2
  },
  {
    "id": "coverage-and-the-gap",
    "text": "Coverage, and the gap",
    "level": 2
  },
  {
    "id": "where-statistics-do-not-exist",
    "text": "Where statistics do not exist",
    "level": 2
  },
  {
    "id": "what-is-excluded-from-statistics",
    "text": "What is excluded from statistics",
    "level": 2
  },
  {
    "id": "how-a-rating-is-computed",
    "text": "How a rating is computed",
    "level": 2
  },
  {
    "id": "the-tier-weights-and-the-evidence-for-them",
    "text": "The tier weights, and the evidence for them",
    "level": 2
  },
  {
    "id": "what-the-simulator-can-be-checked-for-and-what-it-cannot",
    "text": "What the simulator can be checked for, and what it cannot",
    "level": 2
  },
  {
    "id": "the-chain-expects-too-many-deciding-sets",
    "text": "The chain expects too many deciding sets",
    "level": 3
  },
  {
    "id": "the-draw-simulation-beats-knowing-nothing-and-not-by-as-much-as-it-looks",
    "text": "The draw simulation beats knowing nothing, and not by as much as it looks",
    "level": 3
  },
  {
    "id": "tiers",
    "text": "Tiers",
    "level": 2
  }
]
export const html = "<h1 id=\"methodology\">Methodology</h1>\n<p>How the numbers on this site are produced, and — as much as anything — where they stop.</p>\n<blockquote>\n<p>Everything below is current: coverage, the rating engine, the tier weights and what the\nvalidation checks say about all three. The figures come from the run committed in\n<a href=\"https://github.com/sami0076/tennis-wiki/blob/main/docs/validation.json\"><code>validation.json</code></a>, and the page that renders this document fails to\nbuild if the two disagree.</p>\n</blockquote>\n<h2 id=\"what-the-data-is\">What the data is</h2>\n<p>Every match, player, and ranking here descends from <a href=\"https://github.com/JeffSackmann\">Jeff Sackmann&#39;s Tennis\nAbstract</a> datasets, licensed CC BY-NC-SA 4.0. The original\nrepositories were withdrawn during this project; the current sources are mirrors and\nderivatives, recorded in <a href=\"https://github.com/sami0076/tennis-wiki/blob/main/docs/decisions/0002-data-sources-after-upstream-removal.md\">ADR-0002</a>\nand configured in <a href=\"https://github.com/sami0076/tennis-wiki/blob/main/configs/sources.json\"><code>configs/sources.json</code></a>. Full attribution is in\n<a href=\"https://github.com/sami0076/tennis-wiki/blob/main/DATA_LICENSE.md\"><code>DATA_LICENSE.md</code></a>.</p>\n<p>Every match row records which source produced it, so any claim below can be checked against\nthe database rather than taken on trust.</p>\n<h2 id=\"coverage-and-the-gap\">Coverage, and the gap</h2>\n<p><strong>Ask the site, not this page.</strong> <code>GET /api/v1/coverage</code> reports matches, date range, and\nthe share carrying serve statistics per tour and tier, queried live. This page describes\nthe shape; the endpoint has the current numbers.</p>\n<p>Full-schema data — the layout carrying serve statistics — runs out before the present:</p>\n<table>\n<thead>\n<tr>\n<th>Tour</th>\n<th>Full schema through</th>\n</tr>\n</thead>\n<tbody><tr>\n<td>ATP</td>\n<td>2026-01-17</td>\n</tr>\n<tr>\n<td>WTA</td>\n<td>2024-12-31</td>\n</tr>\n</tbody></table>\n<p><a href=\"https://github.com/sami0076/tennis-wiki/blob/main/docs/decisions/0006-accept-and-disclose-the-coverage-gap.md\">ADR-0006</a> decided to accept that\ngap rather than fill it with results-only data or by scraping. The reasoning is there in\nfull; the short version is that mixing two data regimes in one database produces\nplausible-looking wrong numbers, and this project would rather be visibly behind than\nquietly wrong.</p>\n<p>What the gap means in practice:</p>\n<ul>\n<li><strong>Recent form is incomplete.</strong> A &quot;last 12 months&quot; window covers less than it says for the\nWTA, and the site should not pretend otherwise.</li>\n<li><strong>Ratings for active players stop short of the present.</strong> A &quot;current top eight&quot; is\ncurrent as of the coverage date, not as of today.</li>\n<li><strong>The simulator&#39;s inputs do not end where the serve statistics do</strong>, but its anchor does.\nPoint-win probabilities are derived from ratings rather than from per-player serve rates\n(<a href=\"https://github.com/sami0076/tennis-wiki/blob/main/docs/decisions/0007-elo-derived-point-probability.md\">ADR-0007</a>), so any rated player can be\nsimulated; the tour-and-surface average that anchors the derivation is still measured from\nthe 3% of matches that recorded serve statistics.</li>\n</ul>\n<h2 id=\"where-statistics-do-not-exist\">Where statistics do not exist</h2>\n<p>Absence is not zero, and the API keeps three kinds of absence apart rather than collapsing\nthem into one null. <code>serve.availability</code> on a player profile is one of:</p>\n<table>\n<thead>\n<tr>\n<th>Value</th>\n<th>Meaning</th>\n</tr>\n</thead>\n<tbody><tr>\n<td><code>recorded</code></td>\n<td>every eligible match carried statistics</td>\n</tr>\n<tr>\n<td><code>partial</code></td>\n<td>some did, some did not</td>\n</tr>\n<tr>\n<td><code>never_recorded_for_tier</code></td>\n<td>Futures and ITF, where nothing has ever been recorded in any year</td>\n</tr>\n<tr>\n<td><code>never_recorded_in_era</code></td>\n<td>before 1991 anywhere; before roughly 2010 at Challenger level</td>\n</tr>\n<tr>\n<td><code>not_recorded</code></td>\n<td>absent, reason unknown — the honest fallback</td>\n</tr>\n</tbody></table>\n<p>When <code>availability</code> is not <code>recorded</code> or <code>partial</code>, <code>rates</code> is <code>null</code> rather than a set of\nzeroes. Within <code>rates</code>, an individual rate with no denominator is also <code>null</code>: a player who\nnever faced a break point has not saved 0% of them.</p>\n<p>This matters more at full depth than it sounds. Futures and ITF are the majority of the\n1.6 million matches, and no Futures match in any year has ever recorded a serve statistic.\nFor most of the 115,000 players on this site, &quot;we do not have this&quot; is the accurate answer,\nand the interesting engineering problem is saying <em>why</em> rather than showing a wall of\nzeroes.</p>\n<h2 id=\"what-is-excluded-from-statistics\">What is excluded from statistics</h2>\n<ul>\n<li><strong>Retirements and walkovers</strong> count in win/loss records and are reported separately in\n<code>incomplete_matches</code>, but are excluded from every rate. A match abandoned at 2-1 in the\nfirst set is a real result and a meaningless serve sample.</li>\n<li><strong>Rows with no <code>serve_points</code></strong> are excluded from rate denominators. They are never\ntreated as zero: a zeroed ace count for a 1970s match is wrong but plausible-looking,\nwhich is the worst failure mode available.</li>\n<li><strong>Stat lines that contradict themselves</strong> are dropped at ingest, keeping the match. More\nfirst serves in than points served, more won than made, more second serves won than were\nplayed, more break points saved than faced: each is arithmetically impossible, so the row\nis corrupt rather than surprising. <code>cmd/dataqual</code> counts any that predate the check, and\n<code>ingest --stage prune</code> clears them.</li>\n<li><strong>Rows repeating a match already read</strong> are collapsed rather than counted twice. The WTA\nqualifying files carry 2,700 byte-identical repeats across three seasons alone; a match is\nidentified by its draw, its number and the pair who played it, so a repeat updates the\nmatch instead of inventing a second one.</li>\n<li><strong>Team events</strong> (Davis Cup, Billie Jean King Cup) are flagged and excluded from rating\ncalculations by default.</li>\n<li><strong>Walkovers</strong> are excluded from ratings, because no tennis was played and rating one would\nmove two ratings on no evidence. A retirement is rated: it was played, and it has a\nwinner.</li>\n</ul>\n<h2 id=\"how-a-rating-is-computed\">How a rating is computed</h2>\n<p>Every rating here is computed from scratch, in chronological order, over every match in the\ndatabase. Nothing is imported from a published list and no rating is ever patched in place,\nso a bug fix is one full rerun away from correct.</p>\n<p>The engine is Elo with a decaying K-factor, so a player&#39;s first matches move their rating\nfar more than their five-hundredth:</p>\n<p><span class=\"katex\"><math xmlns=\"http://www.w3.org/1998/Math/MathML\" display=\"block\"><semantics><mrow><mi>K</mi><mo stretchy=\"false\">(</mo><mi>n</mi><mo stretchy=\"false\">)</mo><mo>=</mo><mfrac><mn>250</mn><mrow><mo stretchy=\"false\">(</mo><mi>n</mi><mo>+</mo><mn>5</mn><msup><mo stretchy=\"false\">)</mo><mn>0.4</mn></msup></mrow></mfrac></mrow><annotation encoding=\"application/x-tex\">K(n) = \\frac{250}{(n + 5)^{0.4}}</annotation></semantics></math></span></p>\n<p>where <span class=\"katex\"><math xmlns=\"http://www.w3.org/1998/Math/MathML\"><semantics><mrow><mi>n</mi></mrow><annotation encoding=\"application/x-tex\">n</annotation></semantics></math></span> is how many matches that player had completed <strong>before</strong> this one, in that series.\nA debutant gets K = 131.33 and a player at 500 matches gets K = 20.73. The specification&#39;s\ngloss of &quot;about 25&quot; is reached at roughly 311 matches rather than 500, and\n<a href=\"https://github.com/sami0076/tennis-wiki/blob/main/docs/decisions/0004-tier-taxonomy-and-elo-pool.md\">ADR-0004</a> records the correction. K is then\nmultiplied by the match-importance weight in the table below.</p>\n<p><strong>Matches are replayed in draw order, not date order.</strong> Nearly every tournament in the\nsource carries a single date for all of its matches, so ordering by date alone would rate a\nfinal before the semi-final that produced its finalist.</p>\n<p><strong>Five series are kept per player</strong> — overall, hard, clay, grass and carpet. A series is\nsnapshotted only in the weeks it moved, which is what keeps the table at 3 million rows\ninstead of 1.9 billion. For display and for simulation, the surface series and the overall\none are blended:</p>\n<p><span class=\"katex\"><math xmlns=\"http://www.w3.org/1998/Math/MathML\" display=\"block\"><semantics><mrow><mtext>blended</mtext><mo>=</mo><mi>w</mi><mo>⋅</mo><mtext>surface</mtext><mo>+</mo><mo stretchy=\"false\">(</mo><mn>1</mn><mo>−</mo><mi>w</mi><mo stretchy=\"false\">)</mo><mo>⋅</mo><mtext>overall</mtext><mo separator=\"true\">,</mo><mspace width=\"2em\"/><mi>w</mi><mo>=</mo><mi>min</mi><mo>⁡</mo><mrow><mo fence=\"true\">(</mo><mn>0.75</mn><mo separator=\"true\">,</mo><mfrac><mtext>surface matches</mtext><mn>40</mn></mfrac><mo fence=\"true\">)</mo></mrow></mrow><annotation encoding=\"application/x-tex\">\\text{blended} = w \\cdot \\text{surface} + (1 - w) \\cdot \\text{overall}, \\qquad w = \\min\\left(0.75, \\frac{\\text{surface matches}}{40}\\right)</annotation></semantics></math></span></p>\n<p>so a player with five clay matches leans on their overall rating and a clay specialist with\na hundred leans on their clay one. The weight is reported next to the figure wherever the\nfigure appears, because 1900 from a hundred clay matches and 1900 from three are the same\nnumber and different claims. A surface somebody never played is absent rather than blended\nat the base rating.</p>\n<h2 id=\"the-tier-weights-and-the-evidence-for-them\">The tier weights, and the evidence for them</h2>\n<p>A result at Futures level counts for less than a Grand Slam final. How much less is a\njudgement, and <a href=\"https://github.com/sami0076/tennis-wiki/blob/main/docs/decisions/0004-tier-taxonomy-and-elo-pool.md\">ADR-0004</a> picked numbers\nbefore there was any data to check them against. <code>cmd/validate</code> is what checks them.</p>\n<table>\n<thead>\n<tr>\n<th>Level</th>\n<th>Weight</th>\n</tr>\n</thead>\n<tbody><tr>\n<td>Grand Slam final</td>\n<td>1.20</td>\n</tr>\n<tr>\n<td>Grand Slam, other rounds</td>\n<td>1.10</td>\n</tr>\n<tr>\n<td>Tour Finals</td>\n<td>1.10</td>\n</tr>\n<tr>\n<td>Masters 1000 / WTA 1000</td>\n<td>1.05</td>\n</tr>\n<tr>\n<td>Other tour-level</td>\n<td>1.00</td>\n</tr>\n<tr>\n<td>Davis Cup and other team events</td>\n<td>0.80 (excluded from ratings by default)</td>\n</tr>\n<tr>\n<td>Challenger</td>\n<td>0.80</td>\n</tr>\n<tr>\n<td>Futures / ITF</td>\n<td>0.60</td>\n</tr>\n<tr>\n<td>Qualifying</td>\n<td>multiply by 0.90</td>\n</tr>\n</tbody></table>\n<p><strong>What the evidence says.</strong> Tour-level predictive accuracy is 69.9%, inside the 68-72% the\nspecification asks for, and calibration is within 1.6 percentage points at every tier. On\nthose two measures the weights are fine.</p>\n<p>The third measure is less comfortable. A player&#39;s rating should carry across a promotion\nfrom Challenger to tour without a step in it, and it does not: across 7,054 promotions,\nplayers won 33,152 of their first tour-level matches against 31,342 expected. Promoted\nplayers arrive underrated.</p>\n<p><strong>Raising the lower tiers is not the fix, and the numbers say so.</strong> Moving Challenger to\n0.90 and Futures to 0.75 removes about a fifth of that surplus and makes Challenger\ncalibration worse; going further to 1.00 and 0.90 removes a little more and costs a little\nmore. The lever does not fit the problem.</p>\n<p>The likeliest explanation is that promotion selects for players who are improving, whose\nrating is an average over a period when they were genuinely worse. No fixed weight corrects\nfor someone being better than their own record. That is a hypothesis rather than a finding,\nand it is stated here rather than quietly assumed, because the alternative is presenting a\nnumber as settled when the check that would settle it says otherwise.</p>\n<p>So the weights stay as ADR-0004 set them, and the reason is written down: the available\nchange trades a measurable calibration cost for a partial fix to something the weights do\nnot control. <code>make validate</code> reruns all of this, and <code>--weights</code> tries other numbers without\na rebuild.</p>\n<h2 id=\"what-the-simulator-can-be-checked-for-and-what-it-cannot\">What the simulator can be checked for, and what it cannot</h2>\n<p>The point-win probabilities are derived by solving for the pair whose match probability\nequals what the rating already predicts (<a href=\"https://github.com/sami0076/tennis-wiki/blob/main/docs/decisions/0007-elo-derived-point-probability.md\">ADR-0007</a>).\nSo the chain&#39;s match-level answer <strong>is</strong> the rating&#39;s, by construction, and checking it\nagainst results would be checking the rating engine — which the accuracy and calibration\nsections above already do. Reporting the agreement as a finding would be reporting an\nidentity.</p>\n<p>What the chain adds is everything below match level, and that is what <code>make validate</code>\nmeasures.</p>\n<h3 id=\"the-chain-expects-too-many-deciding-sets\">The chain expects too many deciding sets</h3>\n<p>Over 30,000 recent ATP matches, rated as of the day each was played:</p>\n<table>\n<thead>\n<tr>\n<th>Predicted chance of a deciding set</th>\n<th>Matches</th>\n<th>Expected</th>\n<th>Observed</th>\n<th>Gap</th>\n</tr>\n</thead>\n<tbody><tr>\n<td>0-20%</td>\n<td>317</td>\n<td>15.4%</td>\n<td>11.4%</td>\n<td>−4.1</td>\n</tr>\n<tr>\n<td>20-40%</td>\n<td>5,981</td>\n<td>33.7%</td>\n<td>26.6%</td>\n<td>−7.2</td>\n</tr>\n<tr>\n<td>40-60%</td>\n<td>23,702</td>\n<td>47.3%</td>\n<td>36.4%</td>\n<td>−11.0</td>\n</tr>\n<tr>\n<td><strong>All</strong></td>\n<td><strong>30,000</strong></td>\n<td><strong>44.3%</strong></td>\n<td><strong>34.1%</strong></td>\n<td><strong>−10.1</strong></td>\n</tr>\n</tbody></table>\n<p>The model expects nearly half of matches to go the distance and about a third do. The gap\nwidens exactly where the model is least certain, which points at the assumption that\nproduces it: <strong>sets are treated as independent</strong>, and they are not.</p>\n<p>Independence is what makes 2-1 and 3-2 the most likely scorelines between close players.\nReal sets are positively correlated — whoever wins the first is more likely to win the\nsecond, because the things a single set probability averages over do not resample between\nsets — so real matches finish in straight sets more often than independence allows. The\ndirection of the error is the direction that assumption predicts, and its size says the\ncorrelation is not small.</p>\n<p>This is stated rather than corrected because correcting it properly means a model of\nbetween-set correlation, which is a larger piece of work than the one that found the\nproblem. What it does mean today: the chain&#39;s set and match rungs are sound as an ordering\nand its deciding-set implication is not a number to quote. The scorelines under the chain\nand the single match the simulator plays out sample under the same assumption, so a\n2-1 or 3-2 there is likelier than it would be on court, which is why both are captioned as\nthe model&#39;s and neither as a prediction.</p>\n<p>There are no rows above 60% because there cannot be. The chance of a deciding set peaks at\n50% for a best of three and 37.5% for a best of five, both at evenly matched players, so\nthe top buckets are empty by arithmetic rather than for want of data.</p>\n<h3 id=\"the-draw-simulation-beats-knowing-nothing-and-not-by-as-much-as-it-looks\">The draw simulation beats knowing nothing, and not by as much as it looks</h3>\n<p>Over 296 reconstructed ATP draws, each replayed 2,000 times with the ratings as of the week\nit began:</p>\n<table>\n<thead>\n<tr>\n<th></th>\n<th></th>\n</tr>\n</thead>\n<tbody><tr>\n<td>Brier score</td>\n<td><strong>0.8550</strong></td>\n</tr>\n<tr>\n<td>Brier for a model that knows only the field size</td>\n<td>0.9696</td>\n</tr>\n<tr>\n<td>Average probability given to the eventual champion</td>\n<td>14.4%</td>\n</tr>\n<tr>\n<td>How often its favourite actually won</td>\n<td>27.4%</td>\n</tr>\n</tbody></table>\n<p>Better than uninformed, which is the least a rating-driven simulation should manage. The\nabsolute numbers are modest because most of these draws are 32 and 64-player events where\nthe favourite genuinely wins about a quarter of the time — a tennis draw is not a\npredictable object, and a model claiming otherwise would be the suspicious one.</p>\n<p>This check exists at all because the draw simulator replays events that were <strong>played</strong>. A\nforward-looking one could never be scored, which is the argument for the decision as much as\nthe data coverage was.</p>\n<h2 id=\"tiers\">Tiers</h2>\n<p><code>tier</code> is a competitive standard, deliberately distinct from <code>tournaments.level</code>, which\nrecords event prestige. A Grand Slam qualifying draw and a Futures qualifying draw are both\n&quot;qualifying&quot; and nothing alike. Qualifying is a boolean beside the tier, not a tier of its\nown, because a Challenger qualifier is still Challenger standard. The reasoning, and the\nElo pool design that depends on it, is in\n<a href=\"https://github.com/sami0076/tennis-wiki/blob/main/docs/decisions/0004-tier-taxonomy-and-elo-pool.md\">ADR-0004</a>.</p>\n"
