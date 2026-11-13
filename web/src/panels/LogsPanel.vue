<script setup lang="ts">
// An app's last 500 log lines, refreshed every 3 s while Follow is on.
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import * as api from '../api/apps'
import Icon from '../components/Icon.vue'
import { copyText } from '../lib/copy'
import { strings } from '../strings'

const props = defineProps<{ name: string }>()
const text = ref('')
const error = ref('')
const follow = ref(true)
const loaded = ref(false)
const box = ref<HTMLElement | null>(null)
const copied = ref(false)

async function load() {
  try {
    const r = await api.logs(props.name, 500)
    const atEnd = !box.value || box.value.scrollHeight - box.value.scrollTop - box.value.clientHeight < 40
    text.value = r.text
    error.value = ''
    await nextTick()
    if (follow.value && atEnd && box.value) box.value.scrollTop = box.value.scrollHeight
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loaded.value = true
  }
}

let timer: ReturnType<typeof setInterval>
onMounted(async () => {
  await load()
  if (box.value) box.value.scrollTop = box.value.scrollHeight
  timer = setInterval(() => follow.value && !document.hidden && load(), 3000)
})
onBeforeUnmount(() => clearInterval(timer))

async function copyAll() {
  copied.value = await copyText(text.value)
  setTimeout(() => (copied.value = false), 1500)
}
</script>

<template>
  <div class="logs">
    <div class="tools">
      <label class="toggle"><input v-model="follow" type="checkbox" /> {{ strings.follow }}</label>
      <button type="button" class="ghost" @click="load"><Icon name="refresh" :size="15" /> {{ strings.refresh }}</button>
      <button type="button" class="ghost" @click="copyAll">
        <Icon :name="copied ? 'check' : 'copy'" :size="15" /> {{ copied ? strings.copied : strings.copyAll }}
      </button>
    </div>
    <p v-if="error" class="err">{{ error }}</p>
    <pre ref="box" class="out" tabindex="0">{{ loaded ? text || strings.noLogs : strings.loading }}</pre>
  </div>
</template>

<style scoped>
.logs {
  display: grid;
  gap: 12px;
}
.tools {
  display: flex;
  align-items: center;
  gap: 10px;
}
.toggle {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin-right: auto;
  color: var(--mut);
  font-size: 14px;
}
.toggle input {
  accent-color: var(--ac);
}
.ghost {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 7px 14px;
  border-radius: var(--radius-pill);
  border: 1px solid var(--line);
  background: rgba(255, 255, 255, 0.04);
  color: var(--text);
  font: inherit;
  font-size: 13px;
  cursor: pointer;
}
.out {
  height: min(50vh, 460px);
  margin: 0;
  padding: 14px 16px;
  overflow: auto;
  border-radius: 14px;
  border: 1px solid var(--line);
  background: rgba(0, 0, 0, 0.5);
  font-family: var(--font-mono);
  font-size: 12.5px;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-word;
  color: rgba(255, 255, 255, 0.85);
}
.err {
  margin: 0;
  color: #ff8a8a;
}
</style>
