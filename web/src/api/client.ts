// Calls to IceDash's backend and to ZimaOS's own APIs. Both live on the same
// origin, so the ZimaOS sign-in token in localStorage works for both: users
// never sign in twice (PLAN.md, "Same origin, free sign-in").
//
// A 401 goes through POST /v1/users/refresh once, as ZimaOS's own UI and
// lintux-modkit's web/common.js do (measured on 1.8.0, HOST-NOTES item 6).

import { loginUrl } from '../lib/host'

export const TOKEN_KEY = 'access_token'
export const REFRESH_KEY = 'refresh_token'

/** An API failure, with the backend's stable code so the UI can pick a message. */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message)
  }
}

type Store = Pick<Storage, 'getItem' | 'setItem'>

function read(storage: Store | undefined, key: string): string {
  try {
    return (storage ?? localStorage).getItem(key) ?? ''
  } catch {
    return ''
  }
}

function write(storage: Store | undefined, key: string, value: string) {
  try {
    ;(storage ?? localStorage).setItem(key, value)
  } catch {
    // a private window: the session lasts until the token expires
  }
}

export function token(storage?: Store): string {
  return read(storage, TOKEN_KEY)
}

export interface RequestOptions {
  method?: 'GET' | 'POST'
  body?: unknown
  /** Sent as it is (a picture), instead of body as JSON. */
  blob?: Blob
  fetch?: typeof fetch
  storage?: Store
}

let refreshing: Promise<boolean> | null = null

/** Gets a new access token with the refresh token; one refresh at a time. */
export function refreshSession(opts: RequestOptions = {}): Promise<boolean> {
  if (refreshing) return refreshing
  const rt = read(opts.storage, REFRESH_KEY)
  if (!rt) return Promise.resolve(false)
  const doFetch = opts.fetch ?? fetch
  refreshing = (async () => {
    try {
      const res = await doFetch('/v1/users/refresh', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: rt }),
      })
      if (!res.ok) return false
      const body = (await res.json()) as { data?: { access_token?: string; refresh_token?: string; expires_at?: number } }
      const d = body.data
      if (!d?.access_token) return false
      write(opts.storage, TOKEN_KEY, d.access_token)
      if (d.refresh_token) write(opts.storage, REFRESH_KEY, d.refresh_token)
      if (d.expires_at !== undefined) write(opts.storage, 'expires_at', String(d.expires_at))
      return true
    } catch {
      return false
    } finally {
      // callers that joined this refresh hold the promise already
      refreshing = null
    }
  })()
  return refreshing
}

/** Calls path (e.g. "/v2/icedash/system") with the ZimaOS token and returns the JSON body. */
export async function request<T>(path: string, opts: RequestOptions = {}, retried = false): Promise<T> {
  const doFetch = opts.fetch ?? fetch
  const headers: Record<string, string> = { Accept: 'application/json' }
  const t = token(opts.storage)
  // The bare token: CasaOS 0.4.15 answers 401 to "Bearer <token>", ZimaOS takes both (measured).
  if (t) headers.Authorization = t
  if (opts.body !== undefined) headers['Content-Type'] = 'application/json'
  if (opts.blob) headers['Content-Type'] = 'application/octet-stream'

  let res: Response
  try {
    res = await doFetch(path, {
      method: opts.method ?? 'GET',
      headers,
      body: opts.blob ?? (opts.body === undefined ? undefined : JSON.stringify(opts.body)),
      credentials: 'same-origin',
    })
  } catch {
    throw new ApiError(0, 'unreachable', 'network error')
  }
  if (res.status === 401 && !retried && t && (await refreshSession(opts))) {
    return request<T>(path, opts, true)
  }

  const text = await res.text()
  let data: unknown = undefined
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = undefined
    }
  }
  if (!res.ok) {
    const err = (data ?? {}) as { code?: string; error?: string; message?: string }
    const code = err.code ?? (res.status === 502 || res.status === 503 ? 'unreachable' : `http_${res.status}`)
    throw new ApiError(res.status, code, err.error ?? err.message ?? res.statusText)
  }
  return data as T
}

/**
 * Signs in with the system's own login (POST /v1/users/login, measured on
 * CasaOS 0.4.15) and keeps the tokens where its own UI keeps them
 * (access_token, refresh_token, expires_at, user), so both stay signed in.
 * Throws an ApiError with the system's message when it says no.
 */
export async function signIn(username: string, password: string, opts: RequestOptions = {}): Promise<void> {
  const doFetch = opts.fetch ?? fetch
  let res: Response
  try {
    res = await doFetch('/v1/users/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    })
  } catch {
    throw new ApiError(0, 'unreachable', 'network error')
  }
  type Login = {
    message?: string
    data?: { token?: { access_token?: string; refresh_token?: string; expires_at?: number }; user?: unknown }
  }
  let body: Login = {}
  try {
    body = (await res.json()) as Login
  } catch {
    // not JSON
  }
  const t = body.data?.token
  if (!res.ok || !t?.access_token) {
    throw new ApiError(res.status, 'sign_in_failed', body.message ?? res.statusText)
  }
  write(opts.storage, TOKEN_KEY, t.access_token)
  if (t.refresh_token) write(opts.storage, REFRESH_KEY, t.refresh_token)
  if (t.expires_at !== undefined) write(opts.storage, 'expires_at', String(t.expires_at))
  if (body.data?.user) write(opts.storage, 'user', JSON.stringify(body.data.user))
  await keepStockVersion(opts)
}

/**
 * CasaOS's own UI drops the token and shows its login when localStorage has
 * no "version" (its router, measured on 0.4.15); its login saves
 * GET /v1/sys/version's current_version there. IceDash does the same, so the
 * stock UI at /casaos/ stays signed in too.
 */
export async function keepStockVersion(opts: RequestOptions = {}): Promise<void> {
  if (read(opts.storage, 'version')) return
  const t = token(opts.storage)
  if (!t) return
  try {
    const res = await (opts.fetch ?? fetch)('/v1/sys/version', { headers: { Authorization: t } })
    if (!res.ok) return
    const v = ((await res.json()) as { data?: { current_version?: string } }).data?.current_version
    if (v) write(opts.storage, 'version', v)
  } catch {
    // the stock UI asks for a sign-in then; IceDash is unaffected
  }
}

/** Sign out: forget the tokens and go to the stock dashboard's login page. */
export function signOut() {
  for (const k of [TOKEN_KEY, REFRESH_KEY, 'expires_at']) {
    try {
      localStorage.removeItem(k)
    } catch {
      // nothing stored
    }
  }
  location.href = loginUrl()
}

/** Fetches a picture that needs the sign-in (a wallpaper, an avatar); null when there is none. */
export async function fetchBlob(path: string, opts: RequestOptions = {}, retried = false): Promise<Blob | null> {
  const t = token(opts.storage)
  let res: Response
  try {
    res = await (opts.fetch ?? fetch)(path, { headers: t ? { Authorization: t } : {}, credentials: 'same-origin' })
  } catch {
    return null
  }
  if (res.status === 401 && !retried && t && (await refreshSession(opts))) return fetchBlob(path, opts, true)
  if (!res.ok) return null
  try {
    return await res.blob()
  } catch {
    return null
  }
}