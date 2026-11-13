<script setup lang="ts">
// The sign-in screen. On ZimaOS, IceDash sits beside the stock dashboard and
// sends you there to sign in. On CasaOS, IceDash is the main page, so it
// signs you in itself with the CasaOS login, keeping the tokens where CasaOS
// keeps them: both UIs are signed in after one sign-in.
import { ref } from 'vue'
import { ApiError, signIn } from '../api/client'
import { dashboardUrl, host } from '../lib/host'
import { strings } from '../strings'

defineProps<{ ended: boolean }>()
const logo = `${import.meta.env.BASE_URL}appicon.svg`

const username = ref('')
const password = ref('')
const busy = ref(false)
const error = ref('')

async function submit() {
  if (busy.value || !username.value || !password.value) return
  busy.value = true
  error.value = ''
  try {
    await signIn(username.value.trim(), password.value)
    location.reload()
  } catch (e) {
    error.value = e instanceof ApiError && e.code === 'unreachable' ? strings.cantReach : strings.signInFailed
    busy.value = false
  }
}
</script>

<template>
  <main class="signin">
    <img :src="logo" alt="" width="64" height="64" />
    <h1>{{ ended ? strings.sessionEndedTitle : strings.signInTitle(host.name) }}</h1>

    <template v-if="host.id === 'casaos'">
      <p>{{ strings.signInHere(host.name) }}</p>
      <form class="form" @submit.prevent="submit">
        <label>
          <span>{{ strings.username }}</span>
          <input v-model="username" name="username" autocomplete="username" autocapitalize="none" spellcheck="false" required />
        </label>
        <label>
          <span>{{ strings.password }}</span>
          <input v-model="password" name="password" type="password" autocomplete="current-password" required />
        </label>
        <p v-if="error" class="error" role="alert">{{ error }}</p>
        <button class="pill" type="submit" :disabled="busy || !username || !password">
          {{ busy ? strings.signingIn : strings.signInButton }}
        </button>
      </form>
      <a class="stock" :href="dashboardUrl()">{{ strings.openStockDashboard(host.name) }}</a>
    </template>

    <template v-else>
      <p>{{ ended ? strings.sessionEndedBody(host.name) : strings.signInBody(host.name) }}</p>
      <a class="pill" :href="dashboardUrl()">{{ strings.goToSystem(host.name) }}</a>
    </template>
  </main>
</template>

<style scoped>
.signin {
  min-height: 100vh;
  display: grid;
  place-content: center;
  justify-items: center;
  gap: 12px;
  padding: 16px;
  text-align: center;
}
img {
  border-radius: var(--radius-tile);
}
h1 {
  margin: 8px 0 0;
  font-family: var(--font-display);
  font-weight: 800;
  font-size: 26px;
}
p {
  max-width: 420px;
  margin: 0;
  color: var(--mut);
}
.form {
  display: grid;
  gap: 12px;
  width: min(320px, calc(100vw - 32px));
  margin-top: 6px;
  text-align: left;
}
label {
  display: grid;
  gap: 6px;
  font-size: 13px;
  color: var(--mut);
}
input {
  padding: 11px 13px;
  border-radius: 12px;
  border: 1px solid var(--line);
  background: rgba(255, 255, 255, 0.04);
  color: var(--text);
  font: inherit;
  font-size: 15px;
}
input:focus-visible {
  outline: 2px solid var(--ac2);
  outline-offset: 1px;
}
.error {
  color: var(--warn);
  font-size: 13px;
}
.pill {
  margin-top: 10px;
  padding: 10px 22px;
  border: 0;
  border-radius: var(--radius-pill);
  background: var(--ac);
  color: var(--ink);
  font: inherit;
  font-weight: 600;
  text-decoration: none;
  box-shadow: var(--glow);
  cursor: pointer;
  justify-self: center;
}
.pill:hover:not(:disabled) {
  background: var(--acd);
}
.pill:disabled {
  opacity: 0.55;
  cursor: default;
}
.stock {
  margin-top: 4px;
  color: var(--ac);
  font-size: 13px;
}
</style>
