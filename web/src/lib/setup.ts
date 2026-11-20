// Helpers for the setup popup (mock-up 2).

/** The placeholders IceStore leaves for the user (internal/compose/setup.go). */
const placeholder = /CHANGE_?ME|\[ZIMAOS-IP-ADDRESS\]|\[SERVER-IP\]/

export const placeholderIn = (v: string) => placeholder.test(v)

/**
 * A readable label from an env key: DOCUMENSO_ADMIN_EMAIL → "Admin email"
 * (the app's own name in front is dropped), SERVER_IP → "Server IP".
 */
export function inputLabel(key: string, app: string): string {
  let k = key
  const prefix = app.toUpperCase().replace(/[^A-Z0-9]+/g, '_') + '_'
  if (k.startsWith(prefix) && k.length > prefix.length) k = k.slice(prefix.length)
  const words = k
    .split(/_+/)
    .filter(Boolean)
    .map((w) => (/^IFACES?$/.test(w) ? (w.endsWith('S') ? 'network interfaces' : 'network interface') : w))
    .map((w) => (/^(IP|URL|ID|API|SMTP|OIDC|TZ|DNS|LAN|RP|UI|DB|HTTP|HTTPS|SSO)$/.test(w) ? w : w.toLowerCase()))
  if (!words.length) return key
  words[0] = words[0]!.charAt(0).toUpperCase() + words[0]!.slice(1)
  return words.join(' ')
}

/**
 * Puts this server's host into IceStore text: [ZIMAOS-IP-ADDRESS], [SERVER-IP],
 * your-zimaos-ip, and <your ZimaOS IP> (written by some images' credentials).
 */
export function withHost(text: string, host: string): string {
  return text.replace(/\[ZIMAOS-IP-ADDRESS\]|\[SERVER-IP\]|your-zimaos-ip|<your[ -](?:zimaos|casaos|server)[ -]ip>/gi, host)
}

/** A clean absolute path: no "..", no doubled or trailing slash, one line. */
function cleanPath(p: string): boolean {
  if (!p.startsWith('/') || /[\n\r\0$]/.test(p) || p.length > 1024) return false
  if (p === '/') return true
  return !p.endsWith('/') && !p.split('/').slice(1).some((part) => part === '' || part === '.' || part === '..')
}

/** Roots for the user's folders, as internal/compose FolderRoots. */
export const folderRoots = ['/DATA', '/media', '/mnt']

/** A folder of the server IceDash can give an app. */
export const serverFolderOk = (p: string) =>
  cleanPath(p) && folderRoots.some((r) => p === r || p.startsWith(r + '/'))

/** A path inside the app: absolute, not its root, no colon. */
export const appFolderOk = (p: string) => cleanPath(p) && p !== '/' && !p.includes(':')
