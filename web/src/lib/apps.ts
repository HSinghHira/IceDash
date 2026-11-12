// The installed apps, from ZimaOS's /v2/app_management/web/appgrid (measured on
// ZimaOS 1.8.0, docs/HOST-NOTES.md item 1). ZimaOS marks that endpoint "for
// internal use", so all parsing of it lives here.

/** One entry of the app grid as ZimaOS sends it. */
export interface GridEntry {
  name: string
  title?: { custom?: string; en_US?: string; [lang: string]: string | undefined }
  icon?: string
  scheme?: string
  hostname?: string
  port?: string | number
  index?: string
  status?: string
  app_type?: string
}

export type AppStatus = 'running' | 'starting' | 'stopped'

export interface App {
  id: string
  name: string
  icon?: string
  status: AppStatus
  /** Where clicking opens the app; missing for entries without an address. */
  url?: string
}

/**
 * ZimaOS reports Docker states. It went straight from "exited" to "running"
 * when we measured, so "starting" only shows for the in-between Docker states.
 */
export function statusOf(s: string | undefined): AppStatus {
  const v = (s ?? '').toLowerCase()
  if (v === 'running') return 'running'
  if (v === 'restarting' || v === 'created' || v === 'starting') return 'starting'
  return 'stopped'
}

/** The app's own address: scheme, host, port and index path. An empty hostname means the host the browser is on. */
export function appUrl(e: GridEntry, browserHost: string): string | undefined {
  const port = String(e.port ?? '').trim()
  if (!port || !/^\d{1,5}$/.test(port)) return undefined
  const scheme = e.scheme === 'https' ? 'https' : 'http'
  const host = (e.hostname ?? '').trim() || browserHost
  let index = (e.index ?? '').trim() || '/'
  if (!index.startsWith('/')) index = '/' + index
  return `${scheme}://${host}:${port}${index}`
}

/** The name to show: the user's own title first, then the English one, then the ID. */
export function titleOf(e: GridEntry): string {
  return e.title?.custom?.trim() || e.title?.en_US?.trim() || e.name
}

export function toApp(e: GridEntry, browserHost: string): App {
  return {
    id: e.name,
    name: titleOf(e),
    icon: e.icon || undefined,
    status: statusOf(e.status),
    url: appUrl(e, browserHost),
  }
}

/** The grid as apps, sorted by name. */
export function toApps(entries: GridEntry[], browserHost: string): App[] {
  return entries
    .filter((e) => e && typeof e.name === 'string')
    .map((e) => toApp(e, browserHost))
    .sort((a, b) => a.name.localeCompare(b.name, 'en', { sensitivity: 'base' }))
}

export function counts(apps: App[]): Record<AppStatus, number> {
  const c: Record<AppStatus, number> = { running: 0, starting: 0, stopped: 0 }
  for (const a of apps) c[a.status]++
  return c
}
