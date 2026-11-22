<script setup lang="ts">
// One app in an IceStore list: its screenshot, icon and name, a line of
// facts, and its button.
import { computed } from 'vue'
import type { CatalogApp } from '../../api/apps'
import AppIcon from '../../components/AppIcon.vue'
import Icon from '../../components/Icon.vue'
import { ago } from '../../lib/format'
import { shortNum } from '../../lib/icestore'
import { strings } from '../../strings'
import { act, button, fitsHere, installedOf } from './actions'

const props = defineProps<{
  app: CatalogApp
  /** Which date to show in the facts, if any (the list is sorted by it). */
  date?: 'added' | 'updated'
}>()
defineEmits<{ open: [app: CatalogApp] }>()

const b = computed(() => button(props.app))
const inst = computed(() => installedOf(props.app))
const fits = computed(() => fitsHere(props.app))
const when = computed(() => {
  const t = props.date === 'added' ? props.app.added : props.date === 'updated' ? props.app.updated || props.app.added : 0
  return t ? ago(new Date(t * 1000)) : ''
})
</script>

<template>
  <li class="card" :class="{ dim: !fits }" :data-app="app.id">
    <button type="button" class="card-hit" :aria-label="`${app.title}: ${strings.details}`" @click="$emit('open', app)" />
    <div class="art" :class="{ empty: !app.thumbnail }">
      <img v-if="app.thumbnail" class="thumb" :src="app.thumbnail" alt="" loading="lazy" referrerpolicy="no-referrer" />
      <AppIcon v-else :src="app.icon" :name="app.title" :size="52" />
      <span class="flags">
        <span v-if="inst?.update" class="badge hot">{{ strings.updateAvailable }}</span>
        <span v-else-if="inst" class="badge ok"><Icon name="check" :size="12" />{{ strings.installed }}</span>
        <span v-if="!fits" class="badge off">{{ strings.notForDevice }}</span>
      </span>
    </div>
    <div class="body">
      <div class="card-top">
        <AppIcon :src="app.icon" :name="app.title" :size="40" />
        <div class="card-titles">
          <span class="title">{{ app.title }}</span>
          <span class="kicker">{{ app.category }}</span>
        </div>
      </div>
      <p class="tagline">{{ app.tagline }}</p>
      <div class="card-foot">
        <ul class="facts">
          <li v-if="app.stars != null" :title="strings.starsTitle"><Icon name="star" :size="13" />{{ shortNum(app.stars) }}</li>
          <li v-if="app.forks != null && app.forks > 0" :title="strings.forksTitle"><Icon name="fork" :size="13" />{{ shortNum(app.forks) }}</li>
          <li v-if="when"><Icon name="clock" :size="13" />{{ when }}</li>
          <li v-if="app.stars == null && !when" class="mono">{{ app.version }}</li>
        </ul>
        <button type="button" class="pill" :class="b.kind" :disabled="b.disabled" :title="b.title" @click.stop="act(app)">
          {{ b.label }}
        </button>
      </div>
    </div>
  </li>
</template>

<style scoped src="./shared.css"></style>
<style scoped>
.card {
  position: relative;
  display: flex;
  flex-direction: column;
  border-radius: var(--radius-tile);
  border: 1px solid var(--line);
  background: var(--card);
  overflow: hidden;
  transition:
    border-color 0.15s,
    transform 0.15s;
}
.card:hover {
  border-color: rgba(var(--ac-rgb), 0.4);
}
.card.dim .art,
.card.dim .body {
  opacity: 0.6;
}
.card-hit {
  position: absolute;
  inset: 0;
  z-index: 1;
  border: 0;
  border-radius: inherit;
  background: none;
  cursor: pointer;
}
.art {
  position: relative;
  display: grid;
  place-items: center;
  aspect-ratio: 16 / 9;
  border-bottom: 1px solid var(--line);
  background: rgba(var(--fg-rgb), 0.04);
  overflow: hidden;
}
.art.empty {
  background: rgba(var(--ac-rgb), 0.06);
}
.thumb {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: top;
  transition: transform 0.3s ease;
}
.card:hover .thumb {
  transform: scale(1.03);
}
.flags {
  position: absolute;
  top: 9px;
  left: 9px;
  display: flex;
  gap: 6px;
}
.flags .badge {
  background: rgba(var(--surface-rgb), 0.9);
}
.body {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 9px;
  padding: 12px 14px 14px;
}
.card-top {
  display: flex;
  align-items: center;
  gap: 11px;
  margin-top: -32px;
}
.card-top > :first-child {
  flex: none;
  border-radius: 11px;
  box-shadow: 0 0 0 3px rgb(var(--surface-rgb));
}
.card-titles {
  display: grid;
  min-width: 0;
  padding-top: 22px;
}
.kicker {
  font-size: 11.5px;
}
.tagline {
  flex: 1;
}
.card-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}
.facts {
  min-width: 0;
  overflow: hidden;
  flex-wrap: nowrap;
}
.pill {
  z-index: 2;
  flex: none;
}
@media (prefers-reduced-motion: reduce) {
  .thumb {
    transition: none;
  }
  .card:hover .thumb {
    transform: none;
  }
}
</style>
