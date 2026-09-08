import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, request } from './client'

function respond(body: unknown, init: ResponseInit = {}) {
  return Promise.resolve(
    new Response(JSON.stringify(body), {
      status: init.status ?? 200,
      headers: { 'Content-Type': 'application/json' },
    }),
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('request', () => {
  it('drops empty filters rather than sending the string "null"', async () => {
    // Typed with fetch's own parameters so the recorded call is a tuple the
    // assertion below can index into.
    const fetchMock = vi.fn((_input: RequestInfo | URL, _init?: RequestInit) =>
      respond({ data: [] }),
    )
    vi.stubGlobal('fetch', fetchMock)

    await request('/players/x/matches', { surface: null, tier: undefined, season: 2019, opponent: '' })

    const url = String(fetchMock.mock.calls[0]?.[0])
    expect(url).toContain('season=2019')
    expect(url).not.toContain('surface')
    expect(url).not.toContain('tier')
    expect(url).not.toContain('opponent')
  })

  // Every error the API returns is a problem document, and its detail is
  // written to say what the caller can change.
  it('turns a problem document into an ApiError carrying its detail', async () => {
    vi.stubGlobal('fetch', () =>
      respond(
        {
          type: '/problems/bad-request',
          title: 'Invalid request',
          status: 400,
          detail: 'surface must be hard, clay, grass or carpet',
        },
        { status: 400 },
      ),
    )

    await expect(request('/players/x/matches')).rejects.toMatchObject({
      name: 'ApiError',
      status: 400,
      message: 'surface must be hard, clay, grass or carpet',
    })
  })

  it('still reports the status when the body is not a problem document', async () => {
    vi.stubGlobal('fetch', () =>
      Promise.resolve(new Response('<html>502</html>', { status: 502 })),
    )

    const error = await request('/coverage').catch((e: unknown) => e)
    expect(error).toBeInstanceOf(ApiError)
    expect((error as ApiError).status).toBe(502)
  })
})
