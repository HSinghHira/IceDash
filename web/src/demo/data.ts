// The live demo's sample server (see demo/index.ts): a ZimaCube with a dozen
// IceStore apps, some files and a couple of people. Nothing here is real; the
// app list, icons and texts come from IceStore so the desktop looks like one
// a person would really have.
import type { Entry, Kind } from '../api/files'
import type { Notification } from '../api/icedash'

export const STORE = 'https://icestore.hsinghhira.me'
export const icon = (id: string) => `${STORE}/apps/com.icestore.${id}/assets/icon.png`
export const SERVER_IP = '192.168.1.20'
export const DEVICE = 'ZimaCube'

/** An app on the desktop: id, title, port, Docker state, version, an update waiting. */
export interface DemoApp {
  name: string
  title: string
  port: string
  status: 'running' | 'exited' | 'restarting'
  version: string
  latest?: string
  firstLaunch?: boolean
  /** Updates at night, the previous version kept, a held version. */
  auto?: boolean
  back?: string
  held?: string
  heldBy?: 'user' | 'night'
}

export const startApps: DemoApp[] = [
  { name: 'appflowy', title: 'AppFlowy', port: '6101', status: 'running', version: '0.19.2' },
  { name: 'ente-garage', title: 'Ente - Garage S3', port: '6600', status: 'running', version: '1.2.5' },
  { name: 'watchyourlan', title: 'WatchYourLAN', port: '6121', status: 'running', version: '2.1.4' },
  { name: 'pocket-id', title: 'Pocket ID', port: '6157', status: 'running', version: 'v2.19.1' },
  { name: 'stirling-pdf', title: 'Stirling-PDF', port: '6131', status: 'running', version: '3.3.0' },
  { name: 'readeck', title: 'Readeck', port: '6145', status: 'running', version: '0.23.5', back: '0.23.4' },
  { name: 'ntfy', title: 'ntfy', port: '6118', status: 'running', version: '2.32.0' },
  { name: 'searxng', title: 'SearXNG', port: '6170', status: 'running', version: '2026.8.10' },
  { name: 'changedetection', title: 'changedetection.io', port: '6113', status: 'restarting', version: '0.70.3' },
  { name: 'outline', title: 'Outline', port: '6150', status: 'running', version: '1.12.0', latest: '1.13.0', auto: true },
  { name: 'gitsave', title: 'GitSave', port: '6181', status: 'exited', version: '1.4.0', latest: '1.5.0', held: '1.5.0', heldBy: 'night', auto: true },
  { name: 'it-tools', title: 'IT-Tools', port: '6127', status: 'running', version: '2024.10.22' },
  { name: 'dispatcharr', title: 'Dispatcharr', port: '6190', status: 'running', version: '0.33.0' },
  { name: 'cartvibe', title: 'CartVibe', port: '6161', status: 'running', version: '2.79.0', firstLaunch: true },
]

/** CartVibe's first-launch card: the app the demo pretends was just installed. */
export const firstLaunchFor = {
  name: 'cartvibe',
  title: 'CartVibe',
  tagline: 'Household receipts, shared shopping lists and spending insights.',
  steps: [
    'Sign in with the login above.',
    'Change the password: Management, Users, then the key icon on your row.',
    'Add a receipt from a photo, or start a shopping list.',
  ],
  creds: {
    fields: [
      { label: 'User', value: 'admin', secret: false, url: false },
      { label: 'Password', value: 'kQ2-vb7Lr9xWm4Ta', secret: true, url: false },
    ],
    notes: [],
  },
}

export const people = [
  { name: 'Demo', admin: true },
  { name: 'Alex', admin: false },
]

const ago = (min: number) => new Date(Date.now() - min * 60000).toISOString()

export function startNotes(version: string): Notification[] {
  return [
    { id: 5, time: ago(3), kind: 'app_installed', app: 'cartvibe', title: 'CartVibe is ready', body: 'CartVibe is installed and starting.', action: { label: 'Open', kind: 'open_app' }, read: false },
    { id: 4, time: ago(48), kind: 'app_stopped', app: 'gitsave', title: 'GitSave stopped', body: 'GitSave isn’t running now. Start it from its menu, or look at its logs to see why.', action: { label: 'Start', kind: 'start_app' }, read: false },
    { id: 3, time: ago(190), kind: 'app_update', app: 'gitsave', title: 'GitSave 1.5.0 is out', body: 'Update it from its menu or from IceStore.', read: true },
    { id: 2, time: ago(60 * 26), kind: 'updated', version, title: `Updated to ${version}`, body: '', read: true },
  ]
}

// ---- Files ----

const GB = 1024 ** 3
const MB = 1024 ** 2

export interface Node {
  entry: Entry
  children?: Node[]
}

const kindOf = (name: string): Kind => {
  const ext = name.split('.').pop()!.toLowerCase()
  if (['jpg', 'jpeg', 'png', 'webp', 'heic'].includes(ext)) return 'image'
  if (['md', 'txt', 'yml', 'json', 'log', 'conf'].includes(ext)) return 'text'
  if (ext === 'pdf') return 'pdf'
  if (['mp4', 'mkv', 'mov'].includes(ext)) return 'video'
  if (['mp3', 'flac', 'm4a'].includes(ext)) return 'audio'
  return 'other'
}

const daysAgo = (d: number) => new Date(Date.now() - d * 86400000).toISOString()

/** A folder (array) or a file (its size in bytes), with how many days ago it changed. */
type Spec = { [name: string]: Spec | [number, number] }

const spec: Spec = {
  AppData: Object.fromEntries(startApps.map((a, i) => [a.name, { 'config.yml': [2 * 1024 + i * 37, 3 + i] }])),
  Backup: { 'appflowy-2026-11-01.tar.gz': [412 * MB, 19], 'outline-2026-11-01.tar.gz': [96 * MB, 19] },
  Documents: {
    Taxes: { '2025 return.pdf': [1.2 * MB, 210], 'Receipts 2026.pdf': [4.8 * MB, 12] },
    'Home lease.pdf': [880 * 1024, 300],
    'Packing list.md': [3 * 1024, 6],
    'Budget 2026.xlsx': [64 * 1024, 2],
  },
  Downloads: { 'ubuntu-24.04.3-desktop-amd64.iso': [5.9 * GB, 33], 'zimaos-1.8.0.img': [1.6 * GB, 41] },
  Gallery: {
    'Queenstown 2026': Object.fromEntries(Array.from({ length: 9 }, (_, i) => [`IMG_${4410 + i}.jpg`, [3.1 * MB + i * 211e3, 70]])),
    'Birthday.mp4': [312 * MB, 120],
  },
  Media: {
    Movies: { 'Big Buck Bunny (2008).mkv': [1.4 * GB, 400], 'Sintel (2010).mkv': [1.1 * GB, 400] },
    Music: { 'Kevin MacLeod - Local Forecast.mp3': [7.2 * MB, 500], 'Lofi Mix.flac': [48 * MB, 90] },
  },
}

function build(dir: string, s: Spec): Node[] {
  return Object.entries(s).map(([name, v]) => {
    const path = (dir === '/' ? '/' : dir + '/') + name
    if (Array.isArray(v)) {
      const [size, d] = v
      return { entry: { name, path, dir: false, size: Math.round(size), modified: daysAgo(d), mode: 'rw-r--r--', kind: kindOf(name) } }
    }
    return {
      entry: { name, path, dir: true, size: 0, modified: daysAgo(1), mode: 'rwxr-xr-x', kind: 'folder' },
      children: build(path, v),
    }
  })
}

export const startFiles = (): Node[] => build('/', spec)

export const usb = (): Node[] =>
  build('USB_Stick:/', { Music: { 'Mix tape.mp3': [9 * MB, 900] }, 'Photos 2024': { 'IMG_0001.jpg': [2.4 * MB, 600] }, 'notes.txt': [2048, 30] })

export const disks = [
  { id: '', name: 'DATA', path: '/', main: true, trash: true, size: 3.6 * 1024 * GB, free: 2.1 * 1024 * GB },
  { id: 'USB_Stick', name: 'USB Stick', path: 'USB_Stick:/', main: false, trash: false, size: 58 * GB, free: 31 * GB },
]

/** The Storage card: ZimaOS's storages for the same two disks (TB as ZimaOS counts them). */
export const storagePools = () => ({
  health: true,
  pools: [
    {
      name: 'ZimaOS-HD', kind: 'system', size: disks[0]!.size, used: disks[0]!.size - disks[0]!.free, health: 'ok', files: '/',
      drives: [{ name: 'sda', model: 'WD Red Plus 4TB', health: true, temp: 36, hours: 18686 }],
    },
    {
      name: 'USB Stick', kind: 'usb', size: disks[1]!.size, used: disks[1]!.size - disks[1]!.free, health: 'ok', files: 'USB_Stick:/',
      drives: [{ name: 'sdb', model: 'SanDisk Ultra', health: true }],
    },
  ],
})

export const logsFor = (title: string) =>
  [
    `[init] Starting ${title}`,
    '[init] Reading config from /data/config.yml',
    '[db] Connected, schema up to date',
    '[http] Listening on 0.0.0.0:8080',
    '[jobs] 2 scheduled jobs',
    '[http] GET / 200 4ms',
    '[http] GET /api/health 200 1ms',
  ].join('\n') + '\n'
