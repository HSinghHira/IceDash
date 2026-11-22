<script setup lang="ts">
// An app's page in IceStore: header with its buttons (Uninstall once it's
// installed), facts, screenshots (full size on click), the description, and
// the bundles and apps that go with it.
import { computed, ref, watch } from 'vue'
import type { CatalogApp, Meta } from '../../api/apps'
import AppIcon from '../../components/AppIcon.vue'
import Icon from '../../components/Icon.vue'
import { bytes } from '../../lib/format'
import { host } from '../../lib/host'
import { bundlesWith, relatedApps, shortNum } from '../../lib/icestore'
import { markdown } from '../../lib/markdown'
import { jobFor } from '../../store'
import { strings, uiLocale } from '../../strings'
import { act, admin, button, day, fitsHere, installedOf, uninstall } from './actions'
import ShotViewer from './ShotViewer.vue'
import StoreCard from './StoreCard.vue'

const props = defineProps<{ app: CatalogApp; meta: Meta; error?: string; all: CatalogApp[] }>()
const emit = defineEmits<{ back: []; open: [app: CatalogApp]; bundles: [] }>()

const b = computed(() => button(props.app))
const inst = computed(() => installedOf(props.app))
const busy = computed(() => jobFor(props.app.name)?.state === 'running')
const fits = computed(() => fitsHere(props.app))
const shots = computed(() => props.meta.screenshot_link ?? [])
const related = computed(() => relatedApps(props.app, props.all))
const bundles = computed(() => bundlesWith(props.app, props.all))
const cats = computed(() => {
  const main = props.app.category.toLowerCase()
  return (props.app.categories ?? []).filter((c) => c && c.toLowerCase() !== main)
})
const archs = computed(() => (props.app.architectures ?? []).join(', '))

const viewing = ref<number | null>(null)
const top = ref<HTMLElement | null>(null)
// Another app's page opened from the related rows: start at its top.
watch(
  () => props.app.id,
  () => {
    viewing.value = null
    top.value?.scrollIntoView({ block: 'start' })
  },
)
</script>

<template>
  <article ref="top" class="page" :data-app="app.id">
    <button type="button" class="back" @click="emit('back')"><Icon name="arrowLeft" :size="17" /> {{ strings.back }}</button>

    <header class="head">
      <AppIcon :src="app.icon" :name="app.title" :size="76" />
      <div class="titles">
        <span class="kicker">{{ app.category }}<template v-if="app.developer"> · {{ app.developer }}</template></span>
        <h3>{{ app.title }}</h3>
        <p class="mut">{{ app.tagline }}</p>
      </div>
      <div class="actions">
        <button type="button" class="pill big" :class="b.kind" :disabled="b.disabled" :title="b.title" @click="act(app)">
          {{ b.label }}
        </button>
        <button
          v-if="inst && admin"
          type="button"
          class="pill big danger"
          :disabled="busy"
          @click="uninstall(app)"
        >
          <Icon name="trash" :size="15" />{{ strings.uninstall }}
        </button>
      </div>
    </header>

    <ul class="facts big">
      <li v-if="app.stars != null" :title="strings.starsTitle"><Icon name="star" :size="14" />{{ strings.starsN(shortNum(app.stars)) }}</li>
      <li v-if="app.forks != null" :title="strings.forksTitle"><Icon name="fork" :size="14" />{{ strings.forksN(shortNum(app.forks)) }}</li>
      <li class="mono">{{ strings.version(inst && inst.version !== app.version ? `${inst.version} → ${app.version}` : app.version) }}</li>
      <li v-if="app.updated"><Icon name="clock" :size="14" />{{ strings.updatedOn(day(app.updated, uiLocale())) }}</li>
      <li v-if="app.added">{{ strings.addedOn(day(app.added, uiLocale())) }}</li>
      <li v-if="meta.min_memory"><Icon name="chip" :size="14" />{{ strings.needsMemory(bytes(meta.min_memory)) }}</li>
      <li v-if="archs" :class="{ warn: !fits }">
        <Icon :name="fits ? 'check' : 'alert'" :size="14" />{{ strings.runsOn(archs) }}
      </li>
    </ul>
    <p v-if="!fits" class="warn small">{{ strings.notForDeviceLong(host.arch) }}</p>
    <ul v-if="cats.length" class="chips">
      <li v-for="c in cats" :key="c">{{ c }}</li>
    </ul>
    <p v-if="error" class="warn small">{{ error }}</p>

    <ul v-if="shots.length" class="shots" :aria-label="strings.screenshotN(1, shots.length)">
      <li v-for="(s, i) in shots" :key="s">
        <button type="button" class="shot" :aria-label="strings.screenshotN(i + 1, shots.length)" @click="viewing = i">
          <img :src="s" alt="" loading="lazy" referrerpolicy="no-referrer" />
        </button>
      </li>
    </ul>

    <!-- eslint-disable-next-line vue/no-v-html -- markdown() escapes everything first -->
    <div v-if="meta.description" class="md" v-html="markdown(meta.description)" />
    <template v-if="meta.release_note">
      <h4>{{ strings.whatsNew }}</h4>
      <!-- eslint-disable-next-line vue/no-v-html -->
      <div class="md" v-html="markdown(meta.release_note)" />
    </template>
    <p v-if="meta.repo || meta.support || meta.docs" class="links">
      <a v-if="meta.repo" :href="meta.repo" target="_blank" rel="noopener">{{ strings.source }} <Icon name="external" :size="13" /></a>
      <a v-if="meta.docs" :href="meta.docs" target="_blank" rel="noopener">{{ strings.docs }} <Icon name="external" :size="13" /></a>
      <a v-if="meta.support" :href="meta.support" target="_blank" rel="noopener">{{ strings.support }} <Icon name="external" :size="13" /></a>
    </p>

    <section v-for="x in bundles" :key="x.bundle.id" class="bundle" :data-bundle="x.bundle.id">
      <span class="bicon"><Icon :name="x.bundle.icon" :size="18" /></span>
      <div class="btext">
        <span class="title">{{ strings.partOfBundle(strings.bundles[x.bundle.id]?.name ?? x.bundle.id) }}</span>
        <span class="mut small">{{ x.apps.filter((a) => a.id !== app.id).map((a) => a.title).join(', ') }}</span>
      </div>
      <button type="button" class="link" @click="emit('bundles')">{{ strings.seeAll }}</button>
    </section>

    <section v-if="related.length" class="related">
      <header class="row-head"><h4>{{ strings.moreIn(app.category) }}</h4></header>
      <ul class="cards">
        <StoreCard v-for="a in related" :key="a.id" :app="a" @open="emit('open', $event)" />
      </ul>
    </section>

    <ShotViewer v-if="viewing !== null" v-model="viewing" :shots="shots" :title="app.title" @close="viewing = null" />
  </article>
</template>

<style scoped src="./shared.css"></style>
<style scoped>
.back {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 16px;
  padding: 0;
  border: 0;
  background: none;
  color: var(--mut);
  font: inherit;
  cursor: pointer;
}
.back:hover {
  color: var(--text);
}
.head {
  display: flex;
  align-items: center;
  gap: 20px;
  flex-wrap: wrap;
}
.titles {
  flex: 1;
  min-width: 220px;
}
.titles p {
  margin: 0;
}
h3 {
  margin: 2px 0 4px;
  font-size: 24px;
  line-height: 1.2;
}
h4 {
  margin: 26px 0 8px;
  font-size: 16px;
}
.actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.facts.big {
  margin-top: 18px;
  padding: 12px 14px;
  gap: 8px 20px;
  border-radius: 14px;
  border: 1px solid var(--line);
  background: var(--card);
  font-size: 12.5px;
}
.facts .warn {
  color: var(--warn);
}
.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 12px 0 0;
  padding: 0;
  list-style: none;
}
.chips li {
  padding: 3px 10px;
  border-radius: var(--radius-pill);
  border: 1px solid var(--line);
  color: var(--mut);
  font-size: 11.5px;
  text-transform: capitalize;
}
.shots {
  display: flex;
  gap: 12px;
  margin: 20px 0;
  padding: 0 0 6px;
  list-style: none;
  overflow-x: auto;
  scroll-snap-type: x proximity;
}
.shots li {
  flex: none;
  scroll-snap-align: start;
}
.shot {
  display: block;
  padding: 0;
  border: 1px solid var(--line);
  border-radius: 14px;
  background: none;
  overflow: hidden;
  cursor: zoom-in;
}
.shot:hover {
  border-color: rgba(var(--ac-rgb), 0.45);
}
.shot img {
  display: block;
  height: 240px;
  width: auto;
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
.bundle {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 24px;
  padding: 14px 16px;
  border-radius: var(--radius-tile);
  border: 1px solid var(--line);
  background: var(--card);
}
.bicon {
  display: grid;
  place-items: center;
  width: 38px;
  height: 38px;
  flex: none;
  border-radius: 11px;
  background: rgba(var(--ac-rgb), 0.12);
  color: var(--ac);
}
.btext {
  display: grid;
  flex: 1;
  min-width: 0;
}
.btext .title {
  font-size: 14px;
}
.related {
  margin-top: 28px;
}
.cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(215px, 1fr));
  gap: 10px;
  margin: 0;
  padding: 0;
  list-style: none;
}
@media (max-width: 600px) {
  .shot img {
    height: 180px;
  }
}
</style>
