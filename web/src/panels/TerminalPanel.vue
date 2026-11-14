<script setup lang="ts">
// The Terminal (PLAN.md, Phase 3c): the account password again, then a root
// shell in /DATA over a WebSocket. Keystrokes go as text; a text message
// starting with "\x00" carries the size; output comes back as bytes.
import { FitAddon } from '@xterm/addon-fit'
import { Terminal } from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'
import { nextTick, onBeforeUnmount, ref } from 'vue'
import { ApiError } from '../api/client'
import { terminalSocketUrl, terminalTicket } from '../api/files'
import Icon from '../components/Icon.vue'
import { user } from '../store'
import { strings } from '../strings'

defineProps<{ winId: string }>()

type State = 'password' | 'connecting' | 'open' | 'closed'
const state = ref<State>('password')
const password = ref('')
const error = ref('')
const closedWhy = ref('')
const screen = ref<HTMLElement | null>(null)

let term: Terminal | null = null
let fit: FitAddon | null = null
let ws: WebSocket | null = null
let observer: ResizeObserver | null = null

async function unlock() {
  if (!password.value || state.value === 'connecting') return
  error.value = ''
  state.value = 'connecting'
  try {
    const ticket = await terminalTicket(password.value)
    password.value = ''
    await connect(ticket)
  } catch (e) {
    state.value = 'password'
    if (e instanceof ApiError && e.code === 'wrong_password') error.value = strings.termWrongPassword
    else if (e instanceof ApiError && e.code === 'locked') error.value = strings.termLocked
    else error.value = e instanceof Error ? e.message : String(e)
  }
}

async function connect(ticket: string) {
  state.value = 'open'
  await nextTick()
  term = new Terminal({
    cursorBlink: true,
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
    fontSize: 13.5,
    scrollback: 5000,
    theme: {
      background: '#07090c',
      foreground: '#e6edf2',
      cursor: '#4cc9f0',
      selectionBackground: 'rgba(76, 201, 240, 0.3)',
    },
  })
  fit = new FitAddon()
  term.loadAddon(fit)
  term.open(screen.value!)
  fit.fit()
  ws = new WebSocket(terminalSocketUrl(ticket, term.cols, term.rows))
  ws.binaryType = 'arraybuffer'
  ws.onmessage = (e) => term?.write(new Uint8Array(e.data as ArrayBuffer))
  ws.onclose = (e) => {
    closedWhy.value = e.reason || strings.termClosed
    state.value = 'closed'
    teardown(false)
  }
  term.onData((d) => ws?.readyState === WebSocket.OPEN && ws.send(d))
  term.onResize(({ cols, rows }) => ws?.readyState === WebSocket.OPEN && ws.send('\x00' + JSON.stringify({ cols, rows })))
  observer = new ResizeObserver(() => fit?.fit())
  observer.observe(screen.value!)
  term.focus()
}

function teardown(closeSocket = true) {
  observer?.disconnect()
  observer = null
  if (closeSocket) ws?.close()
  ws = null
  term?.dispose()
  term = null
  fit = null
}

function again() {
  closedWhy.value = ''
  state.value = 'password'
}

onBeforeUnmount(() => teardown())
</script>

<template>
  <div class="terminal">
    <form v-if="state === 'password' || state === 'connecting'" class="lock" @submit.prevent="unlock">
      <span class="badge"><Icon name="terminal" :size="22" /></span>
      <h3>{{ strings.termTitle }}</h3>
      <p class="mut">{{ strings.termPasswordHint(user?.name ?? '') }}</p>
      <input
        v-model="password"
        type="password"
        name="password"
        autocomplete="current-password"
        :aria-label="strings.password"
        :placeholder="strings.password"
        autofocus
      />
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <button class="primary" type="submit" :disabled="!password || state === 'connecting'">
        {{ state === 'connecting' ? strings.termOpening : strings.termOpen }}
      </button>
    </form>

    <div v-show="state === 'open'" ref="screen" class="screen" />

    <div v-if="state === 'closed'" class="lock">
      <span class="badge"><Icon name="terminal" :size="22" /></span>
      <h3>{{ closedWhy }}</h3>
      <button class="primary" type="button" @click="again">{{ strings.termAgain }}</button>
    </div>
  </div>
</template>

<style scoped>
.terminal {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: #07090c;
  border-radius: 0 0 18px 18px;
  overflow: hidden;
}
.screen {
  flex: 1;
  min-height: 0;
  padding: 8px 4px 4px 10px;
}
.lock {
  margin: auto;
  display: grid;
  justify-items: center;
  gap: 10px;
  width: min(340px, calc(100% - 32px));
  text-align: center;
}
.badge {
  display: grid;
  place-items: center;
  width: 48px;
  height: 48px;
  border-radius: 14px;
  background: rgba(76, 201, 240, 0.12);
  color: var(--ac);
}
h3 {
  margin: 4px 0 0;
  font-size: 17px;
}
.mut {
  margin: 0;
  color: var(--mut);
  font-size: 13px;
  line-height: 1.45;
}
input {
  width: 100%;
  padding: 11px 13px;
  border-radius: 12px;
  border: 1px solid var(--line);
  background: rgba(255, 255, 255, 0.04);
  color: var(--text);
  font: inherit;
}
input:focus-visible {
  outline: 2px solid var(--ac2);
  outline-offset: 1px;
}
.error {
  margin: 0;
  color: var(--warn);
  font-size: 13px;
}
.primary {
  padding: 9px 22px;
  border: 0;
  border-radius: var(--radius-pill);
  background: var(--ac);
  color: var(--ink);
  font: inherit;
  font-weight: 600;
  cursor: pointer;
  box-shadow: var(--glow);
}
.primary:disabled {
  opacity: 0.55;
  cursor: default;
}
</style>
