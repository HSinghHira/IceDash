<script setup lang="ts">
// "Go back to <version>" asks first, and says plainly that the app's data
// doesn't go back with it. Built like the uninstall popup.
import { computed, ref } from 'vue'
import * as api from '../api/apps'
import { ApiError } from '../api/client'
import AppIcon from '../components/AppIcon.vue'
import Icon from '../components/Icon.vue'
import Modal from '../components/Modal.vue'
import { appInfo, apps, goBackFor, toast, trackJob } from '../store'
import { strings } from '../strings'

const props = defineProps<{ name: string }>()
const app = computed(() => apps.value.find((a) => a.id === props.name))
const info = computed(() => appInfo.value[props.name])
const title = computed(() => app.value?.name ?? props.name)
const to = computed(() => info.value?.back ?? '')
const from = computed(() => info.value?.version ?? '')
const busy = ref(false)
const error = ref('')
/** The server said the app keeps no data folder: no backup to offer. */
const noData = ref(false)

const close = () => (goBackFor.value = null)

async function backup() {
  toast({ title: strings.backingUp(title.value), body: strings.backupHint })
  try {
    await api.downloadBackup(props.name)
    toast({ title: strings.backUpData, body: strings.backupDone(title.value) })
  } catch (e) {
    if (e instanceof ApiError && e.code === 'no_data') noData.value = true
    toast({ title: strings.backUpData, body: e instanceof Error ? e.message : String(e) })
  }
}

async function go() {
  busy.value = true
  error.value = ''
  try {
    trackJob(await api.rollbackApp(props.name))
    toast({ title: strings.goingBack, body: title.value })
    close()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <Modal :label="strings.goBackTitle(title, to)" :width="440" @close="close">
    <div class="gb">
      <header>
        <AppIcon :src="app?.icon" :name="title" :size="44" />
        <h2>{{ strings.goBackTitle(title, to) }}</h2>
      </header>
      <p class="mut">{{ strings.goBackBody(title, to) }}</p>
      <p class="warnbox"><Icon name="alert" :size="16" />{{ strings.goBackData }}</p>
      <p v-if="from" class="mut">{{ strings.goBackHold(from) }}</p>
      <p v-if="error" class="err" role="alert">{{ error }}</p>
      <footer>
        <button v-if="!noData" type="button" class="btn ghost" @click="backup">{{ strings.backUpData }}</button>
        <span class="end">
          <button type="button" class="btn ghost" data-autofocus @click="close">{{ strings.cancel }}</button>
          <button type="button" class="btn primary" :disabled="busy || !to" @click="go">{{ strings.goBackTo(to) }}</button>
        </span>
      </footer>
    </div>
  </Modal>
</template>

<style scoped>
.gb {
  display: grid;
  gap: 16px;
}
header {
  display: flex;
  align-items: center;
  gap: 16px;
}
h2 {
  margin: 0;
  font-size: 18px;
}
p {
  margin: 0;
  line-height: 1.5;
}
.mut {
  color: var(--mut);
}
.warnbox {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 12px;
  border: 1px solid var(--warn);
  color: var(--text);
  font-size: 13.5px;
}
.warnbox :deep(svg) {
  flex: none;
  margin-top: 2px;
  color: var(--warn);
}
/* Theme text on a red edge: reads in Ice (dark) and Frost (light). */
.err {
  padding-left: 10px;
  border-left: 3px solid var(--danger);
  color: var(--text);
}
footer {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
/* Cancel and Go back stay together on the right, also when they wrap. */
.end {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 12px;
  margin-left: auto;
}
.btn {
  padding: 8px 20px;
  border-radius: var(--radius-pill);
  font: inherit;
  font-weight: 600;
  cursor: pointer;
}
.ghost {
  border: 1px solid var(--line);
  background: rgba(var(--fg-rgb), 0.04);
  color: var(--text);
}
.primary {
  border: 0;
  background: var(--ac);
  color: var(--ink);
}
.btn:disabled {
  opacity: 0.5;
}
</style>
