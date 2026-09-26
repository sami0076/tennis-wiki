import { request } from './client'
import type {
  Clutch,
  CommonOpponents,
  CoverageResponse,
  DrawSimulation,
  Edition,
  Event,
  EventSummary,
  HeadToHead,
  Leaderboard,
  MatchSimulation,
  Page,
  Percentiles,
  PlayerMatch,
  PlayerHighlights,
  PlayerProfile,
  PlayerSearchResult,
  PlayerSeasons,
  RankingHistory,
  ReplayableDraws,
  RankingPage,
  RatingSeries,
  RecentFinals,
  SeasonEventsResponse,
  SeasonsResponse,
  ThisWeek,
  Trajectories,
} from './types.gen'

/**
 * The endpoints the API actually serves today. There is deliberately nothing
 * here that pretends to serve one it does not.
 */

export function getCoverage(signal?: AbortSignal): Promise<CoverageResponse> {
  return request<CoverageResponse>('/coverage', {}, signal)
}

export function getPlayer(slug: string, signal?: AbortSignal): Promise<PlayerProfile> {
  return request<PlayerProfile>(`/players/${encodeURIComponent(slug)}`, {}, signal)
}

export interface MatchFilters {
  surface?: string | null
  tier?: string | null
  season?: number | null
  opponent?: string | null
  limit?: number
  cursor?: string | null
}

export function getPlayerMatches(
  slug: string,
  filters: MatchFilters = {},
  signal?: AbortSignal,
): Promise<Page<PlayerMatch>> {
  return request<Page<PlayerMatch>>(
    `/players/${encodeURIComponent(slug)}/matches`,
    { ...filters },
    signal,
  )
}

export function getPlayerRatingSeries(
  slug: string,
  options: { surface?: string; from?: string; to?: string } = {},
  signal?: AbortSignal,
): Promise<RatingSeries> {
  return request<RatingSeries>(
    `/players/${encodeURIComponent(slug)}/ratings`,
    { ...options },
    signal,
  )
}

/**
 * The three under-pressure figures and the baseline they are measured against.
 * Its own request rather than a block on the profile: the profile is one player
 * and this is one player against every match at their level.
 */
export function getPlayerClutch(slug: string, signal?: AbortSignal): Promise<Clutch> {
  return request<Clutch>(`/players/${encodeURIComponent(slug)}/clutch`, {}, signal)
}

/**
 * The career read as what happened rather than as rates: the runs, the wins
 * that cost the most, what was won and who kept turning up.
 *
 * Its own request rather than a block on the profile, for the same reason
 * clutch is: the profile is one player's rows and this walks their whole match
 * list and every opponent's rating history. A page that shows a name should
 * not wait on that.
 */
export function getPlayerHighlights(
  slug: string,
  signal?: AbortSignal,
): Promise<PlayerHighlights> {
  return request<PlayerHighlights>(
    `/players/${encodeURIComponent(slug)}/highlights`,
    {},
    signal,
  )
}

/** A career a year at a time, every rate over its own count of matches. */
export function getPlayerSeasons(slug: string, signal?: AbortSignal): Promise<PlayerSeasons> {
  return request<PlayerSeasons>(`/players/${encodeURIComponent(slug)}/seasons`, {}, signal)
}

/** A player's last tour-level year, each axis a percentile of their tour. */
export function getPlayerPercentiles(slug: string, signal?: AbortSignal): Promise<Percentiles> {
  return request<Percentiles>(`/players/${encodeURIComponent(slug)}/percentiles`, {}, signal)
}

/** Events in progress, from results so far. Provisional and hourly, not live. */
export function getThisWeek(signal?: AbortSignal): Promise<ThisWeek> {
  return request<ThisWeek>('/this-week', {}, signal)
}

export function getPlayerRankings(slug: string, signal?: AbortSignal): Promise<RankingHistory> {
  return request<RankingHistory>(`/players/${encodeURIComponent(slug)}/rankings`, {}, signal)
}

export interface MeetingFilters {
  level?: string | null
  round?: string | null
  best_of?: string | null
  surface?: string | null
  deciders?: string | null
  tiebreaks?: string | null
  from?: string | null
  to?: string | null
}

/**
 * The comparison, in the order the URL asks for. /h2h/a/b and /h2h/b/a are the
 * same rivalry read from opposite ends, so the caller decides which player is
 * on the left and nothing downstream has to. The filters cut the record, the
 * strip, the serve figures and the meetings; the closeness summary is never cut.
 */
export function getHeadToHead(
  a: string,
  b: string,
  filters: MeetingFilters = {},
  signal?: AbortSignal,
): Promise<HeadToHead> {
  return request<HeadToHead>(
    `/h2h/${encodeURIComponent(a)}/${encodeURIComponent(b)}`,
    { ...filters },
    signal,
  )
}

/**
 * Every opponent both players have faced, with each side's record against
 * them. Separate from the head to head itself because it answers a different
 * question and is the more expensive half: two full careers grouped and joined
 * rather than one rivalry's few dozen rows.
 */
export function getCommonOpponents(
  a: string,
  b: string,
  options: { limit?: number } = {},
  signal?: AbortSignal,
): Promise<CommonOpponents> {
  return request<CommonOpponents>(
    `/h2h/${encodeURIComponent(a)}/${encodeURIComponent(b)}/common`,
    { ...options },
    signal,
  )
}

/**
 * The draws the simulator can be pointed at. Its own request rather than a
 * block on the simulation: the page shows one draw and this is the list of
 * every other one, which nothing needs until somebody wants to change it.
 */
export function getReplayableDraws(
  filters: { tour?: string | null; season?: number | null; limit?: number } = {},
  signal?: AbortSignal,
): Promise<ReplayableDraws> {
  return request<ReplayableDraws>('/simulate/draws', { ...filters }, signal)
}

export interface RankingFilters {
  type?: string | null
  tour?: string | null
  surface?: string | null
  date?: string | null
  limit?: number
  cursor?: string | null
}

/**
 * A leaderboard, as of the last week that exists rather than as of today. The
 * response names the week it used, which is why nothing here defaults the date:
 * a caller that guessed one would be guessing at coverage.
 */
export function getRankings(
  filters: RankingFilters = {},
  signal?: AbortSignal,
): Promise<RankingPage> {
  return request<RankingPage>('/rankings', { ...filters }, signal)
}

/**
 * The leaders' rating lines over a window. A separate endpoint from the
 * leaderboard because it answers with lines rather than rows, and squeezing
 * both through one shape would make each of them worse.
 */
export function getTrajectories(
  options: {
    surface?: string | null
    tour?: string | null
    players?: number | null
    months?: number | null
  } = {},
  signal?: AbortSignal,
): Promise<Trajectories> {
  return request<Trajectories>('/rankings/trajectory', { ...options }, signal)
}

/** Every rung between a point and a match, for one hypothetical pair. */
export function simulateMatch(
  a: string,
  b: string,
  options: { surface?: string | null; best_of?: number | null } = {},
  signal?: AbortSignal,
): Promise<MatchSimulation> {
  return request<MatchSimulation>('/simulate/match', { a, b, ...options }, signal)
}

/**
 * A draw that was played, replayed. There is no upcoming tournament in the
 * database and there will not be one, so the event is always a historical one
 * and the answer can be read against what actually happened. An edition is
 * addressed the way its sheet is: the event's slug and the season.
 */
export function simulateDraw(
  event: { event: string; season: number; runs?: number | null },
  signal?: AbortSignal,
): Promise<DrawSimulation> {
  return request<DrawSimulation>('/simulate/draw', { ...event }, signal)
}

export interface EventFilters {
  tour?: string | null
  level?: string | null
  q?: string | null
  limit?: number
  cursor?: string | null
}

/** The tournament index: every event on both tours, grouped by its level. */
export function getEvents(filters: EventFilters = {}, signal?: AbortSignal): Promise<Page<EventSummary>> {
  return request<Page<EventSummary>>('/tournaments', { ...filters }, signal)
}

/** One event across seasons, on ADR-0012's terms, with every edition's final. */
export function getEvent(slug: string, signal?: AbortSignal): Promise<Event> {
  return request<Event>(`/tournaments/${encodeURIComponent(slug)}`, {}, signal)
}

/** One season of an event as its draw sheet: every match in bracket order. */
export function getEdition(slug: string, season: number, signal?: AbortSignal): Promise<Edition> {
  return request<Edition>(`/tournaments/${encodeURIComponent(slug)}/${season}`, {}, signal)
}

/** The finals of the last complete week, both tours, with the data's edge. */
export function getRecentFinals(signal?: AbortSignal): Promise<RecentFinals> {
  return request<RecentFinals>('/recent', {}, signal)
}

/** The calendar: a row per year with both tours and the Slam finals. */
export function getSeasons(signal?: AbortSignal): Promise<SeasonsResponse> {
  return request<SeasonsResponse>('/seasons', {}, signal)
}

/** Every event of one year at one tier, with its final. */
export function getSeasonEvents(
  year: number,
  options: { tour?: string | null; tier?: string | null } = {},
  signal?: AbortSignal,
): Promise<SeasonEventsResponse> {
  return request<SeasonEventsResponse>(`/seasons/${year}`, { ...options }, signal)
}

export interface LeaderFilterParams {
  tour?: string | null
  tier?: string | null
  surface?: string | null
  season?: number | null
  min_matches?: number | null
  limit?: number | null
  /** A player to report on whether or not they are on the board. */
  player?: string | null
}

/**
 * One board: a stat, filtered, ranked, with its population declared and
 * every row's denominator on the row.
 */
export function getLeaders(stat: string, filters: LeaderFilterParams = {}, signal?: AbortSignal): Promise<Leaderboard> {
  return request<Leaderboard>(`/leaders/${encodeURIComponent(stat)}`, { ...filters }, signal)
}

export function searchPlayers(
  q: string,
  options: { tour?: string | null; limit?: number; cursor?: string | null } = {},
  signal?: AbortSignal,
): Promise<Page<PlayerSearchResult>> {
  return request<Page<PlayerSearchResult>>('/players', { q, ...options }, signal)
}
