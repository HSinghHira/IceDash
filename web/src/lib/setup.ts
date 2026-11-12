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
    .map((w) => (/^(IP|URL|ID|API|SMTP|OIDC|TZ|DNS|LAN|RP|UI|DB|HTTP|HTTPS|SSO)$/.test(w) ? w : w.toLowerCase()))
  if (!words.length) return key
  words[0] = words[0]!.charAt(0).toUpperCase() + words[0]!.slice(1)
  return words.join(' ')
}
