<script setup lang="ts">
// One app as a compact row, like the ZimaOS App Store's lists: icon in a
// rounded square, its name, two lines about it, and (when the list is
// sorted by one) its stars, forks or date. With `action`, its button sits
// on the right (the Updates list).
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
  /** The fact under the description, if any. */
  fact?: 'stars' | 'forks' | 'added' | 'updated'
  action?: boolean
}>()
defineEmits<{ open: [app: CatalogApp] }>()

const inst = computed(() => installedOf(props.app))
const fits = computed(() => fitsHere(props.app))
const b = computed(() => button(props.app))
const fact = computed(() => {
  const a = props.app
  if (props.fact === 'stars' && a.stars != null) return { icon: 'star' as const, text: strings.starsN(shortNum(a.stars)) }
  if (props.fact === 'forks' && a.forks != null) return { icon: 'fork' as const, text: strings.forksN(shortNum(a.forks)) }
  const t = props.fact === 'added' ? a.added : props.fact === 'updated' ? a.updated || a.added : 0
  return t ? { icon: 'clock' as const, text: ago(new Date(t * 1000)) } : null
})
</script>

<template>
  <li class="app-row" :class="{ dim: !fits, acting: action }">
    <button type="button" class="row" :data-app="app.id" @click="$emit('open', app)">
      <span class="ico"><AppIcon :src="app.icon" :name="app.title" :size="52" /></span>
      <span class="text">
        <span class="name">
          <span class="nm">{{ app.title }}</span>
          <Icon v-if="inst && !inst.update" name="check" :size="13" class="ok" :aria-label="strings.installed" />
          <span v-if="inst?.update" class="dot-hot" :title="strings.updateAvailable" />
        </span>
        <span class="about">{{ app.tagline }}</span>
        <span v-if="!fits" class="badge off">{{ strings.notForDevice }}</span>
        <span v-else-if="fact" class="fact"><Icon :name="fact.icon" :size="12" />{{ fact.text }}</span>
      </span>
    </button>
    <button v-if="action" type="button" class="pill" :class="b.kind" :disabled="b.disabled" :title="b.title" @click="act(app)">
      {{ b.label }}
    </button>
  </li>
</template>

<style scoped src="./shared.css"></style>
<style scoped>
.app-row {
  position: relative;
  display: flex;
  align-items: center;
}
.app-row.dim .row {
  opacity: 0.6;
}
.row {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  width: 100%;
  min-width: 0;
  padding: 12px 10px;
  border: 0;
  border-radius: 16px;
  background: none;
  color: var(--text);
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition: background 0.15s;
}
.acting {
  gap: 10px;
  padding-right: 10px;
  border-radius: 16px;
  transition: background 0.15s;
}
.acting:hover {
  background: rgba(var(--fg-rgb), 0.05);
}
.acting .row:hover {
  background: none;
}
.row:hover {
  background: rgba(var(--fg-rgb), 0.05);
}
.ico {
  display: grid;
  place-items: center;
  width: 64px;
  height: 64px;
  flex: none;
  border-radius: 16px;
  border: 1px solid var(--line);
  background: rgba(var(--fg-rgb), 0.04);
  overflow: hidden;
}
.text {
  display: grid;
  justify-items: start;
  gap: 3px;
  min-width: 0;
  padding-top: 3px;
}
.name {
  display: flex;
  align-items: center;
  gap: 6px;
  max-width: 100%;
  font-size: 15px;
  font-weight: 700;
}
.nm {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.name .ok {
  flex: none;
  color: var(--ac);
}
.dot-hot {
  flex: none;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--warn);
}
.about {
  color: var(--mut);
  font-size: 12.5px;
  line-height: 1.45;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.fact {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--mut);
  font-size: 11.5px;
}
.badge {
  margin-top: 2px;
}
.pill {
  flex: none;
}
</style>
