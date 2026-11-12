// ZimaOS's own APIs, the source of truth for apps, users and system figures
// (shapes measured on ZimaOS 1.8.0, docs/HOST-NOTES.md).
import type { GridEntry } from '../lib/apps'
import { request } from './client'

interface Wrapped<T> {
  success?: number
  data: T
}

export const appGrid = () => request<Wrapped<GridEntry[]>>('/v2/app_management/web/appgrid').then((r) => r.data ?? [])

export interface Utilization {
  cpu: { percent: number; num: number; temperature?: number }
  mem: { total: number; used: number; available: number; usedPercent: number }
  sys_disk: { size: number; used: number; avail: number }
}

export const utilization = () => request<Wrapped<Utilization>>('/v1/sys/utilization').then((r) => r.data)

export interface User {
  id: number
  username: string
  role: string
  nickname?: string
}

export const currentUser = () => request<Wrapped<User>>('/v1/users/current').then((r) => r.data)

/** The dashboard's saved choices; timezone is only there once someone saved it. */
export interface CustomSystem {
  lang?: string
  timezone?: string
}

export const customSystem = () =>
  request<Wrapped<CustomSystem | unknown[]>>('/v1/users/current/custom/system').then((r) =>
    r.data && !Array.isArray(r.data) ? (r.data as CustomSystem) : {},
  )

interface StoreSource {
  id: number
  url: string
}

/** Whether IceStore has been added as an App Store source in ZimaOS. */
export const iceStoreAdded = () =>
  request<Wrapped<StoreSource[]>>('/v2/app_management/appstore').then((r) =>
    (r.data ?? []).some((s) => /icestore/i.test(s.url ?? '')),
  )
