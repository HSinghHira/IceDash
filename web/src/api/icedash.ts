// IceDash's own backend under /v2/icedash (PLAN.md, "Backend API").
import { fetchBlob, request } from './client'

const base = '/v2/icedash'

export interface Health {
  version: string
  arch: string
  started: string
  host?: string
  host_name?: string
  stock_url?: string
}

export const health = () => request<Health>(`${base}/health`)

export interface SpeedPoint {
  time: string
  down: number
  up: number
}

export interface SystemInfo {
  down: number
  up: number
  history: SpeedPoint[]
  interfaces: string[]
  uptime_sec: number
  device: string
}

export const system = () => request<SystemInfo>(`${base}/system`)

export interface Notification {
  id: number
  time: string
  kind: string
  version?: string
  /** Phase 2: the app an app event is about. */
  app?: string
  title: string
  body: string
  action?: { label: string; kind: string }
  read: boolean
}

export interface NotificationList {
  items: Notification[]
  unread: number
}

export const notifications = (since = 0) => request<NotificationList>(`${base}/notifications?since=${since}`)
export const markRead = (id = 0) => request<NotificationList>(`${base}/notifications/read`, { method: 'POST', body: { id } })
export const clearNotifications = () => request<NotificationList>(`${base}/notifications/clear`, { method: 'POST' })

export interface UpdateStep {
  id: string
  text: string
  state: 'running' | 'done' | 'failed'
}

export interface UpdateStatus {
  current: string
  latest?: string
  notes?: string
  state: 'idle' | 'checking' | 'available' | 'installing' | 'failed'
  error?: string
  checked_at?: string
  steps: UpdateStep[]
}

export const updateStatus = () => request<UpdateStatus>(`${base}/update`)
export const checkUpdate = () => request<UpdateStatus>(`${base}/update/check`, { method: 'POST' })
export const applyUpdate = () => request<UpdateStatus>(`${base}/update/apply`, { method: 'POST' })

export interface Settings {
  auto_check: boolean
  /** Phase 5a: install a waiting update between 02:00 and 05:00 in zone. */
  auto_install?: boolean
  zone?: string
}

export const getSettings = () => request<Settings>(`${base}/settings`)
export const saveSettings = (s: Partial<Settings>) => request<Settings>(`${base}/settings`, { method: 'POST', body: s })

// ---- Phase 5a: each user's own choices, and the accounts ----

export type NotifyGroup = 'updates' | 'apps' | 'stopped'
export type DockLauncher = 'files' | 'icestore' | 'appstore' | 'terminal'

export interface Prefs {
  clock: '' | '12' | '24'
  hide_panel: boolean
  motion: '' | 'reduce' | 'full'
  dock: { hidden: DockLauncher[]; pinned: string[] }
  notify: { off: NotifyGroup[]; quiet: boolean }
  /** The user's own desktop background. */
  wallpaper: Wallpaper
  /** Changes with each new picture; 0 is none. */
  avatar: number
  /** Without a picture: the drawn style ("" is Critters, "initials" the letters) and its seed ("" is the name). */
  avatar_style: string
  avatar_seed: string
}

export type WallpaperPreset = 'aurora' | 'glacier' | 'nightfall' | 'ember' | 'lagoon' | 'dune' | 'graphite' | 'mist'
export interface Wallpaper {
  kind: '' | 'preset' | 'color' | 'image'
  preset: WallpaperPreset | ''
  color: string
  fit: '' | 'contain' | 'center' | 'tile'
  /** 0–80, percent darker. */
  dim: number
  /** 0–30 px. */
  blur: number
  rev: number
}
export const defaultWallpaper = (): Wallpaper => ({ kind: '', preset: '', color: '', fit: '', dim: 0, blur: 0, rev: 0 })

export const defaultPrefs = (): Prefs => ({
  clock: '',
  hide_panel: false,
  motion: '',
  dock: { hidden: [], pinned: [] },
  notify: { off: [], quiet: false },
  wallpaper: defaultWallpaper(),
  avatar: 0,
  avatar_style: '',
  avatar_seed: '',
})

/** Fills in what an older backend leaves out. */
export const fullPrefs = (p: Partial<Prefs>): Prefs => {
  const d = defaultPrefs()
  return { ...d, ...p, wallpaper: { ...d.wallpaper, ...(p.wallpaper ?? {}) }, avatar: p.avatar ?? 0, avatar_style: p.avatar_style ?? '', avatar_seed: p.avatar_seed ?? '' }
}
export const getPrefs = () => request<Prefs>(`${base}/prefs`).then(fullPrefs)
export const savePrefs = (p: Partial<Prefs>) => request<Prefs>(`${base}/prefs`, { method: 'POST', body: p }).then(fullPrefs)

// ---- each user's pictures ----
export type PictureKind = 'wallpaper' | 'avatar'
export const putPicture = (kind: PictureKind, blob: Blob) => request<Prefs>(`${base}/me/${kind}`, { method: 'POST', blob }).then(fullPrefs)
export const dropPicture = (kind: PictureKind) => request<Prefs>(`${base}/me/${kind}/remove`, { method: 'POST' }).then(fullPrefs)
export const myPicture = (kind: PictureKind, rev: number) => fetchBlob(`${base}/me/${kind}?v=${rev}`)
export const avatarOf = (name: string) => fetchBlob(`${base}/users/${encodeURIComponent(name)}/avatar`)

export interface Account {
  name: string
  admin: boolean
  avatar?: boolean
  avatar_style?: string
  avatar_seed?: string
}
export const accounts = () => request<{ items: Account[] }>(`${base}/users`).then((r) => r.items)
