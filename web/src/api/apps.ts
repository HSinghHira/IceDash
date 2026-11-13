// Phase 2 calls to IceDash's backend: the IceStore catalog, installs and the
// installed apps (internal/api/apps.go).
import { ApiError, request, token } from './client'

const base = '/v2/icedash'

export interface CatalogApp {
  id: string
  name: string
  title: string
  tagline: string
  category: string
  categories: string[]
  author: string
  developer: string
  architectures: string[]
  icon: string
  thumbnail: string
  version: string
  content_hash: string
}

export interface CatalogList {
  updated_at: string
  fetched_at: string
  apps: CatalogApp[]
  error?: string
}

export interface Meta {
  id: string
  title: string
  tagline: string
  description: string
  category: string
  developer: string
  version: string
  update_at: string
  release_note: string
  screenshot_link: string[]
  tips: { before_install: string }
  repo: string
  support: string
  docs: string
  min_memory: number
}

export const catalog = () => request<CatalogList>(`${base}/catalog`)
export const refreshCatalog = () => request<CatalogList>(`${base}/catalog/refresh`, { method: 'POST' })
export const catalogApp = (id: string) => request<{ app: CatalogApp; meta: Meta }>(`${base}/catalog/${encodeURIComponent(id)}`)

export interface Input {
  service: string
  key: string
  value: string
  description?: string
  fill?: 'auto' | 'account' | 'generated'
  required: boolean
  main: boolean
  secret: boolean
}

export interface Port {
  service: string
  target: string
  published: string
  protocol: string
  description?: string
}

export interface Access {
  privileged: boolean
  docker_socket: boolean
  host_network: boolean
}

export interface SetupView {
  app: CatalogApp
  installed: boolean
  before_install?: string
  name: string
  address: string
  inputs: Input[]
  ports: Port[]
  data: string
  access: Access
}

export interface Choices {
  values: Record<string, string>
  ports: Record<string, string>
}

export interface CheckResult {
  ok: boolean
  ports_in_use: string[]
  suggest: Record<string, string>
  error?: string
}

export interface Job {
  name: string
  title: string
  icon: string
  kind: 'install' | 'update'
  task_id: string
  state: 'running' | 'succeeded' | 'failed'
  phase: string
  error?: string
  started: string
  ended?: string
}

export const inputKey = (i: Pick<Input, 'service' | 'key'>) => `${i.service}/${i.key}`
export const portKey = (p: Pick<Port, 'service' | 'target' | 'protocol'>) => `${p.service}/${p.target}/${p.protocol}`

export const setup = (id: string) => request<SetupView>(`${base}/catalog/${encodeURIComponent(id)}/setup`)
export const check = (id: string, c: Choices) =>
  request<CheckResult>(`${base}/catalog/${encodeURIComponent(id)}/check`, { method: 'POST', body: c })
export const install = (id: string, c: Choices) =>
  request<Job>(`${base}/catalog/${encodeURIComponent(id)}/install`, { method: 'POST', body: c })
export const jobs = () => request<{ items: Job[] }>(`${base}/jobs`)

export interface AppInfo {
  name: string
  store_id: string
  icestore: boolean
  version: string
  latest?: string
  update: boolean
  first_launch: boolean
}

export const appInfo = () => request<{ items: AppInfo[] }>(`${base}/apps`)

const app = (name: string) => `${base}/apps/${encodeURIComponent(name)}`
export type AppAction = 'start' | 'stop' | 'restart'
export const appAction = (name: string, a: AppAction) => request<{ name: string }>(`${app(name)}/${a}`, { method: 'POST' })
export const updateApp = (name: string) => request<Job>(`${app(name)}/update`, { method: 'POST' })
export const uninstallApp = (name: string, deleteData: boolean) =>
  request<{ name: string; data_kept: boolean; data: string }>(`${app(name)}/uninstall`, {
    method: 'POST',
    body: { delete_data: deleteData },
  })
export const logs = (name: string, lines = 500) => request<{ name: string; text: string }>(`${app(name)}/logs?lines=${lines}`)

export interface Field {
  /** Fields of one group belong together, e.g. an address with its own login. */
  group?: string
  label: string
  value: string
  secret: boolean
  url: boolean
}

export interface Creds {
  fields: Field[]
  notes: string[]
}

export const credentials = (name: string) => request<Creds>(`${app(name)}/credentials`)

export interface FirstLaunch {
  name: string
  title: string
  tagline: string
  steps: string[]
  creds?: Creds
}

/** Whether the app lets itself be shown in a window inside IceDash. */
export const frame = (name: string) => request<{ name: string; frameable: boolean; reason?: string }>(`${app(name)}/frame`)

export const firstLaunch = (name: string) => request<FirstLaunch>(`${app(name)}/first-launch`)
export const dismissFirstLaunch = (name: string) =>
  request<{ name: string }>(`${app(name)}/first-launch/dismiss`, { method: 'POST' })

/** Downloads the app's data as .tar.gz: fetched with the token, then saved by the browser. */
export async function downloadBackup(name: string): Promise<void> {
  const t = token()
  let res: Response
  try {
    res = await fetch(`${app(name)}/backup`, { headers: t ? { Authorization: `Bearer ${t}` } : {}, credentials: 'same-origin' })
  } catch {
    throw new ApiError(0, 'unreachable', 'network error')
  }
  if (!res.ok) {
    let msg = res.statusText
    let code = `http_${res.status}`
    try {
      const j = (await res.json()) as { code?: string; error?: string }
      msg = j.error ?? msg
      code = j.code ?? code
    } catch {
      // not JSON
    }
    throw new ApiError(res.status, code, msg)
  }
  const blob = await res.blob()
  const cd = res.headers.get('Content-Disposition') ?? ''
  const file = /filename="([^"]+)"/.exec(cd)?.[1] ?? `${name}-backup.tar.gz`
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = file
  document.body.appendChild(a)
  a.click()
  a.remove()
  setTimeout(() => URL.revokeObjectURL(url), 10000)
}
