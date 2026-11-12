<script setup lang="ts">
import { computed } from 'vue'
import AppIcon from '../components/AppIcon.vue'
import Icon from '../components/Icon.vue'
import { desktopLaunchers } from '../lib/launchers'
import type { App } from '../lib/apps'
import { appInfo, apps, appsLoaded, iceStoreInZimaOS, jobFor, openApp, openIceStore, openMenu, terminalOk } from '../store'
import { strings } from '../strings'

const launchers = computed(() =>
  desktopLaunchers(location, { terminal: terminalOk.value, iceStoreInZimaOS: iceStoreInZimaOS.value }),
)
const statusWord = { running: strings.running, starting: strings.starting, stopped: strings.stopped }

/** IceStore opens its built-in window; a plain click on the others follows the link. */
function launch(e: MouseEvent, id: string) {
  if (id !== 'icestore' || e.ctrlKey || e.metaKey || e.shiftKey || e.button !== 0) return
  e.preventDefault()
  openIceStore()
}

/** A freshly installed IceStore app shows its welcome popup first. */
function open(e: MouseEvent, a: App) {
  if (e.ctrlKey || e.metaKey || e.shiftKey || e.button !== 0) return
  if (appInfo.value[a.id]?.first_launch) {
    e.preventDefault()
    openApp(a)
  }
}

// Touch: a long press opens the menu.
let press: ReturnType<typeof setTimeout> | undefined
function down(e: PointerEvent, name: string) {
  if (e.pointerType !== 'touch') return
  const target = e.currentTarget as HTMLElement | null
  const at = new MouseEvent('contextmenu', { clientX: e.clientX, clientY: e.clientY })
  press = setTimeout(() => {
    press = undefined
    openMenu(name, at)
    target?.addEventListener('click', (c) => c.preventDefault(), { once: true, capture: true })
  }, 550)
}
function cancel() {
  clearTimeout(press)
}
</script>

<template>
  <nav class="icons" :aria-label="strings.apps">
    <a
      v-for="l in launchers"
      :key="l.id"
      class="item"
      :href="l.url"
      target="_blank"
      rel="noopener"
      @click="launch($event, l.id)"
    >
      <span class="tile sys"><Icon :name="l.icon" :size="26" /></span>
      <span class="label">{{ l.name }}</span>
    </a>

    <template v-for="a in apps" :key="a.id">
      <a
        v-if="a.url"
        class="item"
        :href="a.url"
        target="_blank"
        rel="noopener"
        :aria-label="strings.appStatus(a.name, statusWord[a.status])"
        aria-haspopup="menu"
        @click="open($event, a)"
        @contextmenu.prevent="openMenu(a.id, $event)"
        @pointerdown="down($event, a.id)"
        @pointerup="cancel"
        @pointerleave="cancel"
        @pointermove="cancel"
      >
        <span class="tile" :class="{ busy: jobFor(a.id)?.state === 'running' }">
          <AppIcon :src="a.icon" :name="a.name" /><span class="status" :class="a.status" />
          <span v-if="appInfo[a.id]?.update" class="upd" :title="strings.updateBadge" />
        </span>
        <span class="label">{{ a.name }}</span>
      </a>
      <div
        v-else
        class="item no-link"
        :aria-label="strings.appStatus(a.name, statusWord[a.status])"
        role="button"
        tabindex="0"
        aria-haspopup="menu"
        @contextmenu.prevent="openMenu(a.id, $event)"
        @keydown.enter="openMenu(a.id, $event)"
        @pointerdown="down($event, a.id)"
        @pointerup="cancel"
        @pointerleave="cancel"
      >
        <span class="tile"><AppIcon :src="a.icon" :name="a.name" /><span class="status" :class="a.status" /></span>
        <span class="label">{{ a.name }}</span>
      </div>
    </template>
    <p v-if="appsLoaded && apps.length === 0" class="empty">{{ strings.noApps }}</p>
  </nav>
</template>

<style scoped>
.icons {
  display: grid;
  grid-template-columns: repeat(2, 104px);
  grid-auto-rows: 102px;
  gap: 2px 0;
  align-content: start;
}
.item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 7px;
  padding-top: 4px;
  border-radius: 14px;
  text-decoration: none;
  color: var(--text);
}
.item:hover .tile,
.item:focus-visible .tile {
  transform: translateY(-2px);
  box-shadow: 0 10px 28px rgba(76, 201, 240, 0.18);
}
.item:focus-visible {
  outline-offset: 0;
}
.no-link {
  cursor: default;
}
.tile {
  position: relative;
  width: 60px;
  height: 60px;
  border-radius: var(--radius-tile);
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}
.tile.sys {
  display: grid;
  place-items: center;
  color: var(--ac2);
  background: linear-gradient(160deg, rgba(255, 255, 255, 0.07), rgba(255, 255, 255, 0.02));
  border: 1px solid var(--line);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.05);
}
.status {
  position: absolute;
  right: -4px;
  bottom: -4px;
  width: 13px;
  height: 13px;
  border-radius: 50%;
  border: 2.5px solid var(--bg);
  background: #6b7076;
}
.status.running {
  background: var(--ac2);
}
.status.starting {
  background: var(--warn);
}
.upd {
  position: absolute;
  top: -4px;
  right: -4px;
  width: 13px;
  height: 13px;
  border-radius: 50%;
  border: 2.5px solid var(--bg);
  background: var(--warn);
}
.tile.busy::after {
  content: '';
  position: absolute;
  inset: -5px;
  border-radius: 20px;
  border: 2px solid transparent;
  border-top-color: var(--ac);
  animation: spin 1s linear infinite;
}
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
.label {
  max-width: 100px;
  font-size: 13px;
  font-weight: 500;
  line-height: 1.25;
  text-align: center;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.8);
}
.empty {
  grid-column: 1 / -1;
  color: var(--mut);
  font-size: 13px;
}
@media (max-width: 900px) {
  .icons {
    grid-template-columns: repeat(auto-fill, minmax(92px, 1fr));
  }
}
</style>
