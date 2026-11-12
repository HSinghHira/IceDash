<script setup lang="ts">
// The dock: system launchers | running apps | Settings and Trash. The order is
// fixed in Phase 1 (pinning comes later). A dot means the app is running.
import { computed } from 'vue'
import AppIcon from '../components/AppIcon.vue'
import Icon from '../components/Icon.vue'
import { desktopLaunchers, dockEnd } from '../lib/launchers'
import type { App } from '../lib/apps'
import { appInfo, apps, iceStoreInZimaOS, openApp, openIceStore, openMenu, terminalOk } from '../store'

const launchers = computed(() =>
  desktopLaunchers(location, { terminal: terminalOk.value, iceStoreInZimaOS: iceStoreInZimaOS.value }),
)
const running = computed(() => apps.value.filter((a) => a.status === 'running' && a.url))
const end = dockEnd()

function launch(e: MouseEvent, id: string) {
  if (id !== 'icestore' || e.ctrlKey || e.metaKey || e.shiftKey || e.button !== 0) return
  e.preventDefault()
  openIceStore()
}
function open(e: MouseEvent, a: App) {
  if (e.ctrlKey || e.metaKey || e.shiftKey || e.button !== 0) return
  if (appInfo.value[a.id]?.first_launch) {
    e.preventDefault()
    openApp(a)
  }
}
</script>

<template>
  <nav class="dock" aria-label="Dock">
    <a
      v-for="l in launchers"
      :key="l.id"
      class="slot"
      :href="l.url"
      target="_blank"
      rel="noopener"
      :title="l.name"
      @click="launch($event, l.id)"
    >
      <span class="tile sys"><Icon :name="l.icon" :size="24" /></span>
      <span class="sr">{{ l.name }}</span>
    </a>
    <template v-if="running.length">
      <span class="divider" aria-hidden="true" />
      <a
        v-for="a in running"
        :key="a.id"
        class="slot"
        :href="a.url"
        target="_blank"
        rel="noopener"
        :title="a.name"
        aria-haspopup="menu"
        @click="open($event, a)"
        @contextmenu.prevent="openMenu(a.id, $event)"
      >
        <AppIcon :src="a.icon" :name="a.name" :size="50" />
        <span class="run" aria-hidden="true" />
        <span class="sr">{{ a.name }}</span>
      </a>
    </template>
    <span class="divider" aria-hidden="true" />
    <a v-for="l in end" :key="l.id" class="slot" :href="l.url" target="_blank" rel="noopener" :title="l.name">
      <span class="tile sys"><Icon :name="l.icon" :size="24" /></span>
      <span class="sr">{{ l.name }}</span>
    </a>
  </nav>
</template>

<style scoped>
.dock {
  display: flex;
  align-items: center;
  gap: 10px;
  max-width: 100%;
  overflow-x: auto;
  padding: 12px 14px 14px;
  border-radius: 24px;
  background: rgba(16, 20, 24, 0.6);
  backdrop-filter: var(--blur);
  -webkit-backdrop-filter: var(--blur);
  border: 1px solid var(--line);
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.55);
  scrollbar-width: none;
}
.dock::-webkit-scrollbar {
  display: none;
}
.slot {
  position: relative;
  flex: none;
  display: grid;
  place-items: center;
  width: 54px;
  height: 54px;
  border-radius: var(--radius-tile);
  color: var(--ac2);
  transition: transform 0.15s ease;
}
.slot:hover,
.slot:focus-visible {
  transform: translateY(-4px);
}
.tile.sys {
  display: grid;
  place-items: center;
  width: 54px;
  height: 54px;
  border-radius: var(--radius-tile);
  background: linear-gradient(160deg, rgba(255, 255, 255, 0.07), rgba(255, 255, 255, 0.02));
  border: 1px solid var(--line);
}
.run {
  position: absolute;
  bottom: -9px;
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: var(--ac2);
}
.divider {
  flex: none;
  width: 1px;
  height: 40px;
  margin: 0 4px;
  background: var(--line);
}
.sr {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
}
</style>
