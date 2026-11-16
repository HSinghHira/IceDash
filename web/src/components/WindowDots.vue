<script setup lang="ts">
// The window buttons: close, minimize, maximize. A button without a handler
// is shown dimmed and does nothing (a popup can't be minimized).
//
// Two looks (Settings → General, after the release, 2026-11-16): the
// coloured dots on the left, each with a 22 px target around its 13 px dot
// and its sign always shown; or "classic", − □ × on the right, full height,
// × red on hover. The parent places the root; with "classic" it gets the
// class .classic so the parent can put it on the right.
import { computed } from 'vue'
import { prefs } from '../store'
import { strings } from '../strings'

defineProps<{ canMin?: boolean; canMax?: boolean; maxed?: boolean }>()
const emit = defineEmits<{ close: []; min: []; max: [] }>()
const classic = computed(() => prefs.value.window_buttons === 'classic')
</script>

<template>
  <span v-if="classic" class="classic">
    <button
      v-if="canMin"
      type="button"
      class="cb"
      :aria-label="strings.minimize"
      :title="strings.minimize"
      @click.stop="emit('min')"
     
      @dblclick.stop
    >
      <svg viewBox="0 0 12 12" aria-hidden="true"><path d="M2 6.5h8" /></svg>
    </button>
    <button
      v-if="canMax"
      type="button"
      class="cb"
      :aria-label="maxed ? strings.restoreSize : strings.maximize"
      :title="maxed ? strings.restoreSize : strings.maximize"
      @click.stop="emit('max')"
     
      @dblclick.stop
    >
      <svg v-if="maxed" viewBox="0 0 12 12" aria-hidden="true"><path d="M2.5 4.5h5v5h-5zM4.5 4.5V2.5h5v5h-2" /></svg>
      <svg v-else viewBox="0 0 12 12" aria-hidden="true"><path d="M2.5 2.5h7v7h-7z" /></svg>
    </button>
    <button type="button" class="cb x" :aria-label="strings.close" :title="strings.close" @click.stop="emit('close')" @dblclick.stop>
      <svg viewBox="0 0 12 12" aria-hidden="true"><path d="M2.5 2.5l7 7M9.5 2.5l-7 7" /></svg>
    </button>
  </span>
  <span v-else class="dots">
    <button
      type="button"
      class="dot close"
      :aria-label="strings.close"
      :title="strings.close"
      @click.stop="emit('close')"
     
      @dblclick.stop
    >
      <span class="ball"><svg viewBox="0 0 10 10" aria-hidden="true"><path d="M2.5 2.5l5 5M7.5 2.5l-5 5" /></svg></span>
    </button>
    <button
      type="button"
      class="dot min"
      :disabled="!canMin"
      :aria-label="strings.minimize"
      :title="canMin ? strings.minimize : undefined"
      @click.stop="emit('min')"
     
      @dblclick.stop
    >
      <span class="ball"><svg viewBox="0 0 10 10" aria-hidden="true"><path d="M2 5h6" /></svg></span>
    </button>
    <button
      type="button"
      class="dot max"
      :disabled="!canMax"
      :aria-label="maxed ? strings.restoreSize : strings.maximize"
      :title="canMax ? (maxed ? strings.restoreSize : strings.maximize) : undefined"
      @click.stop="emit('max')"
     
      @dblclick.stop
    >
      <span class="ball"><svg viewBox="0 0 10 10" aria-hidden="true"><path d="M5 2v6M2 5h6" /></svg></span>
    </button>
  </span>
</template>

<style scoped>
/* Dots: a 22 px target each, the 13 px dot in its middle. The negative
   margin keeps the dots where the parent put them before. */
.dots {
  display: inline-flex;
  margin: -4.5px;
}
.dot {
  display: grid;
  place-items: center;
  width: 22px;
  height: 22px;
  padding: 0;
  border: 0;
  border-radius: 50%;
  background: none;
  cursor: pointer;
}
.ball {
  display: grid;
  place-items: center;
  width: 13px;
  height: 13px;
  border-radius: 50%;
  transition: transform 0.12s;
}
.dot:not(:disabled):hover .ball {
  transform: scale(1.15);
}
.dot svg {
  width: 9px;
  height: 9px;
  fill: none;
  stroke: rgba(0, 0, 0, 0.6);
  stroke-width: 1.6;
  stroke-linecap: round;
  opacity: 0.55;
}
.dot:not(:disabled):hover svg,
.dot:focus-visible svg {
  opacity: 1;
}
.dot:focus-visible {
  outline: 2px solid var(--ac2);
  outline-offset: -2px;
}
.close .ball {
  background: var(--dot-close);
}
.min .ball {
  background: var(--dot-min);
}
.max .ball {
  background: var(--dot-max);
}
.dot:disabled {
  cursor: default;
}
.dot:disabled .ball {
  background: rgba(var(--fg-rgb), 0.18);
}
.dot:disabled svg {
  opacity: 0;
}

/* Classic: − □ × on the right, each 40 × 32. */
.classic {
  display: inline-flex;
  align-items: stretch;
}
.cb {
  display: grid;
  place-items: center;
  width: 40px;
  height: 32px;
  padding: 0;
  border: 0;
  border-radius: 8px;
  background: none;
  color: var(--text);
  cursor: pointer;
}
.cb svg {
  width: 12px;
  height: 12px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.3;
  stroke-linejoin: round;
  stroke-linecap: round;
}
.cb:hover {
  background: rgba(var(--fg-rgb), 0.1);
}
.cb.x:hover {
  background: #e81123;
  color: #fff;
}
.cb:focus-visible {
  outline: 2px solid var(--ac2);
  outline-offset: -2px;
}
</style>
