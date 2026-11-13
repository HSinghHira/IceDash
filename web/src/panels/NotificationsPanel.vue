<script setup lang="ts">
// The notification centre: drops down from the bell at the top right, like a
// desktop's notification list. Escape, a click outside or the close dot closes it.
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { clearNotifications, markRead, type NotificationList } from '../api/icedash'
import Icon from '../components/Icon.vue'
import WindowDots from '../components/WindowDots.vue'
import { ago } from '../lib/format'
import { noteText, notes, notesOpen, runNoteAction, setNotes, toast, unread } from '../store'
import { strings } from '../strings'

const busy = ref(false)
const el = ref<HTMLElement | null>(null)

async function run(fn: () => Promise<NotificationList>) {
  busy.value = true
  try {
    setNotes(await fn())
  } catch {
    // the next poll shows the real state
  } finally {
    busy.value = false
  }
}

async function act(kind: string, id: number, app?: string) {
  void run(() => markRead(id))
  notesOpen.value = false
  const err = await runNoteAction(kind, app)
  if (err) toast({ title: kind === 'update' ? strings.updateNow : strings.appName, body: err })
}

const close = () => (notesOpen.value = false)
function onDoc(e: MouseEvent) {
  const t = e.target as HTMLElement
  if (!el.value?.contains(t) && !t.closest('.bell')) close()
}
function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') close()
}
onMounted(() => {
  document.addEventListener('mousedown', onDoc)
  document.addEventListener('keydown', onKey)
  el.value?.focus()
})
onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDoc)
  document.removeEventListener('keydown', onKey)
})
</script>

<template>
  <section ref="el" class="center" role="dialog" :aria-label="strings.notifications" tabindex="-1">
    <header class="head">
      <WindowDots @close="close" />
      <h2>{{ strings.notifications }}<span v-if="unread" class="count">{{ unread }}</span></h2>
      <span class="tools">
        <button
          type="button"
          :title="strings.markAllRead"
          :aria-label="strings.markAllRead"
          :disabled="busy || !notes.some((n) => !n.read)"
          @click="run(() => markRead(0))"
        >
          <Icon name="checks" :size="16" />
        </button>
        <button
          type="button"
          :title="strings.clearAll"
          :aria-label="strings.clearAll"
          :disabled="busy || !notes.length"
          @click="run(clearNotifications)"
        >
          <Icon name="trash" :size="15" />
        </button>
      </span>
    </header>
    <ul v-if="notes.length" class="list">
      <li v-for="n in notes" :key="n.id" :class="{ unread: !n.read }">
        <button type="button" class="item" @click="!n.read && run(() => markRead(n.id))">
          <span class="top">
            <span class="title">{{ noteText(n).title }}</span>
            <time :datetime="n.time" class="when">{{ ago(new Date(n.time)) }}</time>
          </span>
          <span v-if="noteText(n).body" class="body">{{ noteText(n).body }}</span>
        </button>
        <button v-if="n.action" type="button" class="action" @click="act(n.action.kind, n.id, n.app)">
          {{ n.action.kind === 'update' ? strings.updateNow : n.action.label }}
        </button>
      </li>
    </ul>
    <p v-else class="empty"><Icon name="bell" :size="20" /> {{ strings.noNotifications }}</p>
  </section>
</template>

<style scoped>
.center {
  position: fixed;
  top: 46px;
  right: 10px;
  z-index: 220;
  width: 360px;
  max-width: calc(100vw - 20px);
  max-height: calc(100vh - 60px);
  display: flex;
  flex-direction: column;
  border-radius: 18px;
  background: rgba(14, 18, 22, 0.95);
  backdrop-filter: var(--blur);
  -webkit-backdrop-filter: var(--blur);
  border: 1px solid var(--line);
  box-shadow: 0 24px 70px rgba(0, 0, 0, 0.7);
  outline: none;
  animation: drop 0.14s ease-out;
}
@keyframes drop {
  from {
    opacity: 0;
    transform: translateY(-6px);
  }
}
.head {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 10px 10px 14px;
  border-bottom: 1px solid var(--line);
}
h2 {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  font-size: 15px;
  font-weight: 700;
}
.count {
  padding: 1px 8px;
  border-radius: var(--radius-pill);
  background: rgba(76, 201, 240, 0.15);
  color: var(--ac2);
  font-family: var(--font-mono);
  font-size: 11px;
}
.tools {
  display: flex;
  gap: 2px;
  margin-left: auto;
}
.tools button {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: 0;
  border-radius: 8px;
  background: none;
  color: var(--mut);
  cursor: pointer;
}
.tools button:hover:not(:disabled) {
  background: rgba(76, 201, 240, 0.12);
  color: var(--ac2);
}
.tools button:disabled {
  opacity: 0.35;
  cursor: default;
}
.list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  list-style: none;
  margin: 0;
  padding: 8px;
  display: grid;
  align-content: start;
  gap: 6px;
}
li {
  position: relative;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.035);
}
li.unread {
  background: rgba(76, 201, 240, 0.07);
}
li.unread::before {
  content: '';
  position: absolute;
  left: 9px;
  top: 15px;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--ac);
}
.item {
  display: grid;
  gap: 2px;
  width: 100%;
  padding: 9px 12px 9px 22px;
  background: none;
  border: 0;
  text-align: left;
  cursor: pointer;
  border-radius: 12px;
}
.top {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
}
.title {
  font-weight: 600;
  font-size: 13px;
}
.body {
  color: var(--mut);
  font-size: 12.5px;
  line-height: 1.4;
}
.when {
  flex: none;
  color: var(--mut);
  font-size: 11px;
  font-family: var(--font-mono);
}
.action {
  margin: 0 12px 9px 22px;
  border: 0;
  border-radius: var(--radius-pill);
  padding: 5px 12px;
  background: var(--ac);
  color: var(--ink);
  font-weight: 600;
  font-size: 12px;
  cursor: pointer;
}
.action:hover {
  background: var(--acd);
}
.empty {
  display: grid;
  justify-items: center;
  gap: 8px;
  margin: 0;
  padding: 28px 16px;
  color: var(--mut);
  font-size: 13px;
}
</style>
