<script setup lang="ts">
// Mock-up screen 1: top bar, desktop icons on the left, the widget panel on
// the right, search and dock at the bottom, and IceDash's own windows on top.
import { onBeforeUnmount, onMounted, ref } from 'vue'
import AboutPanel from '../panels/AboutPanel.vue'
import CredentialsPanel from '../panels/CredentialsPanel.vue'
import IceStorePanel from '../panels/IceStorePanel.vue'
import LogsPanel from '../panels/LogsPanel.vue'
import NotificationsPanel from '../panels/NotificationsPanel.vue'
import { firstLaunchFor, offline, panelOpen, setupFor, uninstallFor, updating, windows, type WindowKind } from '../store'
import { strings } from '../strings'
import AppMenu from './AppMenu.vue'
import AppWindow from './AppWindow.vue'
import FirstLaunchDialog from './FirstLaunchDialog.vue'
import SetupDialog from './SetupDialog.vue'
import UninstallDialog from './UninstallDialog.vue'
import DesktopIcons from './DesktopIcons.vue'
import Dock from './Dock.vue'
import SearchBar from './SearchBar.vue'
import Toasts from './Toasts.vue'
import TopBar from './TopBar.vue'
import WidgetPanel from './WidgetPanel.vue'

const widths: Record<WindowKind, number> = { about: 480, notifications: 420, icestore: 1060, logs: 780, credentials: 620 }

const now = ref(new Date())
let tick: ReturnType<typeof setInterval>
onMounted(() => {
  // Tick on the second so the minute changes on time.
  tick = setInterval(() => (now.value = new Date()), 1000)
})
onBeforeUnmount(() => clearInterval(tick))
</script>

<template>
  <div class="desktop">
    <TopBar :now="now" />
    <p v-if="updating" class="offline" role="status">{{ strings.updatingBanner }}</p>
    <p v-else-if="offline" class="offline" role="status">{{ strings.cantReach }}</p>

    <div class="area" :class="{ 'no-panel': !panelOpen }">
      <DesktopIcons class="icons" />
      <WidgetPanel v-if="panelOpen" class="widgets" :now="now" />
    </div>

    <div class="bottom">
      <SearchBar />
      <Dock />
    </div>

    <AppWindow v-for="(w, i) in windows" :key="w.id" :win="w" :index="i" :width="widths[w.kind]">
      <AboutPanel v-if="w.kind === 'about'" />
      <NotificationsPanel v-else-if="w.kind === 'notifications'" />
      <IceStorePanel v-else-if="w.kind === 'icestore'" />
      <LogsPanel v-else-if="w.kind === 'logs' && w.app" :name="w.app" />
      <CredentialsPanel v-else-if="w.kind === 'credentials' && w.app" :name="w.app" :win-id="w.id" />
    </AppWindow>
    <AppMenu />
    <SetupDialog v-if="setupFor" :id="setupFor" :key="setupFor" />
    <FirstLaunchDialog v-if="firstLaunchFor" :key="firstLaunchFor" :name="firstLaunchFor" />
    <UninstallDialog v-if="uninstallFor" :key="uninstallFor" :name="uninstallFor" />
    <Toasts />
  </div>
</template>

<style scoped>
.desktop {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}
.offline {
  margin: 8px auto 0;
  padding: 6px 14px;
  border-radius: var(--radius-pill);
  background: rgba(245, 184, 76, 0.14);
  color: var(--warn);
  font-size: 13px;
}
.area {
  flex: 1;
  display: grid;
  grid-template-columns: 1fr 336px;
  gap: 24px;
  padding: 22px 24px 190px 22px;
}
.area.no-panel {
  grid-template-columns: 1fr;
}
.widgets {
  position: sticky;
  top: 58px;
  align-self: start;
}
.bottom {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 18px;
  z-index: 40;
  display: grid;
  justify-items: center;
  gap: 14px;
  padding: 0 16px;
  pointer-events: none;
}
.bottom > * {
  pointer-events: auto;
}
@media (max-width: 900px) {
  .area {
    grid-template-columns: 1fr;
    padding: 16px 16px 180px;
  }
  .widgets {
    position: static;
  }
}
</style>
