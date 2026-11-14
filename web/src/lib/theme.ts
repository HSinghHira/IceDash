// Applies a theme (PLAN.md, Phase 4, "Themes"): eight colours become every
// design token (the overlays follow the text colour, so a light theme gets
// dark overlays by itself), and the wallpaper fills the desktop. Themes have
// no CSS of their own. Ice is the built-in default: no overrides.
import type { ThemeDef } from '../api/mods'

/** "#4cc9f0" → "76, 201, 240". */
export function rgb(hex: string): string {
  const n = parseInt(hex.slice(1), 16)
  return `${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255}`
}

/** The token values a theme sets, by CSS variable. */
export function tokens(t: ThemeDef): Record<string, string> {
  const c = t.colors
  const fg = rgb(c.text!)
  return {
    '--bg': c.bg!,
    '--text': c.text!,
    '--fg-rgb': fg,
    '--mut': `rgba(${fg}, 0.6)`,
    '--line': `rgba(${fg}, ${t.scheme === 'light' ? 0.12 : 0.09})`,
    '--card': `rgba(${fg}, ${t.scheme === 'light' ? 0.045 : 0.035})`,
    '--ac': c.accent!,
    '--ac-rgb': rgb(c.accent!),
    '--ac2': c.accent2!,
    '--ac2-rgb': rgb(c.accent2!),
    '--acd': c.accentDark!,
    '--ink': c.ink!,
    '--warn': c.warn!,
    '--warn-rgb': rgb(c.warn!),
    '--surface-rgb': rgb(c.surface!),
    '--bar-rgb': t.scheme === 'light' ? rgb(c.surface!) : '0, 0, 0',
    '--dim': t.scheme === 'light' ? `rgba(${fg}, 0.16)` : 'rgba(0, 0, 0, 0.5)',
    '--dot-max': c.accent!,
  }
}

let applied: string[] = []

/** Applies t to the page, or Ice when t is null. */
export function applyTheme(t: ThemeDef | null) {
  const root = document.documentElement
  for (const k of applied) root.style.removeProperty(k)
  applied = []
  root.classList.remove('has-wallpaper')
  root.style.colorScheme = t?.scheme === 'light' ? 'light' : 'dark'
  if (!t) return
  for (const [k, v] of Object.entries(tokens(t))) {
    root.style.setProperty(k, v)
    applied.push(k)
  }
  // Only IceDash's own address for an installed theme's wallpaper.
  if (t.wallpaper && t.wallpaper.startsWith('/v2/icedash/mods/')) {
    root.style.setProperty('--wallpaper', `url("${t.wallpaper}")`)
    applied.push('--wallpaper')
    root.classList.add('has-wallpaper')
  }
}

/** Ice's colours, for its card and for widgets. */
export const ICE: ThemeDef = {
  scheme: 'dark',
  colors: {
    bg: '#000000',
    surface: '#0c1014',
    text: '#ffffff',
    accent: '#4cc9f0',
    accent2: '#a5e8ff',
    accentDark: '#2fb4dc',
    ink: '#031219',
    warn: '#f5b84c',
  },
}

/** Frost, the built-in light theme (PLAN.md 5a.2): Ice's blue on frosted white. */
export const FROST: ThemeDef = {
  scheme: 'light',
  colors: {
    bg: '#eaf1f7',
    surface: '#f7fafc',
    text: '#0b1620',
    accent: '#0b78b4',
    accent2: '#0a5f8f',
    accentDark: '#08669a',
    ink: '#ffffff',
    warn: '#9a5b00',
  },
}

/** The id the backend keeps for Frost. */
export const FROST_ID = 'frost'
