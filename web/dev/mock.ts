// Dev only: a stand-in for ZimaOS and IceDash's backend, so `npm run dev` shows
// the desktop on a PC without a box. ICEDASH_MOCK=1 npm run dev
// The answers copy the shapes measured on a ZimaCube with ZimaOS 1.8.0
// (docs/HOST-NOTES.md). Never part of a build.
import type { Plugin } from 'vite'

const enc = (o: object) => Buffer.from(JSON.stringify(o)).toString('base64url')
const fakeToken = `${enc({ alg: 'ES256' })}.${enc({ username: 'Hira', role: 'admin', id: 1, iss: 'zimaos' })}.mock`

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
  app_type: 'v2app', name, port, status, icon: ic, index: '/', scheme: 'http', hostname: '', title: { custom: title, en_US: title },
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
        id: 2, time: new Date(Date.now() - 4 * 60000).toISOString(), kind: 'update_available', version: 'v0.2.0',
        title: 'Update available', body: 'IceDash v0.2.0 is ready to install.', action: { label: 'Update now', kind: 'update' }, read: false,
      },
      { id: 1, time: new Date(Date.now() - 26 * 3600000).toISOString(), kind: 'updated', version: 'v0.1.0', title: 'Updated', body: '', read: true },
    ],
    unread: 1,
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

export function mock(): Plugin {
  return {
    name: 'icedash-mock',
    apply: 'serve',
    transformIndexHtml: () => [
      { tag: 'script', children: `localStorage.setItem('access_token', ${JSON.stringify(fakeToken)})`, injectTo: 'head-prepend' },
    ],
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const path = (req.url ?? '').split('?')[0]
        const fn = routes[`${req.method} ${path}`]
        if (!fn) return next()
        res.setHeader('Content-Type', 'application/json')
        res.end(JSON.stringify(fn()))
      })
    },
  }
}
