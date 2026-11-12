import { strings } from '../strings'

const UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']

/** The unit (power of base) that suits n. */
function unitIndex(n: number, base = 1024): number {
  let i = 0
  while (n >= base && i < UNITS.length - 1) {
    n /= base
    i++
  }
  return i
}

function amount(n: number, i: number, base = 1024): string {
  const v = n / base ** i
  return v >= 100 || i === 0 ? String(Math.round(v)) : v.toFixed(1).replace(/\.0$/, '')
}

/** 6655434752 → "6.2 GB". */
export function bytes(n: number): string {
  if (!Number.isFinite(n) || n < 0) return '–'
  const i = unitIndex(n)
  return `${amount(n, i)} ${UNITS[i]}`
}

/**
 * Used and total in the total's unit: "6.2 / 16 GB" style parts. Memory counts
 * in 1024s; disks pass base 1000, as the ZimaOS dashboard shows them.
 */
export function usedOfTotal(used: number, total: number, base: 1000 | 1024 = 1024): { used: string; total: string; unit: string } {
  const i = unitIndex(Math.max(total, 0), base)
  return { used: amount(Math.max(used, 0), i, base), total: amount(Math.max(total, 0), i, base), unit: UNITS[i] ?? 'B' }
}

/** Bytes per second → "4.1 MB/s". */
export function speed(bps: number): string {
  return `${bytes(Math.max(0, bps))}/s`
}

/** Seconds of uptime → "6 days", "3 hours", "12 minutes". */
export function duration(sec: number): string {
  const d = Math.floor(sec / 86400)
  if (d >= 1) return strings.days(d)
  const h = Math.floor(sec / 3600)
  if (h >= 1) return strings.hours(h)
  return strings.minutes(Math.max(0, Math.floor(sec / 60)))
}

/** A past moment, relative to now: "just now", "5 minutes ago", "yesterday". */
export function ago(when: Date, now: Date = new Date()): string {
  const s = Math.max(0, (now.getTime() - when.getTime()) / 1000)
  if (s < 60) return strings.justNow
  if (s < 3600) return strings.minutesAgo(Math.floor(s / 60))
  if (s < 86400) return strings.hoursAgo(Math.floor(s / 3600))
  return strings.daysAgo(Math.floor(s / 86400))
}

/** A percentage clamped to 0..100. */
export function pct(part: number, whole: number): number {
  if (!whole || whole <= 0) return 0
  return Math.min(100, Math.max(0, (part / whole) * 100))
}

/** Initials for the account button: "Hira" → "HI", "Harman Singh" → "HS". */
export function initials(name: string): string {
  const words = name.trim().split(/[\s._-]+/).filter(Boolean)
  if (words.length === 0) return '?'
  if (words.length === 1) return (words[0] ?? '').slice(0, 2).toUpperCase()
  return ((words[0]?.[0] ?? '') + (words[words.length - 1]?.[0] ?? '')).toUpperCase()
}
