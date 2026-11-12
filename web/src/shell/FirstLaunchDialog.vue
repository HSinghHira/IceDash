<script setup lang="ts">
// Mock-up screen 4: the welcome popup the first time an app installed through
// IceDash is opened. Address and login with copy buttons, and the app's own
// "How to use" steps from IceStore.
import { computed, onMounted, ref } from 'vue'
import * as api from '../api/apps'
import AppIcon from '../components/AppIcon.vue'
import Icon from '../components/Icon.vue'
import Modal from '../components/Modal.vue'
import { copyText } from '../lib/copy'
import { appInfo, apps, firstLaunchFor } from '../store'
import { strings } from '../strings'

const props = defineProps<{ name: string }>()
const app = computed(() => apps.value.find((a) => a.id === props.name))
const fl = ref<api.FirstLaunch | null>(null)
const dontShow = ref(false)
const copied = ref('')
const shown = ref(false)

onMounted(async () => {
  try {
    fl.value = await api.firstLaunch(props.name)
  } catch {
    fl.value = { name: props.name, title: app.value?.name ?? props.name, tagline: '', steps: [] }
  }
})

const title = computed(() => fl.value?.title || app.value?.name || props.name)
/** Address first, then the login fields that aren't addresses. */
const boxes = computed(() => {
  const out: { label: string; value: string; secret: boolean }[] = []
  if (app.value?.url) out.push({ label: strings.address, value: app.value.url, secret: false })
  for (const f of fl.value?.creds?.fields ?? []) if (!f.url) out.push({ label: f.label, value: f.value, secret: f.secret })
  return out
})

async function copy(label: string, v: string) {
  if (await copyText(v)) {
    copied.value = label
    setTimeout(() => (copied.value = copied.value === label ? '' : copied.value), 1500)
  }
}

async function finish(open: boolean) {
  if (dontShow.value) {
    try {
      await api.dismissFirstLaunch(props.name)
      const i = appInfo.value[props.name]
      if (i) i.first_launch = false
    } catch {
      // shows again next time
    }
  }
  firstLaunchFor.value = null
  if (open && app.value?.url) window.open(app.value.url, '_blank', 'noopener')
}
</script>

<template>
  <Modal :label="strings.welcome(title)" :width="760" @close="finish(false)">
    <div class="fl">
      <AppIcon :src="app?.icon" :name="title" :size="88" class="logo" />
      <p class="kicker">{{ strings.firstLaunch }}</p>
      <h2>{{ strings.welcome(title) }}</h2>
      <p v-if="fl?.tagline" class="tag">{{ fl.tagline }}</p>

      <div v-if="boxes.length" class="boxes">
        <div v-for="b in boxes" :key="b.label" class="box" :class="{ wide: b.label === strings.address }">
          <div class="val">
            <span class="lbl">{{ b.label }}</span>
            <span class="mono">{{ b.secret && !shown ? '•'.repeat(10) : b.value }}</span>
          </div>
          <button v-if="b.secret" type="button" class="sq" :aria-label="shown ? strings.hideValue : strings.showValue" @click="shown = !shown">
            <Icon :name="shown ? 'eyeOff' : 'eye'" :size="17" />
          </button>
          <button type="button" class="sq" :aria-label="`${strings.copy} ${b.label}`" @click="copy(b.label, b.value)">
            <Icon :name="copied === b.label ? 'check' : 'copy'" :size="17" />
          </button>
        </div>
      </div>

      <p v-for="(n, i) in fl?.creds?.notes ?? []" :key="i" class="note"><Icon name="info" :size="18" /> <span>{{ n }}</span></p>

      <section v-if="fl?.steps.length" class="how">
        <h3 class="kicker">{{ strings.howToUse }}</h3>
        <ol>
          <li v-for="(s, i) in fl.steps" :key="i"><span class="num">{{ i + 1 }}</span><span>{{ s }}</span></li>
        </ol>
      </section>

      <footer class="foot">
        <label class="check"><input v-model="dontShow" type="checkbox" /> {{ strings.dontShowAgain }}</label>
        <span class="btns">
          <button type="button" class="btn ghost" @click="finish(false)">{{ strings.later }}</button>
          <button type="button" class="btn primary" data-autofocus :disabled="!app?.url" @click="finish(true)">
            <Icon name="arrowRight" :size="18" /> {{ strings.openNamed(title) }}
          </button>
        </span>
      </footer>
    </div>
  </Modal>
</template>

<style scoped>
.fl {
  display: grid;
  gap: 18px;
  text-align: center;
}
.logo {
  justify-self: center;
  box-shadow: 0 0 60px rgba(76, 201, 240, 0.18);
}
.kicker {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--ac2);
}
h2 {
  margin: -6px 0 0;
  font-size: 34px;
  font-weight: 700;
  letter-spacing: -0.01em;
}
.tag {
  margin: -6px 0 0;
  color: var(--mut);
  font-size: 17px;
}
.boxes {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  text-align: left;
}
.box {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 14px 14px 18px;
  border-radius: 16px;
  border: 1px solid var(--line);
  background: rgba(255, 255, 255, 0.03);
  min-width: 0;
}
.box {
  flex: 1 1 210px;
}
.box.wide {
  flex: 2 1 300px;
}
.val {
  flex: 1;
  display: grid;
  gap: 3px;
  min-width: 0;
}
.lbl {
  color: var(--mut);
  font-size: 14px;
}
.mono {
  font-family: var(--font-mono);
  font-size: 15px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sq {
  flex: none;
  display: grid;
  place-items: center;
  width: 42px;
  height: 42px;
  border-radius: 12px;
  border: 1px solid var(--line);
  background: rgba(255, 255, 255, 0.04);
  color: var(--text);
  cursor: pointer;
}
.note {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  margin: 0;
  padding: 14px 18px;
  border-radius: 14px;
  background: rgba(76, 201, 240, 0.08);
  text-align: left;
  line-height: 1.45;
}
.note :deep(svg) {
  flex: none;
  margin-top: 2px;
  color: var(--ac2);
}
.how {
  padding: 18px 22px;
  border-radius: 18px;
  border: 1px solid var(--line);
  text-align: left;
}
.how .kicker {
  margin-bottom: 12px;
}
ol {
  display: grid;
  gap: 12px;
  margin: 0;
  padding: 0;
  list-style: none;
}
li {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  font-size: 16px;
  line-height: 1.45;
}
.num {
  flex: none;
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  border-radius: 50%;
  background: rgba(76, 201, 240, 0.12);
  color: var(--ac2);
  font-family: var(--font-mono);
  font-size: 13px;
  font-weight: 700;
}
.foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  flex-wrap: wrap;
  margin-top: 4px;
}
.check {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  font-size: 16px;
}
.check input {
  width: 20px;
  height: 20px;
  accent-color: var(--ac);
}
.btns {
  display: flex;
  gap: 12px;
}
.btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 14px 28px;
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
  opacity: 0.5;
}
@media (max-width: 600px) {
  h2 {
    font-size: 26px;
  }
}
</style>
