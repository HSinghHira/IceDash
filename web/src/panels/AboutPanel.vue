<script setup lang="ts">
// About IceDash and its updates: versions, release notes, Check now, Update now
// with live steps, and the "Check for updates automatically" switch.
import { computed, ref } from 'vue'
import { checkUpdate, saveSettings } from '../api/icedash'
import Icon from '../components/Icon.vue'
import { ago } from '../lib/format'
import { dashboardUrl, host } from '../lib/host'
import { autoCheck, refreshUpdate, startUpdate, update, user, version } from '../store'
import { strings } from '../strings'

const logo = `${import.meta.env.BASE_URL}appicon.svg`
const busy = ref(false)
const message = ref('')
const admin = computed(() => !!user.value?.admin)
const st = computed(() => update.value)
const installing = computed(() => st.value?.state === 'installing')
const waiting = computed(() => st.value?.state === 'available' || (st.value?.state === 'failed' && !!st.value.latest))
const handedOver = computed(() => installing.value && st.value?.steps.at(-1)?.id === 'handover')
const checked = computed(() => (st.value?.checked_at ? strings.lastChecked(ago(new Date(st.value.checked_at))) : strings.neverChecked))

async function check() {
  busy.value = true
  message.value = ''
  try {
    update.value = await checkUpdate()
  } catch (e) {
    message.value = e instanceof Error ? e.message : String(e)
  } finally {
    busy.value = false
    await refreshUpdate()
  }
}

async function install() {
  message.value = (await startUpdate()) ?? ''
}

async function toggleAuto(e: Event) {
  const on = (e.target as HTMLInputElement).checked
  const before = autoCheck.value
  autoCheck.value = on
  try {
    autoCheck.value = (await saveSettings({ auto_check: on })).auto_check
  } catch {
    autoCheck.value = before
  }
}
</script>

<template>
  <div class="about">
    <div class="head">
      <img :src="logo" alt="" width="52" height="52" />
      <div>
        <h3>{{ strings.appName }}</h3>
        <p class="mut">{{ strings.aboutBody(host.name, host.id === 'casaos') }}</p>
        <a class="stock" :href="dashboardUrl()" target="_blank" rel="noopener">{{ strings.menuDashboard(host.name) }}</a>
      </div>
    </div>

    <dl class="versions mono">
      <div>
        <dt>{{ strings.installedVersion }}</dt>
        <dd>v{{ st?.current ?? version }}</dd>
      </div>
      <div v-if="st?.latest">
        <dt>{{ strings.latestVersion }}</dt>
        <dd>{{ st.latest }}</dd>
      </div>
    </dl>

    <p v-if="installing && handedOver" class="state ok">
      <Icon name="refresh" :size="16" class="spin" /> {{ strings.restarting }}
    </p>
    <p v-else-if="waiting" class="state new">{{ strings.newVersion(st!.latest!) }}</p>
    <p v-else-if="st && !installing && st.state !== 'checking'" class="state ok"><Icon name="check" :size="16" /> {{ strings.upToDate }}</p>

    <ol v-if="st?.steps.length" class="steps">
      <li v-for="s in st.steps" :key="s.id + s.text" :class="s.state">
        <Icon :name="s.state === 'done' ? 'check' : s.state === 'failed' ? 'alert' : 'refresh'" :size="15" :class="{ spin: s.state === 'running' }" />
        <span>{{ s.text }}</span>
        <span class="sr">{{ strings.stepState[s.state] }}</span>
      </li>
    </ol>
    <p v-if="st?.state === 'failed' && st.error" class="state bad">
      <Icon name="alert" :size="16" /> {{ strings.updateFailed }} <span class="mut">({{ st.error }})</span>
    </p>
    <p v-else-if="st?.error && !installing" class="mut small">{{ st.error }}</p>

    <section v-if="waiting && st?.notes" class="notes">
      <h4>{{ strings.releaseNotes }}</h4>
      <p>{{ st.notes }}</p>
    </section>

    <div class="actions">
      <button v-if="waiting" class="primary" type="button" :disabled="installing || !admin" @click="install">
        {{ installing ? strings.installing : strings.updateNow }}
      </button>
      <button class="ghost" type="button" :disabled="busy || installing || !admin" @click="check">
        {{ busy || st?.state === 'checking' ? strings.checking : strings.checkNow }}
      </button>
      <span class="mut small">{{ checked }}</span>
    </div>
    <p v-if="!admin" class="mut small">{{ strings.adminOnly }}</p>
    <p v-if="message" class="state bad" role="alert">{{ message }}</p>

    <label class="switch">
      <input type="checkbox" :checked="autoCheck ?? true" :disabled="!admin || autoCheck === null" @change="toggleAuto" />
      <span class="track" aria-hidden="true"><span /></span>
      <span>
        <strong>{{ strings.autoCheck }}</strong>
        <span class="mut small block">{{ strings.autoCheckHint }}</span>
      </span>
    </label>
  </div>
</template>

<style scoped>
.about {
  display: grid;
  gap: 16px;
  font-size: 14px;
}
.head {
  display: flex;
  gap: 14px;
  align-items: center;
}
.head img {
  border-radius: 14px;
}
h3 {
  margin: 0 0 2px;
  font-family: var(--font-display);
  font-weight: 800;
  font-size: 20px;
}
.head p {
  margin: 0;
  font-size: 13px;
}
.stock {
  display: inline-block;
  margin-top: 6px;
  color: var(--ac);
  font-size: 13px;
}
.mut {
  color: var(--mut);
}
.small {
  font-size: 12.5px;
}
.block {
  display: block;
}
.mono {
  font-family: var(--font-mono);
}
.versions {
  display: flex;
  gap: 28px;
  margin: 0;
}
.versions dt {
  font-size: 11px;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  color: var(--ac2);
}
.versions dd {
  margin: 2px 0 0;
  font-size: 15px;
}
.state {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
}
.state.ok {
  color: var(--ac2);
}
.state.new {
  color: var(--text);
  font-weight: 600;
}
.state.bad {
  color: var(--warn);
  flex-wrap: wrap;
}
.steps {
  list-style: none;
  margin: 0;
  padding: 12px 14px;
  display: grid;
  gap: 8px;
  border-radius: 14px;
  background: rgba(var(--fg-rgb), 0.04);
  font-size: 13px;
}
.steps li {
  display: flex;
  align-items: center;
  gap: 9px;
  color: var(--mut);
}
.steps li.done {
  color: var(--text);
}
.steps li.done :deep(svg) {
  color: var(--ac2);
}
.steps li.running {
  color: var(--text);
}
.steps li.failed {
  color: var(--warn);
}
.notes h4 {
  margin: 0 0 4px;
  font-family: var(--font-mono);
  font-size: 11px;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  color: var(--ac2);
}
.notes p {
  margin: 0;
  white-space: pre-wrap;
}
.actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}
.primary,
.ghost {
  border-radius: var(--radius-pill);
  padding: 9px 18px;
  font-weight: 600;
  cursor: pointer;
}
.primary {
  border: 0;
  background: var(--ac);
  color: var(--ink);
  box-shadow: var(--glow);
}
.primary:hover:not(:disabled) {
  background: var(--acd);
}
.ghost {
  background: none;
  border: 1px solid var(--line);
  color: var(--text);
}
.ghost:hover:not(:disabled) {
  border-color: rgba(var(--ac-rgb), 0.4);
}
button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.switch {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  padding-top: 14px;
  border-top: 1px solid var(--line);
  cursor: pointer;
}
.switch input {
  position: absolute;
  opacity: 0;
  width: 1px;
  height: 1px;
}
.track {
  flex: none;
  width: 38px;
  height: 22px;
  margin-top: 1px;
  border-radius: var(--radius-pill);
  background: rgba(var(--fg-rgb), 0.14);
  position: relative;
  transition: background 0.15s;
}
.track span {
  position: absolute;
  top: 3px;
  left: 3px;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: #fff;
  transition: transform 0.15s;
}
.switch input:checked + .track {
  background: var(--ac);
}
.switch input:checked + .track span {
  transform: translateX(16px);
}
.switch input:focus-visible + .track {
  outline: 2px solid var(--ac2);
  outline-offset: 2px;
}
.switch input:disabled + .track {
  opacity: 0.5;
}
.spin {
  animation: spin 1s linear infinite;
}
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
@media (prefers-reduced-motion: reduce) {
  .spin {
    animation: none;
  }
}
.sr {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
}
</style>
