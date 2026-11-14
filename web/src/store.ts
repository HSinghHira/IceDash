// The desktop's shared state and its polling. One instance per page.
import { computed, reactive, ref } from 'vue'
import { ApiError, keepStockVersion, token } from './api/client'
import * as appsApi from './api/apps'
import * as modsApi from './api/mods'
import * as ice from './api/icedash'
import * as zima from './api/zimaos'
import { toApps, type App } from './lib/apps'
import { host, setHost } from './lib/host'
import { applyTheme } from './lib/theme'
import { poll, type Poller } from './lib/poll'
import { pickZone } from './lib/timezone'
import { strings } from './strings'

export type SessionState = 'loading' | 'ok' | 'signin' | 'ended'

export interface Toast {
  id: number
  title: string
  body: string
  action?: { label: string; kind: string }
  app?: string
}

export type WindowKind = 'settings' | 'notifications' | 'icestore' | 'logs' | 'credentials' | 'app' | 'files' | 'terminal'

export interface Win {
  /** The kind, or kind:app for windows about one app. */
  id: string
  kind: WindowKind
  title: string
  z: number
  app?: string
  /** Folded to its title bar, or filling the desktop. */
  min?: boolean
  max?: boolean
}

/** The signed-in user, read from the ZimaOS token (the backend checks it; this only picks what to show). */
function userFromToken(t: string): { name: string; admin: boolean } | null {
  const part = t.split('.')[1]
  if (!part) return null
  try {
    const json = atob(part.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(part.length / 4) * 4, '='))
    const p = JSON.parse(json) as { username?: string; role?: string }
    // CasaOS tokens have no role: its one account is the admin (measured on 0.4.15).
    return p.username ? { name: p.username, admin: p.role === 'admin' || host.id === 'casaos' } : null
  } catch {
    return null
  }
}

export const session = ref<SessionState>('loading')
export const offline = ref(false)
/** True from Update now until the page reloads on the new version or the old one is back. */
export const updating = ref(false)
export const user = ref<{ name: string; admin: boolean } | null>(null)
export const version = ref('')

export const apps = ref<App[]>([])
export const appsLoaded = ref(false)
export const util = ref<zima.Utilization | null>(null)
export const sys = ref<ice.SystemInfo | null>(null)
export const zone = ref(pickZone(undefined))
export const iceStoreInZimaOS = ref(false)

export const update = ref<ice.UpdateStatus | null>(null)
export const autoCheck = ref<boolean | null>(null)
export const notes = ref<ice.Notification[]>([])
export const unread = ref(0)
export const toasts = ref<Toast[]>([])

export const windows = reactive<Win[]>([])
export const panelOpen = ref(true)
/** The notification centre under the bell. */
export const notesOpen = ref(false)
export const searchOpen = ref(false)

export const updateWaiting = computed(() => update.value?.state === 'available' || update.value?.state === 'failed' && !!update.value.latest)
/** The front window that isn't minimized. */
export const activeWindow = computed(() => [...windows].filter((w) => !w.min).sort((a, b) => b.z - a.z)[0])

let topZ = 10
const windowTitles: Record<WindowKind, (app: string) => string> = {
  settings: () => strings.settingsTitle,
  notifications: () => strings.notifications,
  icestore: () => strings.iceStoreTitle,
  logs: (a) => strings.logsTitle(a),
  credentials: (a) => `${strings.credentials} · ${a}`,
  app: (a) => a,
  files: () => strings.filesTitle,
  terminal: (a) => a,
}
/** Opens (or raises) a window; logs and credentials windows are per app. */
export function openWindow(kind: WindowKind, app?: { name: string; title: string }) {
  const id = app ? `${kind}:${app.name}` : kind
  const w = windows.find((x) => x.id === id)
  if (w) w.z = ++topZ
  else windows.push({ id, kind, title: windowTitles[kind](app?.title ?? ''), z: ++topZ, app: app?.name })
}
export function focusWindow(id: string) {
  const w = windows.find((x) => x.id === id)
  if (w) w.z = ++topZ
}
export function toggleMin(id: string) {
  const w = windows.find((x) => x.id === id)
  if (!w) return
  w.min = !w.min
  if (!w.min) w.z = ++topZ // back in front
}
export function toggleMax(id: string) {
  const w = windows.find((x) => x.id === id)
  if (w) {
    w.max = !w.max
    w.min = false
  }
}
export function closeWindow(id: string) {
  const i = windows.findIndex((x) => x.id === id)
  if (i >= 0) windows.splice(i, 1)
}

let toastId = 0
export function toast(t: Omit<Toast, 'id'>) {
  const id = ++toastId
  toasts.value.push({ ...t, id })
  setTimeout(() => dismissToast(id), 6000)
}
export function dismissToast(id: number) {
  toasts.value = toasts.value.filter((t) => t.id !== id)
}

/** What a notification says, in the UI's words when its kind is known. */
export function noteText(n: Pick<ice.Notification, 'kind' | 'title' | 'body' | 'version'>) {
  const title = strings.notificationTitle[n.kind] ?? n.title
  const body = n.version && strings.notificationBody[n.kind] ? strings.notificationBody[n.kind]!(n.version) : n.body
  return { title, body }
}

function handle(e: unknown) {
  if (e instanceof ApiError) {
    if (e.status === 401) session.value = 'ended'
    offline.value = e.code === 'unreachable' && !updating.value
  }
}

async function loadApps() {
  try {
    apps.value = toApps(await zima.appGrid(), location.hostname)
    appsLoaded.value = true
    void loadFrames()
  } catch (e) {
    handle(e)
  }
}

async function loadStats() {
  const [u, s] = await Promise.allSettled([zima.utilization(), ice.system()])
  if (u.status === 'fulfilled') util.value = u.value
  else handle(u.reason)
  if (s.status === 'fulfilled') {
    sys.value = s.value
    offline.value = false
  } else handle(s.reason)
}

let lastNoteId = 0
let notesLoaded = false

// Update now reloads the page on the new version. The newest notification id
// seen before the reload is kept for that one reload, so what arrived meanwhile
// ("Updated to …", "Update failed …") still shows as a toast.
const seenKey = 'icedash-seen-note'
function keepSeen() {
  try {
    sessionStorage.setItem(seenKey, String(lastNoteId))
  } catch {
    // no storage: the result still shows in the bell
  }
}
function takeSeen() {
  try {
    const v = sessionStorage.getItem(seenKey)
    sessionStorage.removeItem(seenKey)
    const n = v === null ? NaN : Number(v)
    if (Number.isFinite(n)) {
      lastNoteId = n
      notesLoaded = true
    }
  } catch {
    // no storage
  }
}

export async function loadNotes() {
  try {
    const list = await ice.notifications()
    const fresh = list.items.filter((n) => n.id > lastNoteId && !n.read)
    if (notesLoaded) {
      for (const n of fresh.reverse()) toast({ ...noteText(n), action: n.action, app: n.app })
    }
    notes.value = list.items
    unread.value = list.unread
    lastNoteId = Math.max(lastNoteId, ...list.items.map((n) => n.id))
    notesLoaded = true
  } catch (e) {
    handle(e)
  }
}

export function setNotes(list: ice.NotificationList) {
  notes.value = list.items
  unread.value = list.unread
}

let updatePoller: Poller | null = null
let reloading = false
async function loadUpdate() {
  try {
    const st = await ice.updateStatus()
    update.value = st
    updatePoller?.every(st.state === 'installing' || st.state === 'checking' ? 1000 : 30000)
  } catch (e) {
    // While an update installs, IceDash restarts and isn't there for a moment.
    if (update.value?.state === 'installing' || updating.value) return
    handle(e)
  }
}

/**
 * After Update now: wait for the new version to answer, then reload (PLAN.md, step 8).
 * If the same version answers again from a new process, the swap rolled back:
 * stop waiting and show the result.
 */
export async function waitForNewVersion(from: string, startedBefore?: string) {
  if (reloading) return
  reloading = true
  updating.value = true
  const deadline = Date.now() + 5 * 60 * 1000
  while (Date.now() < deadline) {
    await new Promise((r) => setTimeout(r, 2000))
    try {
      const h = await ice.health()
      offline.value = false
      if (h.version !== from) {
        keepSeen()
        location.reload()
        return
      }
      const st = await ice.updateStatus()
      update.value = st
      // Failed before the swap, or the previous version is back after it.
      if (st.state === 'failed' || (startedBefore && h.started !== startedBefore && st.state !== 'installing')) break
    } catch {
      // restarting
    }
  }
  reloading = false
  updating.value = false
  // The result notification may land a moment after the old version is back.
  await refreshUpdate()
  setTimeout(loadNotes, 3000)
}

/** After a check: the new status, and any notification the check raised. */
export async function refreshUpdate() {
  await Promise.all([loadUpdate(), loadNotes()])
}

/**
 * Update now: opens the Updates window, starts the install and, once IceDash
 * restarts, reloads on the new version. Returns an error message to show, if any.
 */
export async function startUpdate(): Promise<string | undefined> {
  openSettings('about')
  if (!user.value?.admin) return strings.adminOnly
  const from = update.value?.current ?? version.value
  const before = await ice.health().catch(() => undefined)
  try {
    update.value = await ice.applyUpdate()
    updatePoller?.every(1000)
    void waitForNewVersion(from, before?.started)
    return undefined
  } catch (e) {
    if (e instanceof ApiError && e.code === 'admin_only') return strings.adminOnly
    await loadUpdate()
    return e instanceof Error ? e.message : String(e)
  }
}

// ---- Phase 2: IceStore inside IceDash ----

/** What the backend knows about each installed app, by app name. */
export const appInfo = ref<Record<string, appsApi.AppInfo>>({})
export const jobs = ref<appsApi.Job[]>([])
export const catalogList = ref<appsApi.CatalogList | null>(null)
export const catalogError = ref('')
/** The IceStore window's current list: all, installed, updates, or a category. */
export const iceStoreView = ref<string>('all')
/** Open popups: setup (a catalog id), first launch and uninstall (app names). */
export const setupFor = ref<string | null>(null)
export const firstLaunchFor = ref<string | null>(null)
export const uninstallFor = ref<string | null>(null)
/** The right-click menu: which app, where. */
export const menu = ref<{ name: string; x: number; y: number } | null>(null)

export const appUpdates = computed(() => Object.values(appInfo.value).filter((a) => a.update))
export const updateCount = computed(() => (updateWaiting.value ? 1 : 0) + appUpdates.value.length)

async function loadAppInfo() {
  try {
    const list = await appsApi.appInfo()
    const next: Record<string, appsApi.AppInfo> = {}
    for (const a of list.items) next[a.name] = a
    appInfo.value = next
    void loadFrames()
  } catch (e) {
    handle(e)
  }
}

let jobsPoller: Poller | null = null
async function loadJobs() {
  try {
    const before = new Set(jobs.value.filter((j) => j.state === 'running').map((j) => j.name))
    jobs.value = (await appsApi.jobs()).items
    const running = jobs.value.some((j) => j.state === 'running')
    jobsPoller?.every(running ? 2000 : 20000)
    // A job just ended: show the app and its notification now.
    if (jobs.value.some((j) => before.has(j.name) && j.state !== 'running')) {
      void loadApps()
      void loadAppInfo()
      void loadNotes()
    }
  } catch (e) {
    handle(e)
  }
}

/** After starting an install or update: watch it closely. */
export function trackJob(j: appsApi.Job) {
  jobs.value = [j, ...jobs.value.filter((x) => x.name !== j.name)]
  jobsPoller?.every(2000)
}

export const jobFor = (name: string) => jobs.value.find((j) => j.name === name)

export async function loadCatalog(force = false) {
  try {
    const c = force ? await appsApi.refreshCatalog() : await appsApi.catalog()
    catalogList.value = c
    catalogError.value = c.error ?? ''
  } catch (e) {
    catalogError.value = e instanceof Error ? e.message : String(e)
    handle(e)
  }
}

// ---- Phase 4: Settings, themes and widgets ----
export type SettingsPage = 'mods' | 'widgets' | 'about'
export const settingsPage = ref<SettingsPage>('mods')
/** Opens Settings on a page (the dock's Settings, the IceDash menu's About). */
export function openSettings(page: SettingsPage = 'mods') {
  settingsPage.value = page
  openWindow('settings')
}

export const modsView = ref<modsApi.View>({ mods: [], prefs: { theme: '', panel: [] } })
export const activeTheme = computed(() => {
  const m = modsView.value.mods.find((x) => x.kind === 'theme' && x.id === modsView.value.prefs.theme)
  return m ? modsApi.themeOf(m) : null
})
/** The user's widgets on the panel, top to bottom: installed and switched on. */
export const panelWidgets = computed(() =>
  modsView.value.prefs.panel
    .map((id) => modsView.value.mods.find((m) => m.id === id && m.kind === 'widget' && m.enabled))
    .filter((m): m is modsApi.Installed => !!m),
)
const THEME_KEY = 'icedash.theme'
/** Takes a fresh view of the mods and applies the user's theme. */
export function setModsView(v: modsApi.View) {
  modsView.value = v
  const t = activeTheme.value
  applyTheme(t)
  try {
    if (t) localStorage.setItem(THEME_KEY, JSON.stringify(t))
    else localStorage.removeItem(THEME_KEY)
  } catch {
    // no storage: the theme still applies once the mods load
  }
}
export async function loadMods() {
  try {
    setModsView(await modsApi.list())
  } catch (e) {
    handle(e)
  }
}
/** The last theme, applied before anything loads so a reload doesn't flash Ice. */
export function applyRememberedTheme() {
  try {
    const raw = localStorage.getItem(THEME_KEY)
    if (raw) applyTheme(JSON.parse(raw))
  } catch {
    // nothing remembered
  }
}

/** The Files window, on a folder or its Trash (the dock's Trash opens that). */
export const filesView = ref<{ view: 'files' | 'trash'; path: string; seq: number }>({ view: 'files', path: '/', seq: 0 })
export function openFiles(view: 'files' | 'trash' = 'files', path = '/') {
  filesView.value = { view, path, seq: filesView.value.seq + 1 }
  openWindow('files')
}

/** A new Terminal window each time; each one asks for the password. */
let terminals = 0
export function openTerminal() {
  terminals++
  openWindow('terminal', { name: String(terminals), title: terminals > 1 ? `${strings.terminal} ${terminals}` : strings.terminal })
}

export function openIceStore(view = 'all') {
  iceStoreView.value = view
  openWindow('icestore')
  if (!catalogList.value) void loadCatalog()
}

/** Opens an app; a freshly installed IceStore app shows its welcome popup first. */
/**
 * Opens an app: a freshly installed IceStore app shows its welcome popup
 * first; then it opens in a window, or in a new tab when it refuses to be
 * shown inside another page (or newTab is set).
 */
export function openApp(a: App, newTab = false) {
  if (appInfo.value[a.id]?.first_launch) {
    firstLaunchFor.value = a.id
    return
  }
  launchApp(a, newTab)
}

/** Opens an app without the welcome popup. */
export function launchApp(a: App, newTab = false) {
  if (!a.url) return
  if (newTab || frameable.value[a.id] === false) {
    window.open(a.url, '_blank', 'noopener')
    return
  }
  openWindow('app', { name: a.id, title: a.name })
}

/** Per app: may it show in a window? Unknown counts as yes. */
export const frameable = ref<Record<string, boolean>>({})
async function loadFrames() {
  for (const a of apps.value) {
    if (!a.url || a.id in frameable.value) continue
    try {
      frameable.value[a.id] = (await appsApi.frame(a.id)).frameable
    } catch {
      // ask again next time
    }
  }
}

/**
 * A notification's button: "update" is IceDash's Update now; the app kinds act
 * on the notification's app. Returns an error to show, if any.
 */
export async function runNoteAction(kind: string, app?: string): Promise<string | undefined> {
  if (kind === 'update') return startUpdate()
  if (!app) return undefined
  const grid = apps.value.find((a) => a.id === app)
  try {
    if (kind === 'open_app' && grid) openApp(grid)
    else if (kind === 'app_update') trackJob(await appsApi.updateApp(app))
    else if (kind === 'start_app') {
      await appsApi.appAction(app, 'start')
      setTimeout(refreshApps, 2000)
    }
  } catch (e) {
    return e instanceof Error ? e.message : String(e)
  }
  return undefined
}

/** The right-click menu at the pointer, or under the element when opened by keyboard. */
export function openMenu(name: string, e: MouseEvent | KeyboardEvent) {
  let x = 'clientX' in e ? e.clientX : 0
  let y = 'clientY' in e ? e.clientY : 0
  if (!x && !y && e.currentTarget instanceof HTMLElement) {
    const r = e.currentTarget.getBoundingClientRect()
    x = r.left + r.width / 2
    y = r.bottom
  }
  menu.value = { name, x, y }
}

export function refreshApps() {
  void loadApps()
  void loadAppInfo()
}

const pollers: Poller[] = []

export async function start() {
  // The system's name and stock dashboard address, needed even on the sign-in
  // screen, and before reading the token (admin rules differ per system).
  await ice.health().then(setHost).catch(() => undefined)
  const t = token()
  user.value = t ? userFromToken(t) : null
  if (!t || !user.value) {
    session.value = 'signin'
    return
  }
  session.value = 'ok'
  takeSeen()
  if (host.id === 'casaos') void keepStockVersion()

  ice.health().then((h) => (version.value = h.version)).catch(handle)
  zima.customSystem().then((c) => (zone.value = pickZone(c.timezone))).catch(() => undefined)
  zima.iceStoreAdded().then((v) => (iceStoreInZimaOS.value = v)).catch(() => undefined)
  ice.getSettings().then((s) => (autoCheck.value = s.auto_check)).catch(() => undefined)
  void loadMods()

  pollers.push(poll(loadStats, 5000))
  pollers.push(poll(loadApps, 15000))
  pollers.push(poll(loadAppInfo, 15000))
  jobsPoller = poll(loadJobs, 20000)
  pollers.push(jobsPoller)
  pollers.push(poll(loadNotes, 30000))
  // Right after an update the new version may raise its result a moment after the page loads.
  if (notesLoaded) for (const ms of [3000, 10000]) setTimeout(loadNotes, ms)
  updatePoller = poll(loadUpdate, 30000)
  pollers.push(updatePoller)
  window.addEventListener('focus', onFocus)
}

function onFocus() {
  loadNotes()
}

export function stop() {
  for (const p of pollers.splice(0)) p.stop()
  frameable.value = {}
  windows.splice(0)
  window.removeEventListener('focus', onFocus)
}
