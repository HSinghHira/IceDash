<script setup lang="ts">
// One app in an IceStore list: icon, name, a line of facts, and its button.
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
    <div class="card-top">
      <AppIcon :src="app.icon" :name="app.title" :size="44" />
      <div class="card-titles">
        <span class="kicker">{{ app.category }}</span>
        <span class="title">{{ app.title }}</span>
      </div>
    </div>
    <p class="tagline">{{ app.tagline }}</p>
    <ul class="facts">
      <li v-if="app.stars != null" :title="strings.starsTitle"><Icon name="star" :size="13" />{{ shortNum(app.stars) }}</li>
      <li v-if="app.forks != null && app.forks > 0" :title="strings.forksTitle"><Icon name="fork" :size="13" />{{ shortNum(app.forks) }}</li>
      <li v-if="when"><Icon name="clock" :size="13" />{{ when }}</li>
      <li v-if="!fits"><span class="badge off">{{ strings.notForDevice }}</span></li>
    </ul>
    <div class="card-foot">
      <span v-if="inst?.update" class="badge hot">{{ strings.updateAvailable }}</span>
      <span v-else-if="inst" class="badge ok"><Icon name="check" :size="12" />{{ strings.installed }}</span>
      <span v-else class="mono ver">{{ app.version }}</span>
      <button type="button" class="pill" :class="b.kind" :disabled="b.disabled" :title="b.title" @click.stop="act(app)">
        {{ b.label }}
      </button>
    </div>
  </li>
</template>

<style scoped src="./shared.css"></style>
<style scoped>
.card {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 9px;
  padding: 14px;
  border-radius: var(--radius-tile);
  border: 1px solid var(--line);
  background: var(--card);
  transition:
    border-color 0.15s,
    background 0.15s;
}
.card:hover {
  border-color: rgba(var(--ac-rgb), 0.35);
  background: rgba(var(--ac-rgb), 0.04);
}
.card.dim > :not(.card-hit) {
  opacity: 0.6;
}
.card-hit {
  position: absolute;
  inset: 0;
  border: 0;
  border-radius: inherit;
  background: none;
  cursor: pointer;
}
.card-top,
.tagline,
.facts,
.ver,
.badge {
  pointer-events: none;
}
.card-top {
  display: flex;
  align-items: center;
  gap: 12px;
}
.card-titles {
  display: grid;
  min-width: 0;
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
.ver {
  font-size: 11px;
  color: var(--mut);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
