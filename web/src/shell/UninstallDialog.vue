<script setup lang="ts">
// Uninstall asks first, and says what happens to the app's data folder.
import { computed, ref } from 'vue'
import * as api from '../api/apps'
import AppIcon from '../components/AppIcon.vue'
import { host } from '../lib/host'
import Modal from '../components/Modal.vue'
import { apps, refreshApps, toast, uninstallFor } from '../store'
import { strings } from '../strings'

const props = defineProps<{ name: string }>()
const app = computed(() => apps.value.find((a) => a.id === props.name))
const title = computed(() => app.value?.name ?? props.name)
const dir = `/DATA/AppData/${props.name}`
const deleteData = ref(false)
const busy = ref(false)
const error = ref('')

async function go() {
  busy.value = true
  error.value = ''
  try {
    const r = await api.uninstallApp(props.name, deleteData.value)
    toast({ title: strings.uninstalled(title.value), body: r.data_kept ? strings.dataKept(r.data) : strings.dataDeleted })
    uninstallFor.value = null
    setTimeout(refreshApps, 2000)
    setTimeout(refreshApps, 8000)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <Modal :label="strings.uninstallTitle(title)" :width="440" @close="uninstallFor = null">
    <div class="un">
      <header>
        <AppIcon :src="app?.icon" :name="title" :size="44" />
        <h2>{{ strings.uninstallTitle(title) }}</h2>
      </header>
      <p class="mut">{{ strings.uninstallBody(host.name) }} {{ deleteData ? '' : strings.dataKept(dir) }}</p>
      <label class="check"><input v-model="deleteData" type="checkbox" /> {{ strings.deleteData(dir) }}</label>
      <p v-if="error" class="err" role="alert">{{ error }}</p>
      <footer>
        <button type="button" class="btn ghost" @click="uninstallFor = null">{{ strings.cancel }}</button>
        <button type="button" class="btn danger" :disabled="busy" @click="go">{{ strings.uninstall }}</button>
      </footer>
    </div>
  </Modal>
</template>

<style scoped>
.un {
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
.mut {
  margin: 0;
  color: var(--mut);
  line-height: 1.5;
}
.check {
  display: flex;
  align-items: center;
  gap: 10px;
}
.check input {
  width: 18px;
  height: 18px;
  accent-color: #ff7d7d;
}
.err {
  margin: 0;
  color: #ff8a8a;
}
footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
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
  background: rgba(255, 255, 255, 0.04);
  color: var(--text);
}
.danger {
  border: 0;
  background: #ff6b6b;
  color: #1a0505;
}
.btn:disabled {
  opacity: 0.5;
}
</style>
