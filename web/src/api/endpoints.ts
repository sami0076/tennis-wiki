import { request } from './client'
import type {
  Clutch,
  CoverageResponse,
  DrawSimulation,
  HeadToHead,
  MatchSimulation,
  Page,
  PlayerMatch,
  PlayerProfile,
  PlayerSearchResult,
  RankingHistory,
  RankingPage,
  RatingSeries,
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

export function getPlayerRankings(slug: string, signal?: AbortSignal): Promise<RankingHistory> {
  return request<RankingHistory>(`/players/${encodeURIComponent(slug)}/rankings`, {}, signal)
}

/**
 * The comparison, in the order the URL asks for. /h2h/a/b and /h2h/b/a are the
 * same rivalry read from opposite ends, so the caller decides which player is
 * on the left and nothing downstream has to.
 */
export function getHeadToHead(a: string, b: string, signal?: AbortSignal): Promise<HeadToHead> {
  return request<HeadToHead>(
    `/h2h/${encodeURIComponent(a)}/${encodeURIComponent(b)}`,
    {},
    signal,
  )
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
 * and the answer can be read against what actually happened.
 */
export function simulateDraw(
  event: { tour: string; season: number; event: string; runs?: number | null },
  signal?: AbortSignal,
): Promise<DrawSimulation> {
  return request<DrawSimulation>('/simulate/draw', { ...event }, signal)
}

export function searchPlayers(
  q: string,
  options: { tour?: string | null; limit?: number; cursor?: string | null } = {},
  signal?: AbortSignal,
): Promise<Page<PlayerSearchResult>> {
  return request<Page<PlayerSearchResult>>('/players', { q, ...options }, signal)
}
