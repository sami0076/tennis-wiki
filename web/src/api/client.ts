import type { Problem } from './types.gen'

export * from './types.gen'

/**
 * ApiError is a failed request. The API answers every error with an RFC 7807
 * problem document, so there is exactly one error shape to handle, and `detail`
 * is written to say what the caller can change.
 */
export class ApiError extends Error {
  readonly status: number
  readonly problem: Problem | null

  constructor(status: number, problem: Problem | null, fallback: string) {
    super(problem?.detail || problem?.title || fallback)
    this.name = 'ApiError'
    this.status = status
    this.problem = problem
  }
}

const base = import.meta.env.VITE_API_BASE ?? '/api/v1'

/**
 * request fetches one endpoint and returns its decoded body.
 *
 * Query values that are null or undefined are dropped rather than sent as the
 * string "null", which the API would reject as an unknown filter value.
 */
export async function request<T>(
  path: string,
  params: Record<string, string | number | null | undefined> = {},
  signal?: AbortSignal,
): Promise<T> {
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== null && value !== undefined && value !== '') {
      query.set(key, String(value))
    }
  }
  const suffix = query.toString() ? `?${query}` : ''

  const response = await fetch(`${base}${path}${suffix}`, {
    headers: { Accept: 'application/json' },
    signal,
  })

  if (!response.ok) {
    let problem: Problem | null = null
    try {
      problem = (await response.json()) as Problem
    } catch {
      // A proxy or a crash can answer with something that is not a problem
      // document. The status is still worth reporting accurately.
    }
    throw new ApiError(response.status, problem, `Request failed with ${response.status}.`)
  }

  return (await response.json()) as T
}
