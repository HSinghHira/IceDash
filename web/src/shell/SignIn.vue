<script setup lang="ts">
import { strings } from '../strings'

defineProps<{ ended: boolean }>()
const logo = `${import.meta.env.BASE_URL}appicon.svg`
</script>

<template>
  <main class="signin">
    <img :src="logo" alt="" width="64" height="64" />
    <h1>{{ ended ? strings.sessionEndedTitle : strings.signInTitle }}</h1>
    <p>{{ ended ? strings.sessionEndedBody : strings.signInBody }}</p>
    <a class="pill" href="/#/">{{ strings.goToZimaOS }}</a>
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
.pill {
  margin-top: 10px;
  padding: 10px 22px;
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
