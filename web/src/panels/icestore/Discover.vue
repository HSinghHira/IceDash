<script setup lang="ts">
// IceStore's home page: a headline beside the featured banner, then rows of
// popular, new and updated apps, then the categories.
import { computed } from 'vue'
import type { CatalogApp } from '../../api/apps'
import Icon from '../../components/Icon.vue'
import { host } from '../../lib/host'
import { categoryIcon, featuredApps, newestApps, popularApps, updatedApps, type SortKey } from '../../lib/icestore'
import { strings } from '../../strings'
import HeroCarousel from './HeroCarousel.vue'
import ShowcaseRow from './ShowcaseRow.vue'

const props = defineProps<{ apps: CatalogApp[]; categories: { name: string; n: number }[] }>()
defineEmits<{ open: [app: CatalogApp]; view: [id: string, sort?: SortKey] }>()

// Apps with a screenshot lead the banner; the rest follow in catalog order.
const featured = computed(() => {
  const f = featuredApps(props.apps)
  return [...f.filter((a) => a.thumbnail), ...f.filter((a) => !a.thumbnail)]
})
const popular = computed(() => popularApps(props.apps, 10))
const newest = computed(() => newestApps(props.apps, 10))
// Updated, leaving out what New already shows.
const updated = computed(() => {
  const shown = new Set(newest.value.map((a) => a.id))
  return updatedApps(props.apps, 20).filter((a) => !shown.has(a.id)).slice(0, 10)
})
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
      </div>
      <HeroCarousel v-if="featured.length" :apps="featured" @open="$emit('open', $event)" />
    </section>

    <ShowcaseRow v-if="popular.length" :title="strings.popular" :apps="popular" fact="stars" @open="$emit('open', $event)" @all="$emit('view', 'all', 'stars')" />
    <ShowcaseRow v-if="newest.length" :title="strings.newInStore" :apps="newest" fact="added" @open="$emit('open', $event)" @all="$emit('view', 'all', 'newest')" />
    <ShowcaseRow v-if="updated.length" :title="strings.recentlyUpdated" :apps="updated" fact="updated" @open="$emit('open', $event)" @all="$emit('view', 'all', 'updated')" />

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
.browse {
  margin-top: 38px;
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
  .hero {
    grid-template-columns: 1fr;
  }
  .pitch {
    order: 2;
  }
}
</style>
