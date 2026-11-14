<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { signOut } from '../api/client'
import Icon from '../components/Icon.vue'
import { initials } from '../lib/format'
import { dashboardUrl, host } from '../lib/host'
import { clockParts } from '../lib/timezone'
import { accountMenu, bellMenu, clockMenu, topBarMenu } from '../menus'
import {
  activeWindow,
  appUpdates,
  avatarUrl,
  notesOpen,
  openContext,
  openIceStore,
  openSettings,
  openWindow,
  panelOpen,
  h24,
  togglePanel,
  refreshUpdate,
  unread,
  updateCount,
  updateWaiting,
  user,
  zone,
} from '../store'
import { strings } from '../strings'
import { checkUpdate } from '../api/icedash'

const props = defineProps<{ now: Date }>()
const menu = ref<'' | 'icedash' | 'account'>('')

const clock = computed(() => clockParts(props.now, zone.value, h24.value))


function toggle(which: 'icedash' | 'account') {
  menu.value = menu.value === which ? '' : which
}
function close() {
  menu.value = ''
}
async function checkForUpdates() {
  close()
  openSettings('about')
  if (user.value?.admin) {
    await checkUpdate().catch(() => undefined)
    await refreshUpdate()
  }
}
function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') close()
}
function onDocClick(e: MouseEvent) {
  if (!(e.target as HTMLElement).closest('.menu-wrap')) close()
}
onMounted(() => {
  document.addEventListener('keydown', onKey)
  document.addEventListener('click', onDocClick)
})
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKey)
  document.removeEventListener('click', onDocClick)
})
</script>

<template>
  <header class="topbar" @contextmenu.self="openContext($event, strings.ctxTopBar, topBarMenu())">
    <div class="left">
      <div class="menu-wrap">
        <button
          class="brand"
          type="button"
          :aria-expanded="menu === 'icedash'"
          aria-haspopup="menu"
          :aria-label="strings.menuLabel"
          @click="toggle('icedash')"
        >
          <Icon name="snowflake" :size="18" />
          <span>{{ strings.appName }}</span>
        </button>
        <div v-if="menu === 'icedash'" class="menu" role="menu">
          <button role="menuitem" type="button" @click="(close(), openSettings('about'))">{{ strings.menuAbout }}</button>
          <button role="menuitem" type="button" @click="checkForUpdates">{{ strings.menuCheck }}</button>
          <a role="menuitem" :href="dashboardUrl()" target="_blank" rel="noopener" @click="close">{{ strings.menuDashboard(host.name) }}</a>
          <hr />
          <button role="menuitem" type="button" @click="signOut">{{ strings.menuSignOut }}</button>
        </div>
      </div>
      <span class="active">{{ activeWindow?.title ?? strings.desktop }}</span>
    </div>

    <div class="right">

      <button
        v-if="updateCount"
        class="badge mono"
        type="button"
        @click="updateWaiting || !appUpdates.length ? openSettings('about') : openIceStore('updates')"
      >
        <Icon name="refresh" :size="15" />
        <span class="hide-sm">{{ strings.updatesBadge(updateCount) }}</span>
      </button>
      <button class="bell" type="button" :aria-label="strings.notificationsButton(unread)" @click="notesOpen = !notesOpen" @contextmenu="openContext($event, strings.notifications, bellMenu())">
        <Icon name="bell" :size="18" />
        <span v-if="unread" class="dot" aria-hidden="true" />
      </button>
      <div class="menu-wrap">
        <button
          class="avatar"
          type="button"
          :aria-label="strings.accountButton(user?.name ?? '')"
          :aria-expanded="menu === 'account'"
          aria-haspopup="menu"
          @click="toggle('account')"
          @contextmenu="openContext($event, user?.name ?? '', accountMenu())"
        >
          <img v-if="avatarUrl" :src="avatarUrl" alt="" />
          <template v-else>{{ initials(user?.name ?? '') }}</template>
        </button>
        <div v-if="menu === 'account'" class="menu end" role="menu">
          <div class="who">
            <span class="big avatar" aria-hidden="true">
              <img v-if="avatarUrl" :src="avatarUrl" alt="" />
              <template v-else>{{ initials(user?.name ?? '') }}</template>
            </span>
            <span>
              <strong>{{ user?.name }}</strong>
              <span class="role">{{ user?.admin ? strings.roleAdmin : strings.roleMember }}</span>
            </span>
          </div>
          <button role="menuitem" type="button" @click="(close(), openSettings('users'))">{{ strings.ctxChangePicture }}</button>
          <button role="menuitem" type="button" @click="(close(), openSettings('appearance'))">{{ strings.ctxAppearance }}</button>
          <hr />
          <button role="menuitem" type="button" @click="signOut">{{ strings.menuSignOut }}</button>
        </div>
      </div>
      <button class="clock" type="button" :aria-pressed="panelOpen" :title="strings.togglePanel" @click="togglePanel()" @contextmenu="openContext($event, strings.ctxClock, clockMenu())">
        <span class="hide-sm">{{ clock.day }}&nbsp;</span>{{ clock.time }}<template v-if="clock.ampm">&nbsp;{{ clock.ampm }}</template>
      </button>
    </div>
  </header>
</template>

<style scoped>
.topbar {
  position: sticky;
  top: 0;
  z-index: 50;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 0 12px;
  background: rgba(var(--bar-rgb), 0.55);
  backdrop-filter: var(--blur);
  -webkit-backdrop-filter: var(--blur);
  border-bottom: 1px solid var(--line);
  font-size: 13.5px;
}
.left,
.right {
  display: flex;
  align-items: center;
  gap: 18px;
  min-width: 0;
}
.right {
  gap: 16px;
}
button {
  background: none;
  border: 0;
  padding: 0;
  cursor: pointer;
}
.brand {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--ac2);
  border-radius: 8px;
  padding: 4px 6px;
}
.brand span {
  color: var(--text);
  font-family: var(--font-display);
  font-weight: 800;
  font-size: 14px;
  letter-spacing: 0.01em;
}
.brand:hover,
.brand[aria-expanded='true'] {
  background: rgba(var(--fg-rgb), 0.07);
}
.active {
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.mono {
  font-family: var(--font-mono);
  font-size: 12.5px;
}

.mut {
  color: var(--mut);
}
.badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--ac2);
  border-radius: var(--radius-pill);
  padding: 3px 8px;
}
.badge:hover {
  background: rgba(var(--ac-rgb), 0.12);
}
.bell {
  position: relative;
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  border-radius: 8px;
}
.bell:hover {
  background: rgba(var(--fg-rgb), 0.07);
}
.dot {
  position: absolute;
  top: 4px;
  right: 4px;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--ac);
  box-shadow: 0 0 8px var(--ac);
}
.avatar {
  display: grid;
  place-items: center;
  width: 26px;
  height: 26px;
  overflow: hidden;
  border-radius: 50%;
  background: var(--ac);
  color: var(--ink);
  font-size: 11px;
  font-weight: 700;
}
.avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.avatar.big {
  flex: none;
  width: 40px;
  height: 40px;
  font-size: 14px;
}
.clock {
  font-weight: 600;
  white-space: nowrap;
  border-radius: 8px;
  padding: 4px 6px;
}
.clock:hover {
  background: rgba(var(--fg-rgb), 0.07);
}
.menu-wrap {
  position: relative;
}
.menu {
  position: absolute;
  top: calc(100% + 8px);
  left: 0;
  min-width: 230px;
  padding: 6px;
  display: grid;
  background: rgba(var(--surface-rgb), 0.92);
  backdrop-filter: var(--blur);
  -webkit-backdrop-filter: var(--blur);
  border: 1px solid var(--line);
  border-radius: 14px;
  box-shadow: 0 18px 50px rgba(0, 0, 0, 0.6);
}
.menu.end {
  left: auto;
  right: 0;
  min-width: 180px;
}
.menu button,
.menu a {
  text-align: left;
  padding: 8px 10px;
  border-radius: 8px;
  text-decoration: none;
  font-size: 14px;
}
.menu button:hover,
.menu a:hover,
.menu button:focus-visible,
.menu a:focus-visible {
  background: rgba(var(--ac-rgb), 0.14);
}
.menu hr {
  border: 0;
  border-top: 1px solid var(--line);
  margin: 4px 2px;
}
.who {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 4px 6px 6px;
  font-size: 13px;
}
.who strong {
  display: block;
  font-size: 14px;
}
.role {
  color: var(--mut);
  font-size: 12px;
}
.menu.end {
  min-width: 220px;
}
@media (max-width: 760px) {
  .hide-sm,
  .active {
    display: none;
  }
  .topbar,
  .right {
    gap: 8px;
  }
  .topbar {
    padding: 0 8px;
  }
}
</style>
