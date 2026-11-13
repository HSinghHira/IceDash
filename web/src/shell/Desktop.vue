<script setup lang="ts">
// Mock-up screen 1: top bar, desktop icons on the left, the widget panel on
// the right, search and dock at the bottom, and IceDash's own windows on top.
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import AboutPanel from '../panels/AboutPanel.vue'
import AppFrame from '../panels/AppFrame.vue'
import CredentialsPanel from '../panels/CredentialsPanel.vue'
import IceStorePanel from '../panels/IceStorePanel.vue'
import LogsPanel from '../panels/LogsPanel.vue'
import NotificationsPanel from '../panels/NotificationsPanel.vue'
import { firstLaunchFor, notesOpen, offline, panelOpen, setupFor, uninstallFor, updating, windows, type WindowKind } from '../store'
import { strings } from '../strings'
import AppMenu from './AppMenu.vue'
import AppWindow from './AppWindow.vue'
import FirstLaunchDialog from './FirstLaunchDialog.vue'
import SetupDialog from './SetupDialog.vue'
import UninstallDialog from './UninstallDialog.vue'
import DesktopIcons from './DesktopIcons.vue'
import Dock from './Dock.vue'
import Toasts from './Toasts.vue'
import TopBar from './TopBar.vue'
import WidgetPanel from './WidgetPanel.vue'

const widths: Record<WindowKind, number> = { about: 440, notifications: 380, icestore: 920, logs: 700, credentials: 520, app: 1080 }

/** Something is open: the rest fades so it stands out. */
const dimmed = computed(() => notesOpen.value || windows.some((w) => !w.min))

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
      <Dock />
    </div>

    <Transition name="fade">
      <div v-if="dimmed" class="dim" :class="{ over: notesOpen }" aria-hidden="true" />
    </Transition>
    <AppWindow v-for="(w, i) in windows" :key="w.id" :win="w" :index="i" :width="widths[w.kind]">
      <AboutPanel v-if="w.kind === 'about'" />
      <IceStorePanel v-else-if="w.kind === 'icestore'" />
      <LogsPanel v-else-if="w.kind === 'logs' && w.app" :name="w.app" />
      <CredentialsPanel v-else-if="w.kind === 'credentials' && w.app" :name="w.app" :win-id="w.id" />
      <AppFrame v-else-if="w.kind === 'app' && w.app" :name="w.app" />
    </AppWindow>
    <NotificationsPanel v-if="notesOpen" />
    <AppMenu />
    <SetupDialog v-if="setupFor" :id="setupFor" :key="setupFor" />
    <FirstLaunchDialog v-if="firstLaunchFor" :key="firstLaunchFor" :name="firstLaunchFor" />
    <UninstallDialog v-if="uninstallFor" :key="uninstallFor" :name="uninstallFor" />
    <Toasts />
  </div>
</template>

<style scoped>
.desktop {
  height: 100vh;
  height: 100dvh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
/* Under the windows (z 11 and up): the desktop and widgets fade; the top bar
   and dock stay bright. With the notification list open it covers windows too. */
.dim {
  position: fixed;
  inset: 0;
  z-index: 10;
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(2px) saturate(0.7);
  -webkit-backdrop-filter: blur(2px) saturate(0.7);
  pointer-events: none;
}
.dim.over {
  z-index: 215;
}
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
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
  min-height: 0;
  overflow-y: auto;
  scrollbar-width: none;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 312px;
  grid-template-rows: minmax(0, 1fr);
  gap: 20px;
  padding: 16px 20px 100px 16px;
}
.area::-webkit-scrollbar {
  display: none;
}
.area.no-panel {
  grid-template-columns: 1fr;
}
.widgets {
  align-self: start;
  max-height: 100%;
  overflow-y: auto;
  scrollbar-width: none;
}
.widgets::-webkit-scrollbar {
  display: none;
}
.bottom {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 14px;
  z-index: 40;
  display: grid;
  justify-items: center;
  gap: 10px;
  padding: 0 16px;
  pointer-events: none;
}
.bottom > * {
  pointer-events: auto;
}
@media (max-width: 900px) {
  .area {
    grid-template-columns: 1fr;
    grid-template-rows: none;
    padding: 14px 14px 100px;
  }
  .widgets {
    max-height: none;
  }
}
</style>
