<script setup lang="ts">
// A picture row on Discover (Popular, Most forked), like "Trending Now" in
// the ZimaOS App Store: big screenshot cards scrolled sideways, the name
// below on the left and the icon on the right.
import { ref } from 'vue'
import type { CatalogApp } from '../../api/apps'
import AppIcon from '../../components/AppIcon.vue'
import Icon from '../../components/Icon.vue'
import { shortNum } from '../../lib/icestore'
import { strings } from '../../strings'
import { installedOf } from './actions'

defineProps<{
  title: string
  apps: CatalogApp[]
  /** The number under each name. */
  fact?: 'stars' | 'forks'
}>()
defineEmits<{ open: [app: CatalogApp]; all: [] }>()

const strip = ref<HTMLElement | null>(null)
function step(dir: 1 | -1) {
  const el = strip.value
  if (el) el.scrollBy({ left: dir * el.clientWidth * 0.9, behavior: 'smooth' })
}
</script>

<template>
  <section class="showcase" :aria-label="title">
    <header class="sec-head">
      <h3>{{ title }}</h3>
      <button type="button" class="link" @click="$emit('all')">{{ strings.seeAll }}</button>
    </header>
    <div class="rail">
      <button type="button" class="side prev" :aria-label="strings.previous" @click="step(-1)"><Icon name="arrowLeft" :size="16" /></button>
      <ul ref="strip" class="strip">
        <li v-for="a in apps" :key="a.id" class="tile" :data-app="a.id">
          <button type="button" class="hit" :aria-label="`${a.title}: ${strings.details}`" @click="$emit('open', a)">
            <span class="art">
              <img class="thumb" :src="a.thumbnail" alt="" loading="lazy" referrerpolicy="no-referrer" />
              <span v-if="installedOf(a)" class="badge" :class="installedOf(a)?.update ? 'hot' : 'ok'">
                <Icon v-if="!installedOf(a)?.update" name="check" :size="11" />{{ installedOf(a)?.update ? strings.updateAvailable : strings.installed }}
              </span>
            </span>
            <span class="foot">
              <span class="names">
                <span class="name">{{ a.title }}</span>
                <span class="fact">
                  <template v-if="fact === 'forks' && a.forks != null"><Icon name="fork" :size="12" />{{ strings.forksN(shortNum(a.forks)) }}</template>
                  <template v-else-if="fact === 'stars' && a.stars != null"><Icon name="star" :size="12" />{{ strings.starsN(shortNum(a.stars)) }}</template>
                  <template v-else>{{ a.category }}</template>
                </span>
              </span>
              <AppIcon :src="a.icon" :name="a.title" :size="44" />
            </span>
          </button>
        </li>
      </ul>
      <button type="button" class="side next" :aria-label="strings.next" @click="step(1)"><Icon name="arrowRight" :size="16" /></button>
    </div>
  </section>
</template>

<style scoped src="./shared.css"></style>
<style scoped>
.showcase {
  margin-top: 44px;
}
.sec-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}
h3 {
  margin: 0;
  font-size: 24px;
  font-weight: 700;
  letter-spacing: -0.02em;
}
.rail {
  position: relative;
}
.side {
  position: absolute;
  top: calc((100% - 64px) / 2);
  z-index: 2;
  display: grid;
  place-items: center;
  width: 38px;
  height: 38px;
  border-radius: 50%;
  border: 1px solid var(--line);
  background: rgba(var(--surface-rgb), 0.92);
  color: var(--text);
  cursor: pointer;
  transform: translateY(-50%);
  opacity: 0;
  transition: opacity 0.15s;
}
.rail:hover .side,
.side:focus-visible {
  opacity: 1;
}
.prev {
  left: -14px;
}
.next {
  right: -14px;
}
.strip {
  display: grid;
  grid-auto-flow: column;
  grid-auto-columns: minmax(250px, calc((100% - 36px) / 3));
  gap: 18px;
  margin: 0;
  padding: 0;
  list-style: none;
  overflow-x: auto;
  scroll-snap-type: x mandatory;
  scrollbar-width: none;
}
.strip::-webkit-scrollbar {
  display: none;
}
.tile {
  scroll-snap-align: start;
}
.hit {
  display: grid;
  gap: 12px;
  width: 100%;
  padding: 0;
  border: 0;
  background: none;
  color: var(--text);
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.art {
  position: relative;
  display: block;
  aspect-ratio: 16 / 10;
  border-radius: var(--radius-card);
  border: 1px solid var(--line);
  background: rgba(var(--fg-rgb), 0.04);
  overflow: hidden;
  transition: border-color 0.15s;
}
.hit:hover .art {
  border-color: rgba(var(--ac-rgb), 0.45);
}
.thumb {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: top;
  transition: transform 0.35s ease;
}
.hit:hover .thumb {
  transform: scale(1.03);
}
.art .badge {
  position: absolute;
  left: 12px;
  top: 12px;
  background: rgba(var(--surface-rgb), 0.9);
}
.foot {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 4px;
}
.names {
  display: grid;
  flex: 1;
  min-width: 0;
}
.name {
  font-size: 15px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.fact {
  display: flex;
  align-items: center;
  gap: 4px;
  color: var(--mut);
  font-size: 12px;
  white-space: nowrap;
}
.foot > :last-child {
  flex: none;
  border-radius: 12px;
}
@media (prefers-reduced-motion: reduce) {
  .thumb {
    transition: none;
  }
  .hit:hover .thumb {
    transform: none;
  }
}
</style>
