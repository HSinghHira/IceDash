<script setup lang="ts">
// IceStore's home page: a headline beside the featured banner, then rows of
// popular, most forked, new and updated apps (each led by its screenshot),
// a few bundles, and the categories.
import { computed } from 'vue'
import type { CatalogApp } from '../../api/apps'
import AppIcon from '../../components/AppIcon.vue'
import Icon from '../../components/Icon.vue'
import { availableBundles } from '../../lib/bundles'
import { host } from '../../lib/host'
import {
  categoryIcon,
  featuredApps,
  forkedApps,
  newestApps,
  popularApps,
  updatedApps,
  withShots,
  type SortKey,
} from '../../lib/icestore'
import { strings } from '../../strings'
import { installedOf } from './actions'
import HeroCarousel from './HeroCarousel.vue'
import RankedList from './RankedList.vue'
import ShowcaseRow from './ShowcaseRow.vue'

const props = defineProps<{ apps: CatalogApp[]; categories: { name: string; n: number }[] }>()
defineEmits<{ open: [app: CatalogApp]; view: [id: string, sort?: SortKey] }>()

// Apps with a screenshot lead the banner; the rest follow in catalog order.
const featured = computed(() => {
  const f = featuredApps(props.apps)
  return [...f.filter((a) => a.thumbnail), ...f.filter((a) => !a.thumbnail)]
})
// The rows show screenshots, so they pick from the apps that have one.
const pictured = computed(() => withShots(props.apps))
const popular = computed(() => popularApps(pictured.value, 10))
const forked = computed(() => forkedApps(pictured.value, 6))
const newest = computed(() => newestApps(pictured.value, 10))
// Updated, leaving out what New already shows.
const updated = computed(() => {
  const shown = new Set(newest.value.map((a) => a.id))
  return updatedApps(pictured.value, 20).filter((a) => !shown.has(a.id)).slice(0, 10)
})
const allBundles = computed(() => availableBundles(props.apps))
const bundles = computed(() =>
  allBundles.value
    .slice(0, 3)
    .map((x) => ({ ...x, words: strings.bundles[x.bundle.id] ?? { name: x.bundle.id, about: '' }, have: x.apps.filter((a) => installedOf(a)).length })),
)
</script>

<template>
  <div class="discover">
    <section class="hero" :class="{ solo: !featured.length }">
      <div class="pitch">
        <h2>{{ strings.heroTitle }}</h2>
        <p>{{ strings.heroBody(apps.length, host.name) }}</p>
        <span class="cta">
          <button type="button" class="pill install big" @click="$emit('view', 'all')">{{ strings.allApps }}</button>
          <button type="button" class="pill big" @click="$emit('view', 'bundles')"><Icon name="package" :size="15" />{{ strings.tabBundles }}</button>
        </span>
        <dl class="stats">
          <div><dt>{{ strings.statApps }}</dt><dd class="mono">{{ apps.length }}</dd></div>
          <div><dt>{{ strings.tabBundles }}</dt><dd class="mono">{{ allBundles.length }}</dd></div>
          <div><dt>{{ strings.categories }}</dt><dd class="mono">{{ categories.length }}</dd></div>
        </dl>
      </div>
      <HeroCarousel v-if="featured.length" :apps="featured" @open="$emit('open', $event)" />
    </section>

    <ShowcaseRow
      v-if="popular.length"
      :title="strings.popular"
      :hint="strings.popularHint"
      :apps="popular"
      fact="stars"
      @open="$emit('open', $event)"
      @all="$emit('view', 'all', 'stars')"
    />
    <RankedList
      v-if="forked.length"
      :title="strings.mostForked"
      :hint="strings.mostForkedHint"
      :apps="forked"
      by="forks"
      @open="$emit('open', $event)"
      @all="$emit('view', 'all', 'forks')"
    />
    <ShowcaseRow
      v-if="newest.length"
      :title="strings.newInStore"
      :hint="strings.newHint"
      :apps="newest"
      fact="added"
      @open="$emit('open', $event)"
      @all="$emit('view', 'all', 'newest')"
    />

    <section v-if="bundles.length" class="bundles" :aria-label="strings.tabBundles">
      <header class="sec-head">
        <div>
          <h3>{{ strings.tabBundles }}</h3>
          <p class="hint">{{ strings.bundlesShort }}</p>
        </div>
        <button type="button" class="link" @click="$emit('view', 'bundles')">{{ strings.seeAll }}</button>
      </header>
      <ul class="bgrid">
        <li v-for="b in bundles" :key="b.bundle.id">
          <button type="button" class="bundle" :data-bundle="b.bundle.id" @click="$emit('view', 'bundles')">
            <span class="stack">
              <AppIcon v-for="a in b.apps.slice(0, 4)" :key="a.id" :src="a.icon" :name="a.title" :size="38" />
            </span>
            <span class="bname"><Icon :name="b.bundle.icon" :size="15" />{{ b.words.name }}</span>
            <span class="babout">{{ b.words.about }}</span>
            <span class="bhave">{{ strings.bundleHave(b.have, b.apps.length) }}</span>
          </button>
        </li>
      </ul>
    </section>

    <ShowcaseRow
      v-if="updated.length"
      :title="strings.recentlyUpdated"
      :hint="strings.updatedHint"
      :apps="updated"
      fact="updated"
      @open="$emit('open', $event)"
      @all="$emit('view', 'all', 'updated')"
    />

    <section class="browse" :aria-label="strings.categories">
      <h3>{{ strings.browseCategories }}</h3>
      <ul class="cats">
        <li v-for="c in categories" :key="c.name">
          <button type="button" class="cat-tile" @click="$emit('view', c.name)">
            <span class="ico"><Icon :name="categoryIcon(c.name)" :size="20" /></span>
            <span class="cat-name">{{ c.name }}</span>
            <span class="cat-n mono">{{ c.n }}</span>
          </button>
        </li>
      </ul>
    </section>
  </div>
</template>

<style scoped src="./shared.css"></style>
<style scoped>
.discover {
  container-type: inline-size;
}
.hero {
  display: grid;
  grid-template-columns: minmax(220px, 0.8fr) 1.4fr;
  gap: 28px;
  align-items: stretch;
}
.hero.solo {
  grid-template-columns: 1fr;
}
.pitch {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 14px;
  padding: 8px 0;
}
h2 {
  margin: 0;
  font-size: clamp(28px, 4.6cqi, 40px);
  font-weight: 700;
  line-height: 1.08;
  letter-spacing: -0.03em;
}
.pitch p {
  margin: 0;
  color: var(--mut);
  font-size: 14px;
  line-height: 1.5;
  max-width: 34ch;
}
.cta {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 4px;
}
.stats {
  display: flex;
  gap: 28px;
  margin: 10px 0 0;
  padding-top: 16px;
  border-top: 1px solid var(--line);
}
.stats div {
  display: flex;
  flex-direction: column-reverse;
  gap: 1px;
}
.stats dt {
  color: var(--mut);
  font-size: 12px;
}
.stats dd {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}
.sec-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}
.sec-head h3 {
  margin: 0;
}
.hint {
  margin: 3px 0 0;
  color: var(--mut);
  font-size: 12.5px;
}
.bundles {
  margin-top: 40px;
}
.bgrid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.bundle {
  display: grid;
  align-content: start;
  gap: 8px;
  width: 100%;
  height: 100%;
  padding: 16px;
  border-radius: var(--radius-tile);
  border: 1px solid var(--line);
  background: var(--card);
  color: var(--text);
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.15s;
}
.bundle:hover {
  border-color: rgba(var(--ac-rgb), 0.45);
}
.stack {
  display: flex;
  margin-bottom: 4px;
}
.stack > :deep(*) {
  border-radius: 11px;
  box-shadow: 0 0 0 3px rgb(var(--surface-rgb));
}
.stack > :deep(* + *) {
  margin-left: -8px;
}
.bname {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 14.5px;
  font-weight: 700;
}
.bname :deep(svg) {
  color: var(--ac);
}
.babout {
  color: var(--mut);
  font-size: 12.5px;
  line-height: 1.45;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.bhave {
  color: var(--mut);
  font-family: var(--font-mono);
  font-size: 11px;
}
.browse {
  margin-top: 40px;
}

h3 {
  margin: 0 0 14px;
  font-size: 20px;
  font-weight: 700;
}
.cats {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(170px, 1fr));
  gap: 10px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.cat-tile {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  padding: 12px 14px;
  border-radius: var(--radius-tile);
  border: 1px solid var(--line);
  background: var(--card);
  color: var(--text);
  font: inherit;
  font-size: 14px;
  font-weight: 600;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.15s;
}
.cat-tile:hover {
  border-color: rgba(var(--ac-rgb), 0.45);
}
.ico {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  flex: none;
  border-radius: 11px;
  background: rgba(var(--ac-rgb), 0.12);
  color: var(--ac);
}
.cat-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.cat-n {
  color: var(--mut);
  font-size: 12px;
  font-weight: 400;
}
@container (max-width: 680px) {
  .bgrid {
    grid-template-columns: 1fr;
  }
  .hero {
    grid-template-columns: 1fr;
  }
  .pitch {
    order: 2;
  }
}
</style>
