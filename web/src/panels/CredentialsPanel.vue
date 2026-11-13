<script setup lang="ts">
// Mock-up screen 3: an IceStore app's generated login, read live from the
// running container each time it opens. Admin only; hides itself after 5 minutes.
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import * as api from '../api/apps'
import { ApiError } from '../api/client'
import AppIcon from '../components/AppIcon.vue'
import Icon from '../components/Icon.vue'
import { copyText } from '../lib/copy'
import { apps, closeWindow } from '../store'
import { strings } from '../strings'

const props = defineProps<{ name: string; winId: string }>()
const HIDE_AFTER = 5 * 60

const app = computed(() => apps.value.find((a) => a.id === props.name))
const creds = ref<api.Creds | null>(null)
const problem = ref('')
const left = ref(HIDE_AFTER)
const shown = ref<Record<number, boolean>>({})
const copied = ref<number | null>(null)

/** Grouped fields (an address and its own login) as sections, in file order. */
const groups = computed(() => {
  const out: { name: string; fields: api.Field[] }[] = []
  for (const f of creds.value?.fields ?? []) {
    if (!f.group) continue
    let g = out.find((x) => x.name === f.group)
    if (!g) out.push((g = { name: f.group, fields: [] }))
    g.fields.push(f)
  }
  return out
})
const loose = computed(() => creds.value?.fields.filter((f) => !f.group) ?? [])
const countdown = computed(() => `${Math.floor(left.value / 60)}:${String(left.value % 60).padStart(2, '0')}`)

let tick: ReturnType<typeof setInterval>
onMounted(async () => {
  tick = setInterval(() => {
    left.value--
    if (left.value <= 0) closeWindow(props.winId)
  }, 1000)
  try {
    creds.value = await api.credentials(props.name)
  } catch (e) {
    if (e instanceof ApiError && e.code === 'no_credentials') problem.value = strings.noCredentials
    else if (e instanceof ApiError && e.code === 'not_running') problem.value = strings.notRunning
    else problem.value = e instanceof Error ? e.message : String(e)
  }
})
onBeforeUnmount(() => {
  clearInterval(tick)
  creds.value = null // drop the values with the window
})

async function copy(i: number, v: string) {
  if (await copyText(v)) {
    copied.value = i
    setTimeout(() => (copied.value = copied.value === i ? null : copied.value), 1500)
  }
}
const idx = (f: api.Field) => creds.value?.fields.indexOf(f) ?? -1
</script>

<template>
  <div class="creds">
    <header class="head">
      <AppIcon :src="app?.icon" :name="app?.name ?? name" :size="44" />
      <div>
        <p class="kicker">{{ strings.credentialsKicker }}</p>
        <h3>{{ app?.name ?? name }}</h3>
      </div>
    </header>
    <div class="chips">
      <span class="chip ok"><Icon name="shield" :size="14" /> {{ strings.verifiedAdmin }}</span>
      <span class="chip mono">{{ strings.hidesIn(countdown) }}</span>
    </div>

    <p v-if="!creds && !problem" class="mut">{{ strings.loading }}</p>
    <p v-if="problem" class="note">
      <Icon name="info" :size="18" /> <span>{{ problem }}</span>
    </p>

    <template v-if="creds">
      <section v-for="g in groups" :key="g.name" class="group">
        <h4 class="kicker mut">{{ g.name }}</h4>
        <template v-for="f in g.fields" :key="idx(f)">
          <div v-if="f.url" class="box">
            <div class="val">
              <span class="lbl">{{ strings.address }}</span>
              <span class="mono">{{ f.value }}</span>
            </div>
            <button type="button" class="sq" :aria-label="`${strings.copy} ${g.name}`" @click="copy(idx(f), f.value)">
              <Icon :name="copied === idx(f) ? 'check' : 'copy'" :size="17" />
            </button>
            <a class="sq" :href="f.value" target="_blank" rel="noopener" :aria-label="`${strings.openLink} ${g.name}`">
              <Icon name="external" :size="17" />
            </a>
          </div>
        </template>
        <div v-if="g.fields.some((f) => !f.url)" class="grid">
          <div v-for="f in g.fields.filter((x) => !x.url)" :key="idx(f)" class="box">
            <div class="val">
              <span class="lbl">{{ f.label }}</span>
              <span class="mono">{{ f.secret && !shown[idx(f)] ? '•'.repeat(Math.min(16, Math.max(8, f.value.length))) : f.value }}</span>
            </div>
            <button
              v-if="f.secret"
              type="button"
              class="sq"
              :aria-label="shown[idx(f)] ? strings.hideValue : strings.showValue"
              @click="shown[idx(f)] = !shown[idx(f)]"
            >
              <Icon :name="shown[idx(f)] ? 'eyeOff' : 'eye'" :size="17" />
            </button>
            <button type="button" class="sq" :aria-label="`${strings.copy} ${f.label}`" @click="copy(idx(f), f.value)">
              <Icon :name="copied === idx(f) ? 'check' : 'copy'" :size="17" />
            </button>
          </div>
        </div>
      </section>

      <div v-if="loose.length" class="grid">
        <div v-for="f in loose" :key="idx(f)" class="box">
          <div class="val">
            <span class="lbl">{{ f.label }}</span>
            <span class="mono">{{ f.secret && !shown[idx(f)] ? '•'.repeat(Math.min(16, Math.max(8, f.value.length))) : f.value }}</span>
          </div>
          <button
            v-if="f.secret"
            type="button"
            class="sq"
            :aria-label="shown[idx(f)] ? strings.hideValue : strings.showValue"
            @click="shown[idx(f)] = !shown[idx(f)]"
          >
            <Icon :name="shown[idx(f)] ? 'eyeOff' : 'eye'" :size="17" />
          </button>
          <button type="button" class="sq" :aria-label="`${strings.copy} ${f.label}`" @click="copy(idx(f), f.value)">
            <Icon :name="copied === idx(f) ? 'check' : 'copy'" :size="17" />
          </button>
        </div>
      </div>

      <p v-for="(n, i) in creds.notes" :key="i" class="note"><Icon name="info" :size="18" /> <span>{{ n }}</span></p>
    </template>

    <p class="foot mut">{{ strings.readLive }}</p>
  </div>
</template>

<style scoped>
.creds {
  display: grid;
  gap: 12px;
}
.head {
  display: flex;
  align-items: center;
  gap: 12px;
}
.kicker {
  margin: 0 0 2px;
  font-family: var(--font-mono);
  font-size: 10.5px;
  font-weight: 700;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--ac2);
}
h3 {
  margin: 0;
  font-size: 19px;
}
h4.kicker {
  margin: 0 0 2px;
  color: var(--mut);
}
.chips {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}
.chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 10px;
  border-radius: var(--radius-pill);
  border: 1px solid var(--line);
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--mut);
}
.chip.ok {
  border-color: rgba(76, 201, 240, 0.45);
  background: rgba(76, 201, 240, 0.1);
  color: var(--ac2);
}
.box {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 8px 8px 13px;
  border-radius: 12px;
  border: 1px solid var(--line);
  background: rgba(255, 255, 255, 0.03);
  min-width: 0;
}
.val {
  flex: 1;
  display: grid;
  gap: 3px;
  min-width: 0;
}
.lbl {
  color: var(--mut);
  font-size: 12px;
}
.val .mono {
  font-family: var(--font-mono);
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sq {
  flex: none;
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border-radius: 9px;
  border: 1px solid var(--line);
  background: rgba(255, 255, 255, 0.04);
  color: var(--text);
  cursor: pointer;
}
.sq:hover {
  background: rgba(255, 255, 255, 0.09);
}
.group {
  display: grid;
  gap: 7px;
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
  gap: 8px;
}
.note {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  margin: 0;
  padding: 10px 13px;
  border-radius: 12px;
  background: rgba(76, 201, 240, 0.08);
  color: rgba(255, 255, 255, 0.9);
  font-size: 13px;
  line-height: 1.45;
}
.note :deep(svg) {
  flex: none;
  margin-top: 2px;
  color: var(--ac2);
}
.foot {
  margin: 2px 0 0;
  font-size: 12px;
}
.mut {
  color: var(--mut);
}
.mono {
  font-family: var(--font-mono);
}
</style>
