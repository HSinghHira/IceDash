<script setup lang="ts">
// Placing a new avatar: the picture under a round frame, dragged to move,
// zoomed with the slider, the wheel or + and −, moved with the arrow keys.
// Save crops it to a square (lib/pictures).
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { clampCrop, cropAvatar, decode, type Crop } from '../lib/pictures'
import { strings } from '../strings'

const props = defineProps<{ file: File }>()
const emit = defineEmits<{ save: [blob: Blob]; cancel: [] }>()

const FRAME = 260
const img = ref<ImageBitmap | null>(null)
const src = ref('')
const error = ref('')
const zoom = ref(1)
const at = ref<Crop>({ x: 0, y: 0, w: FRAME, h: FRAME })
const saving = ref(false)

/** The scale at which the picture just covers the frame. */
const base = computed(() => (img.value ? Math.max(FRAME / img.value.width, FRAME / img.value.height) : 1))

function sizeAt(z: number) {
  const i = img.value!
  return { w: i.width * base.value * z, h: i.height * base.value * z }
}

/** Zooms around the frame's middle, keeping the picture covering it. */
function setZoom(z: number) {
  if (!img.value) return
  z = Math.min(5, Math.max(1, z))
  const c = at.value
  const mx = (FRAME / 2 - c.x) / c.w
  const my = (FRAME / 2 - c.y) / c.h
  const s = sizeAt(z)
  zoom.value = z
  at.value = clampCrop({ x: FRAME / 2 - mx * s.w, y: FRAME / 2 - my * s.h, ...s }, FRAME)
}

onMounted(async () => {
  try {
    img.value = await decode(props.file)
    src.value = URL.createObjectURL(props.file)
    const s = sizeAt(1)
    at.value = { x: (FRAME - s.w) / 2, y: (FRAME - s.h) / 2, ...s }
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  }
})
onBeforeUnmount(() => {
  if (src.value) URL.revokeObjectURL(src.value)
  img.value?.close()
})

let drag: { x: number; y: number; cx: number; cy: number } | null = null
function down(e: PointerEvent) {
  drag = { x: e.clientX, y: e.clientY, cx: at.value.x, cy: at.value.y }
  ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
}
function move(e: PointerEvent) {
  if (!drag) return
  at.value = clampCrop({ ...at.value, x: drag.cx + e.clientX - drag.x, y: drag.cy + e.clientY - drag.y }, FRAME)
}
function up() {
  drag = null
}
function wheel(e: WheelEvent) {
  e.preventDefault()
  setZoom(zoom.value * (e.deltaY < 0 ? 1.08 : 1 / 1.08))
}
function key(e: KeyboardEvent) {
  const step = e.shiftKey ? 30 : 8
  const by: Record<string, [number, number]> = { ArrowLeft: [step, 0], ArrowRight: [-step, 0], ArrowUp: [0, step], ArrowDown: [0, -step] }
  if (by[e.key]) {
    e.preventDefault()
    const [dx, dy] = by[e.key]!
    at.value = clampCrop({ ...at.value, x: at.value.x + dx, y: at.value.y + dy }, FRAME)
  } else if (e.key === '+' || e.key === '=') setZoom(zoom.value * 1.1)
  else if (e.key === '-') setZoom(zoom.value / 1.1)
}

async function save() {
  if (!img.value) return
  saving.value = true
  try {
    emit('save', await cropAvatar(img.value, at.value, FRAME))
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="over" @keydown.esc.stop="emit('cancel')">
    <div class="box" role="dialog" :aria-label="strings.avatarEditTitle">
      <h3>{{ strings.avatarEditTitle }}</h3>
      <p class="mut">{{ strings.avatarEditHint }}</p>
      <p v-if="error" class="warn" role="alert">{{ error }}</p>
      <div
        v-else
        class="frame"
        :style="{ width: `${FRAME}px`, height: `${FRAME}px` }"
        tabindex="0"
        data-autofocus
        :aria-label="strings.avatarEditHint"
        @pointerdown="down"
        @pointermove="move"
        @pointerup="up"
        @pointercancel="up"
        @wheel="wheel"
        @keydown="key"
      >
        <img
          v-if="src"
          :src="src"
          alt=""
          draggable="false"
          :style="{ left: `${at.x}px`, top: `${at.y}px`, width: `${at.w}px`, height: `${at.h}px` }"
        />
        <span class="ring" aria-hidden="true" />
      </div>
      <label class="zoom">
        <span class="mut small">{{ strings.avatarZoom }}</span>
        <input type="range" min="1" max="5" step="0.01" :value="zoom" :aria-label="strings.avatarZoom" @input="setZoom(Number(($event.target as HTMLInputElement).value))" />
      </label>
      <div class="row">
        <button type="button" class="ghost" @click="emit('cancel')">{{ strings.cancel }}</button>
        <button type="button" class="primary" :disabled="!img || saving" @click="save">{{ saving ? strings.avatarSaving : strings.avatarSave }}</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.over {
  position: fixed;
  inset: 0;
  z-index: 300;
  display: grid;
  place-items: center;
  padding: 16px;
  background: rgba(0, 0, 0, 0.55);
}
.box {
  display: grid;
  justify-items: center;
  gap: 12px;
  width: min(380px, 100%);
  padding: 22px;
  border-radius: 20px;
  border: 1px solid var(--line);
  background: rgba(var(--surface-rgb), 0.98);
  box-shadow: 0 30px 90px rgba(0, 0, 0, 0.6);
}
h3 {
  margin: 0;
  font-size: 18px;
}
p {
  margin: 0;
  text-align: center;
  font-size: 13.5px;
}
.mut {
  color: var(--mut);
}
.small {
  font-size: 12.5px;
}
.warn {
  color: var(--warn);
}
.frame {
  position: relative;
  overflow: hidden;
  border-radius: 18px;
  background: rgba(var(--fg-rgb), 0.06);
  cursor: grab;
  touch-action: none;
  user-select: none;
}
.frame:active {
  cursor: grabbing;
}
.frame img {
  position: absolute;
  max-width: none;
  pointer-events: none;
}
/* Everything outside the circle is shaded: that part is cut off. */
.ring {
  position: absolute;
  inset: 0;
  border-radius: 50%;
  box-shadow: 0 0 0 999px rgba(0, 0, 0, 0.5);
  outline: 2px solid rgba(255, 255, 255, 0.85);
  outline-offset: -2px;
  pointer-events: none;
}
.zoom {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
}
.zoom input {
  flex: 1;
  accent-color: var(--ac);
}
.row {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  width: 100%;
}
.primary,
.ghost {
  padding: 9px 20px;
  border-radius: var(--radius-pill);
  font: inherit;
  font-weight: 600;
  cursor: pointer;
}
.primary {
  border: 0;
  background: var(--ac);
  color: var(--ink);
}
.primary:disabled {
  opacity: 0.55;
  cursor: default;
}
.ghost {
  border: 1px solid var(--line);
  background: rgba(var(--fg-rgb), 0.04);
  color: var(--text);
}
</style>
