// The live demo's stand-in for IceDash's backend and ZimaOS: every API call
// the dashboard makes is answered here, in the browser, from demo/data.ts.
// Changes live in memory; a person's look and mods also go to localStorage so
// a reload keeps them. Things a demo can't do answer with a plain message.
import type { CatalogApp, Choices, Job, SetupView } from '../api/apps'
import type { Entry, TrashItem } from '../api/files'
import type { Notification, Prefs, Settings } from '../api/icedash'
import type { Collection, Installed } from '../api/mods'
import { DEVICE, SERVER_IP, STORE, disks, firstLaunchFor, storagePools, icon, logsFor, people, startApps, startFiles, startNotes, usb, type DemoApp, type Node } from './data'

declare const __DEMO_VERSION__: string
export const VERSION = typeof __DEMO_VERSION__ === 'string' ? __DEMO_VERSION__ : 'v1'

type Reply = { status?: number; body: unknown }
const ok = (body: unknown): Reply => ({ body })
const no = (error: string, code = 'demo', status = 409): Reply => ({ status, body: { code, error } })
const DEMO_ONLY = 'The demo can’t do this. On your own server it works.'

const remember = <T>(key: string, fallback: T): T => {
  try {
    const v = localStorage.getItem('icedash-demo-' + key)
    return v ? (JSON.parse(v) as T) : fallback
  } catch {
    return fallback
  }
}
const keep = (key: string, v: unknown) => {
  try {
    localStorage.setItem('icedash-demo-' + key, JSON.stringify(v))
  } catch {
    // a private window: kept until the tab closes
  }
}

// ---- State ----
const apps: DemoApp[] = startApps.map((a) => ({ ...a }))
const notes: Notification[] = startNotes(VERSION)
let prefs: Partial<Prefs> = remember('prefs', {})
let settings: Settings = { auto_check: true, auto_install: false, zone: '', main_address: false }
const jobs = new Map<string, { job: Job; app: DemoApp; done: () => void }>()

// The status an app shows: "restarting" settles after a few seconds.
const settling = new Map<string, number>()
function statusOf(a: DemoApp) {
  const until = settling.get(a.name)
  if (until && Date.now() > until) {
    settling.delete(a.name)
    a.status = 'running'
  }
  return a.status
}
settling.set('changedetection', Date.now() + 12000)

function appUrl() {
  // Apps open the demo's own page about them: a demo has no apps to show.
  const https = location.protocol === 'https:'
  return { scheme: https ? 'https' : 'http', hostname: location.hostname, port: location.port || (https ? '443' : '80') }
}

function grid() {
  const u = appUrl()
  return apps.map((a) => ({
    app_type: 'v2app', name: a.name, status: statusOf(a), icon: icon(a.name),
    title: { custom: a.title, en_US: a.title },
    ...u, index: `${import.meta.env.BASE_URL}app.html?app=${encodeURIComponent(a.name)}`,
  }))
}

function note(n: Omit<Notification, 'id' | 'time' | 'read'>) {
  notes.unshift({ ...n, id: (notes[0]?.id ?? 0) + 1, time: new Date().toISOString(), read: false })
}

// ---- IceStore ----
let catalogCache: Promise<{ updated_at: string; fetched_at: string; apps: CatalogApp[] }> | null = null
function catalog() {
  catalogCache ??= fetch(`${STORE}/index.json`)
    .then((r) => r.json() as Promise<{ apps: (CatalogApp & Record<string, string>)[]; updated_at: string }>)
    .then((idx) => {
      for (const a of idx.apps) {
        a.name = a.id.replace('com.icestore.', '')
        for (const k of ['icon', 'thumbnail', 'meta_url', 'compose_url']) if (a[k]?.startsWith('/')) a[k] = STORE + a[k]
      }
      return { updated_at: idx.updated_at, fetched_at: new Date().toISOString(), apps: idx.apps }
    })
  catalogCache.catch(() => (catalogCache = null))
  return catalogCache
}
const findApp = async (id: string) => (await catalog()).apps.find((a) => a.id === id || a.name === id)

/** A port that no app on the demo uses yet. */
function freePort(seed: string) {
  let p = 6200 + ([...seed].reduce((h, c) => (h * 31 + c.charCodeAt(0)) >>> 0, 7) % 700)
  while (apps.some((a) => a.port === String(p))) p++
  return String(p)
}

async function setupView(id: string): Promise<SetupView | null> {
  const app = await findApp(id)
  if (!app) return null
  const name = app.name
  const installed = apps.some((a) => a.name === name)
  if (name === 'watchyourlan') {
    return {
      app, installed, name, address: `http://${SERVER_IP}:6121`, data: '/DATA/AppData/watchyourlan',
      access: { privileged: false, docker_socket: false, host_network: true }, ports: [],
      folders: [{ service: name, source: '/DATA/AppData/$AppID', target: '/data/WatchYourLAN', description: 'The app’s database, settings and scan history.' }],
      inputs: [
        { service: name, key: 'TZ', value: '$TZ', description: 'Your time zone. ZimaOS fills this in for you.', required: false, main: false, secret: false },
        {
          service: name, key: 'IFACES', value: 'eth0', fill: 'auto', required: false, main: true, secret: false,
          description: 'The network interfaces to scan. Click one to add or remove it.',
          interfaces: [{ name: 'eth0', addr: SERVER_IP, default: true }, { name: 'eth1', addr: '10.0.0.4' }],
        },
      ],
    }
  }
  const port = freePort(name)
  return {
    app, installed, name, address: `http://${SERVER_IP}:${port}`, data: `/DATA/AppData/${name}`,
    access: { privileged: false, docker_socket: false, host_network: false },
    ports: [{ service: name, target: '8080', published: port, protocol: 'tcp', description: `${app.title}'s web page` }],
    folders: [{ service: name, source: '/DATA/AppData/$AppID', target: '/data', description: 'The app’s data and settings.' }],
    inputs: [
      { service: name, key: 'TZ', value: '$TZ', description: 'Your time zone. ZimaOS fills this in for you.', required: false, main: false, secret: false },
      { service: name, key: 'APP_URL', value: `http://${SERVER_IP}:${port}`, fill: 'auto', description: 'The address you open the app at.', required: false, main: true, secret: false },
      { service: name, key: 'ADMIN_PASSWORD', value: 'Vt4-q8Ln2xPw7Rd9', fill: 'generated', description: 'The admin password, made for you. Credentials shows it later.', required: true, main: true, secret: true },
    ],
  }
}

function startJob(kind: Job['kind'], a: DemoApp, done: () => void): Job {
  const job: Job = {
    name: a.name, title: a.title, icon: icon(a.name), kind, task_id: `demo-${Date.now()}`,
    state: 'running', phase: 'accepted', started: new Date().toISOString(),
  }
  jobs.set(a.name, { job, app: a, done })
  return job
}
function jobNow(j: { job: Job; done: () => void }) {
  if (j.job.state !== 'running') return j.job
  const t = Date.now() - Date.parse(j.job.started)
  if (t > 9000) {
    j.job.state = 'succeeded'
    j.job.phase = 'completed'
    j.job.ended = new Date().toISOString()
    j.done()
  } else if (t > 2500) j.job.phase = 'pulling_images'
  return j.job
}

// ---- Files ----
const roots: Record<string, Node[]> = { '/': startFiles(), 'USB_Stick:/': usb() }
const trash: (TrashItem & { node: Node; parent: string })[] = []
const rootOf = (p: string) => (p.startsWith('USB_Stick:/') ? 'USB_Stick:/' : '/')
function childrenAt(path: string): Node[] | null {
  const r = rootOf(path)
  let list = roots[r]!
  const rest = path.slice(r.length).split('/').filter(Boolean)
  for (const part of rest) {
    const n = list.find((x) => x.entry.name === part && x.children)
    if (!n) return null
    list = n.children!
  }
  return list
}
const parentOf = (p: string) => {
  const r = rootOf(p)
  const i = p.lastIndexOf('/')
  return i < r.length ? r : p.slice(0, i)
}
const join = (dir: string, name: string) => (dir.endsWith('/') ? dir + name : dir + '/' + name)
function findNode(path: string) {
  const list = childrenAt(parentOf(path))
  const name = path.slice(path.lastIndexOf('/') + 1)
  const i = list?.findIndex((n) => n.entry.name === name) ?? -1
  return list && i > -1 ? { list, i, node: list[i]! } : null
}
function rebase(n: Node, path: string): Node {
  return {
    entry: { ...n.entry, path, name: path.slice(path.lastIndexOf('/') + 1) },
    children: n.children?.map((c) => rebase(c, join(path, c.entry.name))),
  }
}
function walk(list: Node[], fn: (e: Entry) => void) {
  for (const n of list) {
    fn(n.entry)
    if (n.children) walk(n.children, fn)
  }
}
let fileJobs: { id: string; kind: 'move' | 'copy'; to: string; items: number; total: number; done: number; state: 'done'; started: string; ended: string }[] = []

// ---- Mods: the official collections, copied when the demo was built ----
let collectionsCache: Promise<Collection[]> | null = null
const collections = () =>
  (collectionsCache ??= fetch(`${import.meta.env.BASE_URL}mods.json`)
    .then((r) => (r.ok ? (r.json() as Promise<{ collections: Collection[] }>) : { collections: [] }))
    .then((j) => j.collections)
    .catch(() => []))
let installed: Installed[] = remember('mods', [])
// The panel starts with IceDash's own cards, as the backend's Builtin.
let modPrefs: { theme: string; panel: string[] } = remember('mod-prefs', { theme: '', panel: ['clock', 'system', 'storage', 'network', 'apps'] })
const modView = () => {
  keep('mods', installed)
  keep('mod-prefs', modPrefs)
  return ok({
    mods: installed.map((m) => (m.kind === 'theme' && m.wallpaper ? { ...m, wallpaper: `/v2/icedash/mods/${m.id}/wallpaper` } : m)),
    prefs: modPrefs,
  })
}

// ---- Routes ----
type Handler = (m: RegExpMatchArray, body: Record<string, unknown>, q: URLSearchParams) => Reply | Promise<Reply>
const routes: [string, RegExp, Handler][] = [
  // ZimaOS
  ['GET', /^\/v2\/app_management\/web\/appgrid$/, () => ok({ data: grid() })],
  ['GET', /^\/v2\/app_management\/appstore$/, () => ok({ data: [{ id: 0, url: `${STORE}/store.zip` }] })],
  ['GET', /^\/v1\/sys\/utilization$/, () => {
    const t = Date.now() / 60000
    const used = (3.6 + Math.sin(t) * 0.4) * 1024 ** 3
    return ok({
      success: 200,
      data: {
        cpu: { percent: 9 + Math.random() * 14, num: 6, temperature: 47 },
        mem: { total: 32 * 1024 ** 3, used, available: 32 * 1024 ** 3 - used, usedPercent: (used / (32 * 1024 ** 3)) * 100 },
        sys_disk: { size: disks[0]!.size, used: disks[0]!.size - disks[0]!.free, avail: disks[0]!.free },
      },
    })
  }],
  ['GET', /^\/v1\/users\/current$/, () => ok({ success: 200, data: { id: 1, username: 'Demo', role: 'admin' } })],
  ['GET', /^\/v1\/users\/current\/custom\/system$/, () => ok({ success: 200, data: {} })],
  ['GET', /^\/v1\/sys\/version$/, () => ok({ success: 200, data: { current_version: '1.8.0' } })],
  ['POST', /^\/v1\/users\/(login|refresh)$/, () => no(DEMO_ONLY)],

  // IceDash: system
  ['GET', /^\/v2\/icedash\/health$/, () => ok({ version: VERSION.replace(/^v/, ''), arch: 'amd64', started: new Date(Date.now() - 3 * 86400000).toISOString(), host: 'zimaos', host_name: 'ZimaOS', stock_url: '/' })],
  ['GET', /^\/v2\/icedash\/system$/, () => {
    const now = Date.now()
    const history = Array.from({ length: 30 }, (_, i) => {
      const s = (now / 2000 - (29 - i)) / 3
      return { time: new Date(now - (29 - i) * 2000).toISOString(), down: 2.2e6 + Math.sin(s) * 1.1e6 + Math.random() * 4e5, up: 3e5 + Math.cos(s * 1.7) * 1.2e5 }
    })
    return ok({ down: history.at(-1)!.down, up: history.at(-1)!.up, history, interfaces: ['eth0'], uptime_sec: 1_123_200 + Math.floor((now / 1000) % 86400), device: DEVICE })
  }],
  ['GET', /^\/v2\/icedash\/storage$/, () => ok(storagePools())],
  ['GET', /^\/v2\/icedash\/notifications$/, () => ok({ items: notes, unread: notes.filter((n) => !n.read).length })],
  ['POST', /^\/v2\/icedash\/notifications\/read$/, (_m, b) => {
    for (const n of notes) if (!b.id || n.id === b.id) n.read = true
    return ok({ items: notes, unread: notes.filter((n) => !n.read).length })
  }],
  ['POST', /^\/v2\/icedash\/notifications\/clear$/, () => {
    notes.length = 0
    return ok({ items: [], unread: 0 })
  }],
  ['GET', /^\/v2\/icedash\/update$/, () => ok({ current: VERSION.replace(/^v/, ''), latest: VERSION, state: 'idle', checked_at: new Date(Date.now() - 600000).toISOString(), steps: [] })],
  ['POST', /^\/v2\/icedash\/update\/check$/, () => ok({ current: VERSION.replace(/^v/, ''), latest: VERSION, state: 'idle', checked_at: new Date().toISOString(), steps: [] })],
  ['POST', /^\/v2\/icedash\/update\/apply$/, () => no(DEMO_ONLY)],
  ['GET', /^\/v2\/icedash\/settings$/, () => ok(settings)],
  ['POST', /^\/v2\/icedash\/settings$/, (_m, b) => ok((settings = { ...settings, ...b }))],
  ['GET', /^\/v2\/icedash\/prefs$/, () => ok(prefs)],
  ['POST', /^\/v2\/icedash\/prefs$/, (_m, b) => {
    prefs = { ...prefs, ...b }
    keep('prefs', prefs)
    return ok(prefs)
  }],
  ['POST', /^\/v2\/icedash\/me\/(wallpaper|avatar)(\/remove)?$/, (m) => (m[2] ? ok(prefs) : no('The demo doesn’t keep pictures. Pick a wallpaper or a drawn avatar instead.'))],
  ['GET', /^\/v2\/icedash\/users$/, () => ok({ items: people })],
  ['GET', /^\/v2\/icedash\/places$/, (_m, _b, q) => {
    const all = [
      { name: 'Auckland', region: 'Auckland', country: 'New Zealand', lat: -36.8485, lon: 174.7633 },
      { name: 'Queenstown', region: 'Otago', country: 'New Zealand', lat: -45.0312, lon: 168.6626 },
      { name: 'Berlin', region: 'Berlin', country: 'Germany', lat: 52.52, lon: 13.405 },
      { name: 'London', region: 'England', country: 'United Kingdom', lat: 51.5072, lon: -0.1276 },
      { name: 'Toronto', region: 'Ontario', country: 'Canada', lat: 43.6532, lon: -79.3832 },
      { name: 'Everett', region: 'Washington', country: 'United States', lat: 47.979, lon: -122.202 },
      { name: 'Shanghai', region: 'Shanghai', country: 'China', lat: 31.2304, lon: 121.4737 },
    ]
    const s = (q.get('q') ?? '').toLowerCase()
    return ok({ items: all.filter((p) => p.name.toLowerCase().startsWith(s)).map((p) => ({ ...p, label: `${p.name}, ${p.region}, ${p.country}` })) })
  }],

  // IceDash: apps
  ['GET', /^\/v2\/icedash\/apps$/, () => ok({
    items: apps.map((a) => ({
      name: a.name, store_id: `com.icestore.${a.name}`, icestore: true, version: a.version, latest: a.latest ?? a.version, update: !!a.latest, first_launch: !!a.firstLaunch,
      auto: !!a.auto, back: a.back, held: a.held, held_by: a.heldBy,
    })),
  })],
  ['GET', /^\/v2\/icedash\/jobs$/, () => ok({ items: [...jobs.values()].map(jobNow) })],
  ['GET', /^\/v2\/icedash\/apps\/([^/]+)\/frame$/, (m) => ok({ name: m[1], frameable: true })],
  ['GET', /^\/v2\/icedash\/apps\/([^/]+)\/logs$/, (m) => ok({ name: m[1], text: logsFor(apps.find((a) => a.name === m[1])?.title ?? m[1]!) })],
  ['GET', /^\/v2\/icedash\/apps\/([^/]+)\/credentials$/, (m) => {
    const a = apps.find((x) => x.name === m[1])
    return ok({
      fields: [
        { group: 'Web app', label: 'Address', value: `http://${SERVER_IP}:${a?.port ?? '80'}`, secret: false, url: true },
        { group: 'Web app', label: 'User', value: 'admin', secret: false, url: false },
        { group: 'Web app', label: 'Password', value: 'Vt4-q8Ln2xPw7Rd9', secret: true, url: false },
      ],
      notes: ['Read from the app’s settings when you open this; IceDash doesn’t store it.'],
    })
  }],
  ['GET', /^\/v2\/icedash\/apps\/([^/]+)\/first-launch$/, () => ok(firstLaunchFor)],
  ['POST', /^\/v2\/icedash\/apps\/([^/]+)\/first-launch\/dismiss$/, (m) => {
    const a = apps.find((x) => x.name === m[1])
    if (a) a.firstLaunch = false
    return ok({ name: m[1] })
  }],
  ['POST', /^\/v2\/icedash\/apps\/([^/]+)\/(start|stop|restart)$/, (m) => {
    const a = apps.find((x) => x.name === m[1])
    if (!a) return no('No such app.', 'not_found', 404)
    if (m[2] === 'stop') a.status = 'exited'
    else {
      a.status = 'restarting'
      settling.set(a.name, Date.now() + 3000)
    }
    return ok({ name: a.name })
  }],
  ['POST', /^\/v2\/icedash\/apps\/([^/]+)\/update$/, (m) => {
    const a = apps.find((x) => x.name === m[1])
    if (!a) return no('No such app.', 'not_found', 404)
    return ok(startJob('update', a, () => {
      a.back = a.version
      a.version = a.latest ?? a.version
      a.latest = undefined
      a.held = a.heldBy = undefined
      note({ kind: 'app_updated', app: a.name, title: `${a.title} is up to date`, body: `${a.title} ${a.version} is running.` })
    }))
  }],
  ['POST', /^\/v2\/icedash\/apps\/([^/]+)\/auto$/, (m, b) => {
    const a = apps.find((x) => x.name === m[1])
    if (!a) return no('No such app.', 'not_found', 404)
    const body = b as { on?: boolean; zone?: string }
    if (body.on && !body.zone) return no('updating at night needs the time zone it counts the night in', 'bad_zone', 400)
    a.auto = !!body.on
    if (body.on) settings = { ...settings, zone: body.zone ?? '' }
    return ok({ name: a.name, auto: a.auto })
  }],
  ['POST', /^\/v2\/icedash\/apps\/([^/]+)\/rollback$/, (m) => {
    const a = apps.find((x) => x.name === m[1])
    if (!a) return no('No such app.', 'not_found', 404)
    if (!a.back) return no('there’s no earlier version to go back to', 'no_previous', 404)
    return ok(startJob('rollback', a, () => {
      const from = a.version
      a.latest = from
      a.version = a.back ?? a.version
      a.back = undefined
      a.held = from
      a.heldBy = 'user'
      note({ kind: 'app_rolled_back', app: a.name, title: `${a.title} is back on ${a.version}`, body: `${from} won’t install by itself again.` })
    }))
  }],
  ['POST', /^\/v2\/icedash\/apps\/([^/]+)\/uninstall$/, (m, b) => {
    const i = apps.findIndex((x) => x.name === m[1])
    if (i < 0) return no('No such app.', 'not_found', 404)
    apps.splice(i, 1)
    jobs.delete(m[1]!)
    return ok({ name: m[1], data_kept: !b.delete_data, data: `/DATA/AppData/${m[1]}` })
  }],
  ['GET', /^\/v2\/icedash\/apps\/([^/]+)\/backup$/, () => no(DEMO_ONLY)],

  // IceDash: IceStore
  ['GET', /^\/v2\/icedash\/catalog$/, async () => ok(await catalog())],
  ['POST', /^\/v2\/icedash\/catalog\/refresh$/, async () => ok(await catalog())],
  ['GET', /^\/v2\/icedash\/catalog\/([^/]+)\/setup$/, async (m) => {
    const v = await setupView(decodeURIComponent(m[1]!))
    return v ? ok(v) : no('This app isn’t in IceStore.', 'not_found', 404)
  }],
  ['POST', /^\/v2\/icedash\/catalog\/([^/]+)\/check$/, (_m, b) => {
    const ports = Object.entries(((b as unknown as Choices).ports ?? {}) as Record<string, string>)
    const busy = ports.filter(([, p]) => apps.some((a) => a.port === p))
    return ok(busy.length
      ? { ok: false, ports_in_use: busy.map(([, p]) => p), suggest: Object.fromEntries(busy.map(([k, p]) => [k, freePort(k + p)])) }
      : { ok: true, ports_in_use: [], suggest: {} })
  }],
  ['POST', /^\/v2\/icedash\/catalog\/([^/]+)\/install$/, async (m, b) => {
    const app = await findApp(decodeURIComponent(m[1]!))
    if (!app) return no('This app isn’t in IceStore.', 'not_found', 404)
    if (apps.some((a) => a.name === app.name)) return no(`${app.title} is installed already.`, 'installed')
    const port = Object.values(((b as unknown as Choices).ports ?? {}) as Record<string, string>)[0] ?? freePort(app.name)
    const a: DemoApp = { name: app.name, title: app.title, port, status: 'running', version: app.version || 'latest' }
    return ok(startJob('install', a, () => {
      apps.push(a)
      note({ kind: 'app_installed', app: a.name, title: `${a.title} is ready`, body: `${a.title} is installed and starting.`, action: { label: 'Open', kind: 'open_app' } })
    }))
  }],
  ['GET', /^\/v2\/icedash\/catalog\/([^/]+)$/, async (m) => {
    const app = await findApp(decodeURIComponent(m[1]!))
    if (!app) return no('This app isn’t in IceStore.', 'not_found', 404)
    const meta = (await (await fetch((app as unknown as Record<string, string>).meta_url!)).json()) as Record<string, unknown>
    meta.screenshot_link = ((meta.screenshot_link as string[]) ?? []).map((u) => (u.startsWith('/') ? STORE + u : u))
    return ok({ app, meta })
  }],

  // IceDash: Files
  ['GET', /^\/v2\/icedash\/files\/disks$/, () => ok(disks)],
  ['GET', /^\/v2\/icedash\/files\/list$/, (_m, _b, q) => {
    const path = q.get('path') || '/'
    const list = childrenAt(path)
    return list ? ok({ path, protected: false, entries: list.map((n) => n.entry), hidden: 0 }) : no('This folder doesn’t exist.', 'not_found', 404)
  }],
  ['GET', /^\/v2\/icedash\/files\/search$/, (_m, _b, q) => {
    const s = (q.get('q') ?? '').toLowerCase()
    const path = q.get('path') || '/'
    const found: Entry[] = []
    walk(childrenAt(path) ?? [], (e) => e.name.toLowerCase().includes(s) && found.push(e))
    return ok({ query: s, path, entries: found.slice(0, 200), more: found.length > 200 })
  }],
  ['POST', /^\/v2\/icedash\/files\/folder$/, (_m, b) => {
    const list = childrenAt(String(b.path))
    if (!list) return no('This folder doesn’t exist.', 'not_found', 404)
    const name = String(b.name)
    if (list.some((n) => n.entry.name === name)) return no(`${name} is there already.`, 'exists')
    const entry: Entry = { name, path: join(String(b.path), name), dir: true, size: 0, modified: new Date().toISOString(), mode: 'rwxr-xr-x', kind: 'folder' }
    list.push({ entry, children: [] })
    return ok(entry)
  }],
  ['POST', /^\/v2\/icedash\/files\/rename$/, (_m, b) => {
    const f = findNode(String(b.path))
    if (!f) return no('This file doesn’t exist.', 'not_found', 404)
    f.list[f.i] = rebase(f.node, join(parentOf(String(b.path)), String(b.name)))
    return ok(f.list[f.i]!.entry)
  }],
  ['POST', /^\/v2\/icedash\/files\/delete$/, (_m, b) => {
    let deleted = 0
    for (const p of b.paths as string[]) {
      const f = findNode(p)
      if (!f) continue
      f.list.splice(f.i, 1)
      deleted++
      if (rootOf(p) === '/')
        trash.unshift({ id: `t${Date.now()}${deleted}`, name: f.node.entry.name, from: p, dir: f.node.entry.dir, size: f.node.entry.size, deleted: new Date().toISOString(), kind: f.node.entry.kind, node: f.node, parent: parentOf(p) })
    }
    return ok({ deleted })
  }],
  ['POST', /^\/v2\/icedash\/files\/(move|copy)$/, (m, b) => {
    const to = String(b.to)
    const dest = childrenAt(to)
    if (!dest) return no('This folder doesn’t exist.', 'not_found', 404)
    const paths = b.paths as string[]
    for (const p of paths) {
      const f = findNode(p)
      if (!f) continue
      let name = f.node.entry.name
      while (dest.some((n) => n.entry.name === name)) name = name.replace(/(\.[^.]*)?$/, ' copy$1')
      dest.push(rebase(f.node, join(to, name)))
      if (m[1] === 'move') f.list.splice(f.list.indexOf(f.node), 1)
    }
    const now = new Date().toISOString()
    const job = { id: `f${Date.now()}`, kind: m[1] as 'move' | 'copy', to, items: paths.length, total: paths.length, done: paths.length, state: 'done' as const, started: now, ended: now }
    fileJobs = [job]
    return ok(job)
  }],
  ['GET', /^\/v2\/icedash\/files\/jobs$/, () => ok({ items: fileJobs })],
  ['POST', /^\/v2\/icedash\/files\/jobs\/([^/]+)\/cancel$/, () => ok({})],
  ['GET', /^\/v2\/icedash\/files\/trash$/, () => ok({ items: trash.map(({ node: _n, parent: _p, ...t }) => t) })],
  ['POST', /^\/v2\/icedash\/files\/trash\/restore$/, (_m, b) => {
    const restored: Entry[] = []
    for (const id of b.ids as string[]) {
      const i = trash.findIndex((t) => t.id === id)
      if (i < 0) continue
      const [t] = trash.splice(i, 1)
      childrenAt(t!.parent)?.push(t!.node)
      restored.push(t!.node.entry)
    }
    return ok({ restored })
  }],
  ['POST', /^\/v2\/icedash\/files\/trash\/delete$/, (_m, b) => {
    for (const id of b.ids as string[]) {
      const i = trash.findIndex((t) => t.id === id)
      if (i > -1) trash.splice(i, 1)
    }
    return ok({})
  }],
  ['POST', /^\/v2\/icedash\/files\/trash\/empty$/, () => {
    trash.length = 0
    return ok({})
  }],
  ['POST', /^\/v2\/icedash\/files\/ticket$/, () => no('The demo’s files are only names: there is nothing inside them to open or download.')],
  ['POST', /^\/v2\/icedash\/terminal\/ticket$/, () => ok({ ticket: 'demo' })],

  // IceDash: mods
  ['GET', /^\/v2\/icedash\/mods$/, () => modView()],
  ['GET', /^\/v2\/icedash\/mods\/collections$/, async () => ok({ collections: await collections() })],
  ['POST', /^\/v2\/icedash\/mods\/collections(\/remove)?$/, () => no('The demo shows the official collections only. On your own server you can add any collection from GitHub.')],
  ['POST', /^\/v2\/icedash\/mods\/install$/, async (_m, b) => {
    const it = (await collections()).flatMap((c) => c.items).find((i) => i.id === b.id)
    if (!it) return no('This mod isn’t in a collection.', 'not_found', 404)
    const m: Installed = { ...it, commit: it.version, enabled: true, installed: new Date().toISOString() }
    installed = [...installed.filter((x) => x.id !== m.id), m]
    modView()
    return ok(m)
  }],
  ['POST', /^\/v2\/icedash\/mods\/theme$/, (_m, b) => {
    modPrefs = { ...modPrefs, theme: String(b.id ?? '') }
    return modView()
  }],
  ['POST', /^\/v2\/icedash\/mods\/panel$/, (_m, b) => {
    modPrefs = { ...modPrefs, panel: (b.ids as string[]) ?? [] }
    return modView()
  }],
  ['POST', /^\/v2\/icedash\/mods\/([^/]+)\/remove$/, (m) => {
    installed = installed.filter((x) => x.id !== m[1])
    if (modPrefs.theme === m[1]) modPrefs.theme = ''
    modPrefs.panel = modPrefs.panel.filter((x) => x !== m[1])
    return modView()
  }],
  ['POST', /^\/v2\/icedash\/mods\/([^/]+)\/enabled$/, (m, b) => {
    installed = installed.map((x) => (x.id === m[1] ? { ...x, enabled: !!b.enabled } : x))
    return modView()
  }],
]

/** The demo's answer to an API call, or null to let it through (a mod's files, anything else). */
export async function answer(method: string, url: URL, body: Record<string, unknown>): Promise<Reply | null> {
  const path = url.pathname
  if (!/^\/v[12]\//.test(path)) return null
  if (/^\/v2\/icedash\/mods\/[^/]+\/(files\/|wallpaper$)/.test(path)) return null
  for (const [m, re, h] of routes) {
    const hit = path.match(re)
    if (hit && m === method) return h(hit, body ?? {}, url.searchParams)
  }
  return no(DEMO_ONLY, 'demo', 404)
}
