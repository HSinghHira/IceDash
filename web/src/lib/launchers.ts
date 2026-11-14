// The system launchers. Files, Terminal and Trash open IceDash's own windows
// for admins (Phase 3c); members don't get them. App Store and Settings open
// the stock dashboard in a new tab (on CasaOS 0.4.15 everything is on its
// home page).
import { strings } from '../strings'
import { dashboardUrl } from './host'
import type { IconName } from './icons'

export interface Launcher {
  id: 'files' | 'icestore' | 'appstore' | 'terminal' | 'settings' | 'trash'
  name: string
  icon: IconName
  url: string
  /** Opens IceDash's own window instead of url. */
  open?: 'files' | 'trash' | 'terminal' | 'settings'
}

/** The stock dashboard: App Store and Settings are popups there with no URL of their own. */
export const stockDashboard = () => dashboardUrl()
export const ICESTORE_SITE = 'https://icestore.hsinghhira.me'
export const TERMINAL_PORT = 7681

export interface Where {
  protocol: string // "http:"
  hostname: string // "192.168.68.17"
}

/** Launchers for the desktop (Files, IceStore, App Store, Terminal). */
export function desktopLaunchers(_where: Where, opts: { admin: boolean; iceStoreInZimaOS: boolean }): Launcher[] {
  const list: Launcher[] = []
  if (opts.admin) list.push({ id: 'files', name: strings.files, icon: 'folder', url: '', open: 'files' })
  list.push(
    {
      id: 'icestore',
      name: strings.iceStore,
      icon: 'snowflake',
      url: opts.iceStoreInZimaOS ? dashboardUrl() : ICESTORE_SITE,
    },
    { id: 'appstore', name: strings.appStore, icon: 'grid', url: dashboardUrl() },
  )
  if (opts.admin) list.push({ id: 'terminal', name: strings.terminal, icon: 'terminal', url: '', open: 'terminal' })
  return list
}

/** Settings and Trash, at the end of the dock. */
export function dockEnd(opts: { admin: boolean }): Launcher[] {
  // IceDash's own Settings (Phase 4); the stock settings are one click away inside it.
  const list: Launcher[] = [{ id: 'settings', name: strings.settings, icon: 'settings', url: '', open: 'settings' }]
  if (opts.admin) list.push({ id: 'trash', name: strings.trash, icon: 'trash', url: '', open: 'trash' })
  return list
}

/** Runs a launcher that opens one of IceDash's windows; false: follow its url. */
export function openLauncher(
  l: Launcher,
  open: { files: (view?: 'files' | 'trash') => void; terminal: () => void; settings: () => void },
): boolean {
  if (l.open === 'settings') open.settings()
  else if (l.open === 'files') open.files('files')
  else if (l.open === 'trash') open.files('trash')
  else if (l.open === 'terminal') open.terminal()
  else return false
  return true
}

/** ZimaOS's terminal is ttyd on its own port (7681), same host as the dashboard. */
export function terminalUrl(where: Where): string {
  const host = where.hostname.includes(':') && !where.hostname.startsWith('[') ? `[${where.hostname}]` : where.hostname
  return `${where.protocol}//${host}:${TERMINAL_PORT}/`
}
