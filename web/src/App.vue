<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'
import Desktop from './shell/Desktop.vue'
import SignIn from './shell/SignIn.vue'
import { session, start, stop } from './store'

onMounted(start)
onBeforeUnmount(stop)
</script>

<template>
  <Desktop v-if="session === 'ok'" />
  <SignIn v-else-if="session === 'signin' || session === 'ended'" :ended="session === 'ended'" />
</template>
