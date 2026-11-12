// The system launchers. In Phase 1 each opens a ZimaOS page in a new tab
// (deep links measured on ZimaOS 1.8.0, docs/HOST-NOTES.md item 6).
import { strings } from '../strings'
import type { IconName } from './icons'

export interface Launcher {
  id: 'files' | 'icestore' | 'appstore' | 'terminal' | 'settings' | 'trash'
  name: string
  icon: IconName
  url: string
}

/** The stock dashboard: App Store and Settings are popups there with no URL of their own. */
export const ZIMAOS_DASHBOARD = '/#/'
export const ICESTORE_SITE = 'https://icestore.hsinghhira.me'
export const TERMINAL_PORT = 7681

export interface Where {
  protocol: string // "http:"
  hostname: string // "192.168.68.17"
}

/** Launchers for the desktop (Files, IceStore, App Store, Terminal). */
export function desktopLaunchers(where: Where, opts: { terminal: boolean; iceStoreInZimaOS: boolean }): Launcher[] {
  const list: Launcher[] = [
    { id: 'files', name: strings.files, icon: 'folder', url: '/modules/icewhale_files/index.html#/files' },
    {
      id: 'icestore',
      name: strings.iceStore,
      icon: 'snowflake',
      url: opts.iceStoreInZimaOS ? ZIMAOS_DASHBOARD : ICESTORE_SITE,
    },
    { id: 'appstore', name: strings.appStore, icon: 'grid', url: ZIMAOS_DASHBOARD },
  ]
  if (opts.terminal) list.push({ id: 'terminal', name: strings.terminal, icon: 'terminal', url: terminalUrl(where) })
  return list
}

/** Settings and Trash, at the end of the dock. */
export function dockEnd(): Launcher[] {
  return [
    { id: 'settings', name: strings.settings, icon: 'settings', url: ZIMAOS_DASHBOARD },
    // TODO(Phase 3): the Files app gets its own Trash view; until then Files opens.
    { id: 'trash', name: strings.trash, icon: 'trash', url: '/modules/icewhale_files/index.html#/files' },
  ]
}

/** ZimaOS's terminal is ttyd on its own port (7681), same host as the dashboard. */
export function terminalUrl(where: Where): string {
  const host = where.hostname.includes(':') && !where.hostname.startsWith('[') ? `[${where.hostname}]` : where.hostname
  return `${where.protocol}//${host}:${TERMINAL_PORT}/`
}
