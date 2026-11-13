<script setup lang="ts">
// The three window dots: close, minimize, maximize. A dot without a handler
// is shown dimmed and does nothing (a popup can't be minimized).
import { strings } from '../strings'

defineProps<{ canMin?: boolean; canMax?: boolean; maxed?: boolean }>()
const emit = defineEmits<{ close: []; min: []; max: [] }>()
</script>

<template>
  <span class="dots">
    <button type="button" class="dot close" :aria-label="strings.close" :title="strings.close" @click.stop="emit('close')">
      <svg viewBox="0 0 10 10" aria-hidden="true"><path d="M2.5 2.5l5 5M7.5 2.5l-5 5" /></svg>
    </button>
    <button
      type="button"
      class="dot min"
      :disabled="!canMin"
      :aria-label="strings.minimize"
      :title="canMin ? strings.minimize : undefined"
      @click.stop="emit('min')"
    >
      <svg viewBox="0 0 10 10" aria-hidden="true"><path d="M2 5h6" /></svg>
    </button>
    <button
      type="button"
      class="dot max"
      :disabled="!canMax"
      :aria-label="maxed ? strings.restoreSize : strings.maximize"
      :title="canMax ? (maxed ? strings.restoreSize : strings.maximize) : undefined"
      @click.stop="emit('max')"
    >
      <svg viewBox="0 0 10 10" aria-hidden="true"><path d="M5 2v6M2 5h6" /></svg>
    </button>
  </span>
</template>

<style scoped>
.dots {
  display: inline-flex;
  gap: 7px;
}
.dot {
  display: grid;
  place-items: center;
  width: 13px;
  height: 13px;
  padding: 0;
  border: 0;
  border-radius: 50%;
  cursor: pointer;
}
.dot svg {
  width: 9px;
  height: 9px;
  fill: none;
  stroke: rgba(0, 0, 0, 0.65);
  stroke-width: 1.6;
  stroke-linecap: round;
  opacity: 0;
}
.dots:hover .dot:not(:disabled) svg,
.dot:focus-visible svg {
  opacity: 1;
}
.close {
  background: var(--dot-close);
}
.min {
  background: var(--dot-min);
}
.max {
  background: var(--dot-max);
}
.dot:disabled {
  background: rgba(255, 255, 255, 0.18);
  cursor: default;
}
</style>
