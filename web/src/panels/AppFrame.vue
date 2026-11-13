<script setup lang="ts">
// An app inside an IceDash window. Apps that refuse to be framed are sent to
// a new tab before this opens (store.launchApp); this is the rest.
import { computed, ref } from 'vue'
import { apps } from '../store'
import { strings } from '../strings'

const props = defineProps<{ name: string }>()
const app = computed(() => apps.value.find((a) => a.id === props.name))
const loaded = ref(false)
const frame = ref<HTMLIFrameElement | null>(null)

/** Reloads the app's page. */
function reload() {
  loaded.value = false
  if (frame.value && app.value?.url) frame.value.src = app.value.url
}
defineExpose({ reload })
</script>

<template>
  <div class="frame">
    <p v-if="!loaded" class="loading">{{ strings.loading }}</p>
    <iframe
      v-if="app?.url"
      ref="frame"
      :src="app.url"
      :title="app.name"
      allow="clipboard-read; clipboard-write; fullscreen; camera; microphone"
      referrerpolicy="no-referrer"
      @load="loaded = true"
    />
    <p v-else class="loading">{{ strings.noAddress }}</p>
  </div>
</template>

<style scoped>
.frame {
  position: relative;
  height: 100%;
  min-height: 200px;
  background: #fff;
  border-radius: 0 0 17px 17px;
  overflow: hidden;
}
iframe {
  display: block;
  width: 100%;
  height: 100%;
  border: 0;
}
.loading {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  margin: 0;
  background: rgba(12, 16, 20, 0.96);
  color: var(--mut);
  font-size: 13px;
  pointer-events: none;
}
</style>
