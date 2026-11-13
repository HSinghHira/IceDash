<script setup lang="ts">
// Mock-up screen 3: the right-click menu on an app, desktop or dock.
// Members only get Open; the rest is admin only (the backend checks too).
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as api from '../api/apps'
import Icon from '../components/Icon.vue'
import type { IconName } from '../lib/icons'
import { ZIMAOS_DASHBOARD } from '../lib/launchers'
import { appInfo, apps, jobFor, menu, openApp, openWindow, refreshApps, toast, trackJob, uninstallFor, user } from '../store'
import { strings } from '../strings'

const el = ref<HTMLElement | null>(null)
const pos = ref({ x: 0, y: 0 })
const app = computed(() => apps.value.find((a) => a.id === menu.value?.name))
const info = computed(() => (menu.value ? appInfo.value[menu.value.name] : undefined))
const admin = computed(() => !!user.value?.admin)
const busy = computed(() => (menu.value ? jobFor(menu.value.name)?.state === 'running' : false))
const statusWord = { running: strings.running, starting: strings.starting, stopped: strings.stopped }

interface Item {
  id: string
  label: string
  icon: IconName
  danger?: boolean
  more?: boolean
  run: () => void | Promise<void>
}

const groups = computed<Item[][]>(() => {
  const a = app.value
  if (!a) return []
  const open: Item[] = []
  if (a.url) {
    open.push({ id: 'open', label: strings.openApp, icon: 'arrowRight', run: () => openApp(a) })
    open.push({ id: 'tab', label: strings.openInNewTab, icon: 'window', run: () => void window.open(a.url, '_blank', 'noopener') })
  }
  if (!admin.value) return [open]
  const look: Item[] = []
  if (info.value?.icestore)
    look.push({ id: 'creds', label: strings.credentials, icon: 'key', more: true, run: () => openWindow('credentials', { name: a.id, title: a.name }) })
  look.push({ id: 'logs', label: strings.logs, icon: 'lines', run: () => openWindow('logs', { name: a.id, title: a.name }) })
  const life: Item[] = [{ id: 'restart', label: strings.restart, icon: 'refresh', run: () => action('restart') }]
  life.push(
    a.status === 'stopped'
      ? { id: 'start', label: strings.start, icon: 'play', run: () => action('start') }
      : { id: 'stop', label: strings.stop, icon: 'stop', run: () => action('stop') },
  )
  if (info.value?.update && !busy.value)
    life.push({ id: 'update', label: strings.updateTo(info.value.latest ?? ''), icon: 'download', run: update })
  const more: Item[] = [
    { id: 'settings', label: strings.appSettings, icon: 'settings', run: () => void window.open(ZIMAOS_DASHBOARD, '_blank', 'noopener') },
    { id: 'backup', label: strings.backUpData, icon: 'download', run: backup },
  ]
  const end: Item[] = [{ id: 'uninstall', label: strings.uninstall, icon: 'trash', danger: true, run: () => void (uninstallFor.value = a.id) }]
  return [open, look, life, more, end].filter((g) => g.length)
})

function close() {
  menu.value = null
}

async function run(i: Item) {
  close()
  await i.run()
}

async function action(a: api.AppAction) {
  const m = app.value
  if (!m) return
  try {
    await api.appAction(m.id, a)
    toast({ title: strings.actionDone[a] ?? a, body: m.name })
    setTimeout(refreshApps, 1500)
    setTimeout(refreshApps, 6000)
  } catch (e) {
    toast({ title: m.name, body: e instanceof Error ? e.message : String(e) })
  }
}

async function update() {
  const m = app.value
  if (!m) return
  try {
    trackJob(await api.updateApp(m.id))
    toast({ title: strings.updatingApp, body: m.name })
  } catch (e) {
    toast({ title: m.name, body: e instanceof Error ? e.message : String(e) })
  }
}

async function backup() {
  const m = app.value
  if (!m) return
  toast({ title: strings.backingUp(m.name), body: strings.backupHint })
  try {
    await api.downloadBackup(m.id)
    toast({ title: strings.backUpData, body: strings.backupDone(m.name) })
  } catch (e) {
    toast({ title: strings.backUpData, body: e instanceof Error ? e.message : String(e) })
  }
}

async function place() {
  if (!menu.value) return
  pos.value = { x: menu.value.x, y: menu.value.y }
  await nextTick()
  const r = el.value?.getBoundingClientRect()
  if (!r) return
  pos.value = {
    x: Math.max(8, Math.min(menu.value.x, window.innerWidth - r.width - 8)),
    y: Math.max(8, Math.min(menu.value.y, window.innerHeight - r.height - 8)),
  }
  el.value?.querySelector<HTMLElement>('button')?.focus()
}
watch(menu, place, { immediate: true })

function onDoc(e: Event) {
  if (menu.value && !el.value?.contains(e.target as Node)) close()
}
function onKey(e: KeyboardEvent) {
  if (!menu.value) return
  if (e.key === 'Escape') close()
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    e.preventDefault()
    const btns = [...(el.value?.querySelectorAll<HTMLElement>('button') ?? [])]
    const i = btns.indexOf(document.activeElement as HTMLElement)
    btns[(i + (e.key === 'ArrowDown' ? 1 : -1) + btns.length) % btns.length]?.focus()
  }
}
onMounted(() => {
  document.addEventListener('mousedown', onDoc)
  document.addEventListener('keydown', onKey)
  window.addEventListener('resize', close)
})
onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDoc)
  document.removeEventListener('keydown', onKey)
  window.removeEventListener('resize', close)
})
</script>

<template>
  <div
    v-if="menu && app"
    ref="el"
    class="menu"
    role="menu"
    :aria-label="strings.appMenu(app.name)"
    :style="{ left: `${pos.x}px`, top: `${pos.y}px` }"
    @contextmenu.prevent
  >
    <div class="head">
      <span class="name">{{ app.name }}</span>
      <span class="state" :class="app.status">{{ statusWord[app.status].toLowerCase() }}</span>
    </div>
    <template v-for="(g, gi) in groups" :key="gi">
      <span v-if="gi > 0" class="div" aria-hidden="true" />
      <button
        v-for="i in g"
        :key="i.id"
        type="button"
        role="menuitem"
        class="item"
        :class="{ danger: i.danger, accent: i.more }"
        @click="run(i)"
      >
        <Icon :name="i.icon" :size="15" />
        <span>{{ i.label }}</span>
        <Icon v-if="i.more" name="chevron" :size="14" class="more" />
      </button>
    </template>
  </div>
</template>

<style scoped>
.menu {
  position: fixed;
  z-index: 250;
  min-width: 200px;
  padding: 6px;
  border-radius: 14px;
  background: rgba(14, 18, 22, 0.97);
  border: 1px solid var(--line);
  box-shadow: 0 24px 70px rgba(0, 0, 0, 0.7);
  backdrop-filter: var(--blur);
  -webkit-backdrop-filter: var(--blur);
}
.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding: 7px 10px 6px;
  font-family: var(--font-mono);
  font-size: 10.5px;
}
.name {
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--ac2);
}
.state {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--mut);
}
.state::before {
  content: '';
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #6b7076;
}
.state.running {
  color: var(--ac2);
}
.state.running::before {
  background: var(--ac);
}
.state.starting::before {
  background: var(--warn);
}
.div {
  display: block;
  height: 1px;
  margin: 4px 8px;
  background: var(--line);
}
.item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 6px 10px;
  border: 1px solid transparent;
  border-radius: 9px;
  background: none;
  color: var(--text);
  font: inherit;
  font-size: 13.5px;
  text-align: left;
  cursor: pointer;
}
.item :deep(svg) {
  color: var(--mut);
}
.item:hover,
.item:focus-visible {
  outline: none;
  background: rgba(255, 255, 255, 0.06);
}
.item.accent:hover,
.item.accent:focus-visible {
  border-color: rgba(76, 201, 240, 0.45);
  background: rgba(76, 201, 240, 0.12);
}
.more {
  margin-left: auto;
}
.danger {
  color: #ff7d7d;
}
.danger :deep(svg) {
  color: #ff7d7d;
}
</style>
