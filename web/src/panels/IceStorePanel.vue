<script setup lang="ts">
// The built-in IceStore (mock-up screen 2, behind the popup): categories on
// the left; search and sort; Featured and Recently added on All apps; app
// cards with Install / Open / Update; and an app page (icestore/AppPage.vue).
import { computed, ref, watch } from 'vue'
import * as api from '../api/apps'
import AppIcon from '../components/AppIcon.vue'
import Icon from '../components/Icon.vue'
import { availableBundles } from '../lib/bundles'
import { ago } from '../lib/format'
import { dashboardUrl } from '../lib/host'
import { featuredApps, newestApps, SORTS, sortApps } from '../lib/icestore'
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
import FeaturedRow from './icestore/FeaturedRow.vue'
import StoreCard from './icestore/StoreCard.vue'

const query = ref('')
const page = ref<{ app: api.CatalogApp; meta: api.Meta } | null>(null)
const pageError = ref('')
const refreshing = ref(false)

const all = computed(() => catalogList.value?.apps ?? [])
/** The apps this box lists: all of them, or only those for its CPU. */
const shown = computed(() => (showOtherDevices.value ? all.value : all.value.filter(fitsHere)))
const hiddenCount = computed(() => all.value.length - shown.value.length)

const categories = computed(() => {
  const seen = new Map<string, number>()
  for (const a of shown.value) if (a.category) seen.set(a.category, (seen.get(a.category) ?? 0) + 1)
  return [...seen.entries()].sort((a, b) => a[0].localeCompare(b[0])).map(([name, n]) => ({ name, n }))
})

const searching = computed(() => !!query.value.trim())
const list = computed(() => {
  const q = query.value.trim().toLowerCase()
  const v = iceStoreView.value
  // Installed and Updates show every app the box has, whatever its CPU list says.
  let l = v === 'installed' || v === 'updates' ? all.value : shown.value
  if (v === 'installed') l = l.filter((a) => installedOf(a))
  else if (v === 'updates') l = l.filter((a) => installedOf(a)?.update)
  else if (v !== 'all') l = l.filter((a) => a.category === v)
  if (q)
    l = l.filter((a) =>
      [a.title, a.tagline, a.category, a.developer, ...(a.categories ?? [])].some((s) => (s ?? '').toLowerCase().includes(q)),
    )
  return sortApps(l, iceStoreSort.value)
})
/** Cards show the date the list is sorted by. */
const cardDate = computed(() => (iceStoreSort.value === 'newest' ? 'added' : iceStoreSort.value === 'updated' ? 'updated' : undefined))

// All apps, no search: Featured and Recently added above the list.
const home = computed(() => iceStoreView.value === 'all' && !searching.value)
const featured = computed(() => featuredApps(shown.value))
const newest = computed(() => newestApps(shown.value, 8))
function seeNewest() {
  iceStoreSort.value = 'newest'
  document.querySelector('.store .all-head')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

// ---- Phase 5b: bundles ----
const bundles = computed(() => {
  const q = query.value.trim().toLowerCase()
  return availableBundles(all.value)
    .map((x) => ({ ...x, words: strings.bundles[x.bundle.id] ?? { name: x.bundle.id, about: '' }, have: x.apps.filter((a) => installedOf(a)).length }))
    .filter((x) => !q || [x.words.name, x.words.about, ...x.apps.map((a) => a.title)].some((s) => s.toLowerCase().includes(q)))
})

const updatable = computed(() => all.value.filter((a) => installedOf(a)?.update))
const updatesCount = computed(() => updatable.value.length)
const installedCount = computed(() => all.value.filter((a) => installedOf(a)).length)
function runUpdateAll() {
  void updateAll(updatable.value.map((a) => installedOf(a)!.name))
}

watch(iceStoreView, () => (page.value = null))
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

const views = computed(() => [
  { id: 'all', name: strings.allApps, n: shown.value.length },
  ...categories.value.map((c) => ({ id: c.name, name: c.name, n: c.n })),
])
</script>

<template>
  <div class="store">
    <nav class="side" :aria-label="strings.categories">
      <button
        v-for="v in views"
        :key="v.id"
        type="button"
        class="cat"
        :class="{ on: iceStoreView === v.id }"
        @click="iceStoreView = v.id"
      >
        {{ v.name }} <span class="n">{{ v.n }}</span>
      </button>
      <span class="sep" />
      <button type="button" class="cat" :class="{ on: iceStoreView === 'bundles' }" @click="iceStoreView = 'bundles'">
        {{ strings.tabBundles }} <span class="n">{{ bundles.length }}</span>
      </button>
      <button type="button" class="cat" :class="{ on: iceStoreView === 'installed' }" @click="iceStoreView = 'installed'">
        {{ strings.installed }} <span class="n">{{ installedCount }}</span>
      </button>
      <button type="button" class="cat" :class="{ on: iceStoreView === 'updates' }" @click="iceStoreView = 'updates'">
        {{ strings.updates }} <span v-if="updatesCount" class="n hot">{{ updatesCount }}</span>
      </button>
    </nav>

    <div class="main">
      <template v-if="!page">
        <div class="tools">
          <label class="search">
            <Icon name="search" :size="16" />
            <input v-model="query" type="search" :placeholder="strings.searchIceStore" :aria-label="strings.searchIceStore" />
          </label>
          <label v-if="iceStoreView !== 'bundles'" class="ghost sort" :title="strings.sortBy">
            <Icon name="sort" :size="16" />
            <select v-model="iceStoreSort" :aria-label="strings.sortBy">
              <option v-for="k in SORTS" :key="k" :value="k">{{ strings.sortNames[k] }}</option>
            </select>
          </label>
          <button type="button" class="ghost" :disabled="refreshing" :title="strings.refresh" :aria-label="strings.refresh" @click="refresh">
            <Icon name="refresh" :size="17" :class="{ spin: refreshing }" />
          </button>
          <a class="ghost wide" :href="dashboardUrl()" target="_blank" rel="noopener">
            <Icon name="grid" :size="17" /> {{ strings.officialStore }}
          </a>
        </div>
        <p v-if="catalogError" class="warn small">{{ catalogList ? strings.catalogOffline(catalogError) : catalogError }}</p>

        <template v-if="iceStoreView === 'bundles'">
          <p class="mut small">{{ strings.bundlesHint }}</p>
          <ul class="bundles">
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
        </template>

        <template v-else>
          <template v-if="home && catalogList">
            <FeaturedRow v-if="featured.length" :apps="featured" @open="show" />
            <section v-if="newest.length" class="newest" :aria-label="strings.recentlyAdded">
              <header class="row-head">
                <h4>{{ strings.recentlyAdded }}</h4>
                <button type="button" class="link" @click="seeNewest">{{ strings.seeAll }}</button>
              </header>
              <ul class="mini">
                <li v-for="a in newest" :key="a.id">
                  <button type="button" class="mini-app" :data-app="a.id" @click="show(a)">
                    <AppIcon :src="a.icon" :name="a.title" :size="36" />
                    <span class="mini-text">
                      <span class="mini-title">{{ a.title }}</span>
                      <span class="mut">{{ ago(new Date(a.added! * 1000)) }}</span>
                    </span>
                  </button>
                </li>
              </ul>
            </section>
          </template>

          <header class="row-head all-head">
            <h4 v-if="home && (featured.length || newest.length)">{{ strings.allApps }}</h4>
            <span v-else class="count mono">{{ strings.appCount(list.length) }}</span>
            <span class="head-end">
              <span v-if="home && (featured.length || newest.length)" class="count mono">{{ strings.appCount(list.length) }}</span>
              <button
                v-if="iceStoreView === 'updates' && admin && updatesCount"
                type="button"
                class="pill update"
                :disabled="!!updateRun"
                @click="runUpdateAll"
              >
                {{ updateRun ? strings.updatingN(updateRun.done + 1, updateRun.total) : strings.updateAll(updatesCount) }}
              </button>
            </span>
          </header>
          <p v-if="!catalogList && !catalogError" class="mut">{{ strings.loading }}</p>
          <p v-else-if="catalogList && !list.length" class="mut">{{ strings.catalogEmpty }}</p>

          <ul class="cards">
            <StoreCard v-for="a in list" :key="a.id" :app="a" :date="cardDate" @open="show" />
          </ul>

          <p v-if="hiddenCount || showOtherDevices" class="other mut small">
            <template v-if="!showOtherDevices">{{ strings.hiddenOther(hiddenCount) }}</template>
            <label class="check">
              <input v-model="showOtherDevices" type="checkbox" />
              {{ strings.showOtherDevices }}
            </label>
          </p>
        </template>
      </template>

      <AppPage
        v-else
        :app="page.app"
        :meta="page.meta"
        :error="pageError"
        :all="shown"
        @back="page = null"
        @open="show"
        @bundles="(page = null), (iceStoreView = 'bundles')"
      />
    </div>
  </div>
</template>

<style scoped src="./icestore/shared.css"></style>
<style scoped>
.store {
  display: grid;
  grid-template-columns: 168px 1fr;
  gap: 20px;
  min-height: 50vh;
}
.side {
  display: flex;
  flex-direction: column;
  gap: 2px;
  position: sticky;
  top: 0;
  align-self: start;
}
.cat {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 11px;
  border: 0;
  border-radius: 9px;
  background: none;
  color: var(--mut);
  font: inherit;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
}
.cat:hover {
  color: var(--text);
  background: rgba(var(--fg-rgb), 0.04);
}
.cat.on {
  color: var(--text);
  background: rgba(var(--ac-rgb), 0.12);
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
.sep {
  height: 1px;
  margin: 10px 8px;
  background: var(--line);
}
.main {
  min-width: 0;
}
.tools {
  display: flex;
  gap: 10px;
  margin-bottom: 18px;
}
.search {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  padding: 0 16px;
  border-radius: var(--radius-pill);
  border: 1px solid var(--line);
  background: rgba(var(--fg-rgb), 0.03);
  color: var(--mut);
}
.search input {
  flex: 1;
  min-width: 0;
  padding: 8px 0;
  border: 0;
  background: none;
  color: var(--text);
  font: inherit;
  font-size: 13.5px;
  outline: none;
}
.ghost {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 0 12px;
  border-radius: var(--radius-pill);
  border: 1px solid var(--line);
  background: rgba(var(--fg-rgb), 0.03);
  color: var(--text);
  font: inherit;
  font-size: 12.5px;
  text-decoration: none;
  cursor: pointer;
  white-space: nowrap;
}
.ghost:hover {
  border-color: rgba(var(--fg-rgb), 0.25);
}
.sort {
  color: var(--mut);
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
.sort:focus-within {
  border-color: rgba(var(--ac-rgb), 0.6);
}
.spin {
  animation: spin 0.9s linear infinite;
}
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
.count {
  font-size: 11.5px;
  color: var(--mut);
}
.all-head {
  align-items: center;
  min-height: 30px;
  scroll-margin-top: 12px;
}
.head-end {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  margin-left: auto;
}
.cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(215px, 1fr));
  gap: 10px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.newest {
  margin-bottom: 22px;
}
.mini {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.mini-app {
  display: flex;
  align-items: center;
  gap: 11px;
  width: 100%;
  padding: 7px 10px 7px 7px;
  border: 1px solid transparent;
  border-radius: 12px;
  background: none;
  color: var(--text);
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.mini-app:hover {
  border-color: var(--line);
  background: var(--card);
}
.mini-text {
  display: grid;
  min-width: 0;
  font-size: 11.5px;
}
.mini-title {
  font-size: 13px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.other {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 14px;
  margin: 18px 0 0;
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
@media (max-width: 760px) {
  .store {
    grid-template-columns: 1fr;
  }
  .side {
    position: static;
    flex-direction: row;
    overflow-x: auto;
  }
  .cat {
    flex: none;
  }
  .sep {
    width: 1px;
    height: auto;
    margin: 6px 4px;
  }
  .wide {
    display: none;
  }
}
.bundles {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 14px;
  margin: 12px 0 0;
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
</style>
