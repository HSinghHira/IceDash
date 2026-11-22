<script setup lang="ts">
// A row on Discover (Popular, New in IceStore, Recently updated): big cards
// led by the app's screenshot, scrolled sideways.
import { ref } from 'vue'
import type { CatalogApp } from '../../api/apps'
import AppIcon from '../../components/AppIcon.vue'
import Icon from '../../components/Icon.vue'
import { ago } from '../../lib/format'
import { shortNum } from '../../lib/icestore'
import { strings } from '../../strings'
import { act, button, installedOf } from './actions'

const props = defineProps<{
  title: string
  /** A short line under the title. */
  hint?: string
  apps: CatalogApp[]
  /** What the small line under each name shows. */
  fact?: 'stars' | 'added' | 'updated'
}>()
defineEmits<{ open: [app: CatalogApp]; all: [] }>()

const strip = ref<HTMLElement | null>(null)
function step(dir: 1 | -1) {
  const el = strip.value
  if (el) el.scrollBy({ left: dir * el.clientWidth * 0.8, behavior: 'smooth' })
}

function factOf(a: CatalogApp): string {
  if (props.fact === 'added' && a.added) return ago(new Date(a.added * 1000))
  if (props.fact === 'updated' && (a.updated || a.added)) return ago(new Date((a.updated || a.added)! * 1000))
  if (props.fact === 'stars' && a.stars != null) return strings.starsN(shortNum(a.stars))
  return a.category
}
</script>

<template>
  <section class="showcase" :aria-label="title">
    <header class="sec-head">
      <div>
        <h3>{{ title }}</h3>
        <p v-if="hint" class="hint">{{ hint }}</p>
      </div>
      <span class="tools">
        <button type="button" class="link" @click="$emit('all')">{{ strings.seeAll }}</button>
        <button type="button" class="arrow" :aria-label="strings.previous" @click="step(-1)"><Icon name="arrowLeft" :size="15" /></button>
        <button type="button" class="arrow" :aria-label="strings.next" @click="step(1)"><Icon name="arrowRight" :size="15" /></button>
      </span>
    </header>
    <ul ref="strip" class="strip">
      <li v-for="a in apps" :key="a.id" class="tile" :data-app="a.id">
        <button type="button" class="hit" :aria-label="`${a.title}: ${strings.details}`" @click="$emit('open', a)" />
        <div class="art" :class="{ empty: !a.thumbnail }">
          <img v-if="a.thumbnail" class="thumb" :src="a.thumbnail" alt="" loading="lazy" referrerpolicy="no-referrer" />
          <AppIcon v-else :src="a.icon" :name="a.title" :size="56" />
          <span v-if="installedOf(a)" class="badge" :class="installedOf(a)?.update ? 'hot' : 'ok'">
            <Icon v-if="!installedOf(a)?.update" name="check" :size="11" />{{ installedOf(a)?.update ? strings.updateAvailable : strings.installed }}
          </span>
        </div>
        <div class="foot">
          <AppIcon :src="a.icon" :name="a.title" :size="36" />
          <span class="names">
            <span class="name">{{ a.title }}</span>
            <span class="fact">
              <Icon v-if="fact === 'stars' && a.stars != null" name="star" :size="11" />{{ factOf(a) }}
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
        </div>
      </li>
    </ul>
  </section>
</template>

<style scoped src="./shared.css"></style>
<style scoped>
.showcase {
  margin-top: 40px;
}
.hint {
  margin: 3px 0 0;
  color: var(--mut);
  font-size: 12.5px;
}
.sec-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}
h3 {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
}
.tools {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.tools .link {
  margin-right: 8px;
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
  grid-auto-columns: minmax(250px, calc((100% - 32px) / 3));
  gap: 16px;
  margin: 0;
  padding: 0 0 8px;
  list-style: none;
  overflow-x: auto;
  scroll-snap-type: x mandatory;
  scrollbar-width: none;
}
.tile {
  position: relative;
  display: grid;
  gap: 12px;
  scroll-snap-align: start;
}
.hit {
  position: absolute;
  inset: 0;
  z-index: 1;
  border: 0;
  background: none;
  cursor: pointer;
}
.art {
  position: relative;
  aspect-ratio: 16 / 10;
  border-radius: var(--radius-tile);
  border: 1px solid var(--line);
  background: rgba(var(--fg-rgb), 0.04);
  overflow: hidden;
  transition: border-color 0.15s;
}
.tile:hover .art {
  border-color: rgba(var(--ac-rgb), 0.45);
}
.thumb {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: top;
  transition: transform 0.3s ease;
}
.tile:hover .thumb {
  transform: scale(1.03);
}
.art.empty {
  display: grid;
  place-items: center;
  background: rgba(var(--ac-rgb), 0.06);
}
.art .badge {
  position: absolute;
  left: 10px;
  top: 10px;
  background: rgba(var(--surface-rgb), 0.88);
}
.foot {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 0 2px;
}
.names {
  display: grid;
  flex: 1;
  min-width: 0;
}
.name {
  font-size: 14.5px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.fact {
  display: flex;
  align-items: center;
  gap: 3px;
  color: var(--mut);
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
}
.pill {
  z-index: 2;
  padding: 5px 14px;
  font-size: 12px;
}
@media (prefers-reduced-motion: reduce) {
  .thumb {
    transition: none;
  }
  .tile:hover .thumb {
    transform: none;
  }
}
</style>
