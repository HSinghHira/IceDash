<script setup lang="ts">
// The built-in IceStore, laid out like the ZimaOS App Store: a sidebar with
// search, Discover, the lists and the categories; Discover as the home page
// (icestore/Discover.vue); titled lists with a sort; and an app page
// (icestore/AppPage.vue).
import { computed, ref, watch } from 'vue'
import * as api from '../api/apps'
import AppIcon from '../components/AppIcon.vue'
import Icon from '../components/Icon.vue'
import { availableBundles } from '../lib/bundles'
import { dashboardUrl } from '../lib/host'
import type { IconName } from '../lib/icons'
import { categoryIcon, SORTS, sortApps, type SortKey } from '../lib/icestore'
import {
  catalogError,
  catalogList,
  iceStoreFocus,
  iceStoreSort,
  iceStoreView,
  loadCatalog,
  showOtherDevices,
  startBundle,
  updateAll,
  updateRun,
} from '../store'
import { strings } from '../strings'
import { admin, fitsHere, installedOf } from './icestore/actions'
import AppPage from './icestore/AppPage.vue'
import Discover from './icestore/Discover.vue'
import AppRow from './icestore/AppRow.vue'

const query = ref('')
const page = ref<{ app: api.CatalogApp; meta: api.Meta } | null>(null)
const pageError = ref('')
const refreshing = ref(false)
const main = ref<HTMLElement | null>(null)

const all = computed(() => catalogList.value?.apps ?? [])
/** The apps this box lists: all of them, or only those for its CPU. */
const shown = computed(() => (showOtherDevices.value ? all.value : all.value.filter(fitsHere)))
const hiddenCount = computed(() => all.value.length - shown.value.length)

const categories = computed(() => {
  const seen = new Map<string, number>()
  for (const a of shown.value) if (a.category) seen.set(a.category, (seen.get(a.category) ?? 0) + 1)
  return [...seen.entries()].sort((a, b) => a[0].localeCompare(b[0])).map(([name, n]) => ({ name, n }))
})

const q = computed(() => query.value.trim().toLowerCase())
/** A search on Discover lists every app that matches. */
const view = computed(() => (q.value && iceStoreView.value === 'discover' ? 'all' : iceStoreView.value))

const list = computed(() => {
  const v = view.value
  // My apps and Updates show every app the box has, whatever its CPU list says.
  let l = v === 'installed' || v === 'updates' ? all.value : shown.value
  if (v === 'installed') l = l.filter((a) => installedOf(a))
  else if (v === 'updates') l = l.filter((a) => installedOf(a)?.update)
  else if (v !== 'all') l = l.filter((a) => a.category === v)
  if (q.value)
    l = l.filter((a) =>
      [a.title, a.tagline, a.category, a.developer, ...(a.categories ?? [])].some((s) => (s ?? '').toLowerCase().includes(q.value)),
    )
  return sortApps(l, iceStoreSort.value)
})
/** Rows show the number or date the list is sorted by. */
const rowFact = computed(() => {
  const s = iceStoreSort.value
  return s === 'stars' || s === 'forks' ? s : s === 'newest' ? 'added' : s === 'updated' ? 'updated' : undefined
})

const title = computed(() => {
  const v = view.value
  if (q.value && iceStoreView.value === 'discover') return strings.searchResults(query.value.trim())
  if (v === 'all') return strings.allApps
  if (v === 'installed') return strings.myApps
  if (v === 'updates') return strings.updates
  if (v === 'bundles') return strings.tabBundles
  return v
})

// ---- Phase 5b: bundles ----
const bundles = computed(() =>
  availableBundles(all.value)
    .map((x) => ({ ...x, words: strings.bundles[x.bundle.id] ?? { name: x.bundle.id, about: '' }, have: x.apps.filter((a) => installedOf(a)).length }))
    .filter((x) => !q.value || [x.words.name, x.words.about, ...x.apps.map((a) => a.title)].some((s) => s.toLowerCase().includes(q.value))),
)

const updatable = computed(() => all.value.filter((a) => installedOf(a)?.update))
const installedCount = computed(() => all.value.filter((a) => installedOf(a)).length)
function runUpdateAll() {
  void updateAll(updatable.value.map((a) => installedOf(a)!.name))
}

const nav = computed<{ id: string; name: string; icon: IconName; n?: number; hot?: boolean }[]>(() => [
  { id: 'discover', name: strings.discover, icon: 'compass' },
  { id: 'all', name: strings.allApps, icon: 'grid', n: shown.value.length },
  { id: 'installed', name: strings.myApps, icon: 'app', n: installedCount.value },
  { id: 'updates', name: strings.updates, icon: 'down', n: updatable.value.length || undefined, hot: !!updatable.value.length },
  { id: 'bundles', name: strings.tabBundles, icon: 'package', n: bundles.value.length },
])

function go(id: string, sort?: SortKey) {
  page.value = null
  if (id !== 'discover') query.value = query.value && iceStoreView.value === 'discover' ? query.value : ''
  iceStoreView.value = id
  if (sort) iceStoreSort.value = sort
  main.value?.scrollIntoView?.({ block: 'start' })
}

watch(iceStoreView, () => (page.value = null))
watch(query, (v) => {
  if (v) page.value = null
})
// Opened from search on one app: its page.
watch(
  [iceStoreFocus, all],
  () => {
    const id = iceStoreFocus.value
    const a = id ? all.value.find((x) => x.id === id) : undefined
    if (a) {
      iceStoreFocus.value = null
      void show(a)
    }
  },
  { immediate: true },
)

async function show(a: api.CatalogApp) {
  pageError.value = ''
  page.value = { app: a, meta: { description: '', screenshot_link: [] } as unknown as api.Meta }
  main.value?.scrollIntoView?.({ block: 'start' })
  try {
    const p = await api.catalogApp(a.id)
    if (page.value?.app.id === a.id) page.value = p
  } catch (e) {
    pageError.value = e instanceof Error ? e.message : String(e)
  }
}

async function refresh() {
  refreshing.value = true
  await loadCatalog(true)
  refreshing.value = false
}
</script>

<template>
  <div class="frame">
  <div class="store">
    <aside class="side">
      <h2 class="brand"><Icon name="snowflake" :size="20" />{{ strings.iceStore }}</h2>
      <label class="search">
        <Icon name="search" :size="15" />
        <input v-model="query" type="search" :placeholder="strings.searchIceStore" :aria-label="strings.searchIceStore" />
      </label>
      <nav class="nav" :aria-label="strings.categories">
        <button
          v-for="v in nav"
          :key="v.id"
          type="button"
          class="item"
          :class="{ on: !page && iceStoreView === v.id }"
          :aria-current="!page && iceStoreView === v.id ? 'page' : undefined"
          @click="go(v.id)"
        >
          <Icon :name="v.icon" :size="17" />
          <span class="label">{{ v.name }}</span>
          <span v-if="v.n != null" class="n" :class="{ hot: v.hot }">{{ v.n }}</span>
        </button>
        <p class="group">{{ strings.categories }}</p>
        <button
          v-for="c in categories"
          :key="c.name"
          type="button"
          class="item"
          :class="{ on: !page && iceStoreView === c.name }"
          :aria-current="!page && iceStoreView === c.name ? 'page' : undefined"
          @click="go(c.name)"
        >
          <Icon :name="categoryIcon(c.name)" :size="17" />
          <span class="label">{{ c.name }}</span>
          <span class="n">{{ c.n }}</span>
        </button>
      </nav>
      <div class="side-foot">
        <button type="button" class="foot-btn" :disabled="refreshing" @click="refresh">
          <Icon name="refresh" :size="15" :class="{ spin: refreshing }" />{{ strings.refresh }}
        </button>
        <a class="foot-btn" :href="dashboardUrl()" target="_blank" rel="noopener"><Icon name="external" :size="15" />{{ strings.officialStore }}</a>
      </div>
    </aside>

    <div ref="main" class="main">
      <p v-if="catalogError" class="warn small banner">{{ catalogList ? strings.catalogOffline(catalogError) : catalogError }}</p>

      <AppPage
        v-if="page"
        :app="page.app"
        :meta="page.meta"
        :error="pageError"
        :all="shown"
        @back="page = null"
        @open="show"
        @bundles="go('bundles')"
        @view="go"
      />

      <p v-else-if="!catalogList && !catalogError" class="mut">{{ strings.loading }}</p>

      <Discover v-else-if="view === 'discover'" :apps="shown" :categories="categories" @open="show" @view="go" />

      <template v-else>
        <header class="page-head">
          <div>
            <h2>{{ title }}</h2>
            <p class="mut small">
              {{ view === 'bundles' ? strings.bundlesHint : strings.appCount(list.length) }}
            </p>
          </div>
          <span class="head-tools">
            <button
              v-if="view === 'updates' && admin && updatable.length"
              type="button"
              class="pill update"
              :disabled="!!updateRun"
              @click="runUpdateAll"
            >
              {{ updateRun ? strings.updatingN(updateRun.done + 1, updateRun.total) : strings.updateAll(updatable.length) }}
            </button>
            <label v-if="view !== 'bundles'" class="sort" :title="strings.sortBy">
              <Icon name="sort" :size="16" />
              <select v-model="iceStoreSort" :aria-label="strings.sortBy">
                <option v-for="k in SORTS" :key="k" :value="k">{{ strings.sortNames[k] }}</option>
              </select>
            </label>
          </span>
        </header>

        <ul v-if="view === 'bundles'" class="bundles">
          <li v-for="b in bundles" :key="b.bundle.id" class="bcard" :data-bundle="b.bundle.id">
            <header class="bhead">
              <span class="bicon"><Icon :name="b.bundle.icon" :size="20" /></span>
              <div class="card-titles">
                <span class="title">{{ b.words.name }}</span>
                <span class="mut small">{{ strings.bundleHave(b.have, b.apps.length) }}</span>
              </div>
              <button
                type="button"
                class="pill"
                :class="b.have === b.apps.length ? 'open' : 'install'"
                :disabled="!admin || b.have === b.apps.length"
                :title="!admin ? strings.adminInstallOnly : undefined"
                @click="startBundle(b.bundle.id, b.apps.map((a) => a.id))"
              >
                {{ b.have === b.apps.length ? strings.bundleAllInstalled : strings.bundleInstall(b.apps.length - b.have) }}
              </button>
            </header>
            <p class="tagline">{{ b.words.about }}</p>
            <ul class="bapps">
              <li v-for="a in b.apps" :key="a.id">
                <button type="button" class="bapp" :title="a.tagline" @click="show(a)">
                  <AppIcon :src="a.icon" :name="a.title" :size="30" />
                  <span>{{ a.title }}</span>
                  <Icon v-if="installedOf(a)" name="check" :size="14" class="ok" />
                </button>
              </li>
            </ul>
          </li>
        </ul>

        <template v-else>
          <p v-if="catalogList && !list.length" class="empty mut">{{ strings.catalogEmpty }}</p>
          <ul class="rows" :class="{ wide: view === 'updates' }">
            <AppRow v-for="a in list" :key="a.id" :app="a" :fact="rowFact" :action="view === 'updates'" @open="show" />
          </ul>
        </template>
      </template>

      <p v-if="!page && view !== 'bundles' && (hiddenCount || showOtherDevices)" class="other mut small">
        <template v-if="!showOtherDevices">{{ strings.hiddenOther(hiddenCount) }}</template>
        <label class="check">
          <input v-model="showOtherDevices" type="checkbox" />
          {{ strings.showOtherDevices }}
        </label>
      </p>
    </div>
  </div>
  </div>
</template>

<style scoped src="./icestore/shared.css"></style>
<style scoped>
.frame {
  container-type: inline-size;
}
.store {
  display: grid;
  grid-template-columns: 210px 1fr;
  gap: 28px;
  min-height: 60vh;
}
.side {
  display: flex;
  flex-direction: column;
  gap: 14px;
  position: sticky;
  top: 0;
  align-self: start;
  max-height: calc(100vh - 160px);
  min-width: 0;
}
.brand {
  display: flex;
  align-items: center;
  gap: 9px;
  margin: 2px 0 0;
  font-family: var(--font-display);
  font-size: 21px;
  font-weight: 600;
}
.brand :deep(svg) {
  color: var(--ac);
}
.search {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 12px;
  border-radius: 11px;
  border: 1px solid var(--line);
  background: rgba(var(--fg-rgb), 0.04);
  color: var(--mut);
}
.search:focus-within {
  border-color: rgba(var(--ac-rgb), 0.6);
}
.search input {
  flex: 1;
  min-width: 0;
  padding: 8px 0;
  border: 0;
  background: none;
  color: var(--text);
  font: inherit;
  font-size: 13px;
  outline: none;
}
.nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-height: 0;
  overflow-y: auto;
  scrollbar-width: thin;
}
.item {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 7px 10px;
  border: 0;
  border-radius: 10px;
  background: none;
  color: var(--mut);
  font: inherit;
  font-size: 13.5px;
  text-align: left;
  cursor: pointer;
}
.item:hover {
  color: var(--text);
  background: rgba(var(--fg-rgb), 0.05);
}
.item.on {
  color: var(--text);
  background: rgba(var(--ac-rgb), 0.13);
}
.item.on :deep(svg) {
  color: var(--ac);
}
.label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.n {
  font-family: var(--font-mono);
  font-size: 11.5px;
  color: var(--mut);
}
.n.hot {
  padding: 1px 8px;
  border-radius: var(--radius-pill);
  background: var(--warn);
  color: var(--ink);
}
.group {
  margin: 14px 10px 4px;
  color: var(--mut);
  font-size: 12px;
  font-weight: 600;
}
.side-foot {
  display: grid;
  gap: 2px;
  padding-top: 10px;
  border-top: 1px solid var(--line);
}
.foot-btn {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 6px 10px;
  border: 0;
  border-radius: 9px;
  background: none;
  color: var(--mut);
  font: inherit;
  font-size: 12.5px;
  text-decoration: none;
  cursor: pointer;
}
.foot-btn:hover {
  color: var(--text);
  background: rgba(var(--fg-rgb), 0.05);
}
.spin {
  animation: spin 0.9s linear infinite;
}
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
.main {
  min-width: 0;
  scroll-margin-top: 20px;
  container-type: inline-size;
}
.banner {
  margin: 0 0 14px;
}
.page-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  margin-bottom: 18px;
}
.page-head h2 {
  margin: 0 0 4px;
  font-size: 28px;
  font-weight: 700;
  letter-spacing: -0.02em;
  line-height: 1.15;
}
.page-head p {
  margin: 0;
  max-width: 60ch;
}
.head-tools {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}
.sort {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 0 12px;
  border-radius: var(--radius-pill);
  border: 1px solid var(--line);
  background: rgba(var(--fg-rgb), 0.03);
  color: var(--mut);
}
.sort:focus-within {
  border-color: rgba(var(--ac-rgb), 0.6);
}
.sort select {
  padding: 7px 0;
  border: 0;
  background: none;
  color: var(--text);
  font: inherit;
  font-size: 12.5px;
  cursor: pointer;
  outline: none;
}
.sort select option {
  background: rgb(var(--surface-rgb));
  color: var(--text);
}
.empty {
  margin: 30px 0;
}
.other {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 14px;
  margin: 26px 0 0;
}
.check {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
}
.check input {
  accent-color: var(--ac);
}
.bundles {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 14px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.bcard {
  display: grid;
  gap: 10px;
  padding: 16px;
  border-radius: 18px;
  border: 1px solid var(--line);
  background: var(--card);
}
.bhead {
  display: flex;
  align-items: center;
  gap: 12px;
}
.bicon {
  display: grid;
  place-items: center;
  width: 42px;
  height: 42px;
  flex: none;
  border-radius: 12px;
  background: rgba(var(--ac-rgb), 0.12);
  color: var(--ac);
}
.card-titles {
  display: grid;
  min-width: 0;
}
.bhead .card-titles {
  flex: 1;
}
.bapps {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.bapp {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 4px 10px 4px 4px;
  border: 1px solid var(--line);
  border-radius: 99px;
  background: rgba(var(--fg-rgb), 0.03);
  color: var(--text);
  font: inherit;
  font-size: 12.5px;
  cursor: pointer;
}
.bapp:hover {
  border-color: rgba(var(--ac-rgb), 0.5);
}
.bapp .ok {
  color: var(--ac2);
}
/* A narrow window: the sidebar becomes a strip above the page. */
@container (max-width: 720px) {
  .store {
    grid-template-columns: 1fr;
    gap: 18px;
  }
  .side {
    position: static;
    max-height: none;
  }
  .brand {
    display: none;
  }
  .nav {
    flex-direction: row;
    overflow-x: auto;
    overflow-y: hidden;
    padding-bottom: 4px;
  }
  .item {
    flex: none;
  }
  .item .n {
    display: none;
  }
  .group {
    display: none;
  }
  .side-foot {
    display: none;
  }
}
</style>
