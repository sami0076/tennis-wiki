import { request } from './client'
import type { CoverageResponse, Page, PlayerMatch, PlayerProfile, PlayerSearchResult } from './types.gen'

/**
 * The endpoints the API actually serves today. Ratings and head-to-head arrive
 * with #45 and #44; there is deliberately nothing here that pretends to.
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

export function searchPlayers(
  q: string,
  options: { tour?: string | null; limit?: number; cursor?: string | null } = {},
  signal?: AbortSignal,
): Promise<Page<PlayerSearchResult>> {
  return request<Page<PlayerSearchResult>>('/players', { q, ...options }, signal)
}
