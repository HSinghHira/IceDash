<script setup lang="ts">
// A compact section on Discover and the app page, like "New Arrivals" in
// the ZimaOS App Store: a title, and apps as rows in three columns. No
// screenshots, so it loads at once.
import type { CatalogApp } from '../../api/apps'
import { strings } from '../../strings'
import AppRow from './AppRow.vue'

defineProps<{
  title: string
  apps: CatalogApp[]
  /** Show when each app was added or last updated. */
  when?: 'added' | 'updated'
}>()
defineEmits<{ open: [app: CatalogApp]; all: [] }>()
</script>

<template>
  <section class="grid-sec" :aria-label="title">
    <header class="sec-head">
      <h3>{{ title }}</h3>
      <button type="button" class="link" @click="$emit('all')">{{ strings.seeAll }}</button>
    </header>
    <ul class="rows">
      <AppRow v-for="a in apps" :key="a.id" :app="a" :fact="when" @open="$emit('open', $event)" />
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
</style>
