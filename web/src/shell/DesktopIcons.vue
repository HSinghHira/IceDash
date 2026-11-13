<script setup lang="ts">
// The desktop shows the installed apps; IceDash's own launchers are in the dock.
import AppIcon from '../components/AppIcon.vue'
import type { App } from '../lib/apps'
import { appInfo, apps, appsLoaded, jobFor, openApp, openMenu } from '../store'
import { strings } from '../strings'

const statusWord = { running: strings.running, starting: strings.starting, stopped: strings.stopped }

/** A plain click opens the app in a window (or its welcome popup first); Ctrl/middle-click keeps the new tab. */
function open(e: MouseEvent, a: App) {
  if (e.ctrlKey || e.metaKey || e.shiftKey || e.button !== 0) return
  e.preventDefault()
  openApp(a)
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
          <AppIcon :src="a.icon" :name="a.name" :size="50" /><span class="status" :class="a.status" />
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
        <span class="tile"><AppIcon :src="a.icon" :name="a.name" :size="50" /><span class="status" :class="a.status" /></span>
        <span class="label">{{ a.name }}</span>
      </div>
    </template>
    <p v-if="appsLoaded && apps.length === 0" class="empty">{{ strings.noApps }}</p>
  </nav>
</template>

<style scoped>
.icons {
  height: 100%;
  display: grid;
  grid-auto-flow: column;
  grid-template-rows: repeat(auto-fill, 86px);
  grid-auto-columns: 90px;
  gap: 2px 0;
  align-content: start;
  justify-content: start;
  overflow-x: auto;
  scrollbar-width: none;
}
.icons::-webkit-scrollbar {
  display: none;
}
.item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding-top: 3px;
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
  width: 50px;
  height: 50px;
  border-radius: 13px;
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}
.status {
  position: absolute;
  right: -4px;
  bottom: -4px;
  width: 11px;
  height: 11px;
  border-radius: 50%;
  border: 2px solid var(--bg);
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
  width: 11px;
  height: 11px;
  border-radius: 50%;
  border: 2px solid var(--bg);
  background: var(--warn);
}
.tile.busy::after {
  content: '';
  position: absolute;
  inset: -4px;
  border-radius: 16px;
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
  max-width: 86px;
  font-size: 12px;
  font-weight: 500;
  line-height: 1.25;
  text-align: center;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.8);
}
.empty {
  grid-row: 1;
  color: var(--mut);
  font-size: 13px;
}
@media (max-width: 900px) {
  .icons {
    height: auto;
    grid-auto-flow: row;
    grid-template-rows: none;
    grid-template-columns: repeat(auto-fill, minmax(88px, 1fr));
  }
}
</style>
