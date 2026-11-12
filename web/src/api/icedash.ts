// IceDash's own backend under /v2/icedash (PLAN.md, "Backend API").
import { request } from './client'

const base = '/v2/icedash'

export interface Health {
  version: string
  arch: string
  started: string
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
}

export const getSettings = () => request<Settings>(`${base}/settings`)
export const saveSettings = (s: Settings) => request<Settings>(`${base}/settings`, { method: 'POST', body: s })
