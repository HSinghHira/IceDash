// Themes and widgets (PLAN.md, Phase 4).
import { request } from './client'

const base = '/v2/icedash/mods'

export interface Permissions {
  internet?: string[]
  system?: boolean
  apps?: boolean
  location?: boolean
}

export interface ThemeDef {
  scheme: 'dark' | 'light'
  colors: Record<string, string>
  css?: string
}

export interface Manifest {
  id: string
  name: string
  kind: 'theme' | 'widget'
  version: string
  author: string
  description: string
  entry?: string
  size?: 'small' | 'medium' | 'large'
  permissions?: Permissions
  preview?: ThemeDef
}

export interface Installed extends Manifest {
  source: string
  checked: boolean
  sha256: string
  enabled: boolean
  installed: string
  theme?: ThemeDef
}

export interface View {
  mods: Installed[]
  prefs: { theme: string; panel: string[] }
}

export interface CatalogMod extends Manifest {
  sha256: string
  file: string
  url: string
}

export interface Offer extends Manifest {
  source: string
  checked: boolean
  installed?: string
}

export const list = () => request<View>(base)
export const catalog = () => request<{ mods: CatalogMod[]; error?: string }>(`${base}/catalog`)
export const refreshCatalog = () => request<{ mods: CatalogMod[] }>(`${base}/catalog/refresh`, { method: 'POST' })
export const inspect = (source: string) => request<Offer>(`${base}/inspect`, { method: 'POST', body: { source } })
export const install = (source: string) => request<Installed>(`${base}/install`, { method: 'POST', body: { source } })
export const remove = (id: string) => request<View>(`${base}/${encodeURIComponent(id)}/remove`, { method: 'POST' })
export const setEnabled = (id: string, enabled: boolean) =>
  request<View>(`${base}/${encodeURIComponent(id)}/enabled`, { method: 'POST', body: { enabled } })
export const setTheme = (id: string) => request<View>(`${base}/theme`, { method: 'POST', body: { id } })
export const setPanel = (ids: string[]) => request<View>(`${base}/panel`, { method: 'POST', body: { ids } })

/** Where a widget's page loads from (no sign-in: its sandbox can't carry one). */
export const widgetUrl = (m: Manifest) => `${base}/${encodeURIComponent(m.id)}/files/${encodeURIComponent(m.entry ?? 'index.html')}`
