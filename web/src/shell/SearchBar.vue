<script setup lang="ts">
// Ctrl+Space opens a search over installed apps and system launchers; Enter opens the top match.
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import AppIcon from '../components/AppIcon.vue'
import Icon from '../components/Icon.vue'
import type { IconName } from '../lib/icons'
import { desktopLaunchers, dockEnd } from '../lib/launchers'
import { search } from '../lib/search'
import { apps, iceStoreInZimaOS, openApp, openFiles, openIceStore, openSettings, openTerminal, searchOpen, user } from '../store'
import { strings } from '../strings'

interface Hit {
  id: string
  name: string
  url: string
  icon?: IconName
  img?: string
}

const q = ref('')
const sel = ref(0)
const input = ref<HTMLInputElement | null>(null)

const all = computed<Hit[]>(() => [
  ...desktopLaunchers(location, { admin: !!user.value?.admin, iceStoreInZimaOS: iceStoreInZimaOS.value }).map((l) => ({
    id: 'l:' + l.id,
    name: l.name,
    url: l.url,
    icon: l.icon,
  })),
  ...dockEnd({ admin: !!user.value?.admin }).map((l) => ({ id: 'l:' + l.id, name: l.name, url: l.url, icon: l.icon })),
  ...apps.value.filter((a) => a.url).map((a) => ({ id: 'a:' + a.id, name: a.name, url: a.url!, img: a.icon ?? '' })),
])
const hits = computed(() => search(all.value, q.value))

watch(q, () => (sel.value = 0))
watch(searchOpen, async (open) => {
  if (open) {
    q.value = ''
    await nextTick()
    input.value?.focus()
  }
})

function go(h: Hit | undefined) {
  if (!h) return
  searchOpen.value = false
  const app = h.id.startsWith('a:') ? apps.value.find((a) => 'a:' + a.id === h.id) : undefined
  if (app) openApp(app)
  else if (h.id === 'l:icestore') openIceStore()
  else if (h.id === 'l:files') openFiles()
  else if (h.id === 'l:trash') openFiles('trash')
  else if (h.id === 'l:terminal') openTerminal()
  else if (h.id === 'l:settings') openSettings()
  else window.open(h.url, '_blank', 'noopener')
}
function onInputKey(e: KeyboardEvent) {
  if (e.key === 'ArrowDown') {
    sel.value = Math.min(sel.value + 1, hits.value.length - 1)
    e.preventDefault()
  } else if (e.key === 'ArrowUp') {
    sel.value = Math.max(sel.value - 1, 0)
    e.preventDefault()
  } else if (e.key === 'Enter') {
    go(hits.value[sel.value])
  } else if (e.key === 'Escape') {
    searchOpen.value = false
  }
}
function onGlobalKey(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.code === 'Space') {
    e.preventDefault()
    searchOpen.value = !searchOpen.value
  }
}
onMounted(() => document.addEventListener('keydown', onGlobalKey))
onBeforeUnmount(() => document.removeEventListener('keydown', onGlobalKey))
</script>

<template>
  <button class="slot" type="button" :aria-label="strings.searchLabel" :title="`${strings.searchPlaceholder} (${strings.searchShortcut})`" @click="searchOpen = true">
    <span class="tile"><Icon name="search" :size="21" /></span>
  </button>

  <Teleport to="body">
  <div v-if="searchOpen" class="overlay" @click.self="searchOpen = false">
    <div class="box" role="dialog" aria-modal="true" :aria-label="strings.searchLabel">
      <label class="field">
        <Icon name="search" :size="18" />
        <input
          ref="input"
          v-model="q"
          type="search"
          :placeholder="strings.searchPlaceholder"
          :aria-label="strings.searchLabel"
          autocomplete="off"
          spellcheck="false"
          role="combobox"
          aria-expanded="true"
          aria-controls="search-results"
          :aria-activedescendant="hits[sel] ? `hit-${sel}` : undefined"
          @keydown="onInputKey"
        />
        <kbd>Esc</kbd>
      </label>
      <ul id="search-results" class="results" role="listbox">
        <li
          v-for="(h, i) in hits"
          :id="`hit-${i}`"
          :key="h.id"
          role="option"
          :aria-selected="i === sel"
          :class="{ on: i === sel }"
          @mouseenter="sel = i"
          @click="go(h)"
        >
          <span v-if="h.icon" class="ic"><Icon :name="h.icon" :size="18" /></span>
          <AppIcon v-else :src="h.img" :name="h.name" :size="30" />
          <span>{{ h.name }}</span>
          <Icon class="ext" name="external" :size="14" />
        </li>
        <li v-if="!hits.length" class="none">{{ strings.searchNothing }}</li>
      </ul>
    </div>
  </div>
  </Teleport>
</template>

<style scoped>
.slot {
  flex: none;
  display: grid;
  place-items: center;
  padding: 0;
  border: 0;
  background: none;
  cursor: pointer;
  transition: transform 0.15s ease;
}
.slot:hover,
.slot:focus-visible {
  transform: translateY(-3px);
}
kbd {
  font-family: var(--font-mono);
  font-size: 11px;
  padding: 3px 7px;
  border-radius: 6px;
  border: 1px solid var(--line);
  color: var(--mut);
}
.overlay {
  position: fixed;
  inset: 0;
  z-index: 200;
  display: grid;
  justify-items: center;
  align-content: start;
  padding: 14vh 16px 16px;
  background: rgba(0, 0, 0, 0.45);
}
.box {
  width: min(560px, 100%);
  border-radius: 20px;
  background: rgba(var(--surface-rgb), 0.94);
  backdrop-filter: var(--blur);
  -webkit-backdrop-filter: var(--blur);
  border: 1px solid var(--line);
  box-shadow: 0 30px 80px rgba(0, 0, 0, 0.7), 0 0 0 1px rgba(var(--ac-rgb), 0.08);
  overflow: hidden;
}
.field {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 16px;
  border-bottom: 1px solid var(--line);
  color: var(--mut);
}
.field input {
  flex: 1;
  min-width: 0;
  background: none;
  border: 0;
  outline: none;
  color: var(--text);
  font: inherit;
  font-size: 16px;
}
.results {
  list-style: none;
  margin: 0;
  padding: 6px;
  max-height: 50vh;
  overflow-y: auto;
}
.results li {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 10px;
  border-radius: 12px;
  cursor: pointer;
}
.results li.on {
  background: rgba(var(--ac-rgb), 0.14);
}
.results .ic {
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  border-radius: 9px;
  color: var(--ac2);
  background: rgba(var(--fg-rgb), 0.06);
}
.results .ext {
  margin-left: auto;
  color: var(--mut);
}
.results .none {
  color: var(--mut);
  cursor: default;
}
</style>
