import { afterEach, describe, expect, it } from 'vitest'
import { clockParts } from '../lib/timezone'
import { langOf, strings, uiLang, useLanguage, type Translation } from '../strings'
import de from './de'
import es from './es'
import fr from './fr'
import zh from './zh'

/** Names that are the same in every language. */
const SAME = ['appName', 'iceStore', 'iceName', 'frostName', 'settingsMods', 'settingsWidgets', 'settingsDock', 'settingsNotifications', 'notifications', 'ctxDock']

/** Calls a translated value the way the UI does, with sample arguments. */
function sample(v: unknown): string {
  if (typeof v === 'string') return v
  if (typeof v === 'function') {
    // Numbers for counts, words for names: try both, as the UI calls them.
    for (const args of [[2, 3, 'x'], ['x', 'y', 'z'], ['x', true]]) {
      try {
        return String((v as (...a: unknown[]) => unknown)(...args))
      } catch {
        // the other kind of argument
      }
    }
    return ''
  }
  if (v && typeof v === 'object') return Object.values(v).map(sample).join(' ')
  return ''
}

afterEach(() => useLanguage('en'))

describe.each([
  ['de', de],
  ['fr', fr],
  ['es', es],
  ['zh', zh],
] as [string, Translation][])('%s', (_, t) => {
  it('has every key, and only real ones', () => {
    const missing = Object.keys(strings).filter((k) => !(k in t) && !SAME.includes(k))
    expect(missing).toEqual([])
    const extra = Object.keys(t).filter((k) => !(k in strings))
    expect(extra).toEqual([])
  })
  it('gives text for every key, the same shape as English', () => {
    for (const [k, v] of Object.entries(t)) {
      const en = (strings as Record<string, unknown>)[k]
      expect(typeof v, k).toBe(typeof en)
      expect(sample(v).length, k).toBeGreaterThan(0)
      expect(sample(v), k).not.toContain('undefined')
      if (en && typeof en === 'object') expect(Object.keys(v as object).sort(), k).toEqual(Object.keys(en).sort())
    }
  })
})

describe('switching language', () => {
  it('fills the strings, falls back to English, and dates follow', async () => {
    await useLanguage('de')
    expect(uiLang()).toBe('de')
    expect(strings.settingsGeneral).toBe('Allgemein')
    expect(strings.settingsDock).toBe('IceDock')
    expect(strings.cardName.clock).toBe('Uhr')
    const d = new Date('2026-11-14T01:06:00Z')
    expect(clockParts(d, 'UTC', true).date).toContain('November')
    expect(clockParts(d, 'UTC', true).date).toContain('Samstag')
    await useLanguage('en')
    expect(strings.settingsGeneral).toBe('General')
    expect(strings.cardName.clock).toBe('Clock')
  })
  it('reads system and browser codes', () => {
    expect(langOf('de_DE')).toBe('de')
    expect(langOf('zh-Hans-CN')).toBe('zh')
    expect(langOf('fr-CA')).toBe('fr')
    expect(langOf('pt_BR')).toBeNull()
    expect(langOf('')).toBeNull()
  })
})
