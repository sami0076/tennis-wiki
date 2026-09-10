// Generates web/src/generated/methodology.ts from docs/methodology.md and the
// committed output of `make validate` in docs/validation.json.
//
// Why generate rather than write the page: two copies of the methodology would
// drift, and the copy on the live site is the one that would be wrong. Why
// check the figures: prose quotes numbers, and a number that no longer matches
// the run it came from is the failure this whole project is about. The build
// fails instead.
//
// marked and katex are build-time only. Nothing here ships to the browser:
// KaTeX emits MathML, which browsers render themselves, so the page carries no
// font files and no runtime library.
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, join, posix, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import katex from 'katex'
import { marked } from 'marked'

const here = dirname(fileURLToPath(import.meta.url))
const repo = resolve(here, '..', '..')
const blob = 'https://github.com/sami0076/tennis-wiki/blob/main'
const sourcePath = 'docs/methodology.md'
const reportPath = 'docs/validation.json'
const outPath = 'web/src/generated/methodology.ts'

const markdown = readFileSync(join(repo, sourcePath), 'utf8')
const report = JSON.parse(readFileSync(join(repo, reportPath), 'utf8'))

const checks = figures(report)
verify(markdown, checks)

const { html, headings } = render(markdown)
mkdirSync(dirname(join(repo, outPath)), { recursive: true })
writeFileSync(join(repo, outPath), emit(html, headings, checks), 'utf8')
console.log(`${outPath}: ${headings.length} headings, ${html.length} characters`)

/** The figures the prose quotes, taken from the run rather than from the prose. */
function figures(run) {
  const tour = run.accuracy.find((row) => row.tier === 'tour')
  const sets = run.simulation.deciding_sets
  const sampled = sets.reduce((total, bucket) => total + bucket.matches, 0)
  const weighted = (key) =>
    sets.reduce((total, bucket) => total + bucket[key] * bucket.matches, 0) / sampled

  return {
    generatedAt: run.generated_at,
    matchesReplayed: run.matches_replayed,
    tourAccuracy: tour.accuracy,
    worstCalibration: Math.max(...run.calibration.map((tier) => tier.mean_error)),
    promotions: run.promotion_continuity.promotions,
    promotionMatches: run.promotion_continuity.matches,
    promotionExpected: run.promotion_continuity.expected_wins,
    promotionActual: run.promotion_continuity.actual_wins,
    decidingSampled: sampled,
    decidingExpected: weighted('expected') * 100,
    decidingObserved: weighted('observed') * 100,
    events: run.simulation.draws.events,
    brier: run.simulation.draws.brier,
    brierUniform: run.simulation.draws.brier_uniform,
    topPick: run.simulation.draws.top_pick * 100,
    championOdds: run.simulation.draws.mean_champion_odds * 100,
  }
}

/**
 * Fails the build when the prose and the run disagree. The message names both
 * numbers, because the fix is always to look at which one is stale.
 */
function verify(text, run) {
  const count = (n) => n.toLocaleString('en-GB')
  const expected = [
    ['tour-level accuracy', `${run.tourAccuracy.toFixed(1)}%`],
    ['calibration', `within ${run.worstCalibration.toFixed(1)} percentage points`],
    ['promotions', count(run.promotions)],
    ['wins after promotion', count(run.promotionActual)],
    ['wins expected after promotion', count(Math.round(run.promotionExpected))],
    ['matches sampled for deciding sets', count(run.decidingSampled)],
    ['deciding sets expected', `${run.decidingExpected.toFixed(1)}%`],
    ['deciding sets observed', `${run.decidingObserved.toFixed(1)}%`],
    ['draws replayed', count(run.events)],
    ['Brier score', run.brier.toFixed(4)],
    ['Brier for the uninformed model', run.brierUniform.toFixed(4)],
    ['favourite win rate', `${run.topPick.toFixed(1)}%`],
    ['mean champion odds', `${run.championOdds.toFixed(1)}%`],
  ]

  const missing = expected.filter(([, value]) => !text.includes(value))
  if (missing.length > 0) {
    const lines = missing.map(([label, value]) => `  ${label}: ${reportPath} says ${value}`)
    throw new Error(
      `${sourcePath} does not quote the run in ${reportPath}:\n${lines.join('\n')}\n` +
        `Rerun \`make validation-json\` or correct the prose, whichever is stale.`,
    )
  }
}

function render(text) {
  const math = []
  // Pulled out before the markdown parser sees them: $$ and $ mean nothing to
  // it, and \frac would be mangled as an escape. GitHub renders the same TeX,
  // so the document stays readable where it lives.
  let masked = text.replace(/\$\$([^$]+)\$\$/g, (_, tex) => placeholder(math, tex, true))
  masked = masked.replace(/(^|[^$])\$([^$\n]+)\$/g, (_, before, tex) =>
    `${before}${placeholder(math, tex, false)}`,
  )

  let html = marked.parse(masked, { async: false, gfm: true })

  const headings = []
  html = html.replace(/<h([1-6])>([\s\S]*?)<\/h\1>/g, (_, level, inner) => {
    const plain = inner.replace(/<[^>]+>/g, '').trim()
    const id = slug(plain)
    headings.push({ id, text: plain, level: Number(level) })
    return `<h${level} id="${id}">${inner}</h${level}>`
  })

  html = html.replace(/href="([^"]+)"/g, (_, href) => `href="${link(href)}"`)
  html = html.replace(/MATHTOKEN(\d+)ENDTOKEN/g, (_, index) => math[Number(index)])

  return { html, headings }
}

function placeholder(math, tex, display) {
  const index = math.length
  math.push(
    katex.renderToString(tex.trim(), {
      displayMode: display,
      output: 'mathml',
      throwOnError: true,
    }),
  )
  return `MATHTOKEN${index}ENDTOKEN`
}

/**
 * Links in the document are relative to docs/. On the site they have to point
 * at the repository, which is where the ADRs and the configuration live.
 */
function link(href) {
  if (/^(https?:|#|mailto:)/.test(href)) return href
  // posix, never the platform's: resolve() on Windows would answer C:/docs/...
  const path = posix.resolve('/docs', href).replace(/^\//, '')
  return `${blob}/${path}`
}

function slug(text) {
  return text
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')
}

function emit(html, headings, run) {
  return `// Generated from ${sourcePath} and ${reportPath} by scripts/methodology.mjs.
// Do not edit: run \`npm run methodology\` instead. CI regenerates this file and
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

export const source = ${JSON.stringify(sourcePath)}
export const checks: MethodologyChecks = ${JSON.stringify(run, null, 2)}
export const headings: ReadonlyArray<MethodologyHeading> = ${JSON.stringify(headings, null, 2)}
export const html = ${JSON.stringify(html)}
`
}
