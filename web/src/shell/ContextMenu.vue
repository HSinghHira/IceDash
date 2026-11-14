<script setup lang="ts">
// The right-click menu for everything that isn't an app: the desktop, the
// cards on the panel, the dock, windows and the top bar. It looks and works
// like the app menu (mock-up 3): arrow keys move, Escape closes.
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Icon from '../components/Icon.vue'
import { ctx, type CtxItem } from '../store'

const el = ref<HTMLElement | null>(null)
const pos = ref({ x: 0, y: 0 })

function close() {
  ctx.value = null
}
async function run(i: CtxItem) {
  if (i.disabled) return
  close()
  await i.run()
}

async function place() {
  const m = ctx.value
  if (!m) return
  pos.value = { x: m.x, y: m.y }
  await nextTick()
  const r = el.value?.getBoundingClientRect()
  if (!r) return
  pos.value = {
    x: Math.max(8, Math.min(m.x, window.innerWidth - r.width - 8)),
    y: Math.max(8, Math.min(m.y, window.innerHeight - r.height - 8)),
  }
  el.value?.querySelector<HTMLElement>('button:not(:disabled)')?.focus()
}
watch(ctx, place, { immediate: true })

function onDoc(e: Event) {
  if (ctx.value && !el.value?.contains(e.target as Node)) close()
}
function onKey(e: KeyboardEvent) {
  if (!ctx.value) return
  if (e.key === 'Escape') {
    e.stopPropagation()
    close()
  }
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp' || e.key === 'Home' || e.key === 'End') {
    e.preventDefault()
    const btns = [...(el.value?.querySelectorAll<HTMLElement>('button:not(:disabled)') ?? [])]
    const i = btns.indexOf(document.activeElement as HTMLElement)
    const next = e.key === 'Home' ? 0 : e.key === 'End' ? btns.length - 1 : (i + (e.key === 'ArrowDown' ? 1 : -1) + btns.length) % btns.length
    btns[next]?.focus()
  }
}
onMounted(() => {
  document.addEventListener('mousedown', onDoc)
  document.addEventListener('keydown', onKey, true)
  window.addEventListener('resize', close)
  window.addEventListener('blur', close)
})
onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDoc)
  document.removeEventListener('keydown', onKey, true)
  window.removeEventListener('resize', close)
  window.removeEventListener('blur', close)
})
</script>

<template>
  <div
    v-if="ctx"
    ref="el"
    class="ctx"
    role="menu"
    :aria-label="ctx.title"
    :style="{ left: `${pos.x}px`, top: `${pos.y}px` }"
    @contextmenu.prevent
  >
    <div v-if="ctx.title" class="head">{{ ctx.title }}</div>
    <template v-for="(g, gi) in ctx.groups" :key="gi">
      <span v-if="gi > 0" class="div" aria-hidden="true" />
      <button
        v-for="i in g"
        :key="i.id"
        type="button"
        :role="i.checked === undefined ? 'menuitem' : 'menuitemcheckbox'"
        :aria-checked="i.checked === undefined ? undefined : i.checked"
        class="item"
        :class="{ danger: i.danger }"
        :disabled="i.disabled"
        :data-ctx="i.id"
        @click="run(i)"
      >
        <Icon :name="i.icon" :size="15" />
        <span>{{ i.label }}</span>
        <Icon v-if="i.checked" name="check" :size="14" class="tick" />
      </button>
    </template>
  </div>
</template>

<style scoped>
.ctx {
  position: fixed;
  z-index: 250;
  min-width: 210px;
  max-width: 300px;
  padding: 6px;
  border-radius: 14px;
  background: rgba(var(--surface-rgb), 0.97);
  border: 1px solid var(--line);
  box-shadow: 0 24px 70px rgba(0, 0, 0, 0.55);
  backdrop-filter: var(--blur);
  -webkit-backdrop-filter: var(--blur);
}
.head {
  padding: 7px 10px 6px;
  color: var(--ac2);
  font: 700 10.5px var(--font-mono);
  letter-spacing: 0.12em;
  text-transform: uppercase;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
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
.item:hover:not(:disabled),
.item:focus-visible {
  outline: none;
  background: rgba(var(--fg-rgb), 0.06);
}
.item:disabled {
  opacity: 0.4;
  cursor: default;
}
.item .tick {
  margin-left: auto;
  color: var(--ac);
}
.danger,
.danger :deep(svg) {
  color: #ff7d7d;
}
</style>
