import { describe, expect, it } from 'vitest'
import type { CatalogApp } from '../api/apps'
import { bundlesWith, featuredApps, isSortKey, newestApps, relatedApps, shortNum, sortApps, supportsArch } from './icestore'

const app = (id: string, o: Partial<CatalogApp> = {}): CatalogApp => ({
  id: `com.icestore.${id}`,
  name: id,
  title: id[0]!.toUpperCase() + id.slice(1),
  tagline: '',
  category: 'Tools',
  categories: [],
  author: '',
  developer: '',
  architectures: ['amd64', 'arm64'],
  icon: '',
  thumbnail: '',
  version: '1',
  content_hash: '',
  ...o,
})

const list = [
  app('beta', { stars: 50, forks: 5, added: 300, updated: 900 }),
  app('alpha', { stars: 50, forks: 9, added: 100 }),
  app('gamma', { added: 200, updated: 400 }),
  app('delta', { stars: 900, featured: true }),
]
const ids = (l: CatalogApp[]) => l.map((a) => a.name)

describe('sortApps', () => {
  it('sorts by name both ways', () => {
    expect(ids(sortApps(list, 'az'))).toEqual(['alpha', 'beta', 'delta', 'gamma'])
    expect(ids(sortApps(list, 'za'))).toEqual(['gamma', 'delta', 'beta', 'alpha'])
  })
  it('puts most stars first, ties A–Z, and apps without stars last', () => {
    expect(ids(sortApps(list, 'stars'))).toEqual(['delta', 'alpha', 'beta', 'gamma'])
  })
  it('sorts by forks', () => {
    expect(ids(sortApps(list, 'forks'))).toEqual(['alpha', 'beta', 'delta', 'gamma'])
  })
  it('sorts by date added and by last update (added when never updated)', () => {
    expect(ids(sortApps(list, 'newest'))).toEqual(['beta', 'gamma', 'alpha', 'delta'])
    expect(ids(sortApps(list, 'updated'))).toEqual(['beta', 'gamma', 'alpha', 'delta'])
  })
  it('falls back to A–Z when the index has no numbers', () => {
    const bare = [app('b'), app('c'), app('a')]
    expect(ids(sortApps(bare, 'stars'))).toEqual(['a', 'b', 'c'])
    expect(ids(sortApps(bare, 'newest'))).toEqual(['a', 'b', 'c'])
  })
  it('leaves the list it was given alone', () => {
    const copy = [...list]
    sortApps(list, 'za')
    expect(list).toEqual(copy)
  })
  it('knows its keys', () => {
    expect(isSortKey('stars')).toBe(true)
    expect(isSortKey('nope')).toBe(false)
    expect(isSortKey(null)).toBe(false)
  })
})

describe('supportsArch', () => {
  it('matches Go names and other spellings', () => {
    expect(supportsArch({ architectures: ['amd64'] }, 'amd64')).toBe(true)
    expect(supportsArch({ architectures: ['x86_64'] }, 'amd64')).toBe(true)
    expect(supportsArch({ architectures: ['aarch64'] }, 'arm64')).toBe(true)
    expect(supportsArch({ architectures: ['amd64'] }, 'arm64')).toBe(false)
  })
  it('says yes when it can’t tell', () => {
    expect(supportsArch({ architectures: [] }, 'arm64')).toBe(true)
    expect(supportsArch({ architectures: ['amd64'] }, '')).toBe(true)
  })
})

describe('rows', () => {
  it('relatedApps: same category, most stars first, not the app itself', () => {
    const l = [...list, app('other', { category: 'Media', stars: 1e6 })]
    expect(ids(relatedApps(l[0]!, l))).toEqual(['delta', 'alpha', 'gamma'])
    expect(ids(relatedApps(l[0]!, l, 1))).toEqual(['delta'])
  })
  it('featuredApps and newestApps', () => {
    expect(ids(featuredApps(list))).toEqual(['delta'])
    expect(ids(newestApps(list, 2))).toEqual(['beta', 'gamma'])
    expect(newestApps([app('x')])).toEqual([])
  })
  it('bundlesWith finds the bundles an app is in', () => {
    const music = ['helix', 'trackhop', 'musike'].map((n) => app(n))
    expect(bundlesWith(music[0]!, music).map((b) => b.bundle.id)).toEqual(['music'])
    expect(bundlesWith(list[0]!, list)).toEqual([])
  })
})

describe('shortNum', () => {
  it('shortens big numbers', () => {
    expect(shortNum(950)).toBe('950')
    expect(shortNum(1000)).toBe('1k')
    expect(shortNum(12345)).toBe('12.3k')
    expect(shortNum(97166)).toBe('97.2k')
    expect(shortNum(150000)).toBe('150k')
    expect(shortNum(1520000)).toBe('1.5M')
  })
})
