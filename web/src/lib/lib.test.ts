import { describe, expect, it } from 'vitest'
import { appUrl, counts, statusOf, toApps, type GridEntry } from './apps'
import { ago, bytes, duration, initials, pct, speed, usedOfTotal } from './format'
import { setHost } from './host'
import { desktopLaunchers, dockEnd, openLauncher, terminalUrl } from './launchers'
import { search } from './search'
import { cityOf, clockParts, pickZone } from './timezone'
import { clampCrop } from './pictures'
import { flowLeft, flowRight } from './flow'
import { drawAvatar } from './avatars'
import { PRESETS, wallStyle, wallTone } from './wallpaper'
import { defaultWallpaper, fullPrefs } from '../api/icedash'

// Entries shaped like the ZimaCube's app grid (HOST-NOTES item 1).
const grid: GridEntry[] = [
  {
    name: 'homeassistant', app_type: 'v2app', icon: 'https://cdn.example/ha.svg', index: '/', port: '8123',
    scheme: 'http', status: 'running', title: { en_US: 'Home Assistant' },
  },
  {
    name: 'ente-garage', app_type: 'v2app', hostname: '', icon: 'https://icestore.hsinghhira.me/x.png', index: '/',
    port: '6600', scheme: 'http', status: 'exited', title: { custom: 'Ente - Garage S3', en_US: 'Ente - Garage S3' },
  },
  {
    name: 'c68ca986fe10', app_type: 'container', status: 'running', title: { en_US: 'baby-schedule' },
  },
  { name: 'appflowy', port: 6101, index: 'web', status: 'restarting', title: { custom: 'AppFlowy', en_US: 'x' } },
]

describe('apps', () => {
  it('maps ZimaOS states to the three dots', () => {
    expect(statusOf('running')).toBe('running')
    expect(statusOf('exited')).toBe('stopped')
    expect(statusOf('restarting')).toBe('starting')
    expect(statusOf('created')).toBe('starting')
    expect(statusOf(undefined)).toBe('stopped')
  })

  it('builds the app address, using the browser host when hostname is empty', () => {
    expect(appUrl(grid[0]!, '192.168.68.17')).toBe('http://192.168.68.17:8123/')
    expect(appUrl({ ...grid[0]!, hostname: 'nas.local' }, '192.168.68.17')).toBe('http://nas.local:8123/')
    expect(appUrl(grid[3]!, 'zima')).toBe('http://zima:6101/web')
    expect(appUrl(grid[2]!, 'zima')).toBeUndefined() // a bare container has no address
    expect(appUrl({ name: 'x', port: '80; rm -rf' }, 'zima')).toBeUndefined()
  })

  it('names apps by their custom title, sorts them and counts states', () => {
    const apps = toApps(grid, 'zima')
    expect(apps.map((a) => a.name)).toEqual(['AppFlowy', 'baby-schedule', 'Ente - Garage S3', 'Home Assistant'])
    expect(counts(apps)).toEqual({ running: 2, starting: 1, stopped: 1 })
  })
})

describe('format', () => {
  it('formats sizes and speeds', () => {
    expect(bytes(0)).toBe('0 B')
    expect(bytes(1536)).toBe('1.5 KB')
    expect(bytes(16644636672)).toBe('15.5 GB')
    expect(speed(4.1 * 1024 * 1024)).toBe('4.1 MB/s')
    expect(speed(380 * 1024)).toBe('380 KB/s')
    expect(usedOfTotal(3379572736, 16644636672)).toEqual({ used: '3.1', total: '15.5', unit: 'GB' })
    // Disks as ZimaOS shows them: 140 of 238 GB, not 130 of 222.
    expect(usedOfTotal(140e9, 238e9, 1000)).toEqual({ used: '140', total: '238', unit: 'GB' })
  })

  it('formats uptime and times ago', () => {
    expect(duration(960193)).toBe('11 days')
    expect(duration(7200)).toBe('2 hours')
    expect(duration(59)).toBe('0 minutes')
    const now = new Date('2026-11-12T10:00:00Z')
    expect(ago(new Date('2026-11-12T09:59:30Z'), now)).toBe('just now')
    expect(ago(new Date('2026-11-12T09:55:00Z'), now)).toBe('5 minutes ago')
    expect(ago(new Date('2026-11-11T09:00:00Z'), now)).toBe('yesterday')
  })

  it('makes initials and percentages', () => {
    expect(initials('Hira')).toBe('HI')
    expect(initials('Harman Singh')).toBe('HS')
    expect(initials('')).toBe('?')
    expect(pct(5, 0)).toBe(0)
    expect(pct(150, 100)).toBe(100)
  })
})

describe('launchers', () => {
  const where = { protocol: 'http:', hostname: '192.168.68.17' }
  it('gives admins the Files, Terminal and Trash windows', () => {
    const l = desktopLaunchers(where, { admin: true, iceStoreInZimaOS: false })
    expect(l.map((x) => x.id)).toEqual(['files', 'icestore', 'appstore', 'terminal'])
    expect(l.map((x) => x.open)).toEqual(['files', undefined, undefined, 'terminal'])
    expect(l[1]!.url).toBe('https://icestore.hsinghhira.me')
    expect(l[2]!.url).toBe('/#/')
    expect(dockEnd({ admin: true }).map((x) => x.open)).toEqual(['settings', 'trash'])
    const seen: string[] = []
    const open = { files: (v?: string) => seen.push('files:' + v), terminal: () => seen.push('terminal'), settings: () => seen.push('settings') }
    for (const x of [...l, ...dockEnd({ admin: true })]) openLauncher(x, open)
    expect(seen).toEqual(['files:files', 'terminal', 'settings', 'files:trash'])
  })
  it('gives members no Files, Terminal or Trash, and opens IceStore in ZimaOS once added', () => {
    const l = desktopLaunchers(where, { admin: false, iceStoreInZimaOS: true })
    expect(l.map((x) => x.id)).toEqual(['icestore', 'appstore'])
    expect(l[0]!.url).toBe('/#/')
    expect(dockEnd({ admin: false }).map((x) => x.id)).toEqual(['settings'])
  })
  it('opens CasaOS pages on its home page at /casaos/', () => {
    setHost({ host: 'casaos', host_name: 'CasaOS', stock_url: '/casaos/' })
    try {
      const l = desktopLaunchers(where, { admin: true, iceStoreInZimaOS: true })
      expect(l.filter((x) => !x.open).map((x) => x.url)).toEqual(['/casaos/#/', '/casaos/#/'])
      expect(dockEnd({ admin: true })[0]!.open).toBe('settings')
    } finally {
      setHost({ host: 'zimaos', host_name: 'ZimaOS', stock_url: '/' })
    }
  })
  it('brackets an IPv6 host', () => {
    expect(terminalUrl({ protocol: 'http:', hostname: 'fe80::1' })).toBe('http://[fe80::1]:7681/')
  })
})

describe('search', () => {
  const items = [
    { id: '1', name: 'Home Assistant' },
    { id: '2', name: 'AppFlowy' },
    { id: '3', name: 'App Store' },
    { id: '4', name: 'Pocket ID' },
  ]
  it('ranks names that start with the query first', () => {
    expect(search(items, 'app').map((i) => i.name)).toEqual(['AppFlowy', 'App Store'])
    expect(search(items, 'ass').map((i) => i.name)).toEqual(['Home Assistant'])
    expect(search(items, 'cket').map((i) => i.name)).toEqual(['Pocket ID'])
    expect(search(items, 'zzz')).toEqual([])
    expect(search(items, '  ').length).toBe(4)
  })
})

describe('timezone', () => {
  it('uses the saved zone, else the browser zone', () => {
    expect(pickZone('Pacific/Auckland', 'Europe/London')).toBe('Pacific/Auckland')
    expect(pickZone(undefined, 'Pacific/Auckland')).toBe('Pacific/Auckland')
    expect(pickZone('Not/AZone', 'Pacific/Auckland')).toBe('Pacific/Auckland')
  })
  it('names the city and splits the clock', () => {
    expect(cityOf('Pacific/Auckland')).toBe('Auckland')
    expect(cityOf('America/Argentina/Buenos_Aires')).toBe('Buenos Aires')
    const p = clockParts(new Date('2026-11-11T01:06:00Z'), 'Pacific/Auckland')
    expect(p.time).toBe('2:06')
    expect(p.ampm).toBe('pm')
    expect(p.date).toContain('Wednesday')
    expect(p.date).toContain('11 November')
    const h = clockParts(new Date('2026-11-11T01:06:00Z'), 'Pacific/Auckland', true)
    expect(h.time).toBe('14:06')
    expect(h.ampm).toBe('')
    expect(h.short.endsWith('14:06')).toBe(true)
    expect(clockParts(new Date('2026-11-10T11:05:00Z'), 'Pacific/Auckland', true).time).toBe('00:05')
  })
})

describe('setup helpers', () => {
  it('labels env keys and spots placeholders', async () => {
    const { inputLabel, placeholderIn, withHost } = await import('./setup')
    expect(inputLabel('DOCUMENSO_ADMIN_EMAIL', 'documenso')).toBe('Admin email')
    expect(inputLabel('SERVER_IP', 'autoxpose')).toBe('Server IP')
    expect(inputLabel('PUBLIC_URL', 'documenso')).toBe('Public URL')
    expect(placeholderIn('CHANGEME_PUBLIC_IP')).toBe(true)
    expect(placeholderIn('http://[ZIMAOS-IP-ADDRESS]:6181')).toBe(true)
    expect(placeholderIn('203.0.113.1')).toBe(false)
    expect(withHost('Open http://[ZIMAOS-IP-ADDRESS]:6181 or your-zimaos-ip', '192.168.68.17')).toBe(
      'Open http://192.168.68.17:6181 or 192.168.68.17',
    )
  })

  it('renders markdown safely', async () => {
    const { markdown } = await import('./markdown')
    const html = markdown(
      '**Hi** <script>x</script>\n\n- a [link](https://x.y/?a=1&b=2)\n- [bad](javascript:alert(1))\n\n1. one\n2. two',
    )
    expect(html).toContain('<strong>Hi</strong> &lt;script&gt;')
    expect(html).toContain('<a href="https://x.y/?a=1&amp;b=2"')
    expect(html).not.toContain('href="javascript')
    expect(html).toContain('<ol><li>one</li><li>two</li></ol>')
  })
})

describe('themes', () => {
  it('turns eight colours into every token, with overlays from the text colour', async () => {
    const { tokens, rgb } = await import('./theme')
    expect(rgb('#4cc9f0')).toBe('76, 201, 240')
    const paper = tokens({
      scheme: 'light',
      colors: { bg: '#eef2f7', surface: '#ffffff', text: '#0d1726', accent: '#2f6fec', accent2: '#1d4ed8', accentDark: '#2459c7', ink: '#ffffff', warn: '#b45309' },
    })
    expect(paper['--fg-rgb']).toBe('13, 23, 38') // dark overlays on a light theme
    expect(paper['--surface-rgb']).toBe('255, 255, 255')
    expect(paper['--ac-rgb']).toBe('47, 111, 236')
    expect(paper['--mut']).toBe('rgba(13, 23, 38, 0.6)')
  })
})

describe('wallpapers (5a.8)', () => {
  const w = defaultWallpaper()
  it('leaves the theme alone unless asked', () => {
    expect(wallStyle(w, '', false)).toBeNull()
    expect(wallStyle(w, '', true)).toBeNull()
    expect(wallStyle({ ...w, dim: 30 }, '', true)?.backgroundImage).toBe('var(--wallpaper)')
  })
  it('draws presets, colours and the user’s picture', () => {
    expect(wallStyle({ ...w, kind: 'preset', preset: 'aurora' }, '', false)?.background).toBe(PRESETS[0]!.css)
    expect(wallStyle({ ...w, kind: 'color', color: '#112233' }, '', false)?.background).toBe('#112233')
    expect(wallStyle({ ...w, kind: 'color', color: 'red;x' }, '', false)).toBeNull()
    const img = wallStyle({ ...w, kind: 'image', fit: 'tile' }, 'blob:x', false)!
    expect(img.backgroundImage).toBe('url("blob:x")')
    expect(img.backgroundRepeat).toBe('repeat')
    // A picture not loaded (yet): nothing drawn over the theme.
    expect(wallStyle({ ...w, kind: 'image' }, '', false)).toBeNull()
  })
  it('fills in choices an older backend leaves out', () => {
    const p = fullPrefs({ clock: '24' })
    expect(p.wallpaper).toEqual(defaultWallpaper())
    expect(p.avatar).toBe(0)
    expect(p.clock).toBe('24')
  })
  it('tells a dark wallpaper from a light one', () => {
    expect(wallTone(w, null)).toBeNull()
    expect(wallTone({ ...w, kind: 'preset', preset: 'mist' }, null)).toBe('light')
    expect(wallTone({ ...w, kind: 'preset', preset: 'mist', dim: 60 }, null)).toBe('dark')
    expect(wallTone({ ...w, kind: 'color', color: '#f4efe6' }, null)).toBe('light')
    expect(wallTone({ ...w, kind: 'image' }, 0.1)).toBe('dark')
    expect(wallTone({ ...w, kind: 'image' }, null)).toBeNull()
  })
  it('keeps an avatar covering its frame', () => {
    expect(clampCrop({ x: 20, y: -500, w: 300, h: 400 }, 260)).toEqual({ x: 0, y: -140, w: 300, h: 400 })
  })
})
describe('desktop layout without scrolling', () => {
  const area = { left: 0, right: 1400, top: 50, bottom: 850 }
  const dock = { left: 500, right: 900, top: 780 }
  it('starts a new card column to the left, at the top', () => {
    const { pos, cols } = flowRight([300, 300, 300, 100], area, null, 312, 16)
    expect(pos).toEqual([{ x: 0, y: 0 }, { x: 0, y: 316 }, { x: 328, y: 0 }, { x: 328, y: 316 }])
    expect(cols).toBe(2)
  })
  it('keeps a column above the dock only where it passes over it', () => {
    // Columns at x 1088–1400 and 760–1072: the second is over the dock.
    const { pos } = flowRight([400, 340, 400, 340], area, dock, 312, 16)
    expect(pos[1]).toEqual({ x: 0, y: 416 }) // reaches 756 < 800: fits beside the dock
    expect(pos[3]).toEqual({ x: 656, y: 0 }) // 416 + 340 > 714 above the dock
    expect(flowRight([], area, dock, 312, 16).cols).toBe(0)
  })
  it('fills icon columns from the left and skips the room under the dock', () => {
    const { pos } = flowLeft(30, 90, 86, { left: 480, right: 1000, top: 50, bottom: 850 }, dock, 2)
    // The first column (480–570) passes over the dock: 8 rows fit above 772.
    expect(pos[7]).toEqual({ x: 0, y: 616 })
    expect(pos[8]).toEqual({ x: 90, y: 0 })
    const free = flowLeft(30, 90, 86, { left: 0, right: 400, top: 50, bottom: 850 }, dock, 2)
    expect(free.pos[8]).toEqual({ x: 0, y: 704 }) // beside the dock: 9 rows
  })
})

describe('drawn avatars', () => {
  it('draws the same face for the same name, and none for initials', async () => {
    const a = await drawAvatar('critters', 'Hira')
    expect(a.startsWith('data:image/svg+xml')).toBe(true)
    expect(await drawAvatar('critters', 'Hira')).toBe(a)
    expect(await drawAvatar('critters', 'testmember')).not.toBe(a)
    expect(await drawAvatar('initials', 'Hira')).toBe('')
    expect(await drawAvatar('nope', 'Hira')).toBe('')
  })
})