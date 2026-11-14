<script setup lang="ts">
// Someone's avatar: their own picture, else the drawn one they chose
// (lib/avatars), else their initials while it draws or if they chose letters.
import { ref, watchEffect } from 'vue'
import { drawAvatar, styleFor } from '../lib/avatars'
import { initials } from '../lib/format'

const props = defineProps<{ name: string; src?: string; avatarStyle?: string; seed?: string }>()
const drawn = ref('')
watchEffect(async () => {
  if (props.src) return
  const style = styleFor(props.avatarStyle)
  const seed = props.seed || props.name
  const uri = await drawAvatar(style, seed)
  // Only if still asked for the same one.
  if (styleFor(props.avatarStyle) === style && (props.seed || props.name) === seed) drawn.value = uri
})
</script>

<template>
  <img v-if="src || drawn" class="ua" :src="src || drawn" alt="" draggable="false" />
  <template v-else>{{ initials(name) }}</template>
</template>

<style scoped>
.ua {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
</style>
