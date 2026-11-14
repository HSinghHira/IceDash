<script setup lang="ts">
// Settings pages from Phase 5a (PLAN.md 5a): General, Appearance, Dock,
// Notifications and Users. Each user's own choices are saved for them;
// General's switches are the server's and only an admin changes them.
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as ice from '../api/icedash'
import * as modsApi from '../api/mods'
import AppIcon from '../components/AppIcon.vue'
import AvatarEditor from '../components/AvatarEditor.vue'
import Icon from '../components/Icon.vue'
import { dashboardUrl, host } from '../lib/host'
import { desktopLaunchers } from '../lib/launchers'
import { FROST, FROST_ID, ICE } from '../lib/theme'
import { cityOf } from '../lib/timezone'
import { initials } from '../lib/format'
import { prepareWallpaper } from '../lib/pictures'
import { PRESETS, SWATCHES, wallStyle } from '../lib/wallpaper'
import { shuffleWallpaper } from '../menus'
import {
  activeTheme,
  apps,
  autoCheck,
  avatarUrl,
  iceStoreInZimaOS,
  modsView,
  openSettings,
  prefs,
  savePrefs,
  setModsView,
  setNotes,
  setPicture,
  sys,
  wallpaperUrl,
  toast,
  user,
  zone,
  zoneSaved,
  type SettingsPage,
} from '../store'
import { strings } from '../strings'

const props = defineProps<{ page: SettingsPage }>()
const admin = computed(() => !!user.value?.admin)
const msg = (e: unknown) => (e instanceof Error ? e.message : String(e))

async function save(change: Partial<ice.Prefs>, title: string) {
  const err = await savePrefs(change)
  if (err) toast({ title, body: err })
}

// ---- General ----
const settings = ref<ice.Settings | null>(null)
async function loadSettings() {
  try {
    settings.value = await ice.getSettings()
  } catch {
    settings.value = null
  }
}
async function setSetting(change: Partial<ice.Settings>) {
  const before = settings.value
  if (before) settings.value = { ...before, ...change }
  try {
    settings.value = await ice.saveSettings(change)
    autoCheck.value = settings.value.auto_check
  } catch (e) {
    settings.value = before
    toast({ title: strings.updatesHeading, body: msg(e) })
  }
}
/** Night installs count the night in the clock's zone, saved with the switch. */
const toggleNight = () =>
  setSetting(settings.value?.auto_install ? { auto_install: false } : { auto_install: true, zone: zone.value })
const nightZone = computed(() => cityOf(settings.value?.zone || zone.value))

// ---- Appearance ----
const theme = computed(() => modsView.value.prefs.theme)
const themes = computed(() => [
  { id: '', name: strings.iceName, about: strings.iceAbout, t: ICE },
  { id: FROST_ID, name: strings.frostName, about: strings.frostAbout, t: FROST },
  ...modsView.value.mods
    .filter((m) => m.kind === 'theme')
    .map((m) => ({ id: m.id, name: m.name, about: strings.byAuthor(m.author), t: modsApi.themeOf(m) ?? ICE })),
])
async function applyTheme(id: string) {
  try {
    setModsView(await modsApi.setTheme(id))
  } catch (e) {
    toast({ title: strings.themeHeading, body: msg(e) })
  }
}

// ---- Appearance → Wallpaper ----
const wp = computed(() => prefs.value.wallpaper)
const themeWall = computed(() => activeTheme.value?.wallpaper?.startsWith('/v2/icedash/mods/') ? activeTheme.value.wallpaper : '')
/** What the preview shows: the same layer the desktop draws, or the theme's background. */
const preview = computed(() => wallStyle(wp.value, wallpaperUrl.value, !!themeWall.value) ?? (themeWall.value ? { background: `url("${themeWall.value}") center / cover`, } : { background: 'var(--bg)' }))
const setWall = (change: Partial<ice.Wallpaper>) => save({ wallpaper: { ...wp.value, ...change } }, strings.wallpaperHeading)
/** Sliders show at once and save when let go. */
function slide(key: 'dim' | 'blur', e: Event) {
  prefs.value = { ...prefs.value, wallpaper: { ...wp.value, [key]: Number((e.target as HTMLInputElement).value) } }
}
const wallFile = ref<HTMLInputElement | null>(null)
const wallBusy = ref(false)
const wallDrop = ref(false)
async function useWallpaper(f: File | undefined) {
  if (!f) return
  wallBusy.value = true
  try {
    const err = await setPicture('wallpaper', await prepareWallpaper(f))
    if (err) toast({ title: strings.wallpaperHeading, body: err })
  } catch (e) {
    toast({ title: strings.wallpaperHeading, body: msg(e) })
  } finally {
    wallBusy.value = false
    if (wallFile.value) wallFile.value.value = ''
  }
}
function dropWall(e: DragEvent) {
  wallDrop.value = false
  const f = [...(e.dataTransfer?.files ?? [])].find((x) => x.type.startsWith('image/'))
  if (f) void useWallpaper(f)
}
const hasFiles = (e: DragEvent) => !!e.dataTransfer?.types.includes('Files')
async function dropPicture() {
  const err = await setPicture('wallpaper', null)
  if (err) toast({ title: strings.wallpaperHeading, body: err })
}
const fits: { id: ice.Wallpaper['fit']; name: string }[] = [
  { id: '', name: strings.fitCover },
  { id: 'contain', name: strings.fitContain },
  { id: 'center', name: strings.fitCenter },
  { id: 'tile', name: strings.fitTile },
]

// ---- Users → your picture ----
const avatarFile = ref<HTMLInputElement | null>(null)
const editing = ref<File | null>(null)
function pickAvatar(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (f) editing.value = f
  ;(e.target as HTMLInputElement).value = ''
}
async function saveAvatar(b: Blob) {
  editing.value = null
  const err = await setPicture('avatar', b)
  if (err) toast({ title: strings.avatarHeading, body: err })
  else void loadAvatars()
}
async function dropAvatar() {
  const err = await setPicture('avatar', null)
  if (err) toast({ title: strings.avatarHeading, body: err })
}
/** Other people's pictures, as object URLs. */
const avatars = ref<Record<string, string>>({})
async function loadAvatars() {
  for (const a of accounts.value ?? []) {
    if (!a.avatar || a.name === user.value?.name || avatars.value[a.name]) continue
    const b = await ice.avatarOf(a.name)
    if (b) avatars.value = { ...avatars.value, [a.name]: URL.createObjectURL(b) }
  }
}
onBeforeUnmount(() => Object.values(avatars.value).forEach((u) => URL.revokeObjectURL(u)))
const avatarFor = (name: string) => (name === user.value?.name ? avatarUrl.value : avatars.value[name])

// ---- Dock ----
const launchers = computed(() => desktopLaunchers(location, { admin: admin.value, iceStoreInZimaOS: iceStoreInZimaOS.value }))
const shown = (id: string) => !(prefs.value.dock.hidden as string[]).includes(id)
function toggleLauncher(id: ice.DockLauncher) {
  const hidden = prefs.value.dock.hidden
  void save({ dock: { ...prefs.value.dock, hidden: hidden.includes(id) ? hidden.filter((x) => x !== id) : [...hidden, id] } }, strings.settingsDock)
}
const pinned = computed(() =>
  prefs.value.dock.pinned.map((id) => ({ id, app: apps.value.find((a) => a.id === id) })).filter((x) => !!x.app),
)
const unpinned = computed(() => apps.value.filter((a) => !prefs.value.dock.pinned.includes(a.id)))
const pick = ref('')
function setPinned(list: string[]) {
  void save({ dock: { ...prefs.value.dock, pinned: list } }, strings.settingsDock)
}
function addPinned() {
  if (!pick.value) return
  setPinned([...prefs.value.dock.pinned, pick.value])
  pick.value = ''
}
function move(id: string, by: number) {
  const list = [...prefs.value.dock.pinned]
  const i = list.indexOf(id)
  const j = i + by
  if (i < 0 || j < 0 || j >= list.length) return
  ;[list[i], list[j]] = [list[j]!, list[i]!]
  setPinned(list)
}
// Drag to reorder.
const dragging = ref('')
function drop(before: string) {
  const id = dragging.value
  dragging.value = ''
  if (!id || id === before) return
  const list = prefs.value.dock.pinned.filter((x) => x !== id)
  list.splice(list.indexOf(before), 0, id)
  setPinned(list)
}

// ---- Notifications ----
const groups: { id: ice.NotifyGroup; name: string; hint: string }[] = [
  { id: 'updates', name: strings.notifyUpdates, hint: strings.notifyUpdatesHint },
  { id: 'apps', name: strings.notifyApps, hint: strings.notifyAppsHint },
  { id: 'stopped', name: strings.notifyStopped, hint: strings.notifyStoppedHint },
]
const popsUp = (g: ice.NotifyGroup) => !prefs.value.notify.off.includes(g)
function toggleGroup(g: ice.NotifyGroup) {
  const off = prefs.value.notify.off
  void save({ notify: { ...prefs.value.notify, off: off.includes(g) ? off.filter((x) => x !== g) : [...off, g] } }, strings.settingsNotifications)
}
const cleared = ref(false)
async function clearAll() {
  try {
    setNotes(await ice.clearNotifications())
    cleared.value = true
  } catch (e) {
    toast({ title: strings.settingsNotifications, body: msg(e) })
  }
}

// ---- Users ----
const accounts = ref<ice.Account[] | null>(null)
const usersError = ref('')
async function loadUsers() {
  usersError.value = ''
  try {
    accounts.value = await ice.accounts()
    void loadAvatars()
  } catch {
    usersError.value = strings.usersFailed
  }
}

function enter(p: SettingsPage) {
  if (p === 'general' && !settings.value) void loadSettings()
  if (p === 'users' && !accounts.value) void loadUsers()
  if (p === 'notifications') cleared.value = false
}
onMounted(() => enter(props.page))
watch(() => props.page, enter)
</script>

<template>
  <section v-if="page === 'general'" class="page">
    <h2>{{ strings.settingsGeneral }}</h2>
    <p class="mut lead">{{ strings.generalHint }}</p>

    <h3 class="kicker">{{ strings.updatesHeading }}</h3>
    <div class="rows">
      <div class="row">
        <div class="who">
          <strong>{{ strings.autoCheck }}</strong>
          <span class="mut small">{{ strings.autoCheckHint }}</span>
        </div>
        <button
          type="button"
          class="switch"
          role="switch"
          :aria-checked="settings?.auto_check ?? true"
          :aria-label="strings.autoCheck"
          :disabled="!admin || !settings"
          @click="setSetting({ auto_check: !settings?.auto_check })"
        >
          <span />
        </button>
      </div>
      <div class="row">
        <div class="who">
          <strong>{{ strings.autoInstall }}</strong>
          <span class="mut small">{{ strings.autoInstallHint(nightZone) }}</span>
        </div>
        <button
          type="button"
          class="switch"
          role="switch"
          :aria-checked="!!settings?.auto_install"
          :aria-label="strings.autoInstall"
          :disabled="!admin || !settings"
          @click="toggleNight"
        >
          <span />
        </button>
      </div>
    </div>
    <p v-if="!admin" class="mut small">{{ strings.adminOnlyHere }}</p>

    <h3 class="kicker">{{ strings.serverHeading }}</h3>
    <dl class="facts">
      <dt>{{ strings.serverSystem }}</dt>
      <dd>{{ host.name }}</dd>
      <dt>{{ strings.serverName }}</dt>
      <dd class="mono">{{ sys?.device || '–' }}</dd>
      <dt>{{ strings.serverZone }}</dt>
      <dd>
        <span class="mono">{{ zone }}</span>
        <span class="mut small">· {{ zoneSaved ? strings.serverZoneSaved : strings.serverZoneBrowser }}</span>
      </dd>
    </dl>
    <a class="link" :href="dashboardUrl()" target="_blank" rel="noopener">
      {{ strings.changeInStock(host.name) }} <Icon name="external" :size="13" />
    </a>
  </section>

  <section v-else-if="page === 'appearance'" class="page">
    <h2>{{ strings.settingsAppearance }}</h2>
    <p class="mut lead">{{ strings.appearanceHint }}</p>

    <h3 class="kicker">{{ strings.themeHeading }}</h3>
    <div class="themes" role="radiogroup" :aria-label="strings.themeHeading">
      <button
        v-for="t in themes"
        :key="t.id || 'ice'"
        type="button"
        role="radio"
        class="theme"
        :class="{ on: theme === t.id }"
        :aria-checked="theme === t.id"
        @click="applyTheme(t.id)"
      >
        <span class="swatch" :style="{ background: t.t.colors.bg, borderColor: t.t.colors.text + '22' }">
          <i :style="{ background: t.t.colors.surface }" />
          <b :style="{ background: t.t.colors.accent }" />
          <b :style="{ background: t.t.colors.accent2 }" />
        </span>
        <span class="who">
          <strong>{{ t.name }}</strong>
          <span class="mut small">{{ t.about }}</span>
        </span>
        <Icon v-if="theme === t.id" name="check" :size="16" class="tick" />
      </button>
    </div>
    <button type="button" class="link" @click="openSettings('mods')">{{ strings.moreThemes }} <Icon name="arrowRight" :size="13" /></button>

    <h3 class="kicker">{{ strings.wallpaperHeading }}</h3>
    <div
      class="wallbox"
      :class="{ dropping: wallDrop }"
      @dragover.prevent="hasFiles($event) && (wallDrop = true)"
      @dragleave.self="wallDrop = false"
      @drop.prevent="dropWall"
    >
      <div class="wprev" aria-hidden="true">
        <div class="wprev-img" :style="{ ...preview, filter: wp.blur ? `blur(${wp.blur / 4}px)` : undefined }" />
        <div class="wprev-dim" :style="{ opacity: wp.dim / 100 }" />
        <span class="wbar" />
        <span class="wcard" />
        <span class="wcard two" />
        <span class="wdock"><i /><i /><i /><i /></span>
        <span v-if="wallDrop" class="wdrop">{{ strings.wallDrop }}</span>
      </div>
      <div class="walls" role="radiogroup" :aria-label="strings.wallpaperHeading">
        <button type="button" role="radio" class="wall" :class="{ on: wp.kind === '' }" :aria-checked="wp.kind === ''" :title="strings.wallTheme" @click="setWall({ kind: '' })">
          <span class="wsw" :style="themeWall ? { background: `url('${themeWall}') center / cover` } : { background: 'var(--bg)' }" />
          <span class="wname">{{ strings.wallTheme }}</span>
        </button>
        <button
          v-for="p in PRESETS"
          :key="p.id"
          type="button"
          role="radio"
          class="wall"
          :class="{ on: wp.kind === 'preset' && wp.preset === p.id }"
          :aria-checked="wp.kind === 'preset' && wp.preset === p.id"
          :title="p.name"
          @click="setWall({ kind: 'preset', preset: p.id })"
        >
          <span class="wsw" :style="{ background: p.css }" />
          <span class="wname">{{ p.name }}</span>
        </button>
        <button
          v-if="wallpaperUrl"
          type="button"
          role="radio"
          class="wall"
          :class="{ on: wp.kind === 'image' }"
          :aria-checked="wp.kind === 'image'"
          :title="strings.wallYours"
          @click="setWall({ kind: 'image' })"
        >
          <span class="wsw" :style="{ background: `url('${wallpaperUrl}') center / cover` }" />
          <span class="wname">{{ strings.wallYours }}</span>
        </button>
        <label class="wall" :class="{ on: wp.kind === 'color' }" :title="strings.wallColor">
          <span class="wsw" :style="{ background: wp.color || '#10202e' }"><Icon name="palette" :size="18" class="wicon" /></span>
          <span class="wname">{{ strings.wallColor }}</span>
          <input type="color" class="sr" :value="wp.color || '#10202e'" :aria-label="strings.wallColorPick" @change="setWall({ kind: 'color', color: ($event.target as HTMLInputElement).value })" />
        </label>
      </div>
    </div>
    <div v-if="wp.kind === 'color'" class="swatches">
      <button
        v-for="c in SWATCHES"
        :key="c"
        type="button"
        class="swatchbtn"
        :class="{ on: wp.color === c }"
        :style="{ background: c }"
        :aria-label="c"
        @click="setWall({ kind: 'color', color: c })"
      />
    </div>
    <div class="wactions">
      <input ref="wallFile" type="file" accept="image/*" class="sr" :aria-label="strings.wallUpload" @change="useWallpaper(($event.target as HTMLInputElement).files?.[0])" />
      <button type="button" class="ghost" :disabled="wallBusy" @click="wallFile?.click()">
        <Icon name="upload" :size="15" /> {{ wallBusy ? strings.wallUploading : wallpaperUrl ? strings.wallReplace : strings.wallUpload }}
      </button>
      <button v-if="wallpaperUrl" type="button" class="ghost" @click="dropPicture"><Icon name="trash" :size="15" /> {{ strings.wallRemovePic }}</button>
      <button type="button" class="ghost" @click="shuffleWallpaper"><Icon name="shuffle" :size="15" /> {{ strings.ctxShuffle }}</button>
    </div>
    <p class="mut small">{{ strings.wallHint }}</p>
    <div class="rows">
      <div v-if="wp.kind === 'image'" class="row">
        <div class="who"><strong>{{ strings.wallFit }}</strong></div>
        <div class="seg flat" role="radiogroup" :aria-label="strings.wallFit">
          <button v-for="f in fits" :key="f.id || 'cover'" type="button" role="radio" :aria-checked="wp.fit === f.id" :class="{ on: wp.fit === f.id }" @click="setWall({ fit: f.id })">
            {{ f.name }}
          </button>
        </div>
      </div>
      <div class="row">
        <div class="who"><strong>{{ strings.wallDim }}</strong></div>
        <input type="range" class="slider" min="0" max="80" step="5" :value="wp.dim" :aria-label="strings.wallDim" @input="slide('dim', $event)" @change="setWall({})" />
        <span class="mono small val">{{ wp.dim }}%</span>
      </div>
      <div class="row">
        <div class="who"><strong>{{ strings.wallBlur }}</strong></div>
        <input type="range" class="slider" min="0" max="30" step="1" :value="wp.blur" :aria-label="strings.wallBlur" @input="slide('blur', $event)" @change="setWall({})" />
        <span class="mono small val">{{ wp.blur }} px</span>
      </div>
    </div>
    <button v-if="wp.dim || wp.blur || wp.fit" type="button" class="link" @click="setWall({ dim: 0, blur: 0, fit: '' })">{{ strings.wallReset }}</button>

    <h3 class="kicker">{{ strings.clockHeading }}</h3>
    <div class="seg" role="radiogroup" :aria-label="strings.clockHeading">
      <button type="button" role="radio" :aria-checked="prefs.clock !== '24'" :class="{ on: prefs.clock !== '24' }" @click="save({ clock: '12' }, strings.clockHeading)">
        {{ strings.clock12 }}
      </button>
      <button type="button" role="radio" :aria-checked="prefs.clock === '24'" :class="{ on: prefs.clock === '24' }" @click="save({ clock: '24' }, strings.clockHeading)">
        {{ strings.clock24 }}
      </button>
    </div>

    <div class="rows">
      <div class="row">
        <div class="who">
          <strong>{{ strings.panelShow }}</strong>
          <span class="mut small">{{ strings.panelShowHint }}</span>
        </div>
        <button
          type="button"
          class="switch"
          role="switch"
          :aria-checked="!prefs.hide_panel"
          :aria-label="strings.panelShow"
          @click="save({ hide_panel: !prefs.hide_panel }, strings.panelHeading)"
        >
          <span />
        </button>
      </div>
      <div class="row">
        <div class="who">
          <strong>{{ strings.motionReduce }}</strong>
          <span class="mut small">{{ strings.motionReduceHint }}</span>
        </div>
        <button
          type="button"
          class="switch"
          role="switch"
          :aria-checked="prefs.motion === 'reduce'"
          :aria-label="strings.motionReduce"
          @click="save({ motion: prefs.motion === 'reduce' ? '' : 'reduce' }, strings.motionHeading)"
        >
          <span />
        </button>
      </div>
    </div>
  </section>

  <section v-else-if="page === 'dock'" class="page">
    <h2>{{ strings.settingsDock }}</h2>
    <p class="mut lead">{{ strings.dockHint }}</p>

    <h3 class="kicker">{{ strings.dockLaunchers }}</h3>
    <div class="rows">
      <div v-for="l in launchers" :key="l.id" class="row">
        <span class="tile"><Icon :name="l.icon" :size="19" /></span>
        <div class="who"><strong>{{ l.name }}</strong></div>
        <button
          type="button"
          class="switch"
          role="switch"
          :aria-checked="shown(l.id)"
          :aria-label="l.name"
          @click="toggleLauncher(l.id as ice.DockLauncher)"
        >
          <span />
        </button>
      </div>
    </div>

    <h3 class="kicker">{{ strings.dockPinned }}</h3>
    <p v-if="!pinned.length" class="mut small">{{ strings.dockPinnedNone }}</p>
    <ol class="rows pins">
      <li
        v-for="(p, i) in pinned"
        :key="p.id"
        class="row"
        :class="{ lifted: dragging === p.id }"
        draggable="true"
        @dragstart="dragging = p.id"
        @dragend="dragging = ''"
        @dragover.prevent
        @drop.prevent="drop(p.id)"
      >
        <span class="grip" aria-hidden="true"><Icon name="grip" :size="16" /></span>
        <AppIcon :src="p.app!.icon" :name="p.app!.name" :size="34" />
        <div class="who"><strong>{{ p.app!.name }}</strong></div>
        <button type="button" class="icon-btn" :disabled="i === 0" :aria-label="strings.dockMoveUp(p.app!.name)" :title="strings.dockMoveUp(p.app!.name)" @click="move(p.id, -1)">
          <Icon name="up" :size="15" />
        </button>
        <button
          type="button"
          class="icon-btn"
          :disabled="i === pinned.length - 1"
          :aria-label="strings.dockMoveDown(p.app!.name)"
          :title="strings.dockMoveDown(p.app!.name)"
          @click="move(p.id, 1)"
        >
          <Icon name="down" :size="15" />
        </button>
        <button type="button" class="icon-btn" :aria-label="strings.dockRemove(p.app!.name)" :title="strings.dockRemove(p.app!.name)" @click="setPinned(prefs.dock.pinned.filter((x) => x !== p.id))">
          <Icon name="close" :size="15" />
        </button>
      </li>
    </ol>
    <form v-if="unpinned.length" class="addpin" @submit.prevent="addPinned">
      <select v-model="pick" :aria-label="strings.dockAddApp">
        <option value="">{{ strings.dockAddPick }}</option>
        <option v-for="a in unpinned" :key="a.id" :value="a.id">{{ a.name }}</option>
      </select>
      <button type="submit" class="ghost" :disabled="!pick"><Icon name="plus" :size="15" /> {{ strings.dockAddApp }}</button>
    </form>
  </section>

  <section v-else-if="page === 'notifications'" class="page">
    <h2>{{ strings.settingsNotifications }}</h2>
    <p class="mut lead">{{ strings.notificationsHint }}</p>
    <div class="rows">
      <div class="row">
        <div class="who">
          <strong>{{ strings.notifyQuiet }}</strong>
          <span class="mut small">{{ strings.notifyQuietHint }}</span>
        </div>
        <button
          type="button"
          class="switch"
          role="switch"
          :aria-checked="prefs.notify.quiet"
          :aria-label="strings.notifyQuiet"
          @click="save({ notify: { ...prefs.notify, quiet: !prefs.notify.quiet } }, strings.settingsNotifications)"
        >
          <span />
        </button>
      </div>
      <div v-for="g in groups" :key="g.id" class="row" :class="{ dim: prefs.notify.quiet }">
        <div class="who">
          <strong>{{ g.name }}</strong>
          <span class="mut small">{{ g.hint }}</span>
        </div>
        <button type="button" class="switch" role="switch" :aria-checked="popsUp(g.id)" :aria-label="g.name" @click="toggleGroup(g.id)">
          <span />
        </button>
      </div>
    </div>
    <p class="actions">
      <button type="button" class="ghost" @click="clearAll"><Icon name="trash" :size="15" /> {{ strings.notifyClear }}</button>
      <span v-if="cleared" class="mut small" role="status">{{ strings.notifyCleared }}</span>
    </p>
  </section>

  <section v-else-if="page === 'users'" class="page">
    <h2>{{ strings.settingsUsers }}</h2>
    <p class="mut lead">{{ strings.usersHint(host.name) }}</p>

    <h3 class="kicker">{{ strings.avatarHeading }}</h3>
    <div class="row me">
      <span class="avatar xl" :aria-label="strings.avatarOf(user?.name ?? '')" role="img">
        <img v-if="avatarUrl" :src="avatarUrl" alt="" />
        <template v-else>{{ initials(user?.name ?? '') }}</template>
      </span>
      <div class="who">
        <strong>{{ user?.name }}</strong>
        <span class="mut small">{{ strings.avatarHint }}</span>
      </div>
      <input ref="avatarFile" type="file" accept="image/*" class="sr" :aria-label="strings.avatarChoose" @change="pickAvatar" />
      <button type="button" class="ghost" @click="avatarFile?.click()"><Icon name="user" :size="15" /> {{ avatarUrl ? strings.avatarChange : strings.avatarChoose }}</button>
      <button v-if="avatarUrl" type="button" class="icon-btn" :title="strings.avatarRemove" :aria-label="strings.avatarRemove" @click="dropAvatar">
        <Icon name="trash" :size="15" />
      </button>
    </div>
    <AvatarEditor v-if="editing" :file="editing" @save="saveAvatar" @cancel="editing = null" />

    <h3 class="kicker">{{ strings.settingsUsers }}</h3>
    <p v-if="usersError" class="warnline" role="alert">{{ usersError }}</p>
    <ul v-if="accounts" class="rows">
      <li v-for="a in accounts" :key="a.name" class="row">
        <span class="avatar" aria-hidden="true">
          <img v-if="avatarFor(a.name)" :src="avatarFor(a.name)" alt="" />
          <template v-else>{{ initials(a.name) }}</template>
        </span>
        <div class="who">
          <strong>
            {{ a.name }}
            <span v-if="a.name === user?.name" class="mut small">({{ strings.you }})</span>
          </strong>
        </div>
        <span class="chip" :class="{ ac: a.admin }">{{ a.admin ? strings.roleAdmin : strings.roleMember }}</span>
      </li>
    </ul>
    <p v-if="host.id === 'casaos'" class="mut small">{{ strings.usersOne }}</p>
    <a class="link" :href="dashboardUrl()" target="_blank" rel="noopener">
      {{ strings.changeInStock(host.name) }} <Icon name="external" :size="13" />
    </a>
  </section>
</template>

<style scoped>
.page {
  min-width: 0;
  overflow-y: auto;
  padding: 26px 28px 28px;
}
h2 {
  margin: 0;
  font-size: 30px;
  font-weight: 700;
  letter-spacing: -0.01em;
}
.lead {
  margin: 6px 0 22px;
  max-width: 520px;
  font-size: 14.5px;
}
.mut {
  color: var(--mut);
}
.small {
  font-size: 12.5px;
}
.mono {
  font-family: var(--font-mono);
}
.kicker {
  margin: 24px 0 10px;
  color: var(--ac2);
  font: 600 11px var(--font-mono);
  letter-spacing: 0.12em;
  text-transform: uppercase;
}
.rows {
  display: grid;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.row {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 12px 16px;
  border-radius: 16px;
  border: 1px solid var(--line);
  background: var(--card);
}
.row.dim {
  opacity: 0.55;
}
.row.lifted {
  opacity: 0.5;
}
.who {
  flex: 1;
  min-width: 0;
  display: grid;
  gap: 2px;
}
.who strong {
  font-size: 15px;
  font-weight: 600;
}
.tile {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  border-radius: 11px;
  border: 1px solid var(--line);
  background: rgba(var(--fg-rgb), 0.05);
  color: var(--ac2);
  flex: none;
}
.grip {
  color: var(--mut);
  cursor: grab;
}
.switch {
  position: relative;
  flex: none;
  width: 52px;
  height: 30px;
  border: 0;
  border-radius: 99px;
  background: rgba(var(--fg-rgb), 0.15);
  cursor: pointer;
}
.switch:disabled {
  opacity: 0.5;
  cursor: default;
}
.switch span {
  position: absolute;
  top: 4px;
  left: 4px;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: var(--text);
  transition: transform 0.15s;
}
.switch[aria-checked='true'] {
  background: var(--ac);
}
.switch[aria-checked='true'] span {
  background: var(--ink);
  transform: translateX(22px);
}
.facts {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 8px 18px;
  margin: 0 0 14px;
  font-size: 14px;
}
.facts dt {
  color: var(--mut);
}
.facts dd {
  margin: 0;
  min-width: 0;
  overflow-wrap: anywhere;
}
.link {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-top: 12px;
  padding: 0;
  border: 0;
  background: none;
  color: var(--ac2);
  font: inherit;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  text-decoration: none;
}
.link:hover {
  text-decoration: underline;
}
.themes {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 10px;
}
.theme {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px 10px 10px;
  border: 1px solid var(--line);
  border-radius: 16px;
  background: var(--card);
  color: var(--text);
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.theme.on {
  border-color: var(--ac);
  box-shadow: 0 0 0 1px var(--ac) inset;
}
.swatch {
  position: relative;
  flex: none;
  width: 56px;
  height: 40px;
  border-radius: 10px;
  border: 1px solid;
  overflow: hidden;
}
.swatch i {
  position: absolute;
  right: 6px;
  top: 6px;
  width: 22px;
  height: 12px;
  border-radius: 4px;
}
.swatch b {
  position: absolute;
  bottom: 6px;
  left: 7px;
  width: 9px;
  height: 9px;
  border-radius: 50%;
}
.swatch b + b {
  left: 19px;
}
.tick {
  color: var(--ac);
}
.seg {
  display: inline-flex;
  padding: 4px;
  gap: 4px;
  margin-bottom: 14px;
  border-radius: 99px;
  border: 1px solid var(--line);
  background: var(--card);
}
.seg button {
  padding: 7px 18px;
  border: 0;
  border-radius: 99px;
  background: none;
  color: var(--mut);
  font: inherit;
  font-weight: 600;
  cursor: pointer;
}
.seg button.on {
  background: var(--ac);
  color: var(--ink);
}
.icon-btn {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: rgba(var(--fg-rgb), 0.04);
  color: var(--text);
  cursor: pointer;
}
.icon-btn:disabled {
  opacity: 0.35;
  cursor: default;
}
.ghost {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 8px 18px;
  border: 1px solid var(--line);
  border-radius: var(--radius-pill);
  background: rgba(var(--fg-rgb), 0.04);
  color: var(--text);
  font: inherit;
  font-weight: 600;
  cursor: pointer;
}
.ghost:disabled {
  opacity: 0.45;
  cursor: default;
}
.addpin {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  margin-top: 12px;
}
.addpin select {
  min-width: 200px;
  padding: 8px 12px;
  border: 1px solid var(--line);
  border-radius: 12px;
  background: rgba(var(--surface-rgb), 0.9);
  color: var(--text);
  font: inherit;
}
.actions {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 18px;
}
.avatar {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  overflow: hidden;
  border-radius: 50%;
  background: var(--ac);
  color: var(--ink);
  font: 700 12px var(--font-mono);
  flex: none;
}
.avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.avatar.xl {
  width: 64px;
  height: 64px;
  font-size: 20px;
}
.row.me {
  flex-wrap: wrap;
}
.sr {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
}
.wallbox {
  display: grid;
  grid-template-columns: minmax(220px, 300px) minmax(0, 1fr);
  gap: 16px;
  padding: 12px;
  border: 1px solid var(--line);
  border-radius: 18px;
  background: var(--card);
}
.wallbox.dropping {
  border-color: var(--ac);
  box-shadow: 0 0 0 1px var(--ac) inset;
}
.wprev {
  position: relative;
  aspect-ratio: 16 / 10;
  align-self: start;
  overflow: hidden;
  border-radius: 12px;
  border: 1px solid var(--line);
}
.wprev-img,
.wprev-dim {
  position: absolute;
  inset: -8px;
}
.wprev-dim {
  background: #000;
}
.wbar {
  position: absolute;
  inset: 0 0 auto;
  height: 8%;
  background: rgba(0, 0, 0, 0.35);
}
.wcard {
  position: absolute;
  right: 5%;
  top: 14%;
  width: 24%;
  height: 22%;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.14);
  border: 1px solid rgba(255, 255, 255, 0.16);
}
.wcard.two {
  top: 40%;
}
.wdock {
  position: absolute;
  left: 50%;
  bottom: 6%;
  display: flex;
  gap: 4px;
  padding: 4px 6px;
  border-radius: 7px;
  background: rgba(var(--surface-rgb), 0.7);
  transform: translateX(-50%);
}
.wdock i {
  width: 12px;
  height: 12px;
  border-radius: 3px;
  background: var(--ac);
  opacity: 0.8;
}
.wdrop {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  background: rgba(0, 0, 0, 0.55);
  color: #fff;
  font-weight: 600;
}
.walls {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(84px, 1fr));
  gap: 8px;
  align-content: start;
}
.wall {
  position: relative;
  display: grid;
  gap: 5px;
  padding: 4px;
  border: 1px solid transparent;
  border-radius: 12px;
  background: none;
  color: var(--text);
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.wall:hover {
  background: rgba(var(--fg-rgb), 0.05);
}
.wall.on {
  border-color: var(--ac);
  box-shadow: 0 0 0 1px var(--ac) inset;
}
.wall:focus-within {
  outline: 2px solid var(--ac2);
  outline-offset: 1px;
}
.wsw {
  display: grid;
  place-items: center;
  aspect-ratio: 16 / 10;
  border-radius: 8px;
  border: 1px solid var(--line);
}
.wicon {
  color: #fff;
  filter: drop-shadow(0 1px 2px rgba(0, 0, 0, 0.6));
}
.wname {
  padding: 0 2px;
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.swatches {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 10px;
}
.swatchbtn {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  border: 1px solid var(--line);
  cursor: pointer;
}
.swatchbtn.on {
  outline: 2px solid var(--ac);
  outline-offset: 2px;
}
.wactions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin: 12px 0 6px;
}
.slider {
  flex: 1;
  max-width: 260px;
  accent-color: var(--ac);
}
.val {
  min-width: 46px;
  text-align: right;
  color: var(--mut);
}
.seg.flat {
  margin: 0;
}
.seg.flat button {
  padding: 5px 12px;
  font-size: 13px;
}
@media (max-width: 760px) {
  .wallbox {
    grid-template-columns: 1fr;
  }
}
.chip {
  padding: 4px 12px;
  border-radius: 99px;
  border: 1px solid var(--line);
  color: var(--mut);
  font: 600 12px var(--font-mono);
}
.chip.ac {
  border-color: rgba(var(--ac-rgb), 0.5);
  color: var(--ac2);
}
.warnline {
  color: var(--warn);
}
@media (max-width: 700px) {
  .page {
    padding: 20px 16px;
  }
}
</style>
