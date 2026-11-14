<script setup lang="ts">
// The dock: search, IceDash's own launchers (Files, IceStore, App Store,
// Terminal), the open windows (minimized ones go here), then Settings and
// Trash. Installed apps live on the desktop.
import { computed } from 'vue'
import AppIcon from '../components/AppIcon.vue'
import Icon from '../components/Icon.vue'
import type { IconName } from '../lib/icons'
import { desktopLaunchers, dockEnd, openLauncher, type Launcher } from '../lib/launchers'
import { dockMenu, launcherMenu, windowMenu } from '../menus'
import { activeWindow, apps, focusWindow, iceStoreInZimaOS, openApp, openContext, openFiles, openIceStore, openMenu, openSettings, openTerminal, pinnedApps, prefs, toggleMin, user, windows, type Win } from '../store'
import { strings } from '../strings'
import SearchBar from './SearchBar.vue'

const admin = computed(() => !!user.value?.admin)
const launchers = computed(() =>
  desktopLaunchers(location, { admin: admin.value, iceStoreInZimaOS: iceStoreInZimaOS.value }).filter(
    (l) => !(prefs.value.dock.hidden as string[]).includes(l.id),
  ),
)
/** Pinned apps that have no window open (an open one shows with the windows). */
const pinned = computed(() => pinnedApps.value.filter((a) => !windows.some((w) => w.kind === 'app' && w.app === a.id)))
const end = computed(() => dockEnd({ admin: admin.value }))

/** IceStore, Files, Trash and Terminal open IceDash's windows; the others follow their link. */
function launch(e: MouseEvent, l: Launcher) {
  if (l.open) {
    e.preventDefault()
    openLauncher(l, { files: openFiles, terminal: openTerminal, settings: () => openSettings() })
    return
  }
  if (l.id !== 'icestore' || e.ctrlKey || e.metaKey || e.shiftKey || e.button !== 0) return
  e.preventDefault()
  openIceStore()
}

/** The same as a click: IceDash's window, or the launcher's page in a new tab. */
function runLauncher(l: Launcher) {
  if (l.open) openLauncher(l, { files: openFiles, terminal: openTerminal, settings: () => openSettings() })
  else if (l.id === 'icestore') openIceStore()
  else window.open(l.url, '_blank', 'noopener')
}
const launcherCtx = (e: MouseEvent, l: Launcher) => openContext(e, l.name, launcherMenu(l, () => runLauncher(l)))

const kindIcon: Record<string, IconName> = {
  notifications: 'bell',
  icestore: 'snowflake',
  logs: 'lines',
  credentials: 'key',
  app: 'app',
  files: 'folder',
  terminal: 'terminal',
  settings: 'settings',
}
const appOf = (w: Win) => apps.value.find((a) => a.id === w.app)

/** Minimized: bring it back. In front: minimize it. Behind: bring it to the front. */
function pick(w: Win) {
  if (w.min) toggleMin(w.id)
  else if (activeWindow.value?.id === w.id) toggleMin(w.id)
  else focusWindow(w.id)
}
</script>

<template>
  <nav class="dock" aria-label="Dock" @contextmenu.self="openContext($event, strings.ctxDock, dockMenu())">
    <SearchBar />
    <a
      v-for="l in launchers"
      :key="l.id"
      class="slot"
      :href="l.url || undefined"
      target="_blank"
      rel="noopener"
      :title="l.name"
      @click="launch($event, l)"
      @contextmenu="launcherCtx($event, l)"
    >
      <span class="tile"><Icon :name="l.icon" :size="21" /></span>
      <span class="sr">{{ l.name }}</span>
    </a>
    <template v-if="pinned.length">
      <span class="divider" aria-hidden="true" />
      <button
        v-for="a in pinned"
        :key="a.id"
        type="button"
        class="slot app"
        :title="a.name"
        :data-dock-app="a.id"
        @click="openApp(a)"
        @contextmenu.prevent="openMenu(a.id, $event)"
      >
        <AppIcon :src="a.icon" :name="a.name" :size="44" />
        <span class="sr">{{ a.name }}</span>
      </button>
    </template>
    <template v-if="windows.length">
      <span class="divider" aria-hidden="true" />
      <button
        v-for="w in windows"
        :key="w.id"
        type="button"
        class="slot win"
        :class="{ min: w.min }"
        :data-dock-win="w.id"
        :title="w.min ? strings.restoreWindow(w.title) : w.title"
        @click="pick(w)"
        @contextmenu="openContext($event, w.title, windowMenu(w))"
      >
        <AppIcon v-if="w.kind === 'app'" :src="appOf(w)?.icon" :name="w.title" :size="44" />
        <span v-else class="tile"><Icon :name="kindIcon[w.kind] ?? 'app'" :size="21" /></span>
        <span class="run" aria-hidden="true" />
        <span class="sr">{{ w.title }}</span>
      </button>
    </template>
    <span class="divider" aria-hidden="true" />
    <a v-for="l in end" :key="l.id" class="slot" :href="l.url || undefined" target="_blank" rel="noopener" :title="l.name" @click="launch($event, l)" @contextmenu="launcherCtx($event, l)">
      <span class="tile"><Icon :name="l.icon" :size="21" /></span>
      <span class="sr">{{ l.name }}</span>
    </a>
  </nav>
</template>

<style scoped>
.dock {
  display: flex;
  align-items: center;
  gap: 8px;
  max-width: 100%;
  overflow-x: auto;
  padding: 9px 11px 11px;
  border-radius: 20px;
  background: rgba(var(--surface-rgb), 0.6);
  backdrop-filter: var(--blur);
  -webkit-backdrop-filter: var(--blur);
  border: 1px solid var(--line);
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.55);
  scrollbar-width: none;
}
.dock::-webkit-scrollbar {
  display: none;
}
.slot {
  position: relative;
  flex: none;
  display: grid;
  place-items: center;
  padding: 0;
  border: 0;
  border-radius: 13px;
  background: none;
  color: var(--ac2);
  cursor: pointer;
  transition: transform 0.15s ease;
}
.slot:hover,
.slot:focus-visible {
  transform: translateY(-3px);
}
.dock :deep(.tile) {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 13px;
  background: linear-gradient(160deg, rgba(var(--fg-rgb), 0.07), rgba(var(--fg-rgb), 0.02));
  border: 1px solid var(--line);
  color: var(--ac2);
}
.run {
  position: absolute;
  bottom: -7px;
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: var(--ac2);
}
.win.min .run {
  background: var(--mut);
}
.win.min :deep(img),
.win.min .tile {
  opacity: 0.75;
}
.divider {
  flex: none;
  width: 1px;
  height: 30px;
  margin: 0 3px;
  background: var(--line);
}
.sr {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
}
</style>
