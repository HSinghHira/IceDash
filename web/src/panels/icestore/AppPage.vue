<script setup lang="ts">
// An app's page in IceStore, laid out like the ZimaOS App Store's: a header
// card with the buttons (Uninstall once it's installed), four stat cards,
// the screenshots (full size on click), then About this app: what's new,
// the introduction beside an Info card, the bundles it's in, and more apps
// from its category.
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
import AppGrid from './AppGrid.vue'
import BundleSpotlight from './BundleSpotlight.vue'
import ShotViewer from './ShotViewer.vue'

const props = defineProps<{ app: CatalogApp; meta: Meta; error?: string; all: CatalogApp[] }>()
const emit = defineEmits<{ back: []; open: [app: CatalogApp]; bundles: []; view: [id: string] }>()

const b = computed(() => button(props.app))
const inst = computed(() => installedOf(props.app))
const busy = computed(() => jobFor(props.app.name)?.state === 'running')
const fits = computed(() => fitsHere(props.app))
const shots = computed(() => props.meta.screenshot_link ?? [])
const related = computed(() => relatedApps(props.app, props.all))
const bundles = computed(() => bundlesWith(props.app, props.all))
const archs = computed(() => (props.app.architectures ?? []).join(', '))

/** "41.1 MB" as its number and unit, for the stat cards. */
function split(n: number | undefined): { v: string; u: string } | null {
  if (!n || n <= 0) return null
  const [v, u] = bytes(n).split(' ')
  return { v: v ?? '', u: u ?? '' }
}
const memory = computed(() => split(props.meta.min_memory))
// The download for this box's CPU, else the first one listed.
const disk = computed(() => {
  const m = props.meta.min_image_size ?? {}
  return split(m[host.arch] ?? Object.values(m)[0])
})
const released = computed(() => {
  const t = props.app.updated || props.app.added
  return t ? day(t, uiLocale()) : ''
})

const viewing = ref<number | null>(null)
const top = ref<HTMLElement | null>(null)
// Another app's page opened from the related lists: start at its top.
watch(
  () => props.app.id,
  () => {
    viewing.value = null
    top.value?.scrollIntoView?.({ block: 'start' })
  },
)
</script>

<template>
  <article ref="top" class="page" :data-app="app.id">
    <nav class="crumbs">
      <button type="button" class="back" :aria-label="strings.back" @click="emit('back')"><Icon name="arrowLeft" :size="17" /></button>
      <span class="crumb">{{ app.title }}</span>
    </nav>

    <header class="head card">
      <span class="logo"><AppIcon :src="app.icon" :name="app.title" :size="76" /></span>
      <div class="titles">
        <h2>{{ app.title }}</h2>
        <p class="tagline-lg">{{ app.tagline }}</p>
      </div>
      <div class="actions">
        <span class="btns">
          <button
            v-if="inst && admin"
            type="button"
            class="btn danger"
            :disabled="busy"
            @click="uninstall(app)"
          >
            <Icon name="trash" :size="14" />{{ strings.uninstall }}
          </button>
          <button type="button" class="btn primary" :class="b.kind" :disabled="b.disabled" :title="b.title" @click="act(app)">
            {{ b.label }}
          </button>
        </span>
        <span class="latest mono">
          <template v-if="inst && inst.version !== app.version">{{ inst.version }} → </template>{{ strings.latestIs(app.version) }}
        </span>
      </div>
    </header>

    <p v-if="!fits" class="note warn"><Icon name="alert" :size="15" />{{ strings.notForDeviceLong(host.arch) }}</p>
    <p v-if="error" class="note warn"><Icon name="alert" :size="15" />{{ error }}</p>

    <ul class="stats">
      <li class="card stat">
        <span class="label">{{ strings.category }}</span>
        <button type="button" class="value link-v" @click="emit('view', app.category)">{{ app.category }}</button>
      </li>
      <li class="card stat">
        <span class="label">{{ strings.developer }}</span>
        <span class="value strong">{{ app.developer || '–' }}</span>
        <span v-if="meta.author" class="sub">{{ strings.packagedBy(meta.author) }}</span>
      </li>
      <li class="card stat">
        <span class="label">{{ strings.minMemory }}</span>
        <template v-if="memory"><span class="value big">{{ memory.v }}</span><span class="sub">{{ memory.u }}</span></template>
        <span v-else class="value">–</span>
      </li>
      <li class="card stat">
        <template v-if="app.stars != null">
          <span class="label">{{ strings.githubStars }}</span>
          <span class="value big">{{ shortNum(app.stars) }}</span>
          <span v-if="app.forks != null" class="sub">{{ strings.forksN(shortNum(app.forks)) }}</span>
        </template>
        <template v-else>
          <span class="label">{{ strings.diskUse }}</span>
          <template v-if="disk"><span class="value big">{{ disk.v }}</span><span class="sub">{{ disk.u }}</span></template>
          <span v-else class="value">–</span>
        </template>
      </li>
    </ul>

    <ul v-if="shots.length" class="shots" :aria-label="strings.screenshotN(1, shots.length)">
      <li v-for="(s, i) in shots" :key="s">
        <button type="button" class="shot" :aria-label="strings.screenshotN(i + 1, shots.length)" @click="viewing = i">
          <img :src="s" alt="" loading="lazy" referrerpolicy="no-referrer" />
        </button>
      </li>
    </ul>

    <h3 class="about-h">{{ strings.aboutThisApp }}</h3>

    <section v-if="meta.release_note" class="card news">
      <div>
        <span class="label">{{ strings.whatsNew }}</span>
        <!-- eslint-disable-next-line vue/no-v-html -- markdown() escapes everything first -->
        <div class="md" v-html="markdown(meta.release_note)" />
      </div>
      <span class="news-side mono">
        <span>{{ strings.version(app.version) }}</span>
        <span v-if="released">{{ released }}</span>
      </span>
    </section>

    <div class="about">
      <section class="card intro">
        <span class="label">{{ strings.introduction }}</span>
        <div v-if="meta.tips?.before_install" class="tip">
          <Icon name="info" :size="15" />
          <!-- eslint-disable-next-line vue/no-v-html -->
          <div class="md" v-html="markdown(meta.tips.before_install)" />
        </div>
        <!-- eslint-disable-next-line vue/no-v-html -->
        <div v-if="meta.description" class="md" v-html="markdown(meta.description)" />
        <p v-else class="mut">{{ app.tagline }}</p>
      </section>

      <aside class="card info">
        <span class="label">{{ strings.info }}</span>
        <dl>
          <div>
            <dt>{{ strings.category }}</dt>
            <dd><button type="button" class="link" @click="emit('view', app.category)">{{ app.category }}</button></dd>
          </div>
          <div v-if="meta.repo">
            <dt>{{ strings.sourceCode }}</dt>
            <dd><a :href="meta.repo" target="_blank" rel="noopener">{{ meta.repo.includes('github.com') ? 'GitHub' : strings.source }}</a></dd>
          </div>
          <div v-if="meta.docs">
            <dt>{{ strings.docs }}</dt>
            <dd><a :href="meta.docs" target="_blank" rel="noopener">{{ strings.openLink }}</a></dd>
          </div>
          <div v-if="app.developer">
            <dt>{{ strings.developedBy }}</dt>
            <dd>{{ app.developer }}</dd>
          </div>
          <div v-if="meta.author">
            <dt>{{ strings.packagedByLabel }}</dt>
            <dd>{{ meta.author }}</dd>
          </div>
          <div v-if="archs">
            <dt>{{ strings.runsOnLabel }}</dt>
            <dd :class="{ warn: !fits }">{{ archs }}</dd>
          </div>
          <div v-if="disk">
            <dt>{{ strings.diskUse }}</dt>
            <dd>{{ disk.v }} {{ disk.u }}</dd>
          </div>
          <div v-if="app.added">
            <dt>{{ strings.addedLabel }}</dt>
            <dd>{{ day(app.added, uiLocale()) }}</dd>
          </div>
        </dl>
        <a v-if="meta.support || meta.repo" class="support" :href="meta.support || meta.repo" target="_blank" rel="noopener">
          {{ strings.getSupport }}<Icon name="external" :size="14" />
        </a>
      </aside>
    </div>

    <BundleSpotlight
      v-for="x in bundles"
      :key="x.bundle.id"
      :bundle="x.bundle"
      :apps="x.apps"
      @open="emit('open', $event)"
      @bundles="emit('bundles')"
    />

    <AppGrid
      v-if="related.length"
      :title="strings.moreIn(app.category)"
      :apps="related"
      @open="emit('open', $event)"
      @all="emit('view', app.category)"
    />

    <ShotViewer v-if="viewing !== null" v-model="viewing" :shots="shots" :title="app.title" @close="viewing = null" />
  </article>
</template>

<style scoped src="./shared.css"></style>
<style scoped>
.page {
  container-type: inline-size;
  padding-bottom: 12px;
}
.card {
  border-radius: 18px;
  border: 1px solid var(--line);
  background: var(--card);
}
.label {
  display: block;
  color: var(--mut);
  font-size: 12px;
  font-weight: 600;
}
.crumbs {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
}
.back {
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
.back:hover {
  color: var(--text);
  background: rgba(var(--fg-rgb), 0.06);
}
.crumb {
  font-size: 15px;
  font-weight: 700;
}
/* Header: a faint dot grid behind, as in ZimaOS. */
.head {
  display: flex;
  align-items: center;
  gap: 20px;
  flex-wrap: wrap;
  padding: 22px 24px;
  background-color: var(--card);
  background-image: radial-gradient(rgba(var(--fg-rgb), 0.09) 1px, transparent 1.2px);
  background-size: 14px 14px;
}
.logo {
  display: grid;
  place-items: center;
  flex: none;
}
.logo :deep(img),
.logo > :deep(*) {
  border-radius: 18px;
}
.titles {
  flex: 1;
  min-width: 220px;
}
h2 {
  margin: 0 0 6px;
  font-size: 26px;
  font-weight: 700;
  letter-spacing: -0.02em;
  line-height: 1.15;
}
.tagline-lg {
  margin: 0;
  color: var(--mut);
  font-size: 13.5px;
  line-height: 1.5;
}
.actions {
  display: grid;
  justify-items: end;
  gap: 8px;
}
.btns {
  display: flex;
  gap: 8px;
}
.btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 18px;
  border-radius: 10px;
  border: 1px solid var(--line);
  background: rgba(var(--fg-rgb), 0.06);
  color: var(--text);
  font: inherit;
  font-size: 13.5px;
  font-weight: 600;
  cursor: pointer;
  white-space: nowrap;
}
.btn:hover:not(:disabled) {
  border-color: rgba(var(--fg-rgb), 0.3);
}
.btn.primary.install {
  border-color: transparent;
  background: var(--ac);
  color: var(--ink);
}
.btn.primary.update {
  border-color: transparent;
  background: var(--warn);
  color: var(--ink);
}
.btn.danger {
  color: var(--danger);
  border-color: rgba(var(--danger-rgb), 0.4);
  background: rgba(var(--danger-rgb), 0.07);
}
.btn.danger:hover:not(:disabled) {
  border-color: var(--danger);
}
.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.latest {
  color: var(--mut);
  font-size: 11.5px;
}
.note {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 12px 0 0;
  font-size: 13px;
}
.stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin: 12px 0 0;
  padding: 0;
  list-style: none;
}
.stat {
  display: grid;
  align-content: start;
  justify-items: end;
  gap: 2px;
  min-height: 84px;
  padding: 12px 14px;
}
.stat .label {
  justify-self: start;
  margin-bottom: 4px;
}
.value {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
}
.value.strong,
.value.big,
.link-v {
  color: var(--ac);
  font-weight: 700;
}
.value.big {
  font-size: 20px;
  line-height: 1.1;
}
.link-v {
  padding: 0;
  border: 0;
  background: none;
  font: inherit;
  font-size: 14px;
  font-weight: 700;
  cursor: pointer;
}
.link-v:hover {
  text-decoration: underline;
}
.sub {
  max-width: 100%;
  color: var(--mut);
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.shots {
  display: grid;
  grid-auto-flow: column;
  grid-auto-columns: calc((100% - 28px) / 3);
  gap: 14px;
  margin: 18px 0 0;
  padding: 0 0 6px;
  list-style: none;
  overflow-x: auto;
  scroll-snap-type: x mandatory;
  scrollbar-width: thin;
}
.shots li {
  scroll-snap-align: start;
}
.shot {
  display: block;
  width: 100%;
  aspect-ratio: 16 / 10;
  padding: 0;
  border: 1px solid var(--line);
  border-radius: 14px;
  background: rgba(var(--fg-rgb), 0.04);
  overflow: hidden;
  cursor: zoom-in;
}
.shot:hover {
  border-color: rgba(var(--ac-rgb), 0.5);
}
.shot img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: top;
}
.about-h {
  margin: 28px 0 12px;
  font-size: 18px;
  font-weight: 700;
}
.news {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  padding: 16px 20px;
  margin-bottom: 12px;
}
.news .md {
  margin-top: 6px;
}
.news-side {
  display: grid;
  align-content: start;
  justify-items: end;
  gap: 2px;
  flex: none;
  color: var(--mut);
  font-size: 12px;
}
.about {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 250px;
  gap: 12px;
  align-items: start;
}
.intro {
  padding: 16px 20px;
}
.intro .md {
  margin-top: 8px;
}
.tip {
  display: flex;
  gap: 10px;
  margin: 10px 0 4px;
  padding: 10px 12px;
  border-radius: 12px;
  background: rgba(var(--warn-rgb), 0.1);
  color: var(--text);
}
.tip :deep(svg) {
  flex: none;
  margin-top: 3px;
  color: var(--warn);
}
.tip .md {
  margin: 0;
}
.info {
  display: grid;
  gap: 10px;
  padding: 16px 18px;
}
dl {
  display: grid;
  gap: 9px;
  margin: 0;
}
dl > div {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  font-size: 12.5px;
}
dt {
  color: var(--mut);
}
dd {
  margin: 0;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: right;
}
dd a,
dd .link {
  color: var(--ac);
  font-size: 12.5px;
  text-decoration: none;
}
dd a:hover {
  text-decoration: underline;
}
dd.warn {
  color: var(--warn);
}
.support {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  margin-top: 6px;
  padding: 8px 12px;
  border-radius: 10px;
  border: 1px solid var(--line);
  background: rgba(var(--fg-rgb), 0.05);
  color: var(--text);
  font-size: 13px;
  font-weight: 600;
  text-decoration: none;
}
.support:hover {
  border-color: rgba(var(--fg-rgb), 0.3);
}
.md {
  color: rgba(var(--fg-rgb), 0.85);
  font-size: 13.5px;
  line-height: 1.65;
}
.md :deep(p) {
  margin: 0 0 10px;
}
.md :deep(p:last-child) {
  margin-bottom: 0;
}
.md :deep(h1),
.md :deep(h2),
.md :deep(h3),
.md :deep(h4) {
  margin: 16px 0 6px;
  font-size: 14.5px;
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
  margin: 4px 0 10px;
  padding-left: 20px;
}
@container (max-width: 760px) {
  .stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .about {
    grid-template-columns: 1fr;
  }
  .shots {
    grid-auto-columns: 78%;
  }
  .actions {
    justify-items: start;
  }
}
</style>
