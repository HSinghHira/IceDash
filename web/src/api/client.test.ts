import { describe, expect, it } from 'vitest'
import { ApiError, request } from './client'

const storage = (value: string | null) => ({ getItem: () => value })

function fakeFetch(status: number, body: unknown, seen: { init?: RequestInit } = {}) {
  return (async (_path: string, init?: RequestInit) => {
    seen.init = init
    return new Response(body === undefined ? '' : JSON.stringify(body), { status })
  }) as unknown as typeof fetch
}

describe('request', () => {
  it('sends the ZimaOS token as a bearer token', async () => {
    const seen: { init?: RequestInit } = {}
    const data = await request<{ ok: boolean }>('/v2/icedash/system', {
      fetch: fakeFetch(200, { ok: true }, seen),
      storage: storage('abc.def.ghi'),
    })
    expect(data.ok).toBe(true)
    expect((seen.init?.headers as Record<string, string>).Authorization).toBe('Bearer abc.def.ghi')
  })

  it('sends no Authorization header without a token', async () => {
    const seen: { init?: RequestInit } = {}
    await request('/v2/icedash/health', { fetch: fakeFetch(200, {}, seen), storage: storage(null) })
    expect((seen.init?.headers as Record<string, string>).Authorization).toBeUndefined()
  })

  it('turns an error answer into an ApiError with its code', async () => {
    const err = await request('/v2/icedash/update/apply', {
      method: 'POST',
      fetch: fakeFetch(403, { code: 'admin_only', error: 'only an admin can do this' }),
      storage: storage('t'),
    }).catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).status).toBe(403)
    expect((err as ApiError).code).toBe('admin_only')
  })

  it('reports a network failure as unreachable', async () => {
    const failing = (async () => {
      throw new TypeError('Failed to fetch')
    }) as unknown as typeof fetch
    const err = await request('/v2/icedash/health', { fetch: failing, storage: storage(null) }).catch((e: unknown) => e)
    expect((err as ApiError).code).toBe('unreachable')
  })
})
