// The Files app and the Terminal (PLAN.md, Phase 3c). All admin only.
import { ApiError, refreshSession, request, token } from './client'

const base = '/v2/icedash'

export type Kind = 'folder' | 'image' | 'text' | 'pdf' | 'video' | 'audio' | 'other'

export interface Entry {
  name: string
  path: string
  dir: boolean
  size: number
  modified: string
  mode: string
  link?: boolean
  broken?: boolean
  protected?: boolean
  kind: Kind
}

export interface Listing {
  path: string
  protected: boolean
  entries: Entry[]
  hidden: number
}

export interface Job {
  id: string
  kind: 'move' | 'copy'
  to: string
  items: number
  total: number
  done: number
  state: 'running' | 'done' | 'failed' | 'cancelled'
  error?: string
  started: string
  ended?: string
}

export interface TrashItem {
  id: string
  name: string
  from: string
  dir: boolean
  size: number
  deleted: string
  kind: Kind
}

/** What to do when a name is taken; "" asks (the API answers 409 "exists"). */
export type Clash = '' | 'replace' | 'keep' | 'skip'

export const list = (path: string, hidden: boolean) =>
  request<Listing>(`${base}/files/list?path=${encodeURIComponent(path)}${hidden ? '&hidden=1' : ''}`)

const ticket = (paths: string[], zip = false) =>
  request<{ ticket: string }>(`${base}/files/ticket`, { method: 'POST', body: { paths, zip } }).then((r) => r.ticket)

/** A URL the browser can load without the sign-in header (previews, <video>, downloads). */
export async function rawUrl(path: string, download = false): Promise<string> {
  const t = await ticket([path])
  return `${base}/files/raw?ticket=${t}${download ? '&download=1' : ''}`
}

export async function zipUrl(paths: string[]): Promise<string> {
  const t = await ticket(paths, true)
  return `${base}/files/zip?ticket=${t}`
}

/** Starts a download by following a ticket URL. */
export function startDownload(url: string) {
  const a = document.createElement('a')
  a.href = url
  a.rel = 'noopener'
  document.body.append(a)
  a.click()
  a.remove()
}

export const mkdir = (path: string, name: string) => request<Entry>(`${base}/files/folder`, { method: 'POST', body: { path, name } })
export const rename = (path: string, name: string) => request<Entry>(`${base}/files/rename`, { method: 'POST', body: { path, name } })
export const transfer = (kind: 'move' | 'copy', paths: string[], to: string, clash: Clash = '') =>
  request<Job>(`${base}/files/${kind}`, { method: 'POST', body: { paths, to, clash } })
export const remove = (paths: string[]) => request<{ deleted: number }>(`${base}/files/delete`, { method: 'POST', body: { paths } })
export const jobs = () => request<{ items: Job[] }>(`${base}/files/jobs`).then((r) => r.items)
export const cancelJob = (id: string) => request(`${base}/files/jobs/${encodeURIComponent(id)}/cancel`, { method: 'POST' })

export const trash = () => request<{ items: TrashItem[] }>(`${base}/files/trash`).then((r) => r.items)
export const restore = (ids: string[]) => request<{ restored: Entry[] }>(`${base}/files/trash/restore`, { method: 'POST', body: { ids } })
export const purge = (ids: string[]) => request(`${base}/files/trash/delete`, { method: 'POST', body: { ids } })
export const emptyTrash = () => request(`${base}/files/trash/empty`, { method: 'POST' })

export interface Upload {
  promise: Promise<{ entry?: Entry; skipped: boolean }>
  abort: () => void
}

/**
 * Uploads one file with progress. name may hold "/" for a file inside a
 * dropped folder. A 401 refreshes the session once and starts over.
 */
export function upload(dir: string, name: string, file: Blob, clash: Clash, onProgress: (sent: number) => void): Upload {
  let xhr: XMLHttpRequest | null = null
  let aborted = false
  const send = (retried: boolean): Promise<{ entry?: Entry; skipped: boolean }> =>
    new Promise((resolve, reject) => {
      xhr = new XMLHttpRequest()
      const q = `path=${encodeURIComponent(dir)}&name=${encodeURIComponent(name)}&clash=${clash}`
      xhr.open('POST', `${base}/files/upload?${q}`)
      const t = token()
      if (t) xhr.setRequestHeader('Authorization', t)
      xhr.setRequestHeader('Content-Type', 'application/octet-stream')
      xhr.upload.onprogress = (e) => onProgress(e.loaded)
      xhr.onerror = () => reject(new ApiError(0, 'unreachable', 'network error'))
      xhr.onabort = () => reject(new ApiError(0, 'aborted', 'cancelled'))
      xhr.onload = async () => {
        const x = xhr!
        let data: { code?: string; error?: string; entry?: Entry; skipped?: boolean } = {}
        try {
          data = JSON.parse(x.responseText)
        } catch {
          // not JSON
        }
        if (x.status === 401 && !retried && !aborted && (await refreshSession())) {
          send(true).then(resolve, reject)
          return
        }
        if (x.status >= 200 && x.status < 300) resolve({ entry: data.entry, skipped: !!data.skipped })
        else reject(new ApiError(x.status, data.code ?? `http_${x.status}`, data.error ?? x.statusText))
      }
      xhr.send(file)
    })
  return {
    promise: send(false),
    abort: () => {
      aborted = true
      xhr?.abort()
    },
  }
}

/** The Terminal: the password again gives a ticket for one connection. */
export const terminalTicket = (password: string) =>
  request<{ ticket: string }>(`${base}/terminal/ticket`, { method: 'POST', body: { password } }).then((r) => r.ticket)

export function terminalSocketUrl(ticket: string, cols: number, rows: number): string {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${location.host}${base}/terminal?ticket=${ticket}&cols=${cols}&rows=${rows}`
}
