<script setup lang="ts">
import { ref } from 'vue'
import { clearNotifications, markRead, type NotificationList } from '../api/icedash'
import { ago } from '../lib/format'
import { noteText, notes, setNotes, startUpdate, toast } from '../store'
import { strings } from '../strings'

const busy = ref(false)

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

async function act(kind: string, id: number) {
  void run(() => markRead(id))
  if (kind === 'update') {
    const err = await startUpdate()
    if (err) toast({ title: strings.updateNow, body: err })
  }
}
</script>

<template>
  <div class="notes">
    <div class="tools">
      <button type="button" :disabled="busy || !notes.some((n) => !n.read)" @click="run(() => markRead(0))">{{ strings.markAllRead }}</button>
      <button type="button" :disabled="busy || !notes.length" @click="run(clearNotifications)">{{ strings.clearAll }}</button>
    </div>
    <ul v-if="notes.length">
      <li v-for="n in notes" :key="n.id" :class="{ unread: !n.read }">
        <button type="button" class="item" @click="!n.read && run(() => markRead(n.id))">
          <span class="title">{{ noteText(n).title }}</span>
          <span class="body">{{ noteText(n).body }}</span>
          <time :datetime="n.time" class="when">{{ ago(new Date(n.time)) }}</time>
        </button>
        <button v-if="n.action" type="button" class="action" @click="act(n.action.kind, n.id)">
          {{ n.action.kind === 'update' ? strings.updateNow : n.action.label }}
        </button>
      </li>
    </ul>
    <p v-else class="empty">{{ strings.noNotifications }}</p>
  </div>
</template>

<style scoped>
.notes {
  display: grid;
  gap: 12px;
}
.tools {
  display: flex;
  justify-content: flex-end;
  gap: 6px;
}
.tools button {
  background: none;
  border: 0;
  padding: 4px 8px;
  border-radius: 8px;
  color: var(--ac2);
  font-size: 13px;
  cursor: pointer;
}
.tools button:hover:not(:disabled) {
  background: rgba(76, 201, 240, 0.1);
}
.tools button:disabled {
  color: var(--mut);
  cursor: default;
}
ul {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 8px;
}
li {
  position: relative;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.035);
  border: 1px solid transparent;
}
li.unread {
  border-color: rgba(76, 201, 240, 0.25);
}
li.unread::before {
  content: '';
  position: absolute;
  left: 10px;
  top: 18px;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--ac);
}
.item {
  display: grid;
  gap: 2px;
  width: 100%;
  padding: 12px 14px 12px 24px;
  background: none;
  border: 0;
  text-align: left;
  cursor: pointer;
  border-radius: 14px;
}
.title {
  font-weight: 600;
  font-size: 14px;
}
.body {
  color: var(--mut);
  font-size: 13px;
}
.when {
  color: var(--mut);
  font-size: 12px;
  font-family: var(--font-mono);
}
.action {
  margin: 0 14px 12px 24px;
  border: 0;
  border-radius: var(--radius-pill);
  padding: 6px 14px;
  background: var(--ac);
  color: var(--ink);
  font-weight: 600;
  font-size: 13px;
  cursor: pointer;
}
.action:hover {
  background: var(--acd);
}
.empty {
  margin: 8px 0;
  color: var(--mut);
  text-align: center;
}
</style>
