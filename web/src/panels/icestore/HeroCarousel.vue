<script setup lang="ts">
// The banner on Discover: one featured app at a time, its screenshot large,
// with dots and arrows. It moves on by itself unless the pointer is on it,
// it has focus, or the user prefers less motion.
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { CatalogApp } from '../../api/apps'
import AppIcon from '../../components/AppIcon.vue'
import Icon from '../../components/Icon.vue'
import { shortNum } from '../../lib/icestore'
import { strings } from '../../strings'
import { act, button } from './actions'

const props = defineProps<{ apps: CatalogApp[] }>()
defineEmits<{ open: [app: CatalogApp] }>()

const i = ref(0)
const n = computed(() => props.apps.length)
const app = computed(() => props.apps[i.value % Math.max(1, n.value)])
const b = computed(() => (app.value ? button(app.value) : null))
const go = (d: number) => (i.value = (i.value + d + n.value) % n.value)
watch(n, () => (i.value = 0))

const held = ref(false)
let timer: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  const still = typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches
  if (!still)
    timer = setInterval(() => {
      if (!held.value && n.value > 1 && !document.hidden) go(1)
    }, 7000)
})
onBeforeUnmount(() => clearInterval(timer))
</script>

<template>
  <section
    v-if="app"
    class="hero-card"
    :aria-label="strings.featured"
    aria-roledescription="carousel"
    @mouseenter="held = true"
    @mouseleave="held = false"
    @focusin="held = true"
    @focusout="held = false"
  >
    <button type="button" class="hit" :aria-label="`${app.title}: ${strings.details}`" @click="$emit('open', app)" />
    <div class="art" :class="{ empty: !app.thumbnail }">
      <img v-if="app.thumbnail" :key="app.id" class="thumb" :src="app.thumbnail" alt="" referrerpolicy="no-referrer" />
      <AppIcon v-else :src="app.icon" :name="app.title" :size="96" />
      <div v-if="n > 1" class="controls">
        <span class="dots" role="tablist">
          <button
            v-for="(a, k) in apps"
            :key="a.id"
            type="button"
            role="tab"
            class="dot"
            :class="{ on: k === i }"
            :aria-selected="k === i"
            :aria-label="a.title"
            @click.stop="i = k"
          />
        </span>
        <button type="button" class="arrow" :aria-label="strings.previous" @click.stop="go(-1)"><Icon name="arrowLeft" :size="15" /></button>
        <button type="button" class="arrow" :aria-label="strings.next" @click.stop="go(1)"><Icon name="arrowRight" :size="15" /></button>
      </div>
    </div>
    <div class="info" :data-app="app.id">
      <AppIcon :src="app.icon" :name="app.title" :size="52" />
      <div class="text">
        <span class="kicker">
          {{ strings.featured }} · {{ app.category }}<template v-if="app.stars != null"> · <Icon name="star" :size="11" /> {{ shortNum(app.stars) }}</template>
        </span>
        <span class="name">{{ app.title }}</span>
        <span class="line">{{ app.tagline }}</span>
      </div>
      <button v-if="b" type="button" class="pill" :class="b.kind" :disabled="b.disabled" :title="b.title" @click.stop="act(app)">
        {{ b.label }}
      </button>
    </div>
  </section>
</template>

<style scoped src="./shared.css"></style>
<style scoped>
.hero-card {
  position: relative;
  display: grid;
  grid-template-rows: 1fr auto;
  min-height: 300px;
  border-radius: var(--radius-card);
  border: 1px solid var(--line);
  background: var(--card);
  overflow: hidden;
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
  min-height: 0;
  overflow: hidden;
  background: rgba(var(--fg-rgb), 0.04);
}
.thumb {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: top;
  animation: show 0.4s ease;
}
@keyframes show {
  from {
    opacity: 0;
  }
}
.art.empty {
  display: grid;
  place-items: center;
  background: rgba(var(--ac-rgb), 0.07);
}
.info {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 18px;
  border-top: 1px solid var(--line);
  background: rgba(var(--surface-rgb), 0.92);
}
.text {
  display: grid;
  flex: 1;
  min-width: 0;
}
.kicker {
  display: flex;
  align-items: center;
  gap: 3px;
  white-space: pre;
  overflow: hidden;
}
.name {
  font-size: 18px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.line {
  color: var(--mut);
  font-size: 12.5px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pill {
  z-index: 2;
}
.controls {
  position: absolute;
  right: 12px;
  bottom: 12px;
  z-index: 2;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 4px 4px 10px;
  border-radius: var(--radius-pill);
  background: rgba(var(--surface-rgb), 0.8);
  backdrop-filter: var(--blur);
}
.dots {
  display: flex;
  gap: 5px;
  margin-right: 4px;
}
.dot {
  width: 7px;
  height: 7px;
  padding: 0;
  border: 0;
  border-radius: 50%;
  background: rgba(var(--fg-rgb), 0.3);
  cursor: pointer;
}
.dot.on {
  width: 18px;
  border-radius: var(--radius-pill);
  background: var(--ac);
}
.arrow {
  display: grid;
  place-items: center;
  width: 26px;
  height: 26px;
  border: 0;
  border-radius: 50%;
  background: rgba(var(--fg-rgb), 0.08);
  color: var(--text);
  cursor: pointer;
}
.arrow:hover {
  background: rgba(var(--fg-rgb), 0.16);
}
</style>
