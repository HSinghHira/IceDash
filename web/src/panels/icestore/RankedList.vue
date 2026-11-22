<script setup lang="ts">
// A chart on Discover (Most forked): numbered apps in two columns, each
// with its screenshot, the number it's ranked by, and its button.
import type { CatalogApp } from '../../api/apps'
import AppIcon from '../../components/AppIcon.vue'
import Icon from '../../components/Icon.vue'
import { shortNum } from '../../lib/icestore'
import { strings } from '../../strings'
import { act, button } from './actions'

defineProps<{ title: string; hint?: string; apps: CatalogApp[]; by: 'forks' | 'stars' }>()
defineEmits<{ open: [app: CatalogApp]; all: [] }>()
</script>

<template>
  <section class="ranked" :aria-label="title">
    <header class="sec-head">
      <div>
        <h3>{{ title }}</h3>
        <p v-if="hint" class="hint">{{ hint }}</p>
      </div>
      <button type="button" class="link" @click="$emit('all')">{{ strings.seeAll }}</button>
    </header>
    <ol class="chart">
      <li v-for="(a, k) in apps" :key="a.id" class="entry" :data-app="a.id">
        <button type="button" class="hit" :aria-label="`${a.title}: ${strings.details}`" @click="$emit('open', a)" />
        <span class="rank mono">{{ k + 1 }}</span>
        <span class="shot" :class="{ empty: !a.thumbnail }">
          <img v-if="a.thumbnail" class="thumb" :src="a.thumbnail" alt="" loading="lazy" referrerpolicy="no-referrer" />
          <AppIcon v-else :src="a.icon" :name="a.title" :size="34" />
        </span>
        <span class="who">
          <span class="name"><AppIcon :src="a.icon" :name="a.title" :size="20" />{{ a.title }}</span>
          <span class="tag">{{ a.tagline }}</span>
          <span class="num">
            <Icon :name="by === 'forks' ? 'fork' : 'star'" :size="12" />
            {{ by === 'forks' ? strings.forksN(shortNum(a.forks ?? 0)) : strings.starsN(shortNum(a.stars ?? 0)) }}
          </span>
        </span>
        <button
          type="button"
          class="pill"
          :class="button(a).kind"
          :disabled="button(a).disabled"
          :title="button(a).title"
          @click.stop="act(a)"
        >
          {{ button(a).label }}
        </button>
      </li>
    </ol>
  </section>
</template>

<style scoped src="./shared.css"></style>
<style scoped>
.ranked {
  margin-top: 40px;
}
.sec-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}
h3 {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
}
.hint {
  margin: 3px 0 0;
  color: var(--mut);
  font-size: 12.5px;
}
.chart {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  column-gap: 28px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.entry {
  position: relative;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 12px 4px;
  border-bottom: 1px solid var(--line);
}
.hit {
  position: absolute;
  inset: 0;
  z-index: 1;
  border: 0;
  border-radius: 12px;
  background: none;
  cursor: pointer;
}
.entry:hover .shot {
  border-color: rgba(var(--ac-rgb), 0.45);
}
.entry:hover .name {
  color: var(--ac);
}
.rank {
  width: 20px;
  flex: none;
  color: var(--mut);
  font-size: 15px;
  font-weight: 600;
  text-align: center;
}
.shot {
  display: grid;
  place-items: center;
  width: 118px;
  aspect-ratio: 16 / 10;
  flex: none;
  border-radius: 10px;
  border: 1px solid var(--line);
  background: rgba(var(--fg-rgb), 0.04);
  overflow: hidden;
  transition: border-color 0.15s;
}
.shot.empty {
  background: rgba(var(--ac-rgb), 0.06);
}
.thumb {
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: top;
}
.who {
  display: grid;
  gap: 2px;
  flex: 1;
  min-width: 0;
}
.name {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 14px;
  font-weight: 700;
  overflow: hidden;
  white-space: nowrap;
  transition: color 0.15s;
}
.tag {
  color: var(--mut);
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.num {
  display: flex;
  align-items: center;
  gap: 4px;
  color: var(--mut);
  font-size: 11.5px;
  white-space: nowrap;
  overflow: hidden;
}
.pill {
  z-index: 2;
  flex: none;
  padding: 5px 14px;
  font-size: 12px;
}
@container (max-width: 720px) {
  .chart {
    grid-template-columns: 1fr;
  }
}
</style>
