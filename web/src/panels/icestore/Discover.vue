<script setup lang="ts">
// IceStore's home page, laid out like Discover in the ZimaOS App Store: a
// big headline beside the featured banner; picture rows (Popular, Most
// forked); compact three-column lists (New in IceStore, Recently updated,
// the biggest categories); and bundles as spotlight panels between them.
// Only the banner and the two picture rows load screenshots.
import { computed } from 'vue'
import type { CatalogApp } from '../../api/apps'
import { availableBundles } from '../../lib/bundles'
import { host } from '../../lib/host'
import { featuredApps, forkedApps, newestApps, popularApps, sortApps, updatedApps, withShots, type SortKey } from '../../lib/icestore'
import { strings } from '../../strings'
import AppGrid from './AppGrid.vue'
import BundleSpotlight from './BundleSpotlight.vue'
import HeroCarousel from './HeroCarousel.vue'
import ShowcaseRow from './ShowcaseRow.vue'

const props = defineProps<{ apps: CatalogApp[]; categories: { name: string; n: number }[] }>()
defineEmits<{ open: [app: CatalogApp]; view: [id: string, sort?: SortKey] }>()

// Apps with a screenshot lead the banner; the rest follow in catalog order.
const featured = computed(() => {
  const f = featuredApps(props.apps)
  return [...f.filter((a) => a.thumbnail), ...f.filter((a) => !a.thumbnail)]
})
// The picture rows pick from apps that have a screenshot, and don't repeat each other.
const pictured = computed(() => withShots(props.apps))
const popular = computed(() => popularApps(pictured.value, 9))
const forked = computed(() => {
  const seen = new Set(popular.value.slice(0, 3).map((a) => a.id))
  return forkedApps(pictured.value, 12).filter((a) => !seen.has(a.id)).slice(0, 9)
})
const newest = computed(() => newestApps(props.apps, 6))
const updated = computed(() => {
  const seen = new Set(newest.value.map((a) => a.id))
  return updatedApps(props.apps, 12).filter((a) => !seen.has(a.id)).slice(0, 6)
})
// The two bundles with the most apps, as spotlights.
const spots = computed(() => [...availableBundles(props.apps)].sort((a, b) => b.apps.length - a.apps.length).slice(0, 2))
// The three biggest categories, their most-starred apps first, leaving out
// what the picture rows already show.
const sections = computed(() => {
  const seen = new Set([...popular.value, ...forked.value].map((a) => a.id))
  return [...props.categories]
    .sort((a, b) => b.n - a.n)
    .slice(0, 3)
    .map((c) => ({ name: c.name, apps: sortApps(props.apps.filter((a) => a.category === c.name && !seen.has(a.id)), 'stars').slice(0, 6) }))
    .filter((s) => s.apps.length)
})
</script>

<template>
  <div class="discover">
    <section class="hero" :class="{ solo: !featured.length }">
      <div class="pitch">
        <h2>
          <span class="soft">{{ strings.heroSoft }}</span>
          <span class="bold">{{ strings.heroBold }}</span>
        </h2>
        <p>{{ strings.heroBody(apps.length, host.name) }}</p>
        <span class="cta">
          <button type="button" class="pill install" @click="$emit('view', 'all')">{{ strings.allApps }}</button>
          <button type="button" class="pill" @click="$emit('view', 'bundles')">{{ strings.tabBundles }}</button>
        </span>
      </div>
      <HeroCarousel v-if="featured.length" :apps="featured" @open="$emit('open', $event)" />
    </section>

    <ShowcaseRow
      v-if="popular.length"
      :title="strings.popular"
      :apps="popular"
      fact="stars"
      @open="$emit('open', $event)"
      @all="$emit('view', 'all', 'stars')"
    />

    <AppGrid
      v-if="newest.length"
      :title="strings.newInStore"
      :apps="newest"
      when="added"
      @open="$emit('open', $event)"
      @all="$emit('view', 'all', 'newest')"
    />

    <BundleSpotlight
      v-if="spots[0]"
      :bundle="spots[0].bundle"
      :apps="spots[0].apps"
      @open="$emit('open', $event)"
      @bundles="$emit('view', 'bundles')"
    />

    <ShowcaseRow
      v-if="forked.length"
      :title="strings.mostForked"
      :apps="forked"
      fact="forks"
      @open="$emit('open', $event)"
      @all="$emit('view', 'all', 'forks')"
    />

    <AppGrid
      v-if="sections[0]"
      :title="sections[0].name"
      :apps="sections[0].apps"
      @open="$emit('open', $event)"
      @all="$emit('view', sections[0].name)"
    />

    <BundleSpotlight
      v-if="spots[1]"
      :bundle="spots[1].bundle"
      :apps="spots[1].apps"
      @open="$emit('open', $event)"
      @bundles="$emit('view', 'bundles')"
    />

    <AppGrid
      v-if="updated.length"
      :title="strings.recentlyUpdated"
      :apps="updated"
      when="updated"
      @open="$emit('open', $event)"
      @all="$emit('view', 'all', 'updated')"
    />

    <AppGrid
      v-for="s in sections.slice(1)"
      :key="s.name"
      :title="s.name"
      :apps="s.apps"
      @open="$emit('open', $event)"
      @all="$emit('view', s.name)"
    />
  </div>
</template>

<style scoped src="./shared.css"></style>
<style scoped>
.discover {
  container-type: inline-size;
  padding-bottom: 12px;
}
.hero {
  display: grid;
  grid-template-columns: minmax(0, 0.85fr) minmax(0, 1.3fr);
  gap: 36px;
  align-items: start;
}
.hero.solo {
  grid-template-columns: 1fr;
}
.pitch {
  display: flex;
  flex-direction: column;
  gap: 18px;
  padding-top: 18px;
}
h2 {
  display: grid;
  margin: 0;
  font-size: clamp(34px, 6.4cqi, 58px);
  font-weight: 800;
  line-height: 0.98;
  letter-spacing: -0.045em;
}
.soft {
  color: rgba(var(--fg-rgb), 0.38);
  font-weight: 700;
}
.pitch p {
  margin: 0;
  color: var(--mut);
  font-size: 14px;
  line-height: 1.55;
  max-width: 32ch;
}
.cta {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
.cta .pill {
  padding: 8px 18px;
  border-radius: 10px;
  font-size: 13.5px;
}
@container (max-width: 700px) {
  .hero {
    grid-template-columns: 1fr;
    gap: 24px;
  }
  .pitch {
    padding-top: 0;
  }
}
</style>
