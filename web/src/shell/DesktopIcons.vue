<script setup lang="ts">
import { computed } from 'vue'
import AppIcon from '../components/AppIcon.vue'
import Icon from '../components/Icon.vue'
import { desktopLaunchers } from '../lib/launchers'
import { apps, appsLoaded, iceStoreInZimaOS, terminalOk } from '../store'
import { strings } from '../strings'

const launchers = computed(() =>
  desktopLaunchers(location, { terminal: terminalOk.value, iceStoreInZimaOS: iceStoreInZimaOS.value }),
)
const statusWord = { running: strings.running, starting: strings.starting, stopped: strings.stopped }
</script>

<template>
  <nav class="icons" :aria-label="strings.apps">
    <a
      v-for="l in launchers"
      :key="l.id"
      class="item"
      :href="l.url"
      target="_blank"
      rel="noopener"
    >
      <span class="tile sys"><Icon :name="l.icon" :size="26" /></span>
      <span class="label">{{ l.name }}</span>
    </a>

    <template v-for="a in apps" :key="a.id">
      <a
        v-if="a.url"
        class="item"
        :href="a.url"
        target="_blank"
        rel="noopener"
        :aria-label="strings.appStatus(a.name, statusWord[a.status])"
      >
        <span class="tile"><AppIcon :src="a.icon" :name="a.name" /><span class="status" :class="a.status" /></span>
        <span class="label">{{ a.name }}</span>
      </a>
      <div v-else class="item no-link" :aria-label="strings.appStatus(a.name, statusWord[a.status])" role="img">
        <span class="tile"><AppIcon :src="a.icon" :name="a.name" /><span class="status" :class="a.status" /></span>
        <span class="label">{{ a.name }}</span>
      </div>
    </template>
    <p v-if="appsLoaded && apps.length === 0" class="empty">{{ strings.noApps }}</p>
  </nav>
</template>

<style scoped>
.icons {
  display: grid;
  grid-template-columns: repeat(2, 104px);
  grid-auto-rows: 102px;
  gap: 2px 0;
  align-content: start;
}
.item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 7px;
  padding-top: 4px;
  border-radius: 14px;
  text-decoration: none;
  color: var(--text);
}
.item:hover .tile,
.item:focus-visible .tile {
  transform: translateY(-2px);
  box-shadow: 0 10px 28px rgba(76, 201, 240, 0.18);
}
.item:focus-visible {
  outline-offset: 0;
}
.no-link {
  cursor: default;
}
.tile {
  position: relative;
  width: 60px;
  height: 60px;
  border-radius: var(--radius-tile);
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}
.tile.sys {
  display: grid;
  place-items: center;
  color: var(--ac2);
  background: linear-gradient(160deg, rgba(255, 255, 255, 0.07), rgba(255, 255, 255, 0.02));
  border: 1px solid var(--line);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.05);
}
.status {
  position: absolute;
  right: -4px;
  bottom: -4px;
  width: 13px;
  height: 13px;
  border-radius: 50%;
  border: 2.5px solid var(--bg);
  background: #6b7076;
}
.status.running {
  background: var(--ac2);
}
.status.starting {
  background: var(--warn);
}
.label {
  max-width: 100px;
  font-size: 13px;
  font-weight: 500;
  line-height: 1.25;
  text-align: center;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.8);
}
.empty {
  grid-column: 1 / -1;
  color: var(--mut);
  font-size: 13px;
}
@media (max-width: 900px) {
  .icons {
    grid-template-columns: repeat(auto-fill, minmax(92px, 1fr));
  }
}
</style>
