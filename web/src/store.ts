// The desktop's shared state and its polling. One instance per page.
import { computed, reactive, ref } from 'vue'
import { ApiError, token } from './api/client'
import * as ice from './api/icedash'
import * as zima from './api/zimaos'
import { toApps, type App } from './lib/apps'
import { poll, type Poller } from './lib/poll'
import { pickZone } from './lib/timezone'
import { strings } from './strings'

export type SessionState = 'loading' | 'ok' | 'signin' | 'ended'

export interface Toast {
  id: number
  title: string
  body: string
  action?: { label: string; kind: string }
}

export type WindowId = 'about' | 'notifications'

export interface Win {
  id: WindowId
  title: string
  z: number
}

/** The signed-in user, read from the ZimaOS token (the backend checks it; this only picks what to show). */
function userFromToken(t: string): { name: string; admin: boolean } | null {
  const part = t.split('.')[1]
  if (!part) return null
  try {
    const json = atob(part.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(part.length / 4) * 4, '='))
    const p = JSON.parse(json) as { username?: string; role?: string }
    return p.username ? { name: p.username, admin: p.role === 'admin' } : null
  } catch {
    return null
  }
}

export const session = ref<SessionState>('loading')
export const offline = ref(false)
export const user = ref<{ name: string; admin: boolean } | null>(null)
export const version = ref('')

export const apps = ref<App[]>([])
export const appsLoaded = ref(false)
export const util = ref<zima.Utilization | null>(null)
export const sys = ref<ice.SystemInfo | null>(null)
export const zone = ref(pickZone(undefined))
export const terminalOk = ref(false)
export const iceStoreInZimaOS = ref(false)

export const update = ref<ice.UpdateStatus | null>(null)
export const autoCheck = ref<boolean | null>(null)
export const notes = ref<ice.Notification[]>([])
export const unread = ref(0)
export const toasts = ref<Toast[]>([])

export const windows = reactive<Win[]>([])
export const panelOpen = ref(true)
export const searchOpen = ref(false)

export const updateWaiting = computed(() => update.value?.state === 'available' || update.value?.state === 'failed' && !!update.value.latest)
export const activeWindow = computed(() => [...windows].sort((a, b) => b.z - a.z)[0])

let topZ = 10
export function openWindow(id: WindowId) {
  const title = id === 'about' ? strings.aboutTitle : strings.notifications
  const w = windows.find((x) => x.id === id)
  if (w) w.z = ++topZ
  else windows.push({ id, title, z: ++topZ })
}
export function focusWindow(id: WindowId) {
  const w = windows.find((x) => x.id === id)
  if (w) w.z = ++topZ
}
export function closeWindow(id: WindowId) {
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
    offline.value = e.code === 'unreachable'
  }
}

async function loadApps() {
  try {
    apps.value = toApps(await zima.appGrid(), location.hostname)
    appsLoaded.value = true
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
async function loadNotes() {
  try {
    const list = await ice.notifications()
    const fresh = list.items.filter((n) => n.id > lastNoteId && !n.read)
    if (notesLoaded) {
      for (const n of fresh.reverse()) toast({ ...noteText(n), action: n.action })
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
    if (update.value?.state === 'installing') return
    handle(e)
  }
}

/** After Update now: wait for the new version to answer, then reload (PLAN.md, step 8). */
export async function waitForNewVersion(from: string) {
  if (reloading) return
  reloading = true
  const deadline = Date.now() + 5 * 60 * 1000
  while (Date.now() < deadline) {
    await new Promise((r) => setTimeout(r, 2000))
    try {
      const h = await ice.health()
      if (h.version !== from) {
        location.reload()
        return
      }
      // Still the old version: if the updater says it failed before the swap, stop waiting.
      const st = await ice.updateStatus()
      update.value = st
      if (st.state === 'failed') break
    } catch {
      // restarting
    }
  }
  reloading = false
}

export async function refreshUpdate() {
  await loadUpdate()
}

/**
 * Update now: opens the Updates window, starts the install and, once IceDash
 * restarts, reloads on the new version. Returns an error message to show, if any.
 */
export async function startUpdate(): Promise<string | undefined> {
  openWindow('about')
  if (!user.value?.admin) return strings.adminOnly
  const from = update.value?.current ?? version.value
  try {
    update.value = await ice.applyUpdate()
    updatePoller?.every(1000)
    void waitForNewVersion(from)
    return undefined
  } catch (e) {
    if (e instanceof ApiError && e.code === 'admin_only') return strings.adminOnly
    await loadUpdate()
    return e instanceof Error ? e.message : String(e)
  }
}

async function probeTerminal() {
  // ttyd answers on its own port; a no-cors request tells only whether it's there.
  try {
    await fetch(`${location.protocol}//${location.hostname}:7681/`, { mode: 'no-cors', cache: 'no-store' })
    terminalOk.value = true
  } catch {
    terminalOk.value = false
  }
}

const pollers: Poller[] = []

export async function start() {
  const t = token()
  user.value = t ? userFromToken(t) : null
  if (!t || !user.value) {
    session.value = 'signin'
    return
  }
  session.value = 'ok'

  ice.health().then((h) => (version.value = h.version)).catch(handle)
  zima.customSystem().then((c) => (zone.value = pickZone(c.timezone))).catch(() => undefined)
  zima.iceStoreAdded().then((v) => (iceStoreInZimaOS.value = v)).catch(() => undefined)
  ice.getSettings().then((s) => (autoCheck.value = s.auto_check)).catch(() => undefined)
  probeTerminal()

  pollers.push(poll(loadStats, 5000))
  pollers.push(poll(loadApps, 15000))
  pollers.push(poll(loadNotes, 30000))
  updatePoller = poll(loadUpdate, 30000)
  pollers.push(updatePoller)
  window.addEventListener('focus', onFocus)
}

function onFocus() {
  loadNotes()
}

export function stop() {
  for (const p of pollers.splice(0)) p.stop()
  window.removeEventListener('focus', onFocus)
}
