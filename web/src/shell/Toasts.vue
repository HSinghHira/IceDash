<script setup lang="ts">
// New notifications also show as a toast in the bottom-right for 6 s (PLAN.md).
import Icon from '../components/Icon.vue'
import { dismissToast, runNoteAction, toast, toasts } from '../store'
import { strings } from '../strings'

async function act(id: number, kind: string, app?: string) {
  dismissToast(id)
  const err = await runNoteAction(kind, app)
  if (err) toast({ title: strings.appName, body: err })
}
</script>

<template>
  <div class="toasts" role="status" aria-live="polite">
    <div v-for="t in toasts" :key="t.id" class="toast">
      <div class="text">
        <strong>{{ t.title }}</strong>
        <span>{{ t.body }}</span>
      </div>
      <button v-if="t.action" type="button" class="action" @click="act(t.id, t.action.kind, t.app)">
        {{ t.action.kind === 'update' ? strings.updateNow : t.action.label }}
      </button>
      <button type="button" class="x" :aria-label="strings.close" @click="dismissToast(t.id)">
        <Icon name="close" :size="14" />
      </button>
    </div>
  </div>
</template>

<style scoped>
.toasts {
  position: fixed;
  right: 16px;
  bottom: 16px;
  z-index: 300;
  display: grid;
  gap: 10px;
  width: min(360px, calc(100vw - 32px));
}
.toast {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 14px 12px 14px 16px;
  border-radius: 18px;
  background: rgba(var(--surface-rgb), 0.94);
  backdrop-filter: var(--blur);
  -webkit-backdrop-filter: var(--blur);
  border: 1px solid rgba(var(--ac-rgb), 0.3);
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.6);
  animation: rise 0.25s ease;
}
.text {
  flex: 1;
  display: grid;
  gap: 2px;
  font-size: 13.5px;
}
.text span {
  color: var(--mut);
  font-size: 13px;
}
.action {
  align-self: center;
  border: 0;
  border-radius: var(--radius-pill);
  padding: 6px 12px;
  background: var(--ac);
  color: var(--ink);
  font-weight: 600;
  font-size: 12.5px;
  cursor: pointer;
  white-space: nowrap;
}
.x {
  display: grid;
  place-items: center;
  width: 24px;
  height: 24px;
  border: 0;
  border-radius: 6px;
  background: none;
  color: var(--mut);
  cursor: pointer;
}
.x:hover {
  background: rgba(var(--fg-rgb), 0.08);
}
@keyframes rise {
  from {
    opacity: 0;
    transform: translateY(8px);
  }
}
@media (prefers-reduced-motion: reduce) {
  .toast {
    animation: none;
  }
}
</style>
