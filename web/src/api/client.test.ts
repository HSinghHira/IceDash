import { describe, expect, it } from 'vitest'
import { ApiError, request } from './client'

function storage(init: Record<string, string> = {}) {
  const m = new Map(Object.entries(init))
  return { getItem: (k: string) => m.get(k) ?? null, setItem: (k: string, v: string) => void m.set(k, v), m }
}

type Seen = { calls: { path: string; init?: RequestInit }[] }

/** A fetch that answers each path from a queue of [status, body] replies. */
function fakeFetch(routes: Record<string, [number, unknown][]>, seen: Seen = { calls: [] }) {
  return (async (path: string, init?: RequestInit) => {
    seen.calls.push({ path, init })
    const reply = routes[path]?.shift() ?? [404, { code: 'not_found' }]
    return new Response(reply[1] === undefined ? '' : JSON.stringify(reply[1]), { status: reply[0] })
  }) as unknown as typeof fetch
}

const auth = (s: Seen, i: number) => (s.calls[i]?.init?.headers as Record<string, string>)?.Authorization

describe('request', () => {
  it('sends the ZimaOS token as a bearer token', async () => {
    const seen: Seen = { calls: [] }
    const data = await request<{ ok: boolean }>('/v2/icedash/system', {
      fetch: fakeFetch({ '/v2/icedash/system': [[200, { ok: true }]] }, seen),
      storage: storage({ access_token: 'abc.def.ghi' }),
    })
    expect(data.ok).toBe(true)
    expect(auth(seen, 0)).toBe('Bearer abc.def.ghi')
  })

  it('sends no Authorization header without a token', async () => {
    const seen: Seen = { calls: [] }
    await request('/v2/icedash/health', { fetch: fakeFetch({ '/v2/icedash/health': [[200, {}]] }, seen), storage: storage() })
    expect(auth(seen, 0)).toBeUndefined()
  })

  it('turns an error answer into an ApiError with its code', async () => {
    const err = await request('/v2/icedash/update/apply', {
      method: 'POST',
      fetch: fakeFetch({ '/v2/icedash/update/apply': [[403, { code: 'admin_only', error: 'only an admin can do this' }]] }),
      storage: storage({ access_token: 't' }),
    }).catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).status).toBe(403)
    expect((err as ApiError).code).toBe('admin_only')
  })

  it('reports a network failure, or a gateway with nothing behind it, as unreachable', async () => {
    const failing = (async () => {
      throw new TypeError('Failed to fetch')
    }) as unknown as typeof fetch
    const a = await request('/v2/icedash/health', { fetch: failing, storage: storage() }).catch((e: unknown) => e)
    expect((a as ApiError).code).toBe('unreachable')
    const b = await request('/v2/icedash/health', {
      fetch: fakeFetch({ '/v2/icedash/health': [[502, undefined]] }),
      storage: storage(),
    }).catch((e: unknown) => e)
    expect((b as ApiError).code).toBe('unreachable')
  })

  it('refreshes an expired session once and retries', async () => {
    const seen: Seen = { calls: [] }
    const store = storage({ access_token: 'old', refresh_token: 'r1' })
    const data = await request<{ ok: boolean }>('/v2/icedash/system', {
      fetch: fakeFetch(
        {
          '/v2/icedash/system': [
            [401, { code: 'auth_invalid' }],
            [200, { ok: true }],
          ],
          '/v1/users/refresh': [[200, { success: 200, data: { access_token: 'new', refresh_token: 'r2', expires_at: 1 } }]],
        },
        seen,
      ),
      storage: store,
    })
    expect(data.ok).toBe(true)
    expect(seen.calls.map((c) => c.path)).toEqual(['/v2/icedash/system', '/v1/users/refresh', '/v2/icedash/system'])
    expect(auth(seen, 2)).toBe('Bearer new')
    expect(store.m.get('refresh_token')).toBe('r2')
  })

  it('gives up when the refresh fails', async () => {
    const err = await request('/v2/icedash/system', {
      fetch: fakeFetch({
        '/v2/icedash/system': [[401, { code: 'auth_invalid' }]],
        '/v1/users/refresh': [[401, { success: 20006, message: 'Verification failure' }]],
      }),
      storage: storage({ access_token: 'old', refresh_token: 'bad' }),
    }).catch((e: unknown) => e)
    expect((err as ApiError).status).toBe(401)
    expect((err as ApiError).code).toBe('auth_invalid')
  })
})
