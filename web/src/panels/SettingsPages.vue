<script setup lang="ts">
// Settings pages from Phase 5a (PLAN.md 5a): General, Appearance, Dock,
// Notifications and Users. Each user's own choices are saved for them;
// General's switches are the server's and only an admin changes them.
import { computed, onMounted, ref, watch } from 'vue'
import * as ice from '../api/icedash'
import * as modsApi from '../api/mods'
import AppIcon from '../components/AppIcon.vue'
import Icon from '../components/Icon.vue'
import { dashboardUrl, host } from '../lib/host'
import { desktopLaunchers } from '../lib/launchers'
import { FROST, FROST_ID, ICE } from '../lib/theme'
import { cityOf } from '../lib/timezone'
import {
  apps,
  autoCheck,
  iceStoreInZimaOS,
  modsView,
  openSettings,
  prefs,
  savePrefs,
  setModsView,
  setNotes,
  sys,
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
    <p v-if="usersError" class="warnline" role="alert">{{ usersError }}</p>
    <ul v-if="accounts" class="rows">
      <li v-for="a in accounts" :key="a.name" class="row">
        <span class="avatar" aria-hidden="true">{{ a.name.slice(0, 2).toUpperCase() }}</span>
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
  border-radius: 50%;
  background: var(--ac);
  color: var(--ink);
  font: 700 12px var(--font-mono);
  flex: none;
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
