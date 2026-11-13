<script setup lang="ts">
// A centred popup over a dimmed desktop (mock-ups 2 and 4). Its dots close it
// or widen it (a popup can't be minimized, so that dot is dimmed); Escape and
// the backdrop close it too. Focus moves into it.
import { nextTick, onMounted, ref } from 'vue'
import WindowDots from './WindowDots.vue'

const props = withDefaults(defineProps<{ label: string; width?: number; closeOnBackdrop?: boolean }>(), {
  width: 560,
  closeOnBackdrop: true,
})
const emit = defineEmits<{ close: [] }>()
const card = ref<HTMLElement | null>(null)
const wide = ref(false)

onMounted(async () => {
  await nextTick()
  const first =
    card.value?.querySelector<HTMLElement>('[data-autofocus]') ??
    card.value?.querySelector<HTMLElement>('.content input, .content button, .content a[href], .content textarea')
  ;(first ?? card.value)?.focus()
})

function backdrop(e: MouseEvent) {
  if (props.closeOnBackdrop && e.target === e.currentTarget) emit('close')
}
</script>

<template>
  <div class="overlay" @mousedown="backdrop" @keydown.esc.stop="emit('close')">
    <section
      ref="card"
      class="modal"
      role="dialog"
      aria-modal="true"
      :aria-label="label"
      tabindex="-1"
      :style="{ width: wide ? 'min(1000px, 100%)' : `${width}px` }"
    >
      <WindowDots class="dots" can-max :maxed="wide" @close="emit('close')" @max="wide = !wide" />
      <div class="content"><slot /></div>
    </section>
  </div>
</template>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  z-index: 300;
  display: grid;
  place-items: center;
  padding: 16px;
  background: rgba(0, 0, 0, 0.55);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
}
.modal {
  position: relative;
  max-width: 100%;
  max-height: calc(100vh - 32px);
  overflow-y: auto;
  padding: 38px 24px 20px;
  border-radius: 20px;
  background:
    radial-gradient(110% 55% at 100% 0%, rgba(76, 201, 240, 0.12), transparent 55%),
    rgba(12, 16, 20, 0.96);
  border: 1px solid var(--line);
  box-shadow: 0 40px 120px rgba(0, 0, 0, 0.75);
  outline: none;
  transition: width 0.15s ease;
}
.dots {
  position: absolute;
  top: 14px;
  left: 16px;
}
@media (max-width: 600px) {
  .modal {
    padding: 36px 16px 18px;
    border-radius: 18px;
  }
}
</style>
