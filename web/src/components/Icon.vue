<script setup lang="ts">
import { icons, type IconName } from '../lib/icons'

withDefaults(defineProps<{ name: IconName; size?: number }>(), { size: 20 })
</script>

<template>
  <svg
    class="icon"
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="2"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
    focusable="false"
  >
    <path :d="icons[name]" />
  </svg>
</template>

<style scoped>
.icon {
  flex: none;
  display: block;
}
</style>
