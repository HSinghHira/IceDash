<script setup lang="ts">
// One widget on the panel (PLAN.md, Phase 4, "Widgets"). It runs in an
// iframe with sandbox="allow-scripts": no same-origin, so it can't touch
// IceDash's page, sign-in or storage. It hears from IceDash only by
// postMessage, and only what it was granted.
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { widgetUrl, type Installed } from '../api/mods'
import { ICE } from '../lib/theme'
import { cityOf } from '../lib/timezone'
import { activeTheme, apps, sys, util, zone } from '../store'

const props = defineProps<{ mod: Installed }>()
const frame = ref<HTMLIFrameElement | null>(null)
const height = computed(() => ({ small: 132, medium: 220, large: 340 })[props.mod.size ?? 'small'])
const grants = computed(() => props.mod.permissions ?? {})
let ready = false

function post(icedash: string, data: unknown) {
  // The sandbox's origin is opaque, so "*": what goes out is only what the
  // widget was granted, and it goes to this frame's window alone.
  // A plain copy: Vue's reactive objects can't be sent.
  frame.value?.contentWindow?.postMessage({ icedash, data: data === undefined ? null : JSON.parse(JSON.stringify(data)) }, '*')
}

function sendTheme() {
  const t = activeTheme.value?.theme ?? ICE
  post('theme', { scheme: t.scheme, colors: t.colors })
}
function sendSystem() {
  if (!grants.value.system || !util.value) return
  const u = util.value
  post('system', {
    cpu: { percent: u.cpu.percent, cores: u.cpu.num },
    memory: { used: u.mem.total - u.mem.available, total: u.mem.total },
    storage: { used: u.sys_disk.used, total: u.sys_disk.size },
    network: { down: sys.value?.down ?? 0, up: sys.value?.up ?? 0 },
    uptime: sys.value?.uptime_sec ?? 0,
  })
}
function sendApps() {
  if (!grants.value.apps) return
  post('apps', apps.value.map((a) => ({ name: a.name, icon: a.icon ?? '', status: a.status })))
}

function onMessage(e: MessageEvent) {
  if (!frame.value || e.source !== frame.value.contentWindow) return
  const m = e.data as { icedash?: string }
  if (m?.icedash === 'hello') {
    ready = true
    sendTheme()
    sendSystem()
    sendApps()
  } else if (m?.icedash === 'location') {
    post('location', grants.value.location ? { city: cityOf(zone.value), zone: zone.value } : null)
  }
}

onMounted(() => window.addEventListener('message', onMessage))
onBeforeUnmount(() => window.removeEventListener('message', onMessage))
watch(activeTheme, () => ready && sendTheme())
watch([util, sys], () => ready && sendSystem())
watch(apps, () => ready && sendApps(), { deep: true })
</script>

<template>
  <iframe
    ref="frame"
    class="widget"
    :src="widgetUrl(mod)"
    :title="mod.name"
    :style="{ height: `${height}px` }"
    sandbox="allow-scripts"
    referrerpolicy="no-referrer"
    loading="lazy"
  />
</template>

<style scoped>
.widget {
  display: block;
  width: 100%;
  border: 0;
  background: transparent;
  /* No color-scheme of its own: it follows the page, as the widget's does,
     so the browser keeps the frame see-through. */
}
</style>
