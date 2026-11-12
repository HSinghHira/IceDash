<script setup lang="ts">
// Mock-up screen 2: "Set up before install". The server's address and what
// IceDash can fill in are already there; the ports are checked with a dry run
// as you type; Install stays off until every placeholder is replaced.
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import * as api from '../api/apps'
import AppIcon from '../components/AppIcon.vue'
import Icon from '../components/Icon.vue'
import Modal from '../components/Modal.vue'
import { inputLabel, placeholderIn } from '../lib/setup'
import { setupFor, toast, trackJob } from '../store'
import { strings } from '../strings'

const props = defineProps<{ id: string }>()

const view = ref<api.SetupView | null>(null)
const loadError = ref('')
const values = reactive<Record<string, string>>({})
const ports = reactive<Record<string, string>>({})
const shown = reactive<Record<string, boolean>>({})
const advancedOpen = ref(false)
const checking = ref(false)
const result = ref<api.CheckResult | null>(null)
const busy = ref(false)
const error = ref('')

onMounted(async () => {
  try {
    const v = await api.setup(props.id)
    for (const i of v.inputs) values[api.inputKey(i)] = i.value
    for (const p of v.ports) ports[api.portKey(p)] = p.published
    view.value = v
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : String(e)
  }
})

const app = computed(() => view.value?.app)
/** The field that holds the app's address (PUBLIC_URL and the like), shown first. */
const addressInput = computed(() =>
  view.value?.inputs.find((i) => i.main && i.fill === 'auto' && /^https?:\/\//.test(i.value)),
)
const mainInputs = computed(() => view.value?.inputs.filter((i) => i.main && i !== addressInput.value) ?? [])
const otherInputs = computed(() => view.value?.inputs.filter((i) => !i.main) ?? [])
const missing = computed(() =>
  (view.value?.inputs ?? []).filter((i) => placeholderIn(values[api.inputKey(i)] ?? '')).map((i) => i.key),
)
const portState = (key: string) => {
  if (checking.value || !result.value) return 'checking'
  return result.value.ports_in_use.includes(ports[key] ?? '') ? 'busy' : 'free'
}
const canInstall = computed(
  () => !!view.value && !busy.value && !checking.value && missing.value.length === 0 && !!result.value?.ok,
)

const choices = (): api.Choices => ({ values: { ...values }, ports: { ...ports } })

let timer: ReturnType<typeof setTimeout> | undefined
let seq = 0
function scheduleCheck() {
  clearTimeout(timer)
  result.value = null
  error.value = ''
  if (!view.value || missing.value.length) return
  checking.value = true
  timer = setTimeout(async () => {
    const mine = ++seq
    try {
      const r = await api.check(props.id, choices())
      if (mine !== seq) return
      result.value = r
      if (r.error) error.value = r.error
    } catch (e) {
      if (mine === seq) error.value = e instanceof Error ? e.message : String(e)
    } finally {
      if (mine === seq) checking.value = false
    }
  }, 450)
}
watch([values, ports, () => view.value], scheduleCheck, { deep: true })
onBeforeUnmount(() => clearTimeout(timer))

function usePort(key: string, p: string) {
  ports[key] = p
}

async function doInstall() {
  if (!canInstall.value || !view.value) return
  busy.value = true
  error.value = ''
  try {
    const j = await api.install(props.id, choices())
    trackJob(j)
    toast({ title: strings.installing, body: view.value.app.title })
    setupFor.value = null
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    busy.value = false
  }
}

const close = () => (setupFor.value = null)
const fillMark = (f?: string) =>
  f === 'auto' ? strings.autoFilled : f === 'account' ? strings.fromAccount : f === 'generated' ? strings.generated : ''
</script>

<template>
  <Modal :label="strings.setUpBeforeInstall" :width="720" @close="close">
    <p v-if="loadError" class="error" role="alert">{{ loadError }}</p>
    <p v-else-if="!view" class="mut">{{ strings.loading }}</p>
    <form v-else class="setup" @submit.prevent="doInstall">
      <header class="head">
        <AppIcon :src="app?.icon" :name="app?.title ?? ''" :size="66" />
        <div class="titles">
          <p class="kicker">{{ strings.setUpBeforeInstall }}</p>
          <h2>{{ app?.title }}</h2>
          <p class="sub">IceStore · {{ app?.category }} · {{ app?.version }}</p>
        </div>
        <button type="button" class="x" :aria-label="strings.close" @click="close"><Icon name="close" :size="18" /></button>
      </header>

      <p v-if="view.installed" class="warn small">{{ strings.alreadyInstalled }}</p>

      <div v-if="addressInput" class="field">
        <div class="label-row">
          <label :for="`in-${addressInput.key}`">{{ strings.addressYouOpen(app?.title ?? '') }}</label>
          <span class="chip ok"><Icon name="check" :size="13" /> {{ strings.autoFilled }}</span>
        </div>
        <input
          :id="`in-${addressInput.key}`"
          v-model.trim="values[api.inputKey(addressInput)]"
          class="mono"
          spellcheck="false"
          autocomplete="off"
        />
        <p class="hint">{{ strings.addressHint }}</p>
      </div>

      <div v-for="i in mainInputs" :key="api.inputKey(i)" class="field">
        <div class="label-row">
          <label :for="`in-${i.key}`">{{ inputLabel(i.key, view.name) }}</label>
          <span v-if="placeholderIn(values[api.inputKey(i)] ?? '')" class="chip need">{{ strings.required }}</span>
          <span v-else-if="i.fill" class="chip" :class="{ ok: i.fill === 'auto' }">{{ fillMark(i.fill) }}</span>
        </div>
        <div class="with-btn">
          <input
            :id="`in-${i.key}`"
            v-model.trim="values[api.inputKey(i)]"
            :type="i.secret && !shown[i.key] ? 'password' : 'text'"
            class="mono"
            spellcheck="false"
            autocomplete="off"
          />
          <button
            v-if="i.secret"
            type="button"
            class="icon-btn"
            :aria-label="shown[i.key] ? strings.hideValue : strings.showValue"
            @click="shown[i.key] = !shown[i.key]"
          >
            <Icon :name="shown[i.key] ? 'eyeOff' : 'eye'" :size="18" />
          </button>
        </div>
        <p v-if="i.description" class="hint">{{ i.description }}</p>
      </div>

      <div class="row">
        <div v-for="p in view.ports" :key="api.portKey(p)" class="cell">
          <label :for="`port-${api.portKey(p)}`">{{ strings.port }}</label>
          <input
            :id="`port-${api.portKey(p)}`"
            v-model.trim="ports[api.portKey(p)]"
            class="mono port"
            inputmode="numeric"
            maxlength="5"
            :title="p.description"
          />
          <span v-if="portState(api.portKey(p)) === 'checking'" class="chip">{{ strings.portChecking }}</span>
          <span v-else-if="portState(api.portKey(p)) === 'free'" class="chip ok"><Icon name="check" :size="13" /> {{ strings.portFree }}</span>
          <template v-else>
            <span class="chip need">{{ strings.portInUse }}</span>
            <button
              v-if="result?.suggest[api.portKey(p)]"
              type="button"
              class="link"
              @click="usePort(api.portKey(p), result!.suggest[api.portKey(p)]!)"
            >
              {{ strings.useFreePort(result!.suggest[api.portKey(p)]!) }}
            </button>
          </template>
        </div>
        <div class="cell">
          <span class="cell-label">{{ strings.data }}</span>
          <span class="mono data" :title="view.data">{{ view.data }}</span>
        </div>
      </div>

      <ul v-if="view.access.privileged || view.access.docker_socket || view.access.host_network" class="access">
        <li v-if="view.access.docker_socket"><Icon name="alert" :size="15" /> {{ strings.accessDocker }}</li>
        <li v-if="view.access.privileged"><Icon name="alert" :size="15" /> {{ strings.accessPrivileged }}</li>
        <li v-if="view.access.host_network"><Icon name="alert" :size="15" /> {{ strings.accessHost }}</li>
      </ul>

      <div v-if="otherInputs.length" class="advanced" :class="{ open: advancedOpen }">
        <button type="button" class="adv-head" :aria-expanded="advancedOpen" @click="advancedOpen = !advancedOpen">
          <span>
            <strong>{{ strings.advanced }}</strong>
            <span class="adv-sub">{{ otherInputs.map((i) => inputLabel(i.key, view!.name)).slice(0, 6).join(', ') }}<template v-if="otherInputs.length > 6">…</template></span>
          </span>
          <Icon name="chevron" :size="18" class="chev" />
        </button>
        <div v-if="advancedOpen" class="adv-body">
          <div v-for="i in otherInputs" :key="api.inputKey(i)" class="field small-field">
            <label :for="`in-${i.service}-${i.key}`">{{ i.key }}</label>
            <input
              :id="`in-${i.service}-${i.key}`"
              v-model.trim="values[api.inputKey(i)]"
              :type="i.secret && !shown[i.key] ? 'password' : 'text'"
              class="mono"
              spellcheck="false"
              autocomplete="off"
            />
            <p v-if="i.description" class="hint">{{ i.description }}</p>
          </div>
        </div>
      </div>

      <p v-if="missing.length" class="warn small">{{ strings.fillRequired(missing.join(', ')) }}</p>
      <p v-if="error" class="error" role="alert">{{ error }}</p>

      <footer class="foot">
        <span class="mut">{{ strings.firstStart }}</span>
        <span class="btns">
          <button type="button" class="btn ghost" @click="close">{{ strings.cancel }}</button>
          <button type="submit" class="btn primary" :disabled="!canInstall">
            {{ busy ? strings.installing : strings.install }}
          </button>
        </span>
      </footer>
    </form>
  </Modal>
</template>

<style scoped>
.setup {
  display: grid;
  gap: 22px;
}
.head {
  display: flex;
  align-items: center;
  gap: 20px;
}
.titles {
  flex: 1;
  min-width: 0;
}
.kicker {
  margin: 0 0 4px;
  font-family: var(--font-mono);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--ac2);
}
h2 {
  margin: 0;
  font-size: 28px;
  font-weight: 700;
  letter-spacing: -0.01em;
}
.sub {
  margin: 4px 0 0;
  color: var(--mut);
  font-size: 15px;
}
.x {
  align-self: flex-start;
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 12px;
  border: 1px solid var(--line);
  background: rgba(255, 255, 255, 0.03);
  color: var(--text);
  cursor: pointer;
}
.x:hover {
  background: rgba(255, 255, 255, 0.08);
}
.field {
  display: grid;
  gap: 8px;
}
.label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
label,
.cell-label {
  font-weight: 600;
  font-size: 16px;
}
.small-field label {
  font-family: var(--font-mono);
  font-size: 13px;
  font-weight: 500;
}
input {
  width: 100%;
  min-width: 0;
  padding: 16px 18px;
  border-radius: 14px;
  border: 1px solid rgba(76, 201, 240, 0.35);
  background: rgba(0, 0, 0, 0.35);
  color: var(--text);
  font-size: 15px;
}
.small-field input {
  padding: 11px 14px;
  border-color: var(--line);
}
input:focus {
  outline: none;
  border-color: var(--ac);
  box-shadow: 0 0 0 3px rgba(76, 201, 240, 0.18);
}
.mono {
  font-family: var(--font-mono);
}
.hint {
  margin: 0;
  color: var(--mut);
  font-size: 14px;
  line-height: 1.45;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  flex: none;
  padding: 5px 12px;
  border-radius: var(--radius-pill);
  border: 1px solid var(--line);
  font-family: var(--font-mono);
  font-size: 12.5px;
  color: var(--mut);
  white-space: nowrap;
}
.chip.ok {
  border-color: rgba(76, 201, 240, 0.45);
  background: rgba(76, 201, 240, 0.1);
  color: var(--ac2);
}
.chip.need {
  border-color: rgba(245, 184, 76, 0.5);
  background: rgba(245, 184, 76, 0.1);
  color: var(--warn);
}
.with-btn {
  display: flex;
  gap: 8px;
}
.icon-btn {
  flex: none;
  display: grid;
  place-items: center;
  width: 52px;
  border-radius: 14px;
  border: 1px solid var(--line);
  background: rgba(255, 255, 255, 0.04);
  color: var(--text);
  cursor: pointer;
}
.row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 14px;
}
.cell {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  padding: 14px 18px;
  border-radius: 16px;
  border: 1px solid var(--line);
  background: rgba(255, 255, 255, 0.025);
}
.cell label,
.cell-label {
  margin-right: auto;
}
.port {
  width: 84px;
  padding: 8px 10px;
  text-align: right;
  border-color: var(--line);
}
.data {
  color: var(--mut);
  font-size: 14px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
}
.link {
  border: 0;
  background: none;
  color: var(--ac);
  font: inherit;
  font-size: 14px;
  cursor: pointer;
  text-decoration: underline;
}
.access {
  display: grid;
  gap: 6px;
  margin: 0;
  padding: 12px 16px;
  list-style: none;
  border-radius: 14px;
  background: rgba(245, 184, 76, 0.08);
  border: 1px solid rgba(245, 184, 76, 0.3);
  color: var(--warn);
  font-size: 14px;
}
.access li {
  display: flex;
  align-items: center;
  gap: 8px;
}
.advanced {
  border: 1.5px dashed var(--line);
  border-radius: 16px;
}
.adv-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
  padding: 16px 20px;
  border: 0;
  background: none;
  color: var(--text);
  text-align: left;
  font: inherit;
  cursor: pointer;
}
.adv-head strong {
  display: block;
  font-size: 17px;
}
.adv-sub {
  display: block;
  margin-top: 2px;
  color: var(--mut);
  font-size: 14px;
}
.chev {
  transition: transform 0.15s;
}
.open .chev {
  transform: rotate(90deg);
}
.adv-body {
  display: grid;
  gap: 16px;
  padding: 4px 20px 20px;
}
.foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}
.btns {
  display: flex;
  gap: 12px;
}
.btn {
  padding: 14px 30px;
  border-radius: var(--radius-pill);
  font: inherit;
  font-size: 17px;
  font-weight: 600;
  cursor: pointer;
}
.btn.ghost {
  border: 1px solid var(--line);
  background: rgba(255, 255, 255, 0.04);
  color: var(--text);
}
.btn.primary {
  border: 0;
  background: var(--ac);
  color: var(--ink);
  box-shadow: var(--glow);
}
.btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
  box-shadow: none;
}
.mut {
  color: var(--mut);
}
.small {
  font-size: 14px;
}
.warn {
  margin: 0;
  color: var(--warn);
}
.error {
  margin: 0;
  color: #ff8a8a;
}
@media (max-width: 600px) {
  .head {
    gap: 14px;
  }
  h2 {
    font-size: 22px;
  }
  .btn {
    padding: 12px 22px;
  }
}
</style>
