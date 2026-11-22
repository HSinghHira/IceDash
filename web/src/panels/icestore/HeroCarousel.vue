<script setup lang="ts">
// The banner on Discover, as in the ZimaOS App Store: one featured app at a
// time, its screenshot full bleed with its icon, name and tagline on it, and
// dots with an arrow below. It moves on by itself unless the pointer is on
// it, it has focus, or the user prefers less motion.
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { CatalogApp } from '../../api/apps'
import AppIcon from '../../components/AppIcon.vue'
import Icon from '../../components/Icon.vue'
import { strings } from '../../strings'

const props = defineProps<{ apps: CatalogApp[] }>()
defineEmits<{ open: [app: CatalogApp] }>()

const i = ref(0)
const n = computed(() => props.apps.length)
const app = computed(() => props.apps[i.value % Math.max(1, n.value)])
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
    <button type="button" class="banner" :class="{ empty: !app.thumbnail }" :data-app="app.id" :aria-label="`${app.title}: ${strings.details}`" @click="$emit('open', app)">
      <img v-if="app.thumbnail" :key="app.id" class="thumb" :src="app.thumbnail" alt="" referrerpolicy="no-referrer" />
      <span class="info">
        <AppIcon :src="app.icon" :name="app.title" :size="46" />
        <span class="small-name">{{ app.title }}</span>
        <span class="line">{{ app.tagline }}</span>
      </span>
    </button>
    <div v-if="n > 1" class="pager">
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
          @click="i = k"
        >
          <Icon v-if="k === i" name="star" :size="13" />
        </button>
      </span>
      <button type="button" class="next" :aria-label="strings.next" @click="go(1)"><Icon name="chevron" :size="16" /></button>
    </div>
  </section>
</template>

<style scoped>
.hero-card {
  display: grid;
  gap: 16px;
}
.banner {
  position: relative;
  display: block;
  aspect-ratio: 16 / 10;
  width: 100%;
  padding: 0;
  border: 1px solid var(--line);
  border-radius: var(--radius-card);
  background: rgba(var(--ac-rgb), 0.08);
  color: #fff;
  font: inherit;
  text-align: left;
  overflow: hidden;
  cursor: pointer;
}
.thumb {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: top;
  animation: show 0.45s ease;
  transition: transform 0.5s ease;
}
.banner:hover .thumb {
  transform: scale(1.02);
}
@keyframes show {
  from {
    opacity: 0;
  }
}
/* The app's name on the picture: a dark panel so it reads on any screenshot. */
.info {
  position: absolute;
  top: 18px;
  left: 18px;
  display: grid;
  justify-items: start;
  gap: 4px;
  max-width: min(52%, 290px);
  padding: 12px 14px;
  border-radius: 18px;
  background: rgba(8, 12, 18, 0.72);
  backdrop-filter: blur(10px);
}
.info > :first-child {
  margin-bottom: 4px;
  border-radius: 12px;
  box-shadow: 0 0 0 3px rgba(255, 255, 255, 0.9);
}
.small-name {
  font-size: 12.5px;
  font-weight: 600;
  opacity: 0.85;
}
.line {
  font-size: 14.5px;
  font-weight: 700;
  line-height: 1.3;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.pager {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 18px;
}
.dots {
  display: flex;
  align-items: center;
  gap: 12px;
}
.dot {
  display: grid;
  place-items: center;
  width: 14px;
  height: 14px;
  padding: 0;
  border: 0;
  border-radius: 50%;
  background: rgba(var(--fg-rgb), 0.18);
  color: var(--text);
  cursor: pointer;
}
.dot.on {
  background: none;
}
.dot:hover:not(.on) {
  background: rgba(var(--fg-rgb), 0.35);
}
.next {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  border-radius: 10px;
  border: 1px solid var(--line);
  background: rgba(var(--fg-rgb), 0.04);
  color: var(--text);
  cursor: pointer;
}
.next:hover {
  border-color: rgba(var(--fg-rgb), 0.3);
}
@media (prefers-reduced-motion: reduce) {
  .thumb {
    animation: none;
    transition: none;
  }
}
</style>
