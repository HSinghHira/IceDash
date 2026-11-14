// Each user's wallpaper (PLAN.md 5a.8): a built-in preset, a plain colour or
// a picture of their own, with how it fits, how dark and how blurred it is.
// Presets are drawn with gradients, so they cost nothing to load.
import type { Wallpaper, WallpaperPreset } from '../api/icedash'

export interface Preset {
  id: WallpaperPreset
  name: string
  scheme: 'dark' | 'light'
  css: string
}

export const PRESETS: Preset[] = [
  {
    id: 'aurora',
    name: 'Aurora',
    scheme: 'dark',
    css: 'radial-gradient(70% 55% at 18% 8%, rgba(29, 233, 182, 0.55), transparent 62%), radial-gradient(60% 55% at 82% 22%, rgba(76, 201, 240, 0.5), transparent 60%), radial-gradient(90% 70% at 50% 110%, #0b3d5c, transparent 70%), #02060b',
  },
  {
    id: 'glacier',
    name: 'Glacier',
    scheme: 'dark',
    css: 'radial-gradient(80% 60% at 70% 0%, rgba(165, 232, 255, 0.35), transparent 65%), linear-gradient(165deg, #06121d 0%, #0f3150 50%, #3f87ad 100%)',
  },
  {
    id: 'nightfall',
    name: 'Nightfall',
    scheme: 'dark',
    css: 'radial-gradient(120% 75% at 50% 118%, #ff7a59 0%, #a2456a 28%, #2a1d4a 55%, #07070f 100%)',
  },
  {
    id: 'ember',
    name: 'Ember',
    scheme: 'dark',
    css: 'radial-gradient(65% 55% at 72% 82%, rgba(255, 107, 53, 0.75), transparent 62%), radial-gradient(55% 45% at 18% 18%, rgba(245, 184, 76, 0.35), transparent 60%), #120805',
  },
  {
    id: 'lagoon',
    name: 'Lagoon',
    scheme: 'dark',
    css: 'linear-gradient(140deg, #002a33 0%, #07575b 42%, #3f8f97 78%, #9fd0d6 100%)',
  },
  {
    id: 'dune',
    name: 'Dune',
    scheme: 'light',
    css: 'radial-gradient(90% 60% at 80% 10%, rgba(255, 244, 222, 0.9), transparent 60%), linear-gradient(180deg, #f3d9b1 0%, #e0a96d 50%, #8a5634 100%)',
  },
  {
    id: 'graphite',
    name: 'Graphite',
    scheme: 'dark',
    css: 'radial-gradient(100% 80% at 50% 0%, #2c323a 0%, #111418 60%, #08090b 100%)',
  },
  {
    id: 'mist',
    name: 'Mist',
    scheme: 'light',
    css: 'radial-gradient(70% 60% at 25% 15%, rgba(255, 255, 255, 0.95), transparent 60%), linear-gradient(180deg, #eef3f8 0%, #d6e2ed 60%, #b5cadb 100%)',
  },
]

export const presetOf = (id: string) => PRESETS.find((p) => p.id === id)

/** Colours offered next to the picker. */
export const SWATCHES = ['#000000', '#0c1014', '#10202e', '#132a1f', '#2b1a12', '#2a1d3a', '#eaf1f7', '#f4efe6']

const FIT: Record<Wallpaper['fit'], { size: string; repeat: string }> = {
  '': { size: 'cover', repeat: 'no-repeat' },
  contain: { size: 'contain', repeat: 'no-repeat' },
  center: { size: 'auto', repeat: 'no-repeat' },
  tile: { size: 'auto', repeat: 'repeat' },
}

/**
 * The style of the layer behind the desktop, or null to leave the theme's
 * own background. image is the user's picture (an object URL) and
 * themeWall whether the theme has a wallpaper of its own.
 */
export function wallStyle(w: Wallpaper, image: string, themeWall: boolean): Record<string, string> | null {
  const fit = FIT[w.fit] ?? FIT['']
  const base = { backgroundPosition: 'center', backgroundSize: fit.size, backgroundRepeat: fit.repeat, backgroundColor: 'var(--bg)' }
  if (w.kind === 'image' && image) return { ...base, backgroundImage: `url("${image}")` }
  if (w.kind === 'preset') {
    const p = presetOf(w.preset)
    if (p) return { background: p.css }
  }
  if (w.kind === 'color' && /^#[0-9a-f]{6}$/i.test(w.color)) return { background: w.color }
  // The theme's: only drawn here when it is dimmed or blurred.
  if (themeWall && (w.dim || w.blur)) return { ...base, backgroundSize: 'cover', backgroundImage: 'var(--wallpaper)' }
  if (w.dim) return { background: 'transparent' }
  return null
}

/** "#rrggbb" → its lightness, 0–1. */
export function hexLight(hex: string): number {
  const n = parseInt(hex.slice(1), 16)
  return (0.2126 * ((n >> 16) & 255) + 0.7152 * ((n >> 8) & 255) + 0.0722 * (n & 255)) / 255
}

/**
 * Whether the wallpaper reads as dark or light, so the desktop's labels and
 * cards can stay readable on it; null when it is the theme's own.
 */
export function wallTone(w: Wallpaper, pictureLight: number | null): 'dark' | 'light' | null {
  let light: number | null = null
  if (w.kind === 'preset') light = presetOf(w.preset)?.scheme === 'light' ? 0.8 : 0.2
  else if (w.kind === 'color' && /^#[0-9a-f]{6}$/i.test(w.color)) light = hexLight(w.color)
  else if (w.kind === 'image') light = pictureLight
  if (light === null) return null
  light *= 1 - w.dim / 100
  return light > 0.55 ? 'light' : 'dark'
}