<script setup lang="ts">
// A compact section on Discover, like "New Arrivals" in the ZimaOS App
// Store: a title, and apps in three columns, each an icon, its name and two
// lines about it. No screenshots, so it loads at once.
import type { CatalogApp } from '../../api/apps'
import AppIcon from '../../components/AppIcon.vue'
import Icon from '../../components/Icon.vue'
import { ago } from '../../lib/format'
import { strings } from '../../strings'
import { installedOf } from './actions'

defineProps<{
  title: string
  apps: CatalogApp[]
  /** Show when each app was added or last updated beside its name. */
  when?: 'added' | 'updated'
}>()
defineEmits<{ open: [app: CatalogApp]; all: [] }>()

const stamp = (a: CatalogApp, when?: 'added' | 'updated') => {
  const t = when === 'added' ? a.added : when === 'updated' ? a.updated || a.added : 0
  return t ? ago(new Date(t * 1000)) : ''
}
</script>

<template>
  <section class="grid-sec" :aria-label="title">
    <header class="sec-head">
      <h3>{{ title }}</h3>
      <button type="button" class="link" @click="$emit('all')">{{ strings.seeAll }}</button>
    </header>
    <ul class="grid">
      <li v-for="a in apps" :key="a.id">
        <button type="button" class="row" :data-app="a.id" @click="$emit('open', a)">
          <span class="ico"><AppIcon :src="a.icon" :name="a.title" :size="52" /></span>
          <span class="text">
            <span class="name">
              <span class="nm">{{ a.title }}</span>
              <Icon v-if="installedOf(a) && !installedOf(a)?.update" name="check" :size="13" class="ok" />
              <span v-if="installedOf(a)?.update" class="dot-hot" :title="strings.updateAvailable" />
            </span>
            <span class="about">{{ a.tagline }}</span>
            <span v-if="stamp(a, when)" class="stamp">{{ stamp(a, when) }}</span>
          </span>
        </button>
      </li>
    </ul>
  </section>
</template>

<style scoped src="./shared.css"></style>
<style scoped>
.grid-sec {
  margin-top: 40px;
}
.sec-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}
h3 {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
  letter-spacing: -0.01em;
}
.grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 4px 18px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.row {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  width: 100%;
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
  gap: 3px;
  min-width: 0;
  padding-top: 4px;
}
.name {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 15px;
  font-weight: 700;
  min-width: 0;
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
.stamp {
  color: var(--mut);
  font-size: 11.5px;
  opacity: 0.8;
}
@container (max-width: 760px) {
  .grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@container (max-width: 480px) {
  .grid {
    grid-template-columns: 1fr;
  }
}
</style>
