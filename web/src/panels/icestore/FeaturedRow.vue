<script setup lang="ts">
// The Featured row at the top of All apps: big cards with a screenshot,
// scrolled sideways. IceStore picks them (featured: true in an app's README).
import { ref } from 'vue'
import type { CatalogApp } from '../../api/apps'
import AppIcon from '../../components/AppIcon.vue'
import Icon from '../../components/Icon.vue'
import { shortNum } from '../../lib/icestore'
import { strings } from '../../strings'
import { act, button, installedOf } from './actions'

defineProps<{ apps: CatalogApp[] }>()
defineEmits<{ open: [app: CatalogApp] }>()

const strip = ref<HTMLElement | null>(null)
function step(dir: 1 | -1) {
  const el = strip.value
  if (!el) return
  const card = el.querySelector<HTMLElement>('.feat')
  el.scrollBy({ left: dir * ((card?.offsetWidth ?? 320) + 12), behavior: 'smooth' })
}
</script>

<template>
  <section class="featured" :aria-label="strings.featured">
    <header class="row-head">
      <h4>{{ strings.featured }}</h4>
      <span class="nav">
        <button type="button" class="arrow" :aria-label="strings.previous" @click="step(-1)"><Icon name="arrowLeft" :size="15" /></button>
        <button type="button" class="arrow" :aria-label="strings.next" @click="step(1)"><Icon name="arrowRight" :size="15" /></button>
      </span>
    </header>
    <ul ref="strip" class="strip">
      <li v-for="a in apps" :key="a.id" class="feat" :data-app="a.id">
        <button type="button" class="hit" :aria-label="`${a.title}: ${strings.details}`" @click="$emit('open', a)" />
        <div class="shot" :class="{ empty: !a.thumbnail }">
          <img v-if="a.thumbnail" class="thumb" :src="a.thumbnail" alt="" loading="lazy" referrerpolicy="no-referrer" />
          <AppIcon v-else :src="a.icon" :name="a.title" :size="64" />
        </div>
        <div class="body">
          <AppIcon :src="a.icon" :name="a.title" :size="40" />
          <div class="titles">
            <span class="title">{{ a.title }}</span>
            <span class="kicker">
              {{ a.category }}<template v-if="a.stars != null"> · <Icon name="star" :size="11" /> {{ shortNum(a.stars) }}</template>
            </span>
          </div>
          <button
            type="button"
            class="pill"
            :class="button(a).kind"
            :disabled="button(a).disabled"
            :title="button(a).title"
            @click.stop="act(a)"
          >
            <Icon v-if="installedOf(a) && !installedOf(a)?.update" name="check" :size="13" />{{ button(a).label }}
          </button>
        </div>
        <p class="tagline">{{ a.tagline }}</p>
      </li>
    </ul>
  </section>
</template>

<style scoped src="./shared.css"></style>
<style scoped>
.featured {
  margin-bottom: 22px;
}
.nav {
  display: inline-flex;
  gap: 6px;
}
.arrow {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  border: 1px solid var(--line);
  background: rgba(var(--fg-rgb), 0.03);
  color: var(--text);
  cursor: pointer;
}
.arrow:hover {
  border-color: rgba(var(--fg-rgb), 0.25);
}
.strip {
  display: grid;
  grid-auto-flow: column;
  grid-auto-columns: minmax(280px, calc((100% - 24px) / 3));
  gap: 12px;
  margin: 0;
  padding: 0 0 6px;
  list-style: none;
  overflow-x: auto;
  scroll-snap-type: x mandatory;
  scrollbar-width: thin;
}
.feat {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding-bottom: 14px;
  border-radius: var(--radius-tile);
  border: 1px solid var(--line);
  background: var(--card);
  overflow: hidden;
  scroll-snap-align: start;
  transition: border-color 0.15s;
}
.feat:hover {
  border-color: rgba(var(--ac-rgb), 0.4);
}
.hit {
  position: absolute;
  inset: 0;
  border: 0;
  background: none;
  cursor: pointer;
}
.shot {
  aspect-ratio: 16 / 9;
  border-bottom: 1px solid var(--line);
  background: rgba(var(--fg-rgb), 0.04);
  overflow: hidden;
  pointer-events: none;
}
.thumb {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: top;
}
.shot.empty {
  display: grid;
  place-items: center;
  background: rgba(var(--ac-rgb), 0.06);
}
.body {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 0 14px;
}
.titles {
  display: grid;
  flex: 1;
  min-width: 0;
  pointer-events: none;
}
.kicker {
  display: flex;
  align-items: center;
  gap: 3px;
  white-space: pre;
  overflow: hidden;
}
.tagline {
  padding: 0 14px;
  pointer-events: none;
}
.body > :first-child {
  pointer-events: none;
}
</style>
