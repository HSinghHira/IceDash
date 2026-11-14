<script setup lang="ts">
// A window for IceDash's panels and for apps: dragged by its title bar,
// focused on click, closed with Escape. The dots close it, minimize it into
// its dock icon (and back, with the same animation), or let it fill the
// desktop above the dock.
import { computed, onMounted, ref } from 'vue'
import Icon from '../components/Icon.vue'
import WindowDots from '../components/WindowDots.vue'
import { apps, closeWindow, focusWindow, toggleMax, toggleMin, type Win } from '../store'
import { strings } from '../strings'

const props = defineProps<{ win: Win; width?: number; index: number }>()

const el = ref<HTMLElement | null>(null)
const pos = ref({ x: 0, y: 0 })
let drag: { dx: number; dy: number } | null = null

const isApp = computed(() => props.win.kind === 'app')
/** Apps, Files and the Terminal fill a tall window edge to edge. */
const full = computed(() => isApp.value || props.win.kind === 'files' || props.win.kind === 'terminal' || props.win.kind === 'settings')
const appUrl = computed(() => (isApp.value ? apps.value.find((a) => a.id === props.win.app)?.url : undefined))

onMounted(() => {
  const w = Math.min(props.width ?? 440, window.innerWidth - 16)
  const vw = window.innerWidth
  // Open a little apart from each other; apps a little more to the left.
  const x = isApp.value ? (vw - w) / 2 : (vw - w) / 2 - 100
  pos.value = { x: Math.max(8, Math.min(vw - w - 8, x + props.index * 28)), y: 56 + props.index * 24 }
  el.value?.focus()
})

function down(e: PointerEvent) {
  if ((e.target as HTMLElement).closest('button, a') || props.win.max) return
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

const style = computed(() =>
  props.win.max
    ? { zIndex: props.win.z }
    : { left: `${pos.value.x}px`, top: `${pos.value.y}px`, zIndex: props.win.z, width: `${props.width ?? 440}px` },
)

/** Shrinks toward (or grows from) this window's icon in the dock. */
function towardDock(node: Element) {
  const target = document.querySelector(`[data-dock-win="${CSS.escape(props.win.id)}"]`)
  const r = (node as HTMLElement).getBoundingClientRect()
  const t = target?.getBoundingClientRect()
  const x = t ? t.left + t.width / 2 : window.innerWidth / 2
  const y = t ? t.top + t.height / 2 : window.innerHeight
  ;(node as HTMLElement).style.transformOrigin = `${x - r.left}px ${y - r.top}px`
}
</script>

<template>
  <Transition name="genie" @before-leave="towardDock" @before-enter="towardDock">
    <section
      v-show="!win.min"
      ref="el"
      class="win"
      :class="{ max: win.max, full, app: isApp }"
      role="dialog"
      :aria-label="win.title"
      tabindex="-1"
      :style="style"
      @pointerdown="focusWindow(win.id)"
      @keydown.esc="closeWindow(win.id)"
    >
      <header
        class="bar"
        @pointerdown="down"
        @pointermove="move"
        @pointerup="up"
        @pointercancel="up"
        @dblclick="toggleMax(win.id)"
      >
        <WindowDots
          class="dots"
          can-min
          can-max
          :maxed="win.max"
          @close="closeWindow(win.id)"
          @min="toggleMin(win.id)"
          @max="toggleMax(win.id)"
        />
        <h2>{{ win.title }}</h2>
        <a
          v-if="appUrl"
          class="tab"
          :href="appUrl"
          target="_blank"
          rel="noopener"
          :title="strings.openInNewTab"
          :aria-label="strings.openInNewTab"
        >
          <Icon name="external" :size="15" />
        </a>
      </header>
      <div class="body"><slot /></div>
    </section>
  </Transition>
</template>

<style scoped>
.win {
  position: fixed;
  max-width: calc(100vw - 16px);
  max-height: calc(100vh - 168px);
  display: flex;
  flex-direction: column;
  border-radius: 18px;
  background:
    radial-gradient(120% 60% at 100% 0%, rgba(var(--ac-rgb), 0.12), transparent 55%),
    rgba(var(--surface-rgb), 0.92);
  backdrop-filter: blur(24px);
  -webkit-backdrop-filter: blur(24px);
  border: 1px solid var(--line);
  box-shadow: 0 30px 90px rgba(0, 0, 0, 0.7);
  outline: none;
}
.win.full {
  height: calc(100vh - 168px);
}
.win.max {
  left: 10px;
  right: 10px;
  top: 46px;
  bottom: 96px;
  width: auto;
  height: auto;
  max-width: none;
  max-height: none;
}
.win:focus-visible {
  box-shadow: 0 0 0 2px var(--ac2), 0 30px 90px rgba(0, 0, 0, 0.7);
}
.bar {
  position: relative;
  flex: none;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 36px;
  padding: 0 12px;
  border-bottom: 1px solid var(--line);
  cursor: grab;
  touch-action: none;
  user-select: none;
}
.max .bar {
  cursor: default;
}
.bar:active {
  cursor: grabbing;
}
.dots {
  position: absolute;
  left: 13px;
}
h2 {
  margin: 0;
  max-width: calc(100% - 160px);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--font-mono);
  font-size: 11.5px;
  font-weight: 600;
  color: var(--mut);
  letter-spacing: 0.04em;
}
.tab {
  position: absolute;
  right: 10px;
  display: grid;
  place-items: center;
  width: 26px;
  height: 26px;
  border-radius: 7px;
  color: var(--mut);
}
.tab:hover {
  background: rgba(var(--fg-rgb), 0.08);
  color: var(--text);
}
.body {
  flex: 1;
  min-height: 0;
  padding: 16px 18px 18px;
  overflow-y: auto;
}
.full .body {
  padding: 0;
  overflow: hidden;
}

/* Minimize: shrink into the dock icon (transform-origin set in towardDock). */
.genie-leave-active,
.genie-enter-active {
  transition:
    transform 0.34s cubic-bezier(0.5, 0, 0.3, 1),
    opacity 0.34s ease;
}
.genie-leave-to,
.genie-enter-from {
  transform: scale(0.04);
  opacity: 0;
}
@media (prefers-reduced-motion: reduce) {
  .genie-leave-active,
  .genie-enter-active {
    transition: none;
  }
}
</style>
