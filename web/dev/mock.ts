// Dev only: a stand-in for ZimaOS and IceDash's backend, so `npm run dev` shows
// the desktop on a PC without a box: `make preview`, or `npm run mock` in web/.
// The answers copy the shapes measured on a ZimaCube with ZimaOS 1.8.0
// (docs/HOST-NOTES.md). Never part of a build.
import type { Plugin } from 'vite'

const enc = (o: object) => Buffer.from(JSON.stringify(o)).toString('base64url')
const fakeToken = `${enc({ alg: 'ES256' })}.${enc({ username: 'Hira', role: 'admin', id: 1, iss: 'zimaos' })}.mock`

// Apps point at the real box, so app windows show the real apps when this PC
// is on the same network. ICEDASH_PREVIEW_HOST changes it.
const BOX = process.env.ICEDASH_PREVIEW_HOST ?? '192.168.68.17'

const icon = (id: string) => `https://icestore.hsinghhira.me/apps/com.icestore.${id}/assets/icon.png`
const appgrid = [
  ['ente-garage', 'Ente - Garage S3', '6600', 'running', icon('ente-garage')],
  ['homeassistant', 'Home Assistant', '8123', 'running', 'https://cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore@gh-pages/apps/org.icewhale.homeassistant/assets/icon.svg'],
  ['yuvomi', 'Yuvomi', '6161', 'running', icon('yuvomi')],
  ['appflowy', 'AppFlowy', '6101', 'running', icon('appflowy')],
  ['gitsave', 'GitSave', '6181', 'exited', icon('gitsave')],
  ['pocket-id', 'Pocket ID', '6157', 'running', icon('pocket-id')],
  ['isponsorblocktv', 'iSponsorBlockTV', '6129', 'restarting', icon('isponsorblocktv')],
  ['cloudflared', 'Cloudflared', '14333', 'running', 'https://cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore@gh-pages/apps/org.icewhale.cloudflared/assets/icon.svg'],
  ['doory-art', 'Doory Art', '6143', 'running', icon('doory-art')],
].map(([name, title, port, status, ic]) => ({
  app_type: 'v2app', name, port, status, icon: ic, index: '/', scheme: 'http', hostname: BOX, title: { custom: title, en_US: title },
}))
appgrid.push({ app_type: 'container', name: 'c68ca986fe10', status: 'running', title: { en_US: 'baby-schedule' } } as never)

function history() {
  const now = Date.now()
  return Array.from({ length: 30 }, (_, i) => ({
    time: new Date(now - (29 - i) * 2000).toISOString(),
    down: 1.5e6 + Math.sin(i / 3) * 9e5 + Math.random() * 6e5 + i * 6e4,
    up: 2.5e5 + Math.random() * 1.4e5,
  }))
}

const routes: Record<string, () => unknown> = {
  'GET /v2/icedash/health': () => ({ version: '0.1.0', arch: 'amd64', started: new Date().toISOString() }),
  'GET /v2/icedash/system': () => {
    const h = history()
    return { down: h.at(-1)!.down, up: h.at(-1)!.up, history: h, interfaces: ['wlan0'], uptime_sec: 960193, device: 'ZimaCube' }
  },
  'GET /v2/icedash/notifications': () => ({
    items: [
      {
        id: 4, time: new Date(Date.now() - 2 * 60000).toISOString(), kind: 'app_installed', app: 'yuvomi',
        title: 'Yuvomi is ready', body: 'Yuvomi is installed and starting.', action: { label: 'Open', kind: 'open_app' }, read: false,
      },
      {
        id: 3, time: new Date(Date.now() - 50 * 60000).toISOString(), kind: 'app_stopped', app: 'gitsave',
        title: 'GitSave stopped', body: 'GitSave isn’t running now. Start it from its menu, or look at its logs to see why.',
        action: { label: 'Start', kind: 'start_app' }, read: true,
      },
      {
        id: 2, time: new Date(Date.now() - 4 * 60000).toISOString(), kind: 'update_available', version: 'v0.2.0',
        title: 'Update available', body: 'IceDash v0.2.0 is ready to install.', action: { label: 'Update now', kind: 'update' }, read: false,
      },
      { id: 1, time: new Date(Date.now() - 26 * 3600000).toISOString(), kind: 'updated', version: 'v0.1.0', title: 'Updated', body: '', read: true },
    ],
    unread: 2,
  }),
  'GET /v2/icedash/update': () => ({
    current: '0.1.0', latest: 'v0.2.0', notes: 'The desktop from mock-up 1: apps, dock, widgets and search.',
    state: 'available', checked_at: new Date(Date.now() - 6 * 60000).toISOString(), steps: [],
  }),
  'GET /v2/icedash/settings': () => ({ auto_check: true }),
  'GET /v2/app_management/web/appgrid': () => ({ data: appgrid, message: 'This data is for internal use ONLY' }),
  'GET /v1/sys/utilization': () => ({
    success: 200,
    data: {
      cpu: { percent: 14 + Math.random() * 8, num: 4, temperature: 49 },
      mem: { total: 16644636672, used: 3379572736, available: 12595781632, usedPercent: 20.3 },
      sys_disk: { size: 238030327808, used: 140109729792, avail: 97903820800, health: true },
    },
  }),
  'GET /v1/users/current/custom/system': () => ({ success: 200, data: { lang: 'en_US' } }),
  'GET /v2/app_management/appstore': () => ({ data: [{ id: 0, url: 'https://cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore@gh-pages/store/main.zip' }] }),
}

// ---- Phase 2 ----
const SITE = 'https://icestore.hsinghhira.me'
let indexCache: unknown = null
async function catalog() {
  if (!indexCache) {
    const idx = (await (await fetch(`${SITE}/index.json`)).json()) as { apps: Record<string, string>[]; updated_at: string }
    for (const a of idx.apps) {
      a.name = a.id!.replace('com.icestore.', '')
      for (const k of ['icon', 'thumbnail', 'meta_url', 'compose_url']) if (a[k]?.startsWith('/')) a[k] = SITE + a[k]
    }
    indexCache = { updated_at: idx.updated_at, fetched_at: new Date().toISOString(), apps: idx.apps }
  }
  return indexCache
}

const appInfo = [
  { name: 'gitsave', store_id: 'com.icestore.gitsave', icestore: true, version: '1.4.0', latest: '1.5.0', update: true, first_launch: false },
  { name: 'pocket-id', store_id: 'com.icestore.pocket-id', icestore: true, version: 'v2.19.1', latest: 'v2.19.1', update: false, first_launch: false },
  { name: 'yuvomi', store_id: 'com.icestore.yuvomi', icestore: true, version: '2.79.0', latest: '2.79.0', update: false, first_launch: true },
  { name: 'appflowy', store_id: 'com.icestore.appflowy', icestore: true, version: '0.19.2', latest: '0.19.2', update: false, first_launch: false },
  { name: 'homeassistant', store_id: 'org.icewhale.homeassistant', icestore: false, version: '2026.5.4', update: false, first_launch: false },
]

const setupView = (id: string) => ({
  app: { id, name: 'documenso', title: 'Documenso', category: 'Productivity', version: 'v2.21.1', icon: `${SITE}/apps/com.icestore.documenso/assets/icon.png` },
  installed: false,
  name: 'documenso',
  address: 'http://192.168.68.17:6142',
  data: '/DATA/AppData/documenso',
  access: { privileged: true, docker_socket: false, host_network: false },
  ports: [{ service: 'documenso', target: '8080', published: '6142', protocol: 'tcp', description: 'Documenso web app' }],
  inputs: [
    { service: 'documenso', key: 'TZ', value: '$TZ', description: 'Your time zone.', required: false, main: false, secret: false },
    { service: 'documenso', key: 'PUBLIC_URL', value: 'http://192.168.68.17:6142', fill: 'auto', description: 'The address you open Documenso at.', required: false, main: true, secret: false },
    { service: 'documenso', key: 'DOCUMENSO_ADMIN_EMAIL', value: '', description: 'The account you sign up with becomes the admin. Leave blank for no admin.', required: false, main: true, secret: false },
    { service: 'documenso', key: 'SMTP_HOST', value: '', description: 'Email (SMTP) server.', required: false, main: false, secret: false },
    { service: 'documenso', key: 'SMTP_PASSWORD', value: '', description: 'SMTP password.', required: false, main: false, secret: true },
    { service: 'documenso', key: 'NEXT_PRIVATE_GOOGLE_CLIENT_ID', value: '', description: 'Google sign-in.', required: false, main: false, secret: false },
  ],
})

const creds = {
  fields: [
    { group: 'Web app', label: 'Address', value: 'http://192.168.68.17:6142', secret: false, url: true },
    { group: 'Mail viewer', label: 'Address', value: 'http://192.168.68.17:6142/mail', secret: false, url: true },
    { group: 'Mail viewer', label: 'User', value: 'admin', secret: false, url: false },
    { group: 'Mail viewer', label: 'Password', value: 'kQ2-vb7Lr9xWm4Ta', secret: true, url: false },
  ],
  notes: ['Create your account on first visit. Sign-up emails land in the mail viewer.'],
}

const dynamic: [RegExp, string, (m: RegExpMatchArray) => unknown | Promise<unknown>][] = [
  [/^\/v2\/icedash\/catalog$/, 'GET', () => catalog()],
  [/^\/v2\/icedash\/catalog\/refresh$/, 'POST', () => catalog()],
  [/^\/v2\/icedash\/catalog\/([^/]+)\/setup$/, 'GET', (m) => setupView(m[1]!)],
  [/^\/v2\/icedash\/catalog\/([^/]+)\/check$/, 'POST', () => ({ ok: true, ports_in_use: [], suggest: {} })],
  [/^\/v2\/icedash\/catalog\/([^/]+)$/, 'GET', async (m) => {
    const c = (await catalog()) as { apps: Record<string, string>[] }
    const app = c.apps.find((a) => a.id === m[1] || a.name === m[1])
    const meta = (await (await fetch(app!.meta_url!)).json()) as Record<string, unknown>
    meta.screenshot_link = ((meta.screenshot_link as string[]) ?? []).map((u) => (u.startsWith('/') ? SITE + u : u))
    return { app, meta }
  }],
  [/^\/v2\/icedash\/apps$/, 'GET', () => ({ items: appInfo })],
  [/^\/v2\/icedash\/jobs$/, 'GET', () => ({ items: [] })],
  [/^\/v2\/icedash\/apps\/([^/]+)\/credentials$/, 'GET', () => creds],
  [/^\/v2\/icedash\/apps\/([^/]+)\/frame$/, 'GET', (m) => frameCheck(m[1]!)],
  [/^\/v2\/icedash\/apps\/([^/]+)\/logs$/, 'GET', (m) => ({ name: m[1], text: 'Server starting\nServer running on port 3000\nScheduling 2 backup jobs\n' })],
  [/^\/v2\/icedash\/apps\/([^/]+)\/first-launch$/, 'GET', () => ({
    name: 'yuvomi', title: 'Yuvomi', tagline: 'Household receipts, shared shopping lists and spending insights.',
    steps: ['Sign in with the login above.', 'Change the password: Management, Users, then the key icon on your row.', 'Add a receipt from a photo, or start a shopping list.'],
    creds: { fields: [{ label: 'User', value: 'admin', secret: false, url: false }, { label: 'Password', value: 'admin', secret: true, url: false }], notes: [] },
  })],
]

/** The real app's headers decide, as the backend does (internal/apps/frame.go). */
async function frameCheck(name: string) {
  const a = appgrid.find((x) => x.name === name)
  if (!a?.port) return { name, frameable: false, reason: 'no address' }
  let url = `http://${BOX}:${a.port}/`
  for (let hop = 0; hop < 6; hop++) {
    let res: Response
    try {
      res = await fetch(url, { redirect: 'manual', signal: AbortSignal.timeout(4000) })
    } catch {
      return { name, frameable: true }
    }
    const xfo = res.headers.get('x-frame-options')
    if (xfo) return { name, frameable: false, reason: `X-Frame-Options: ${xfo}` }
    const fa = /frame-ancestors([^;]*)/i.exec(res.headers.get('content-security-policy') ?? '')
    if (fa && !/(^|\s)(\*|https?:)(\s|$)/.test(fa[1]!)) return { name, frameable: false, reason: 'frame-ancestors' }
    const loc = res.headers.get('location')
    if (res.status < 300 || res.status >= 400 || !loc) break
    const next = new URL(loc, url)
    if ((next.port || '80') !== a.port) break
    url = `http://${BOX}:${a.port}${next.pathname}${next.search}`
  }
  return { name, frameable: true }
}

export function mock(): Plugin {
  return {
    name: 'icedash-mock',
    apply: 'serve',
    transformIndexHtml: () => [
      { tag: 'script', children: `localStorage.setItem('access_token', ${JSON.stringify(fakeToken)})`, injectTo: 'head-prepend' },
    ],
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const path = (req.url ?? '').split('?')[0] ?? ''
        const fn = routes[`${req.method} ${path}`]
        if (fn) {
          res.setHeader('Content-Type', 'application/json')
          res.end(JSON.stringify(fn()))
          return
        }
        for (const [re, method, h] of dynamic) {
          const m = path.match(re)
          if (m && req.method === method) {
            Promise.resolve(h(m)).then(
              (v) => {
                res.setHeader('Content-Type', 'application/json')
                res.end(JSON.stringify(v))
              },
              (e) => {
                res.statusCode = 502
                res.end(JSON.stringify({ code: 'upstream', error: String(e) }))
              },
            )
            return
          }
        }
        next()
      })
    },
  }
}
