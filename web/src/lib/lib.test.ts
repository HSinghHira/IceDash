import { describe, expect, it } from 'vitest'
import { appUrl, counts, statusOf, toApps, type GridEntry } from './apps'
import { ago, bytes, duration, initials, pct, speed, usedOfTotal } from './format'
import { setHost } from './host'
import { desktopLaunchers, dockEnd, openLauncher, terminalUrl } from './launchers'
import { search } from './search'
import { cityOf, clockParts, pickZone } from './timezone'

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
    expect(dockEnd({ admin: true }).map((x) => x.open ?? x.url)).toEqual(['/#/', 'trash'])
    const seen: string[] = []
    const open = { files: (v?: string) => seen.push('files:' + v), terminal: () => seen.push('terminal') }
    for (const x of [...l, ...dockEnd({ admin: true })]) openLauncher(x, open)
    expect(seen).toEqual(['files:files', 'terminal', 'files:trash'])
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
      expect(dockEnd({ admin: true })[0]!.url).toBe('/casaos/#/')
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
