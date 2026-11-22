// IceStore list helpers: sorting, the box's CPU, related apps. Stars, forks
// and dates come from index.json (the IceStore site build writes them); an
// older index has none, and those sorts then fall back to A–Z.
import type { CatalogApp } from '../api/apps'
import { availableBundles, type Bundle } from './bundles'
import type { IconName } from './icons'

export const SORTS = ['az', 'za', 'newest', 'updated', 'stars', 'forks'] as const
export type SortKey = (typeof SORTS)[number]

export const isSortKey = (s: unknown): s is SortKey => SORTS.includes(s as SortKey)

const byTitle = (a: CatalogApp, b: CatalogApp) => a.title.localeCompare(b.title, 'en', { sensitivity: 'base' })

/** Highest first; apps without the number go last, ties A–Z. */
const byNumber = (pick: (a: CatalogApp) => number | undefined) => (a: CatalogApp, b: CatalogApp) => {
  const x = pick(a) ?? -1
  const y = pick(b) ?? -1
  return y - x || byTitle(a, b)
}

const ORDER: Record<SortKey, (a: CatalogApp, b: CatalogApp) => number> = {
  az: byTitle,
  za: (a, b) => byTitle(b, a),
  newest: byNumber((a) => a.added || undefined),
  updated: byNumber((a) => a.updated || a.added || undefined),
  stars: byNumber((a) => a.stars),
  forks: byNumber((a) => a.forks),
}

/** A sorted copy of list. */
export function sortApps(list: CatalogApp[], key: SortKey): CatalogApp[] {
  return [...list].sort(ORDER[key] ?? byTitle)
}

/** Go's GOARCH names and the spellings app manifests use for the same CPUs. */
const ALIASES: Record<string, string[]> = {
  amd64: ['amd64', 'x86_64', 'x64'],
  arm64: ['arm64', 'aarch64', 'arm64v8'],
  arm: ['arm', 'armv7', 'arm/v7', 'armhf'],
}

/** Whether app runs on the box. An unknown CPU or an app that lists none: yes. */
export function supportsArch(app: Pick<CatalogApp, 'architectures'>, arch: string): boolean {
  const list = (app.architectures ?? []).map((a) => a.toLowerCase())
  if (!arch || !list.length) return true
  const names = ALIASES[arch.toLowerCase()] ?? [arch.toLowerCase()]
  return list.some((a) => names.includes(a))
}

/** Other apps in app's category, most-starred first. */
export function relatedApps(app: CatalogApp, all: CatalogApp[], n = 6): CatalogApp[] {
  return sortApps(
    all.filter((a) => a.id !== app.id && a.category && a.category === app.category),
    'stars',
  ).slice(0, n)
}

/** The bundles app is part of, with their apps. */
export function bundlesWith(app: CatalogApp, all: CatalogApp[]): { bundle: Bundle; apps: CatalogApp[] }[] {
  return availableBundles(all).filter((b) => b.apps.some((a) => a.id === app.id))
}

/** The featured apps, in catalog order. */
export const featuredApps = (all: CatalogApp[]) => all.filter((a) => a.featured)

/** The n apps added last. Empty when the index has no dates. */
export const newestApps = (all: CatalogApp[], n = 8) => sortApps(all.filter((a) => a.added), 'newest').slice(0, n)

const CATEGORY_ICONS: Record<string, IconName> = {
  ai: 'sparkle',
  developer: 'terminal',
  finance: 'coins',
  home: 'home',
  media: 'film',
  music: 'music',
  networking: 'globe',
  network: 'globe',
  productivity: 'checks',
  security: 'shield',
  social: 'users',
  utilities: 'settings',
  others: 'grid',
}

/** The sidebar icon for an IceStore category. */
export const categoryIcon = (name: string): IconName => CATEGORY_ICONS[name.trim().toLowerCase()] ?? 'package'

/** The n apps changed last (a version bump, else when added). Empty without dates. */
export const updatedApps = (all: CatalogApp[], n = 10) => sortApps(all.filter((a) => a.updated || a.added), 'updated').slice(0, n)

/** The n most-starred apps. Empty when the index has no stars. */
export const popularApps = (all: CatalogApp[], n = 10) => sortApps(all.filter((a) => a.stars != null), 'stars').slice(0, n)

/** The n most-forked apps. Empty when the index has no forks. */
export const forkedApps = (all: CatalogApp[], n = 10) => sortApps(all.filter((a) => (a.forks ?? 0) > 0), 'forks').slice(0, n)

/** The apps with a screenshot to show, for the picture-led rows. */
export const withShots = (all: CatalogApp[]) => all.filter((a) => a.thumbnail)

/** 950 → "950", 12345 → "12.3k", 1520000 → "1.5M". */
export function shortNum(n: number): string {
  if (!Number.isFinite(n)) return '–'
  const v = Math.abs(n)
  const fmt = (x: number, unit: string) => `${x >= 100 ? Math.round(x) : x.toFixed(1).replace(/\.0$/, '')}${unit}`
  if (v >= 1e6) return fmt(n / 1e6, 'M')
  if (v >= 1e3) return fmt(n / 1e3, 'k')
  return String(Math.round(n))
}
