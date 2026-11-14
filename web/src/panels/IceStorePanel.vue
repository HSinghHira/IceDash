<script setup lang="ts">
// The built-in IceStore (mock-up screen 2, behind the popup): categories on
// the left, search, app cards with Install / Open / Update, and an app page.
import { computed, ref, watch } from 'vue'
import * as api from '../api/apps'
import AppIcon from '../components/AppIcon.vue'
import Icon from '../components/Icon.vue'
import { availableBundles } from '../lib/bundles'
import { bytes } from '../lib/format'
import { dashboardUrl } from '../lib/host'
import { markdown } from '../lib/markdown'
import {
  appInfo,
  apps,
  catalogError,
  catalogList,
  iceStoreFocus,
  iceStoreView,
  jobFor,
  loadCatalog,
  openApp,
  setupFor,
  startBundle,
  toast,
  trackJob,
  user,
} from '../store'
import { strings } from '../strings'

const query = ref('')
const page = ref<{ app: api.CatalogApp; meta: api.Meta } | null>(null)
const pageError = ref('')
const refreshing = ref(false)
const admin = computed(() => !!user.value?.admin)

const all = computed(() => catalogList.value?.apps ?? [])
const categories = computed(() => {
  const seen = new Map<string, number>()
  for (const a of all.value) if (a.category) seen.set(a.category, (seen.get(a.category) ?? 0) + 1)
  return [...seen.entries()].sort((a, b) => a[0].localeCompare(b[0])).map(([name, n]) => ({ name, n }))
})

/** The installed copy of a catalog app, by its store id. */
const installedOf = (a: api.CatalogApp) => Object.values(appInfo.value).find((i) => i.store_id === a.id)

const list = computed(() => {
  const q = query.value.trim().toLowerCase()
  let l = all.value
  const v = iceStoreView.value
  if (v === 'installed') l = l.filter((a) => installedOf(a))
  else if (v === 'updates') l = l.filter((a) => installedOf(a)?.update)
  else if (v !== 'all') l = l.filter((a) => a.category === v)
  if (q)
    l = l.filter((a) =>
      [a.title, a.tagline, a.category, a.developer].some((s) => (s ?? '').toLowerCase().includes(q)),
    )
  return [...l].sort((a, b) => a.title.localeCompare(b.title, 'en', { sensitivity: 'base' }))
})

// ---- Phase 5b: bundles ----
const bundles = computed(() => {
  const q = query.value.trim().toLowerCase()
  return availableBundles(all.value)
    .map((x) => ({ ...x, words: strings.bundles[x.bundle.id] ?? { name: x.bundle.id, about: '' }, have: x.apps.filter((a) => installedOf(a)).length }))
    .filter((x) => !q || [x.words.name, x.words.about, ...x.apps.map((a) => a.title)].some((s) => s.toLowerCase().includes(q)))
})
function installBundle(id: string, ids: string[]) {
  startBundle(id, ids)
}

const updatesCount = computed(() => all.value.filter((a) => installedOf(a)?.update).length)
const installedCount = computed(() => all.value.filter((a) => installedOf(a)).length)

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
    page.value = await api.catalogApp(a.id)
  } catch (e) {
    pageError.value = e instanceof Error ? e.message : String(e)
  }
}

async function refresh() {
  refreshing.value = true
  await loadCatalog(true)
  refreshing.value = false
}

type Btn = { label: string; kind: 'install' | 'open' | 'update' | 'busy'; disabled?: boolean }
function button(a: api.CatalogApp): Btn {
  const inst = installedOf(a)
  const job = jobFor(a.name)
  if (job?.state === 'running') return { label: job.kind === 'update' ? strings.updatingApp : strings.installing, kind: 'busy', disabled: true }
  if (inst?.update) return { label: strings.update, kind: 'update', disabled: !admin.value }
  if (inst) return { label: strings.openApp, kind: 'open' }
  return { label: strings.install, kind: 'install', disabled: !admin.value }
}

async function act(a: api.CatalogApp) {
  const b = button(a)
  if (b.kind === 'install') setupFor.value = a.id
  else if (b.kind === 'open') {
    const grid = apps.value.find((x) => x.id === installedOf(a)?.name)
    if (grid) openApp(grid)
  } else if (b.kind === 'update') {
    const inst = installedOf(a)
    if (!inst) return
    try {
      trackJob(await api.updateApp(inst.name))
      toast({ title: strings.updatingApp, body: a.title })
    } catch (e) {
      toast({ title: strings.update, body: e instanceof Error ? e.message : String(e) })
    }
  }
}

const views = computed(() => [
  { id: 'all', name: strings.allApps, n: all.value.length },
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
        {{ v.name }}
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
          <button type="button" class="ghost" :disabled="refreshing" :title="strings.refresh" @click="refresh">
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
                  @click="installBundle(b.bundle.id, b.apps.map((a) => a.id))"
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
        <p class="count mono">{{ strings.appCount(list.length) }}</p>
        <p v-if="!catalogList && !catalogError" class="mut">{{ strings.loading }}</p>
        <p v-else-if="catalogList && !list.length" class="mut">{{ strings.catalogEmpty }}</p>

        <ul class="cards">
          <li v-for="a in list" :key="a.id" class="card">
            <button type="button" class="card-hit" :aria-label="`${a.title}: ${strings.details}`" @click="show(a)" />
            <div class="card-top">
              <AppIcon :src="a.icon" :name="a.title" :size="42" />
              <div class="card-titles">
                <span class="kicker">{{ a.category }}</span>
                <span class="title">{{ a.title }}</span>
              </div>
            </div>
            <p class="tagline">{{ a.tagline }}</p>
            <div class="card-foot">
              <span class="mono ver">{{ a.version }}</span>
              <button
                type="button"
                class="pill"
                :class="button(a).kind"
                :disabled="button(a).disabled"
                :title="button(a).disabled && button(a).kind !== 'busy' ? strings.adminInstallOnly : undefined"
                @click.stop="act(a)"
              >
                {{ button(a).label }}
              </button>
            </div>
          </li>
        </ul>
        </template>
      </template>

      <article v-else class="page">
        <button type="button" class="back" @click="page = null"><Icon name="arrowLeft" :size="17" /> {{ strings.back }}</button>
        <header class="page-head">
          <AppIcon :src="page.app.icon" :name="page.app.title" :size="60" />
          <div class="page-titles">
            <span class="kicker">{{ page.app.category }}</span>
            <h3>{{ page.app.title }}</h3>
            <p class="mut">{{ page.app.tagline }}</p>
            <p class="mono small mut">
              {{ strings.version(page.app.version) }}<template v-if="page.app.developer"> · {{ page.app.developer }}</template>
              <template v-if="page.meta.min_memory"> · {{ strings.needsMemory(bytes(page.meta.min_memory)) }}</template>
            </p>
          </div>
          <button
            type="button"
            class="pill big"
            :class="button(page.app).kind"
            :disabled="button(page.app).disabled"
            :title="button(page.app).disabled && button(page.app).kind !== 'busy' ? strings.adminInstallOnly : undefined"
            @click="act(page.app)"
          >
            {{ button(page.app).label }}
          </button>
        </header>
        <p v-if="pageError" class="warn small">{{ pageError }}</p>
        <div v-if="page.meta.screenshot_link?.length" class="shots">
          <img v-for="s in page.meta.screenshot_link" :key="s" :src="s" alt="" loading="lazy" referrerpolicy="no-referrer" />
        </div>
        <!-- eslint-disable-next-line vue/no-v-html -- markdown() escapes everything first -->
        <div v-if="page.meta.description" class="md" v-html="markdown(page.meta.description)" />
        <template v-if="page.meta.release_note">
          <h4>{{ strings.whatsNew }}</h4>
          <!-- eslint-disable-next-line vue/no-v-html -->
          <div class="md" v-html="markdown(page.meta.release_note)" />
        </template>
        <p v-if="page.meta.repo || page.meta.support || page.meta.docs" class="links">
          <a v-if="page.meta.repo" :href="page.meta.repo" target="_blank" rel="noopener">{{ strings.source }} <Icon name="external" :size="13" /></a>
          <a v-if="page.meta.docs" :href="page.meta.docs" target="_blank" rel="noopener">{{ strings.docs }} <Icon name="external" :size="13" /></a>
          <a v-if="page.meta.support" :href="page.meta.support" target="_blank" rel="noopener">{{ strings.support }} <Icon name="external" :size="13" /></a>
        </p>
      </article>
    </div>
  </div>
</template>

<style scoped>
.store {
  display: grid;
  grid-template-columns: 158px 1fr;
  gap: 18px;
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
  font-size: 12px;
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
  margin-bottom: 10px;
}
.search {
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
.spin {
  animation: spin 0.9s linear infinite;
}
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
.count {
  margin: 4px 0 10px;
  text-align: right;
  font-size: 11.5px;
  color: var(--mut);
}
.cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(205px, 1fr));
  gap: 10px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.card {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 9px;
  padding: 13px;
  border-radius: 16px;
  border: 1px solid var(--line);
  background: var(--card);
  transition: border-color 0.15s, background 0.15s;
}
.card:hover {
  border-color: rgba(var(--ac-rgb), 0.35);
  background: rgba(var(--ac-rgb), 0.04);
}
.card-hit {
  position: absolute;
  inset: 0;
  border: 0;
  border-radius: inherit;
  background: none;
  cursor: pointer;
}
.card-top {
  display: flex;
  align-items: center;
  gap: 11px;
  pointer-events: none;
}
.card-titles {
  display: grid;
  min-width: 0;
}
.kicker {
  font-family: var(--font-mono);
  font-size: 9.5px;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--ac2);
}
.title {
  font-size: 15px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.tagline {
  flex: 1;
  margin: 0;
  color: var(--mut);
  font-size: 12.5px;
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  pointer-events: none;
}
.card-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}
.ver {
  font-size: 11px;
  color: var(--mut);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  pointer-events: none;
}
.pill {
  position: relative;
  padding: 6px 16px;
  border-radius: var(--radius-pill);
  border: 1px solid var(--line);
  background: rgba(var(--fg-rgb), 0.05);
  color: var(--text);
  font: inherit;
  font-size: 12.5px;
  font-weight: 600;
  cursor: pointer;
  white-space: nowrap;
}
.pill.install {
  border: 0;
  background: var(--ac);
  color: var(--ink);
}
.pill.update {
  border: 0;
  background: var(--warn);
  color: var(--ink);
}
.pill:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.pill.big {
  padding: 9px 22px;
  font-size: 14px;
}
.back {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 14px;
  border: 0;
  background: none;
  color: var(--mut);
  font: inherit;
  cursor: pointer;
}
.back:hover {
  color: var(--text);
}
.page-head {
  display: flex;
  align-items: center;
  gap: 20px;
  flex-wrap: wrap;
}
.page-titles {
  flex: 1;
  min-width: 200px;
}
h3 {
  margin: 2px 0 4px;
  font-size: 21px;
}
h4 {
  margin: 22px 0 8px;
  font-size: 16px;
}
.page-titles p {
  margin: 0 0 4px;
}
.shots {
  display: flex;
  gap: 12px;
  margin: 20px 0;
  overflow-x: auto;
}
.shots img {
  height: 170px;
  border-radius: 14px;
  border: 1px solid var(--line);
  flex: none;
}
.md {
  color: rgba(var(--fg-rgb), 0.82);
  font-size: 13.5px;
  line-height: 1.6;
}
.md :deep(a) {
  color: var(--ac);
}
.md :deep(code) {
  font-family: var(--font-mono);
  font-size: 0.9em;
  padding: 1px 5px;
  border-radius: 5px;
  background: rgba(var(--fg-rgb), 0.07);
}
.md :deep(ul),
.md :deep(ol) {
  padding-left: 22px;
}
.links {
  display: flex;
  gap: 18px;
  margin-top: 20px;
}
.links a {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: var(--ac);
  text-decoration: none;
}
.mut {
  color: var(--mut);
}
.small {
  font-size: 13px;
}
.mono {
  font-family: var(--font-mono);
}
.warn {
  color: var(--warn);
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
.bhead .card-titles {
  flex: 1;
  min-width: 0;
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
