// Calls to IceDash's backend and to ZimaOS's own APIs. Both live on the same
// origin, so the ZimaOS sign-in token in localStorage works for both: users
// never sign in twice (PLAN.md, "Same origin, free sign-in").

export const TOKEN_KEY = 'access_token'

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

export function token(storage: Pick<Storage, 'getItem'> = localStorage): string {
  try {
    return storage.getItem(TOKEN_KEY) ?? ''
  } catch {
    return ''
  }
}

export interface RequestOptions {
  method?: 'GET' | 'POST'
  body?: unknown
  fetch?: typeof fetch
  storage?: Pick<Storage, 'getItem'>
}

/** Calls path (e.g. "/v2/icedash/system") with the ZimaOS token and returns the JSON body. */
export async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const doFetch = opts.fetch ?? fetch
  const headers: Record<string, string> = { Accept: 'application/json' }
  const t = token(opts.storage)
  if (t) headers.Authorization = `Bearer ${t}`
  if (opts.body !== undefined) headers['Content-Type'] = 'application/json'

  let res: Response
  try {
    res = await doFetch(path, {
      method: opts.method ?? 'GET',
      headers,
      body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
      credentials: 'same-origin',
    })
  } catch {
    throw new ApiError(0, 'unreachable', 'network error')
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
    const err = (data ?? {}) as { code?: string; error?: string }
    throw new ApiError(res.status, err.code ?? `http_${res.status}`, err.error ?? res.statusText)
  }
  return data as T
}

export interface Health {
  version: string
  arch: string
  started: string
}

export const health = (opts?: RequestOptions) => request<Health>('/v2/icedash/health', opts)
