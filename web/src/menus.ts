// What each right-click menu holds, outside the app menu (mock-up 3 is for
// apps). The components only say where the menu opens.
import { signOut } from './api/client'
import * as ice from './api/icedash'
import { dashboardUrl, host } from './lib/host'
import type { Launcher } from './lib/launchers'
import { PRESETS } from './lib/wallpaper'
import {
  activeWindow,
  apps,
  closeWindow,
  focusWindow,
  isPinned,
  launchApp,
  moveCard,
  notesOpen,
  openFiles,
  openIceStore,
  openSettings,
  openTerminal,
  panelOpen,
  prefs,
  refreshApps,
  savePrefs,
  setNotes,
  setPicture,
  showPanel,
  toast,
  toggleCard,
  toggleMax,
  toggleMin,
  togglePin,
  unread,
  user,
  type CtxItem,
  type PanelCard,
  type Win,
} from './store'
import { strings } from './strings'

const admin = () => !!user.value?.admin
const say = (title: string) => (err?: string) => err && toast({ title, body: err })

/** A different preset each time. */
export function shuffleWallpaper() {
  const w = prefs.value.wallpaper
  const pool = PRESETS.filter((p) => !(w.kind === 'preset' && p.id === w.preset))
  const pick = pool[Math.floor(Math.random() * pool.length)]!
  return savePrefs({ wallpaper: { ...w, kind: 'preset', preset: pick.id } }).then(say(strings.wallpaperHeading))
}

const panelItems = (): CtxItem[] => [
  { id: 'panel', label: strings.panelShow, icon: 'cards', checked: panelOpen.value, run: () => showPanel(!panelOpen.value) },
  { id: 'cards', label: strings.ctxAddCards, icon: 'plus', run: () => openSettings('widgets') },
]

/** The desktop's empty space. */
export function desktopMenu(): CtxItem[][] {
  const open: CtxItem[] = []
  if (admin()) open.push({ id: 'files', label: strings.files, icon: 'folder', run: () => openFiles() })
  open.push({ id: 'icestore', label: strings.iceStore, icon: 'snowflake', run: () => openIceStore() })
  if (admin()) open.push({ id: 'terminal', label: strings.terminal, icon: 'terminal', run: openTerminal })
  return [
    [
      { id: 'wallpaper', label: strings.ctxWallpaper, icon: 'image', run: () => openSettings('appearance') },
      { id: 'shuffle', label: strings.ctxShuffle, icon: 'shuffle', run: shuffleWallpaper },
    ],
    panelItems(),
    [{ id: 'refresh', label: strings.refresh, icon: 'refresh', run: refreshApps }, ...open],
    [{ id: 'settings', label: strings.settings, icon: 'settings', run: () => openSettings() }],
  ]
}

/** One card on the panel, at index i of n. */
export function cardMenu(c: PanelCard, i: number, n: number, reload?: () => void): CtxItem[][] {
  const own: CtxItem[] = []
  if (c.id === 'clock') {
    own.push(
      { id: '12', label: strings.clock12, icon: 'clock', checked: prefs.value.clock !== '24', run: () => savePrefs({ clock: '12' }) },
      { id: '24', label: strings.clock24, icon: 'clock', checked: prefs.value.clock === '24', run: () => savePrefs({ clock: '24' }) },
    )
  }
  if (c.id === 'apps') own.push({ id: 'icestore', label: strings.iceStore, icon: 'snowflake', run: () => openIceStore() })
  if (!c.builtin && reload) own.push({ id: 'reload', label: strings.ctxReload, icon: 'refresh', run: reload })
  return [
    own,
    [
      { id: 'up', label: strings.ctxMoveUp, icon: 'up', disabled: i === 0, run: () => moveCard(c.id, -1) },
      { id: 'down', label: strings.ctxMoveDown, icon: 'down', disabled: i === n - 1, run: () => moveCard(c.id, 1) },
      { id: 'off', label: strings.removeFromPanel, icon: 'close', run: () => toggleCard(c.id) },
    ],
    [
      { id: 'cards', label: strings.ctxCardSettings, icon: 'cards', run: () => openSettings('widgets') },
      { id: 'hide', label: strings.ctxHidePanel, icon: 'eyeOff', run: () => showPanel(false) },
    ],
  ]
}

/** The panel's empty space. */
export const panelMenu = (): CtxItem[][] => [panelItems()]

/** The name a card goes by in its menu. */
export const cardTitle = (c: PanelCard) => (c.builtin ? (strings.cardName[c.id] ?? c.id) : c.mod.name)

const hideable = ['files', 'icestore', 'appstore', 'terminal']

/** A launcher in the dock. */
export function launcherMenu(l: Launcher, open: () => void): CtxItem[][] {
  const first: CtxItem[] = [{ id: 'open', label: strings.ctxOpen, icon: 'arrowRight', run: open }]
  if (l.url) first.push({ id: 'tab', label: strings.openInNewTab, icon: 'window', run: () => void window.open(l.url, '_blank', 'noopener') })
  if (l.id === 'trash') first.splice(0, 1, { id: 'open', label: strings.ctxOpenTrash, icon: 'trash', run: () => openFiles('trash') })
  const pages: CtxItem[] =
    l.id === 'settings'
      ? [
          { id: 'appearance', label: strings.settingsAppearance, icon: 'palette', run: () => openSettings('appearance') },
          { id: 'mods', label: strings.settingsMods, icon: 'grid', run: () => openSettings('mods') },
          { id: 'widgets', label: strings.settingsWidgets, icon: 'cards', run: () => openSettings('widgets') },
          { id: 'about', label: strings.settingsAbout, icon: 'info', run: () => openSettings('about') },
          { id: 'stock', label: strings.openStockSettings(host.name), icon: 'external', run: () => void window.open(dashboardUrl(), '_blank', 'noopener') },
        ]
      : []
  const dock: CtxItem[] = []
  if (hideable.includes(l.id)) {
    dock.push({
      id: 'hide',
      label: strings.ctxRemoveFromDock,
      icon: 'minus',
      run: () => savePrefs({ dock: { ...prefs.value.dock, hidden: [...prefs.value.dock.hidden, l.id as ice.DockLauncher] } }).then(say(strings.settingsDock)),
    })
  }
  dock.push({ id: 'dock', label: strings.ctxDockSettings, icon: 'dock', run: () => openSettings('dock') })
  return [first, pages, dock]
}

/** An open window, in the dock or by its title bar. */
export function windowMenu(w: Win): CtxItem[][] {
  const front = activeWindow.value?.id === w.id
  const look: CtxItem[] = [
    w.min
      ? { id: 'show', label: strings.restoreWindow(w.title), icon: 'window', run: () => toggleMin(w.id) }
      : front
        ? { id: 'min', label: strings.minimize, icon: 'minus', run: () => toggleMin(w.id) }
        : { id: 'front', label: strings.ctxOpen, icon: 'window', run: () => focusWindow(w.id) },
    { id: 'max', label: w.max ? strings.restoreSize : strings.maximize, icon: 'maximize', run: () => toggleMax(w.id) },
  ]
  const app: CtxItem[] = []
  const a = w.kind === 'app' ? apps.value.find((x) => x.id === w.app) : undefined
  if (a) {
    app.push({ id: 'tab', label: strings.openInNewTab, icon: 'external', run: () => launchApp(a, true) })
    app.push({
      id: 'pin',
      label: isPinned(a.id) ? strings.unpinFromDock : strings.pinToDock,
      icon: 'pin',
      run: () => togglePin(a.id).then(say(strings.settingsDock)),
    })
  }
  return [look, app, [{ id: 'close', label: strings.close, icon: 'close', danger: true, run: () => closeWindow(w.id) }]]
}

/** The dock's own background. */
export const dockMenu = (): CtxItem[][] => [[{ id: 'dock', label: strings.ctxDockSettings, icon: 'dock', run: () => openSettings('dock') }]]

/** The clock in the top bar. */
export const clockMenu = (): CtxItem[][] => [
  [
    { id: '12', label: strings.clock12, icon: 'clock', checked: prefs.value.clock !== '24', run: () => savePrefs({ clock: '12' }) },
    { id: '24', label: strings.clock24, icon: 'clock', checked: prefs.value.clock === '24', run: () => savePrefs({ clock: '24' }) },
  ],
  panelItems(),
  [{ id: 'appearance', label: strings.ctxAppearance, icon: 'palette', run: () => openSettings('appearance') }],
]

/** The bell. */
export function bellMenu(): CtxItem[][] {
  const n = prefs.value.notify
  return [
    [
      { id: 'show', label: strings.ctxShowNotifications, icon: 'bell', run: () => (notesOpen.value = true) },
      { id: 'read', label: strings.markAllRead, icon: 'checks', disabled: !unread.value, run: () => ice.markRead().then(setNotes).catch(() => undefined) },
      { id: 'clear', label: strings.clearAll, icon: 'trash', run: () => ice.clearNotifications().then(setNotes).catch(() => undefined) },
    ],
    [{ id: 'quiet', label: strings.notifyQuiet, icon: 'eyeOff', checked: n.quiet, run: () => savePrefs({ notify: { ...n, quiet: !n.quiet } }) }],
    [{ id: 'settings', label: strings.ctxNotifySettings, icon: 'settings', run: () => openSettings('notifications') }],
  ]
}

/** The avatar. */
export function accountMenu(): CtxItem[][] {
  const pic: CtxItem[] = [{ id: 'picture', label: strings.ctxChangePicture, icon: 'user', run: () => openSettings('users') }]
  if (prefs.value.avatar) pic.push({ id: 'drop', label: strings.ctxRemovePicture, icon: 'trash', run: () => setPicture('avatar', null).then(say(strings.avatarHeading)) })
  return [pic, [{ id: 'signout', label: strings.menuSignOut, icon: 'arrowLeft', run: signOut }]]
}

/** The top bar's empty space. */
export const topBarMenu = (): CtxItem[][] => [
  [
    { id: 'appearance', label: strings.ctxAppearance, icon: 'palette', run: () => openSettings('appearance') },
    { id: 'about', label: strings.menuAbout, icon: 'info', run: () => openSettings('about') },
  ],
  [{ id: 'settings', label: strings.settings, icon: 'settings', run: () => openSettings() }],
]
