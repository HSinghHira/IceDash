<script setup lang="ts">
// A centred popup over a dimmed desktop (mock-ups 2 and 4): Escape or the
// backdrop closes it, and focus moves into it.
import { nextTick, onMounted, ref } from 'vue'

const props = withDefaults(defineProps<{ label: string; width?: number; closeOnBackdrop?: boolean }>(), {
  width: 640,
  closeOnBackdrop: true,
})
const emit = defineEmits<{ close: [] }>()
const card = ref<HTMLElement | null>(null)

onMounted(async () => {
  await nextTick()
  const first =
    card.value?.querySelector<HTMLElement>('[data-autofocus]') ??
    card.value?.querySelector<HTMLElement>('input, button, a[href], textarea')
  ;(first ?? card.value)?.focus()
})

function backdrop(e: MouseEvent) {
  if (props.closeOnBackdrop && e.target === e.currentTarget) emit('close')
}
</script>

<template>
  <div class="overlay" @mousedown="backdrop" @keydown.esc.stop="emit('close')">
    <section ref="card" class="modal" role="dialog" aria-modal="true" :aria-label="label" tabindex="-1" :style="{ width: `${width}px` }">
      <slot />
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
  max-width: 100%;
  max-height: calc(100vh - 32px);
  overflow-y: auto;
  padding: 30px 32px 28px;
  border-radius: 28px;
  background:
    radial-gradient(110% 55% at 100% 0%, rgba(76, 201, 240, 0.12), transparent 55%),
    rgba(12, 16, 20, 0.96);
  border: 1px solid var(--line);
  box-shadow: 0 40px 120px rgba(0, 0, 0, 0.75);
  outline: none;
}
@media (max-width: 600px) {
  .modal {
    padding: 22px 18px;
    border-radius: 22px;
  }
}
</style>
