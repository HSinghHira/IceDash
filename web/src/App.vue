<script setup lang="ts">
// Placeholder screen until the desktop shell (mock-up screen 1) lands: shows
// that the web app, the gateway route and the backend work together.
import { onMounted, ref } from 'vue'
import { health, type Health } from './api/client'
import { strings } from './strings'

const logo = `${import.meta.env.BASE_URL}appicon.svg`
const info = ref<Health | null>(null)
const problem = ref('')

onMounted(async () => {
  try {
    info.value = await health()
  } catch {
    problem.value = strings.cantReach
  }
})
</script>

<template>
  <main class="hello">
    <img class="logo" :src="logo" alt="" width="64" height="64" />
    <h1>{{ strings.appName }}</h1>
    <p v-if="info" class="mono">{{ strings.version(info.version) }} · {{ info.arch }}</p>
    <p v-else-if="problem">{{ problem }}</p>
    <p v-else class="mut">{{ strings.loading }}</p>
    <a class="pill" href="/#/">{{ strings.openZimaOS }}</a>
  </main>
</template>

<style scoped>
.hello {
  min-height: 100vh;
  display: grid;
  place-content: center;
  justify-items: center;
  gap: 12px;
  padding: 16px;
  text-align: center;
}
.logo {
  border-radius: var(--radius-tile);
}
h1 {
  margin: 0;
  font-family: var(--font-display);
  font-weight: 800;
  font-size: 32px;
}
.mono {
  font-family: var(--font-mono);
  color: var(--ac2);
}
.mut {
  color: var(--mut);
}
.pill {
  margin-top: 8px;
  padding: 10px 20px;
  border-radius: var(--radius-pill);
  background: var(--ac);
  color: var(--ink);
  font-weight: 600;
  text-decoration: none;
  box-shadow: var(--glow);
}
.pill:hover {
  background: var(--acd);
}
</style>
