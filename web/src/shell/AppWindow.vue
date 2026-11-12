<script setup lang="ts">
// A window for IceDash's own panels: dragged by its title bar, focused on click,
// closed with the button or Escape. Apps never open in windows in Phase 1.
import { onMounted, ref } from 'vue'
import Icon from '../components/Icon.vue'
import { closeWindow, focusWindow, type Win } from '../store'
import { strings } from '../strings'

const props = defineProps<{ win: Win; width?: number; index: number }>()

const el = ref<HTMLElement | null>(null)
const pos = ref({ x: 0, y: 0 })
let drag: { dx: number; dy: number } | null = null

onMounted(() => {
  const w = props.width ?? 460
  const vw = window.innerWidth
  // Open a little apart from each other, left of the widget panel.
  pos.value = { x: Math.max(12, Math.min(vw - w - 12, (vw - w) / 2 - 120 + props.index * 32)), y: 70 + props.index * 32 }
  el.value?.focus()
})

function down(e: PointerEvent) {
  if ((e.target as HTMLElement).closest('button')) return
  focusWindow(props.win.id)
  drag = { dx: e.clientX - pos.value.x, dy: e.clientY - pos.value.y }
  ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
}
function move(e: PointerEvent) {
  if (!drag) return
  const w = el.value?.offsetWidth ?? 400
  pos.value = {
    x: Math.min(Math.max(e.clientX - drag.dx, 8 - w + 80), window.innerWidth - 80),
    y: Math.min(Math.max(e.clientY - drag.dy, 40), window.innerHeight - 60),
  }
}
function up() {
  drag = null
}
</script>

<template>
  <section
    ref="el"
    class="win"
    role="dialog"
    :aria-label="win.title"
    tabindex="-1"
    :style="{ left: `${pos.x}px`, top: `${pos.y}px`, zIndex: win.z, width: `${width ?? 460}px` }"
    @pointerdown="focusWindow(win.id)"
    @keydown.esc="closeWindow(win.id)"
  >
    <header class="bar" @pointerdown="down" @pointermove="move" @pointerup="up" @pointercancel="up">
      <span class="dots" aria-hidden="true"><i /><i /><i /></span>
      <h2>{{ win.title }}</h2>
      <button type="button" class="x" :aria-label="strings.close" @click="closeWindow(win.id)">
        <Icon name="close" :size="16" />
      </button>
    </header>
    <div class="body"><slot /></div>
  </section>
</template>

<style scoped>
.win {
  position: fixed;
  max-width: calc(100vw - 16px);
  max-height: calc(100vh - 140px);
  display: flex;
  flex-direction: column;
  border-radius: 22px;
  background:
    radial-gradient(120% 60% at 100% 0%, rgba(76, 201, 240, 0.14), transparent 55%),
    rgba(12, 16, 20, 0.9);
  backdrop-filter: blur(24px);
  -webkit-backdrop-filter: blur(24px);
  border: 1px solid var(--line);
  box-shadow: 0 30px 90px rgba(0, 0, 0, 0.7);
  outline: none;
}
.win:focus-visible {
  box-shadow: 0 0 0 2px var(--ac2), 0 30px 90px rgba(0, 0, 0, 0.7);
}
.bar {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 44px;
  padding: 0 14px;
  border-bottom: 1px solid var(--line);
  cursor: grab;
  touch-action: none;
  user-select: none;
}
.bar:active {
  cursor: grabbing;
}
.dots {
  position: absolute;
  left: 16px;
  display: flex;
  gap: 7px;
}
.dots i {
  width: 11px;
  height: 11px;
  border-radius: 50%;
  background: var(--ac);
}
.dots i:nth-child(2) {
  background: var(--ac2);
}
.dots i:nth-child(3) {
  background: rgba(255, 255, 255, 0.25);
}
h2 {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 12.5px;
  font-weight: 600;
  color: var(--mut);
  letter-spacing: 0.04em;
}
.x {
  position: absolute;
  right: 10px;
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  border: 0;
  border-radius: 8px;
  background: none;
  color: var(--mut);
  cursor: pointer;
}
.x:hover {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text);
}
.body {
  padding: 20px 22px 22px;
  overflow-y: auto;
}
</style>
