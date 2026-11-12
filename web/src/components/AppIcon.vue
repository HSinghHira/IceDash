<script setup lang="ts">
// An app's icon from ZimaOS, or a letter tile when it has none or it fails to load.
import { ref, watch } from 'vue'

const props = defineProps<{ src?: string; name: string; size?: number }>()
const failed = ref(false)
watch(
  () => props.src,
  () => (failed.value = false),
)
</script>

<template>
  <img
    v-if="src && !failed"
    class="app-img"
    :src="src"
    alt=""
    :width="size ?? 60"
    :height="size ?? 60"
    loading="lazy"
    decoding="async"
    referrerpolicy="no-referrer"
    @error="failed = true"
  />
  <span v-else class="app-letter" :style="{ width: `${size ?? 60}px`, height: `${size ?? 60}px` }" aria-hidden="true">
    {{ name.trim().charAt(0).toUpperCase() || '?' }}
  </span>
</template>

<style scoped>
.app-img {
  display: block;
  object-fit: cover;
  border-radius: var(--radius-tile);
  background: rgba(255, 255, 255, 0.06);
}
.app-letter {
  display: grid;
  place-items: center;
  border-radius: var(--radius-tile);
  background: linear-gradient(145deg, rgba(76, 201, 240, 0.28), rgba(76, 201, 240, 0.08));
  border: 1px solid var(--line);
  color: var(--ac2);
  font-family: var(--font-display);
  font-weight: 800;
  font-size: 22px;
}
</style>
