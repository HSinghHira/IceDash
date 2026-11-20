<script setup lang="ts">
// Ctrl+Space searches everything (PLAN.md 5b.3): apps and launchers, every
// setting, the look (themes, wallpapers, cards), IceStore's apps and bundles,
// and, for admins, file names under /DATA. Enter opens the selected one.
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as filesApi from '../api/files'
import * as modsApi from '../api/mods'
import AppIcon from '../components/AppIcon.vue'
import Icon from '../components/Icon.vue'
import { availableBundles } from '../lib/bundles'
import { host } from '../lib/host'
import type { IconName } from '../lib/icons'
import { desktopLaunchers, dockEnd } from '../lib/launchers'
import { search } from '../lib/search'
import { settingsIndex } from '../lib/settingsIndex'
import { FROST, FROST_ID } from '../lib/theme'
import { PRESETS } from '../lib/wallpaper'
import {
  BUILTIN_CARDS,
  apps,
  catalogList,
  iceStoreFocus,
  iceStoreInZimaOS,
  loadCatalog,
  modsView,
  openApp,
  openFiles,
  openIceStore,
  openSettings,
  openTerminal,
  prefs,
  savePrefs,
  searchOpen,
  setModsView,
  toast,
  user,
} from '../store'
import { strings } from '../strings'

type Group = 'apps' | 'settings' | 'look' | 'store' | 'files'
interface Hit {
  id: string
  name: string
  keywords?: string
  group: Group
  /** A second line: where it is. */
  sub?: string
  icon?: IconName
  img?: string
  /** A swatch for themes and wallpapers. */
  swatch?: string
  run: () => unknown
}

const q = ref('')
const sel = ref(0)
const input = ref<HTMLInputElement | null>(null)
const admin = computed(() => !!user.value?.admin)

const launchHits = computed<Hit[]>(() => [
  ...desktopLaunchers(location, { admin: admin.value, iceStoreInZimaOS: iceStoreInZimaOS.value }),
  ...dockEnd({ admin: admin.value }),
].map((l) => ({
  id: 'l:' + l.id,
  name: l.name,
  group: 'apps' as const,
  icon: l.icon,
  run: () => {
    if (l.id === 'icestore') openIceStore()
    else if (l.id === 'files') openFiles()
    else if (l.id === 'trash') openFiles('trash')
    else if (l.id === 'terminal') openTerminal()
    else if (l.id === 'settings') openSettings()
    else window.open(l.url, '_blank', 'noopener')
  },
})))
const appHits = computed<Hit[]>(() =>
  apps.value.filter((a) => a.url).map((a) => ({ id: 'a:' + a.id, name: a.name, group: 'apps' as const, img: a.icon ?? '', run: () => openApp(a) })),
)
const settingHits = computed<Hit[]>(() =>
  settingsIndex({ zimaos: host.id === 'zimaos' }).map((s) => ({ ...s, group: 'settings' as const, sub: strings.searchIn(s.pageName), run: () => openSettings(s.page) })),
)
async function useTheme(id: string) {
  try {
    setModsView(await modsApi.setTheme(id))
  } catch (e) {
    toast({ title: strings.themeHeading, body: e instanceof Error ? e.message : String(e) })
  }
}
const lookHits = computed<Hit[]>(() => [
  { id: 't:ice', name: strings.iceName, keywords: strings.themeHeading, group: 'look', sub: strings.themeHeading, swatch: '#000000', run: () => useTheme('') },
  { id: 't:frost', name: strings.frostName, keywords: strings.themeHeading, group: 'look', sub: strings.themeHeading, swatch: FROST.colors.bg, run: () => useTheme(FROST_ID) },
  ...modsView.value.mods
    .filter((m) => m.kind === 'theme')
    .map((m) => ({ id: 't:' + m.id, name: m.name, keywords: `${strings.themeHeading} ${m.author}`, group: 'look' as const, sub: strings.themeHeading, swatch: m.colors?.bg, run: () => useTheme(m.id) })),
  ...PRESETS.map((p) => ({
    id: 'w:' + p.id,
    name: p.name,
    keywords: strings.wallpaperHeading,
    group: 'look' as const,
    sub: strings.wallpaperHeading,
    swatch: p.css,
    run: () => savePrefs({ wallpaper: { ...prefs.value.wallpaper, kind: 'preset', preset: p.id } }),
  })),
  ...BUILTIN_CARDS.map((c) => ({ id: 'c:' + c, name: strings.cardName[c] ?? c, keywords: `${strings.settingsWidgets} ${strings.cardAbout[c] ?? ''}`, group: 'look' as const, sub: strings.settingsWidgets, icon: 'cards' as IconName, run: () => openSettings('widgets') })),
  ...modsView.value.mods
    .filter((m) => m.kind === 'widget')
    .map((m) => ({ id: 'c:' + m.id, name: m.name, keywords: strings.settingsWidgets, group: 'look' as const, sub: strings.settingsWidgets, icon: 'puzzle' as IconName, run: () => openSettings('widgets') })),
])
const storeHits = computed<Hit[]>(() => {
  const all = catalogList.value?.apps ?? []
  return [
    ...availableBundles(all).map(({ bundle, apps: list }) => ({
      id: 'b:' + bundle.id,
      name: strings.bundles[bundle.id]?.name ?? bundle.id,
      keywords: `${strings.tabBundles} ${list.map((a) => a.title).join(' ')}`,
      group: 'store' as const,
      sub: strings.tabBundles,
      icon: 'package' as IconName,
      run: () => openIceStore('bundles'),
    })),
    ...all.map((a) => ({
      id: 's:' + a.id,
      name: a.title,
      keywords: `${a.category} ${a.tagline}`,
      group: 'store' as const,
      sub: a.tagline,
      img: a.icon,
      run: () => {
        openIceStore()
        iceStoreFocus.value = a.id
      },
    })),
  ]
})

// ---- files: asked of the server as you type (admins only, like Files) ----
const fileHits = ref<Hit[]>([])
const filesState = ref<'' | 'busy' | 'more' | 'failed'>('')
let timer: ReturnType<typeof setTimeout> | undefined
let asked = 0
function askFiles(text: string) {
  clearTimeout(timer)
  fileHits.value = []
  filesState.value = ''
  if (!admin.value || text.trim().length < 2) return
  filesState.value = 'busy'
  const n = ++asked
  timer = setTimeout(async () => {
    try {
      const f = await filesApi.search(text.trim())
      if (n !== asked) return
      fileHits.value = f.entries.slice(0, 8).map((e) => {
        const dir = e.path.slice(0, e.path.lastIndexOf('/')) || '/'
        return {
          id: 'f:' + e.path,
          name: e.name,
          group: 'files' as const,
          sub: strings.searchIn(dir),
          icon: (e.dir ? 'folder' : 'file') as IconName,
          run: () => (e.dir ? openFiles('files', e.path) : openFiles('files', dir, e.path)),
        }
      })
      filesState.value = f.more || f.entries.length > 8 ? 'more' : ''
    } catch {
      if (n === asked) filesState.value = 'failed'
    }
  }, 280)
}

const ORDER: Group[] = ['apps', 'settings', 'look', 'store', 'files']
const groups = computed(() => {
  const text = q.value
  const found: Record<Group, Hit[]> = {
    apps: search([...launchHits.value, ...appHits.value], text, text ? 6 : 8),
    settings: text ? search(settingHits.value, text, 5) : [],
    look: text ? search(lookHits.value, text, 5) : [],
    store: text ? search(storeHits.value, text, 4) : [],
    files: fileHits.value,
  }
  return ORDER.map((g) => ({ g, hits: found[g] })).filter((x) => x.hits.length)
})
/** Every hit in the order shown, for the arrow keys. */
const flat = computed(() => groups.value.flatMap((x) => x.hits))
const indexOf = (h: Hit) => flat.value.indexOf(h)

watch(q, (v) => {
  sel.value = 0
  askFiles(v)
})
watch(searchOpen, async (open) => {
  if (open) {
    q.value = ''
    if (!catalogList.value) void loadCatalog()
    await nextTick()
    input.value?.focus()
  }
})

function go(h: Hit | undefined) {
  if (!h) return
  searchOpen.value = false
  void h.run()
}
function onInputKey(e: KeyboardEvent) {
  if (e.key === 'ArrowDown') {
    sel.value = Math.min(sel.value + 1, flat.value.length - 1)
    e.preventDefault()
  } else if (e.key === 'ArrowUp') {
    sel.value = Math.max(sel.value - 1, 0)
    e.preventDefault()
  } else if (e.key === 'Enter') {
    go(flat.value[sel.value])
  } else if (e.key === 'Escape') {
    searchOpen.value = false
  }
}
watch(sel, async () => {
  await nextTick()
  document.getElementById(`hit-${sel.value}`)?.scrollIntoView({ block: 'nearest' })
})
function onGlobalKey(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.code === 'Space') {
    e.preventDefault()
    searchOpen.value = !searchOpen.value
  }
}
onMounted(() => document.addEventListener('keydown', onGlobalKey))
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onGlobalKey)
  clearTimeout(timer)
})
</script>

<template>
  <button class="slot" type="button" :aria-label="strings.searchLabel" :title="`${strings.searchPlaceholder} (${strings.searchShortcut})`" @click="searchOpen = true">
    <span class="tile"><Icon name="search" :size="21" /></span>
  </button>

  <Teleport to="body">
  <div v-if="searchOpen" class="overlay" @click.self="searchOpen = false">
    <div class="box" role="dialog" aria-modal="true" :aria-label="strings.searchLabel">
      <label class="field">
        <Icon name="search" :size="18" />
        <input
          ref="input"
          v-model="q"
          type="search"
          :placeholder="strings.searchPlaceholder"
          :aria-label="strings.searchLabel"
          autocomplete="off"
          spellcheck="false"
          role="combobox"
          aria-expanded="true"
          aria-controls="search-results"
          :aria-activedescendant="flat[sel] ? `hit-${sel}` : undefined"
          @keydown="onInputKey"
        />
        <kbd>Esc</kbd>
      </label>
      <ul id="search-results" class="results" role="listbox">
        <template v-for="grp in groups" :key="grp.g">
          <li class="ghead" role="presentation">{{ strings.searchGroups[grp.g] }}</li>
          <li
            v-for="h in grp.hits"
            :id="`hit-${indexOf(h)}`"
            :key="h.id"
            role="option"
            :data-hit="h.id"
            :aria-selected="indexOf(h) === sel"
            :class="{ on: indexOf(h) === sel }"
            @mouseenter="sel = indexOf(h)"
            @click="go(h)"
          >
            <span v-if="h.swatch" class="sw" :style="{ background: h.swatch }" />
            <span v-else-if="h.icon" class="ic"><Icon :name="h.icon" :size="18" /></span>
            <AppIcon v-else :src="h.img" :name="h.name" :size="30" />
            <span class="txt">
              <span class="nm">{{ h.name }}</span>
              <span v-if="h.sub" class="sub">{{ h.sub }}</span>
            </span>
            <Icon class="ext" :name="h.group === 'look' && h.swatch ? 'check' : 'arrowRight'" :size="14" />
          </li>
        </template>
        <li v-if="filesState === 'busy'" class="none" role="status">{{ strings.searchFilesBusy }}</li>
        <li v-else-if="filesState === 'more'" class="none">
          <button type="button" class="more" @click="(searchOpen = false), openFiles()">{{ strings.searchFilesMore }}</button>
        </li>
        <li v-else-if="filesState === 'failed'" class="none">{{ strings.searchFilesFailed }}</li>
        <li v-if="!flat.length && filesState !== 'busy'" class="none">{{ strings.searchNothing }}</li>
      </ul>
    </div>
  </div>
  </Teleport>
</template>
<style scoped>
.slot {
  flex: none;
  display: grid;
  place-items: center;
  padding: 0;
  border: 0;
  background: none;
  cursor: pointer;
  transition: transform 0.15s ease;
}
.slot:hover,
.slot:focus-visible {
  transform: translateY(-3px);
}
kbd {
  font-family: var(--font-mono);
  font-size: 11px;
  padding: 3px 7px;
  border-radius: 6px;
  border: 1px solid var(--line);
  color: var(--mut);
}
.overlay {
  position: fixed;
  inset: 0;
  z-index: 200;
  display: grid;
  justify-items: center;
  align-content: start;
  padding: 14vh 16px 16px;
  background: rgba(0, 0, 0, 0.45);
}
.box {
  width: min(560px, 100%);
  border-radius: 20px;
  background: rgba(var(--surface-rgb), 0.94);
  backdrop-filter: var(--blur);
  -webkit-backdrop-filter: var(--blur);
  border: 1px solid var(--line);
  box-shadow: 0 30px 80px rgba(0, 0, 0, 0.7), 0 0 0 1px rgba(var(--ac-rgb), 0.08);
  overflow: hidden;
}
.field {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 16px;
  border-bottom: 1px solid var(--line);
  color: var(--mut);
}
.field input {
  flex: 1;
  min-width: 0;
  background: none;
  border: 0;
  outline: none;
  color: var(--text);
  font: inherit;
  font-size: 16px;
}
.results {
  list-style: none;
  margin: 0;
  padding: 6px;
  max-height: 60vh;
  overflow-y: auto;
}
.results li {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 10px;
  border-radius: 12px;
  cursor: pointer;
}
.results li.on {
  background: rgba(var(--ac-rgb), 0.14);
}
.results .ic {
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  border-radius: 9px;
  color: var(--ac2);
  background: rgba(var(--fg-rgb), 0.06);
}
.results .ext {
  margin-left: auto;
  flex: none;
  color: var(--mut);
}
.results .ghead {
  padding: 10px 10px 4px;
  color: var(--mut);
  font: 500 12px var(--font);
  cursor: default;
}
.results .txt {
  display: grid;
  min-width: 0;
}
.results .nm,
.results .sub {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.results .sub {
  color: var(--mut);
  font-size: 12px;
}
.results .sw {
  flex: none;
  width: 30px;
  height: 30px;
  border-radius: 9px;
  border: 1px solid var(--line);
}
.more {
  padding: 0;
  border: 0;
  background: none;
  color: var(--ac2);
  font: inherit;
  cursor: pointer;
  text-decoration: underline;
}
.results .none {
  color: var(--mut);
  cursor: default;
}
</style>
