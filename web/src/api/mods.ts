// Themes and widgets from collections: GitHub repos of plain folders
// (PLAN.md, Phase 4). The official ones are IceColors and IceCards.
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
  wallpaper?: string
}

/** A mod in a collection. */
export interface Item {
  id: string
  collection: string
  folder: string
  official: boolean
  kind: 'theme' | 'widget'
  name: string
  author: string
  description: string
  version: string
  scheme?: 'dark' | 'light'
  colors?: Record<string, string>
  wallpaper?: string
  entry?: string
  size?: 'small' | 'medium' | 'large'
  permissions?: Permissions
}

export interface Installed extends Item {
  commit: string
  enabled: boolean
  installed: string
  update?: boolean
}

export interface Collection {
  source: string
  official: boolean
  commit?: string
  items: Item[]
  error?: string
}

export interface View {
  mods: Installed[]
  prefs: { theme: string; panel: string[] }
}

export const list = () => request<View>(base)
export const collections = (refresh = false) =>
  request<{ collections: Collection[] }>(`${base}/collections${refresh ? '?refresh=1' : ''}`).then((r) => r.collections)
export const addCollection = (source: string) => request<Collection>(`${base}/collections`, { method: 'POST', body: { source } })
export const removeCollection = (source: string) => request(`${base}/collections/remove`, { method: 'POST', body: { source } })
export const install = (id: string) => request<Installed>(`${base}/install`, { method: 'POST', body: { id } })
export const remove = (id: string) => request<View>(`${base}/${encodeURIComponent(id)}/remove`, { method: 'POST' })
export const setEnabled = (id: string, enabled: boolean) =>
  request<View>(`${base}/${encodeURIComponent(id)}/enabled`, { method: 'POST', body: { enabled } })
export const setTheme = (id: string) => request<View>(`${base}/theme`, { method: 'POST', body: { id } })
export const setPanel = (ids: string[]) => request<View>(`${base}/panel`, { method: 'POST', body: { ids } })

/** Where a widget's page loads from (no sign-in: its sandbox can't carry one). */
export const widgetUrl = (m: Item) => `${base}/${encodeURIComponent(m.id)}/files/${encodeURIComponent(m.entry ?? 'index.html')}`

/** A theme's colours and wallpaper, as applyTheme takes them. */
export const themeOf = (m: Item): ThemeDef | null =>
  m.kind === 'theme' && m.colors ? { scheme: m.scheme ?? 'dark', colors: m.colors, wallpaper: m.wallpaper } : null
