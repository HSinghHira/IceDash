<script setup lang="ts">
// Settings (mock-up 5): Mods & Themes, Widgets and About. Themes and widgets
// come from collections: GitHub repos of plain folders, the official ones
// being IceColors and IceCards. Everyone picks their own theme and panel;
// adding collections and installing, removing and switching mods is for admins.
import { computed, onMounted, ref } from 'vue'
import * as api from '../api/mods'
import type { Collection, Installed, Item } from '../api/mods'
import Icon from '../components/Icon.vue'
import { dashboardUrl, host } from '../lib/host'
import { ICE } from '../lib/theme'
import { modsView, settingsPage, setModsView, toast, user } from '../store'
import { strings } from '../strings'
import AboutPanel from './AboutPanel.vue'

const admin = computed(() => !!user.value?.admin)
const mods = computed(() => modsView.value.mods)
const current = computed(() => modsView.value.prefs.theme)
const installed = (id: string) => mods.value.find((m) => m.id === id)

// ---- collections ----
const cols = ref<Collection[]>([])
const loading = ref(false)
async function loadCollections(refresh = false) {
  loading.value = true
  try {
    cols.value = await api.collections(refresh)
  } catch (e) {
    toast({ title: strings.tabCollections, body: msg(e) })
  } finally {
    loading.value = false
  }
}
onMounted(() => loadCollections())
const items = computed(() => cols.value.flatMap((c) => c.items))

// ---- tabs ----
const tab = ref<'themes' | 'widgets' | 'collections'>('themes')
interface Card {
  m: Item
  inst?: Installed
  ice?: boolean
}
const iceItem: Item = {
  id: '',
  collection: '',
  folder: '',
  official: true,
  kind: 'theme',
  name: strings.iceName,
  author: 'IceDash',
  description: '',
  version: '',
  scheme: 'dark',
  colors: ICE.colors,
}
function cardsOf(kind: 'theme' | 'widget'): Card[] {
  const list: Card[] = kind === 'theme' ? [{ m: iceItem, ice: true }] : []
  for (const i of mods.value.filter((x) => x.kind === kind)) list.push({ m: i, inst: i })
  for (const it of items.value.filter((x) => x.kind === kind && !installed(x.id))) list.push({ m: it })
  return list
}
const themeCards = computed(() => cardsOf('theme'))
const widgetCards = computed(() => cardsOf('widget'))

// ---- actions ----
const busy = ref('')
async function apply(id: string) {
  try {
    setModsView(await api.setTheme(id))
  } catch (e) {
    toast({ title: strings.themeApply, body: msg(e) })
  }
}

/** Install or update: say what it is and what it may read first. */
const offer = ref<Item | null>(null)
const latest = (id: string) => items.value.find((x) => x.id === id)
async function confirmInstall() {
  const o = offer.value
  if (!o) return
  busy.value = o.id
  try {
    await api.install(o.id)
    setModsView(await api.list())
    offer.value = null
    if (o.kind === 'theme') await apply(o.id)
  } catch (e) {
    toast({ title: strings.modInstall, body: msg(e) })
  } finally {
    busy.value = ''
  }
}
const removing = ref<Installed | null>(null)
async function confirmRemove() {
  const m = removing.value
  removing.value = null
  if (!m) return
  try {
    setModsView(await api.remove(m.id))
  } catch (e) {
    toast({ title: strings.modRemove, body: msg(e) })
  }
}
async function toggle(m: Installed) {
  try {
    setModsView(await api.setEnabled(m.id, !m.enabled))
  } catch (e) {
    toast({ title: m.name, body: msg(e) })
  }
}
const panel = computed(() => modsView.value.prefs.panel)
async function togglePanel(id: string) {
  const next = panel.value.includes(id) ? panel.value.filter((x) => x !== id) : [...panel.value, id]
  try {
    setModsView(await api.setPanel(next))
  } catch (e) {
    toast({ title: strings.addToPanel, body: msg(e) })
  }
}
function dragWidget(e: DragEvent, id: string) {
  e.dataTransfer?.setData('application/x-icedash-widget', id)
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'copy'
}

// ---- adding a collection ----
const address = ref('')
const addError = ref('')
async function addCollection() {
  const a = address.value.trim()
  if (!a) return
  busy.value = 'add'
  addError.value = ''
  try {
    await api.addCollection(a)
    address.value = ''
    await loadCollections()
    tab.value = 'collections'
  } catch (e) {
    addError.value = msg(e)
  } finally {
    busy.value = ''
  }
}
async function dropCollection(col: Collection) {
  try {
    await api.removeCollection(col.source)
    await loadCollections()
  } catch (e) {
    toast({ title: strings.tabCollections, body: msg(e) })
  }
}

// ---- words ----
function permWords(p?: api.Permissions): string[] {
  if (!p) return []
  const out: string[] = []
  if (p.internet?.length) out.push(strings.permInternet(p.internet.join(', ')))
  if (p.system) out.push(strings.permSystem)
  if (p.apps) out.push(strings.permApps)
  if (p.location) out.push(strings.permLocation)
  return out
}
function permChips(p?: api.Permissions): string[] {
  if (!p) return []
  return [
    p.internet?.length ? strings.chipInternet : '',
    p.system ? strings.chipSystem : '',
    p.apps ? strings.chipApps : '',
    p.location ? strings.chipLocation : '',
  ].filter(Boolean)
}
const msg = (e: unknown) => (e instanceof Error ? e.message : String(e))
const c = (m: Item, k: string) => m.colors?.[k] ?? '#000'
const shortSource = (src: string) => src.replace(/^github\.com\//, '')
const count = (col: Collection, kind: string) => col.items.filter((i) => i.kind === kind).length
</script>

<template>
  <div class="settings">
    <nav class="side" :aria-label="strings.settingsTitle">
      <button type="button" class="item" :class="{ on: settingsPage === 'mods' }" @click="settingsPage = 'mods'">
        <Icon name="palette" :size="18" /> {{ strings.settingsMods }}
      </button>
      <button type="button" class="item" :class="{ on: settingsPage === 'widgets' }" @click="settingsPage = 'widgets'">
        <Icon name="puzzle" :size="18" /> {{ strings.settingsWidgets }}
      </button>
      <button type="button" class="item" :class="{ on: settingsPage === 'about' }" @click="settingsPage = 'about'">
        <Icon name="info" :size="18" /> {{ strings.settingsAbout }}
      </button>
      <span class="grow" />
      <a class="item stock" :href="dashboardUrl()" target="_blank" rel="noopener">
        <Icon name="external" :size="16" /> {{ strings.openStockSettings(host.name) }}
      </a>
    </nav>

    <section v-if="settingsPage === 'mods'" class="page">
      <header class="head">
        <div>
          <h2>{{ strings.settingsMods }}</h2>
          <p class="mut">{{ strings.modsSubtitle }}</p>
        </div>
        <form v-if="admin" class="add" @submit.prevent="addCollection">
          <label class="field">
            <Icon name="github" :size="17" />
            <input v-model="address" :placeholder="strings.addCollectionPlaceholder" :aria-label="strings.addCollectionPlaceholder" spellcheck="false" />
          </label>
          <button class="primary" type="submit" :disabled="!address.trim() || !!busy">
            <Icon name="plus" :size="16" /> {{ busy === 'add' ? strings.addingCollection : strings.addCollection }}
          </button>
        </form>
      </header>
      <p v-if="addError" class="warnline" role="alert">{{ addError }}</p>

      <div class="tabs" role="tablist">
        <button type="button" role="tab" class="tab" :aria-selected="tab === 'themes'" :class="{ on: tab === 'themes' }" @click="tab = 'themes'">
          {{ strings.tabThemes(themeCards.length) }}
        </button>
        <button type="button" role="tab" class="tab" :aria-selected="tab === 'widgets'" :class="{ on: tab === 'widgets' }" @click="tab = 'widgets'">
          {{ strings.tabWidgets(widgetCards.length) }}
        </button>
        <button type="button" role="tab" class="tab" :aria-selected="tab === 'collections'" :class="{ on: tab === 'collections' }" @click="tab = 'collections'">
          {{ strings.tabCollections }} · {{ cols.length }}
        </button>
        <span class="grow" />
        <button v-if="admin" type="button" class="icon-btn" :title="strings.refresh" :aria-label="strings.refresh" :disabled="loading" @click="loadCollections(true)">
          <Icon name="refresh" :size="15" />
        </button>
      </div>

      <div v-if="tab !== 'collections'" class="cards">
        <article
          v-for="x in tab === 'themes' ? themeCards : widgetCards"
          :key="x.ice ? 'ice' : x.m.id"
          class="card"
          :class="{ active: x.m.kind === 'theme' && (x.ice ? !current : current === x.m.id) }"
        >
          <div v-if="x.m.kind === 'theme'" class="preview" :style="{ background: `radial-gradient(90% 80% at 70% 20%, ${c(x.m, 'accent')}33, transparent 60%), ${c(x.m, 'bg')}` }">
            <img v-if="x.m.wallpaper" class="wall" :src="x.m.wallpaper" alt="" loading="lazy" referrerpolicy="no-referrer" />
            <span class="pdot" :style="{ background: c(x.m, 'accent2') }" />
            <span class="pdot two" :style="{ background: c(x.m, 'accent') }" />
            <span class="pcard" :style="{ background: c(x.m, 'surface'), borderColor: c(x.m, 'text') + '22' }" />
            <span class="pcard two" :style="{ background: c(x.m, 'surface'), borderColor: c(x.m, 'text') + '22' }" />
            <span class="pdock" :style="{ background: c(x.m, 'surface') }">
              <i :style="{ background: c(x.m, 'accent') }" /><i :style="{ background: c(x.m, 'accent2') }" /><i :style="{ background: c(x.m, 'accent') }" /><i :style="{ background: c(x.m, 'accent2') }" />
            </span>
          </div>
          <div v-else class="wpreview">
            <span class="wicon"><Icon name="puzzle" :size="26" /></span>
            <span class="chips">
              <span v-for="p in permChips(x.m.permissions)" :key="p" class="chip mono">{{ strings.modCanRead.toLowerCase() }}: {{ p }}</span>
            </span>
          </div>
          <footer class="meta">
            <div class="who">
              <strong>{{ x.m.name }}</strong>
              <span class="mut">
                {{ x.ice ? strings.builtIn : strings.byAuthor(x.m.author) }}
                <Icon v-if="!x.ice && x.m.official" name="shield" :size="12" class="ok" :title="strings.modOfficial" />
                <Icon v-else-if="!x.ice" name="alert" :size="12" class="tp" :title="strings.modThirdParty" />
              </span>
            </div>
            <template v-if="x.m.kind === 'theme' && (x.ice || x.inst)">
              <span v-if="x.ice ? !current : current === x.m.id" class="state"><Icon name="check" :size="14" /> {{ strings.themeActive }}</span>
              <button v-else type="button" class="ghost" @click="apply(x.ice ? '' : x.m.id)">{{ strings.themeApply }}</button>
            </template>
            <button v-else-if="x.inst" type="button" class="ghost" :disabled="!x.inst.enabled" @click="togglePanel(x.m.id)">
              {{ panel.includes(x.m.id) ? strings.removeFromPanel : strings.addToPanel }}
            </button>
            <button v-else-if="admin" type="button" class="primary" :disabled="!!busy" @click="offer = x.m">
              {{ strings.modInstall }}
            </button>
            <button v-if="admin && x.inst?.update && latest(x.m.id)" type="button" class="ghost" :title="strings.modUpdateTitle" @click="offer = latest(x.m.id)!">
              {{ strings.modUpdateShort }}
            </button>
            <button v-if="admin && x.inst" type="button" class="icon-btn" :title="strings.modRemove" :aria-label="`${strings.modRemove} ${x.m.name}`" @click="removing = x.inst">
              <Icon name="trash" :size="15" />
            </button>
          </footer>
        </article>
      </div>

      <ul v-else class="clist">
        <li v-for="col in cols" :key="col.source" class="crow">
          <Icon name="github" :size="20" class="cicon" />
          <div class="who">
            <strong class="mono">{{ shortSource(col.source) }}</strong>
            <span class="mut">
              <template v-if="col.official"><Icon name="shield" :size="12" class="ok" /> {{ strings.modOfficial }}</template>
              <template v-else><Icon name="alert" :size="12" class="tp" /> {{ strings.modThirdParty }}</template>
              · {{ strings.collectionCount(count(col, 'theme'), count(col, 'widget')) }}
            </span>
            <span v-if="col.error" class="tp small">{{ col.error }}</span>
          </div>
          <a class="icon-btn" :href="`https://${col.source}`" target="_blank" rel="noopener" :title="strings.openOnGitHub" :aria-label="strings.openOnGitHub">
            <Icon name="external" :size="15" />
          </a>
          <button
            v-if="admin && !col.official"
            type="button"
            class="icon-btn"
            :title="strings.removeCollection"
            :aria-label="`${strings.removeCollection} ${col.source}`"
            @click="dropCollection(col)"
          >
            <Icon name="trash" :size="15" />
          </button>
        </li>
        <li class="mut small hint">{{ strings.collectionsHint }}</li>
      </ul>
      <p v-if="!admin" class="mut small">{{ strings.modsAdminOnly }}</p>
    </section>

    <section v-else-if="settingsPage === 'widgets'" class="page">
      <header class="head">
        <div>
          <h2>{{ strings.settingsWidgets }}</h2>
          <p class="mut">{{ strings.widgetsHint }}</p>
        </div>
      </header>
      <p v-if="!mods.some((m) => m.kind === 'widget')" class="mut">{{ strings.widgetsNone }}</p>
      <ul class="wlist">
        <li v-for="w in mods.filter((m) => m.kind === 'widget')" :key="w.id" class="wrow" draggable="true" @dragstart="dragWidget($event, w.id)">
          <span class="grip" aria-hidden="true"><Icon name="grip" :size="16" /></span>
          <span class="wicon sm"><Icon name="puzzle" :size="20" /></span>
          <div class="who">
            <strong>{{ w.name }}</strong>
            <span class="mut">
              {{ w.author }}
              <template v-if="!w.official"> · <span class="tp">{{ strings.modThirdParty }}</span></template>
            </span>
          </div>
          <span class="chips">
            <span v-for="p in permChips(w.permissions)" :key="p" class="chip mono">{{ strings.modCanRead.toLowerCase() }}: {{ p }}</span>
          </span>
          <button type="button" class="ghost" :disabled="!w.enabled" @click="togglePanel(w.id)">
            {{ panel.includes(w.id) ? strings.removeFromPanel : strings.addToPanel }}
          </button>
          <button
            v-if="admin"
            type="button"
            class="switch"
            role="switch"
            :aria-checked="w.enabled"
            :aria-label="`${w.name}: ${strings.widgetOn}`"
            :title="strings.widgetOn"
            @click="toggle(w)"
          >
            <span />
          </button>
        </li>
      </ul>
    </section>

    <section v-else class="page about"><AboutPanel /></section>

    <div v-if="offer" class="over" @keydown.esc.stop="offer = null">
      <div class="box" role="alertdialog" :aria-label="strings.modInstallTitle(offer.name)">
        <h3>{{ installed(offer.id) ? strings.modUpdateTitleOf(offer.name) : strings.modInstallTitle(offer.name) }}</h3>
        <p class="mut">{{ strings.byAuthor(offer.author) }} · {{ offer.description }}</p>
        <p class="mut small mono">{{ shortSource(offer.collection) }}/{{ offer.folder }}</p>
        <p class="mark" :class="offer.official ? 'ok' : 'tp'">
          <Icon :name="offer.official ? 'shield' : 'alert'" :size="15" />
          {{ offer.official ? strings.modOfficial : strings.modThirdParty }}
        </p>
        <template v-if="offer.kind === 'widget'">
          <p v-if="permWords(offer.permissions).length">
            <strong>{{ strings.modCanRead }}:</strong> {{ permWords(offer.permissions).join(', ') }}.
          </p>
          <p v-else>{{ strings.modReadsNothing }}</p>
        </template>
        <p v-else class="mut">{{ strings.modThemeNote }}</p>
        <div class="row">
          <button type="button" class="ghost" @click="offer = null">{{ strings.cancel }}</button>
          <button type="button" class="primary" :disabled="!!busy" data-autofocus @click="confirmInstall">
            {{ busy ? strings.modInstalling : installed(offer.id) ? strings.modUpdateShort : strings.modInstall }}
          </button>
        </div>
      </div>
    </div>
    <div v-if="removing" class="over" @keydown.esc.stop="removing = null">
      <div class="box" role="alertdialog">
        <p>{{ strings.modRemoveConfirm(removing.name) }}</p>
        <div class="row">
          <button type="button" class="ghost" @click="removing = null">{{ strings.cancel }}</button>
          <button type="button" class="primary warn" @click="confirmRemove">{{ strings.modRemove }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.settings {
  position: relative;
  height: 100%;
  display: grid;
  grid-template-columns: 230px minmax(0, 1fr);
}
.side {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 18px 12px;
  border-right: 1px solid var(--line);
}
.grow {
  flex: 1;
}
.item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  border: 1px solid transparent;
  border-radius: 12px;
  background: none;
  color: var(--text);
  font: inherit;
  font-size: 14.5px;
  text-align: left;
  text-decoration: none;
  cursor: pointer;
}
.item:hover {
  background: rgba(var(--fg-rgb), 0.05);
}
.item.on {
  background: rgba(var(--ac-rgb), 0.1);
  border-color: rgba(var(--ac-rgb), 0.35);
}
.item.stock {
  color: var(--mut);
  font-size: 13px;
}
.page {
  min-width: 0;
  overflow-y: auto;
  padding: 26px 28px 28px;
}
.page.about {
  padding: 20px 24px;
}
.head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
  flex-wrap: wrap;
}
h2 {
  margin: 0;
  font-size: 30px;
  font-weight: 700;
  letter-spacing: -0.01em;
}
.mut {
  color: var(--mut);
}
.head .mut {
  margin: 6px 0 0;
  max-width: 420px;
  font-size: 14.5px;
}
.small {
  font-size: 12.5px;
}
.add {
  display: flex;
  gap: 10px;
  flex: 1;
  max-width: 520px;
}
.field {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 16px;
  border-radius: var(--radius-pill);
  border: 1px solid var(--line);
  background: rgba(var(--fg-rgb), 0.03);
  color: var(--mut);
}
.field input {
  flex: 1;
  min-width: 0;
  padding: 12px 0;
  border: 0;
  background: none;
  color: var(--text);
  font-family: var(--font-mono);
  font-size: 13px;
  outline: none;
}
.field:focus-within {
  border-color: var(--ac);
}
.primary {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 10px 20px;
  border: 0;
  border-radius: var(--radius-pill);
  background: var(--ac);
  color: var(--ink);
  font: inherit;
  font-weight: 600;
  white-space: nowrap;
  cursor: pointer;
}
.primary:hover:not(:disabled) {
  background: var(--acd);
}
.primary:disabled {
  opacity: 0.55;
  cursor: default;
}
.primary.warn {
  background: var(--warn);
}
.ghost {
  padding: 8px 18px;
  border: 1px solid var(--line);
  border-radius: var(--radius-pill);
  background: rgba(var(--fg-rgb), 0.04);
  color: var(--text);
  font: inherit;
  font-weight: 600;
  white-space: nowrap;
  cursor: pointer;
}
.ghost:disabled {
  opacity: 0.45;
  cursor: default;
}
.warnline {
  margin: 10px 0 0;
  color: var(--warn);
  font-size: 13px;
}
.tabs {
  display: flex;
  gap: 8px;
  margin: 22px 0 18px;
}
.tab {
  padding: 6px 14px;
  border: 1px solid var(--line);
  border-radius: var(--radius-pill);
  background: none;
  color: var(--text);
  font-family: var(--font-mono);
  font-size: 12.5px;
  cursor: pointer;
}
.tab.on {
  border-color: rgba(var(--ac-rgb), 0.5);
  background: rgba(var(--ac-rgb), 0.1);
  color: var(--ac2);
}
.cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(230px, 1fr));
  gap: 16px;
}
.card {
  display: grid;
  gap: 12px;
  padding: 12px 12px 14px;
  border-radius: 20px;
  border: 1px solid var(--line);
  background: var(--card);
}
.card.active {
  border-color: rgba(var(--ac-rgb), 0.6);
  box-shadow: 0 0 0 1px rgba(var(--ac-rgb), 0.25);
}
.preview {
  position: relative;
  height: 128px;
  border-radius: 14px;
  overflow: hidden;
}
.wall {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.tabs .grow {
  flex: 1;
}
.clist {
  display: grid;
  gap: 10px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.crow {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 18px;
  border-radius: 18px;
  border: 1px solid var(--line);
  background: var(--card);
}
.cicon {
  color: var(--mut);
  flex: none;
}
.hint {
  padding: 6px 4px;
}
.pdot {
  position: absolute;
  left: 18px;
  top: 18px;
  width: 15px;
  height: 15px;
  border-radius: 4px;
}
.pdot.two {
  top: 42px;
}
.pcard {
  position: absolute;
  right: 14px;
  top: 16px;
  width: 64px;
  height: 30px;
  border-radius: 8px;
  border: 1px solid;
}
.pcard.two {
  top: 52px;
}
.pdock {
  position: absolute;
  left: 50%;
  bottom: 12px;
  display: flex;
  gap: 4px;
  padding: 4px 6px;
  border-radius: 7px;
  transform: translateX(-50%);
}
.pdock i {
  width: 13px;
  height: 13px;
  border-radius: 3px;
}
.wpreview {
  height: 128px;
  display: grid;
  place-content: center;
  justify-items: center;
  gap: 10px;
  border-radius: 14px;
  background: rgba(var(--fg-rgb), 0.03);
}
.wicon {
  display: grid;
  place-items: center;
  width: 52px;
  height: 52px;
  border-radius: 14px;
  background: rgba(var(--ac-rgb), 0.12);
  color: var(--ac);
}
.wicon.sm {
  width: 42px;
  height: 42px;
  border-radius: 12px;
}
.chips {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 6px;
}
.chip {
  padding: 4px 10px;
  border-radius: var(--radius-pill);
  border: 1px solid var(--line);
  color: var(--mut);
  font-size: 11.5px;
}
.meta {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 4px;
}
.who {
  flex: 1;
  min-width: 0;
  display: grid;
}
.who strong {
  font-size: 16px;
}
.who .mut {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 13px;
}
.ok {
  color: var(--ac2);
}
.tp {
  color: var(--warn);
}
.state {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 7px 14px;
  border-radius: var(--radius-pill);
  border: 1px solid rgba(var(--ac-rgb), 0.5);
  color: var(--ac2);
  font-family: var(--font-mono);
  font-size: 12.5px;
}
.state.quiet {
  border-color: var(--line);
  color: var(--mut);
}
.icon-btn {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border: 0;
  border-radius: 9px;
  background: none;
  color: var(--mut);
  cursor: pointer;
}
.icon-btn:hover {
  background: rgba(var(--fg-rgb), 0.07);
  color: var(--warn);
}
.wlist {
  display: grid;
  gap: 10px;
  margin: 20px 0 0;
  padding: 0;
  list-style: none;
}
.wrow {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 18px;
  border-radius: 18px;
  border: 1px solid var(--line);
  background: var(--card);
  cursor: grab;
}
.wrow .chips {
  justify-content: flex-end;
}
.grip {
  color: var(--mut);
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
.over {
  position: absolute;
  inset: 0;
  z-index: 5;
  display: grid;
  place-items: center;
  padding: 16px;
  background: rgba(0, 0, 0, 0.45);
  border-radius: 0 0 18px 18px;
}
.box {
  display: grid;
  gap: 12px;
  width: min(460px, 100%);
  padding: 20px;
  border-radius: 18px;
  border: 1px solid var(--line);
  background: rgba(var(--surface-rgb), 0.98);
}
.box h3 {
  margin: 0;
  font-size: 18px;
}
.box p {
  margin: 0;
  font-size: 14px;
  line-height: 1.45;
}
.mark {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
}
.mark.ok {
  color: var(--ac2);
}
.mark.tp {
  color: var(--warn);
}
.row {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 4px;
}
@media (max-width: 760px) {
  .settings {
    grid-template-columns: 1fr;
  }
  .side {
    flex-direction: row;
    flex-wrap: wrap;
    border-right: 0;
    border-bottom: 1px solid var(--line);
  }
  .grow {
    display: none;
  }
}
</style>
