import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from './App.vue'
import { openSettings, stop, update, updating, waitForNewVersion } from './store'

const enc = (o: object) => btoa(JSON.stringify(o)).replace(/=+$/, '').replace(/\+/g, '-').replace(/\//g, '_')
const adminToken = `${enc({ alg: 'ES256' })}.${enc({ username: 'Hira', role: 'admin', id: 1 })}.sig`

const answers: Record<string, unknown> = {
  '/v2/icedash/health': { version: '0.2.0', arch: 'amd64', started: '2026-11-12T00:00:00Z' },
  '/v2/icedash/system': { down: 1000, up: 500, history: [], interfaces: ['wlan0'], uptime_sec: 3600, device: 'ZimaCube' },
  '/v2/icedash/notifications?since=0': { items: [], unread: 0 },
  '/v2/icedash/update': { current: '0.2.0', state: 'idle', steps: [] },
  '/v2/icedash/settings': { auto_check: true },
  '/v2/app_management/web/appgrid': {
    data: [
      { name: 'homeassistant', title: { en_US: 'Home Assistant' }, port: '8123', status: 'running' },
      { name: 'gitsave', title: { custom: 'GitSave' }, port: '6181', status: 'exited' },
    ],
  },
  '/v1/sys/utilization': {
    data: {
      cpu: { percent: 12, num: 4 },
      mem: { total: 16e9, used: 4e9, available: 12e9, usedPercent: 25 },
      sys_disk: { size: 238e9, used: 140e9, avail: 98e9 },
    },
  },
  '/v1/users/current/custom/system': { data: { lang: 'en_US' } },
  '/v2/app_management/appstore': { data: [] },
}

function stubFetch() {
  vi.stubGlobal('fetch', async (path: string) => {
    if (path.includes(':7681')) throw new TypeError('no terminal')
    const body = answers[path]
    return new Response(body === undefined ? '{"code":"not_found"}' : JSON.stringify(body), { status: body === undefined ? 404 : 200 })
  })
}

afterEach(() => {
  stop()
  localStorage.clear()
  vi.unstubAllGlobals()
})

describe('App', () => {
  it('asks to sign in to ZimaOS when there is no token', async () => {
    stubFetch()
    const w = mount(App)
    await flushPromises()
    expect(w.text()).toContain('Sign in to ZimaOS first')
    w.unmount()
  })

  it('shows the desktop with the apps from ZimaOS', async () => {
    localStorage.setItem('access_token', adminToken)
    stubFetch()
    const w = mount(App, { attachTo: document.body })
    await flushPromises()
    await flushPromises()
    const text = w.text()
    expect(text).toContain('Home Assistant')
    expect(text).toContain('GitSave')
    expect(w.find('nav.dock').text()).toContain('Files')
    expect(w.find('a[href="http://localhost:8123/"]').exists()).toBe(true)
    expect(w.find('.status.stopped').exists()).toBe(true)
    expect(w.find('.avatar').text()).toBe('HI')
    w.unmount()
  })

  it('shows the update result that arrived while the page reloaded', async () => {
    localStorage.setItem('access_token', adminToken)
    sessionStorage.setItem('icedash-seen-note', '1')
    const before = answers['/v2/icedash/notifications?since=0']
    answers['/v2/icedash/notifications?since=0'] = {
      items: [
        { id: 2, time: '2026-11-12T08:08:07Z', kind: 'updated', version: 'v0.2.1', title: 'Updated to v0.2.1', body: 'IceDash is now running v0.2.1.', read: false },
        { id: 1, time: '2026-11-12T08:07:59Z', kind: 'update_available', version: 'v0.2.1', title: 'Update available', body: '', read: false },
      ],
      unread: 2,
    }
    try {
      stubFetch()
      const w = mount(App, { attachTo: document.body })
      await flushPromises()
      await flushPromises()
      const toasts = w.findAll('.toast').map((t) => t.text())
      expect(toasts).toHaveLength(1)
      expect(toasts[0]).toContain('v0.2.1')
      expect(sessionStorage.getItem('icedash-seen-note')).toBeNull()
      w.unmount()
    } finally {
      answers['/v2/icedash/notifications?since=0'] = before
    }
  })

  it('stops waiting when the previous version comes back after a rollback', async () => {
    vi.useFakeTimers()
    try {
      stubFetch()
      const done = waitForNewVersion('0.2.0', '2026-11-11T00:00:00Z')
      expect(updating.value).toBe(true)
      await vi.advanceTimersByTimeAsync(2500)
      await done
      expect(updating.value).toBe(false)
      expect(update.value?.state).toBe('idle')
    } finally {
      vi.useRealTimers()
    }
  })
})

describe('Phase 2 app menu', () => {
  const memberToken = `${enc({ alg: 'ES256' })}.${enc({ username: 'testmember', role: 'user', id: 2 })}.sig`

  async function desktop(tok: string) {
    localStorage.setItem('access_token', tok)
    answers['/v2/icedash/apps'] = {
      items: [{ name: 'homeassistant', store_id: 'com.icestore.homeassistant', icestore: true, version: '1', latest: '2', update: true, first_launch: false }],
    }
    answers['/v2/icedash/jobs'] = { items: [] }
    stubFetch()
    const w = mount(App, { attachTo: document.body })
    await flushPromises()
    await flushPromises()
    return w
  }

  it('opens on right-click with the admin actions', async () => {
    const w = await desktop(adminToken)
    await w.find('a[href="http://localhost:8123/"]').trigger('contextmenu', { clientX: 100, clientY: 100 })
    await flushPromises()
    const menu = document.querySelector('.menu')
    expect(menu).not.toBeNull()
    const items = [...menu!.querySelectorAll('[role="menuitem"]')].map((b) => b.textContent?.trim())
    expect(items).toEqual(['Open', 'Open in new tab', 'Credentials', 'Logs', 'Restart', 'Stop', 'Update to 2', 'Settings', 'Back up data', 'Uninstall'])
    // The update badge shows on the icon.
    expect(w.find('.upd').exists()).toBe(true)
    w.unmount()
  })

  it('gives a member only Open', async () => {
    const w = await desktop(memberToken)
    await w.find('a[href="http://localhost:8123/"]').trigger('contextmenu', { clientX: 100, clientY: 100 })
    await flushPromises()
    const items = [...document.querySelectorAll('.menu [role="menuitem"]')].map((b) => b.textContent?.trim())
    expect(items).toEqual(['Open', 'Open in new tab'])
    w.unmount()
  })
})

describe('Desktop layout', () => {
  it('puts apps on the desktop and IceDash launchers in the dock', async () => {
    localStorage.setItem('access_token', adminToken)
    stubFetch()
    const w = mount(App, { attachTo: document.body })
    await flushPromises()
    await flushPromises()
    const desk = w.find('nav.icons').text()
    const dock = w.find('nav.dock').text()
    expect(desk).toContain('Home Assistant')
    expect(desk).not.toContain('Files')
    expect(dock).toContain('Files')
    expect(dock).not.toContain('Home Assistant')
    w.unmount()
  })

  it('drops notifications from the bell and closes them with the close dot', async () => {
    localStorage.setItem('access_token', adminToken)
    stubFetch()
    const w = mount(App, { attachTo: document.body })
    await flushPromises()
    await w.find('button.bell').trigger('click')
    expect(w.find('.center').exists()).toBe(true)
    await w.find('.center .dot.close').trigger('click')
    expect(w.find('.center').exists()).toBe(false)
    w.unmount()
  })

  it('closes, folds and widens windows with the dots', async () => {
    localStorage.setItem('access_token', adminToken)
    stubFetch()
    const w = mount(App, { attachTo: document.body })
    await flushPromises()
    openSettings('about')
    await flushPromises()
    const win = () => w.find('section.win')
    await win().find('.dot.min').trigger('click')
    expect(win().isVisible()).toBe(false)
    await w.find('[data-dock-win="settings"]').trigger('click')
    expect(win().isVisible()).toBe(true)
    await win().find('.dot.max').trigger('click')
    expect(win().classes()).toContain('max')
    await win().find('.dot.close').trigger('click')
    expect(w.find('section.win').exists()).toBe(false)
    w.unmount()
  })
})

describe('Apps in windows', () => {
  it('opens an app in a window, minimizes it to the dock and brings it back', async () => {
    localStorage.setItem('access_token', adminToken)
    answers['/v2/icedash/apps/homeassistant/frame'] = { name: 'homeassistant', frameable: true }
    stubFetch()
    const w = mount(App, { attachTo: document.body })
    await flushPromises()
    await flushPromises()
    await w.find('a[href="http://localhost:8123/"]').trigger('click', { button: 0 })
    await flushPromises()
    const win = () => w.find('section.win.app')
    expect(win().exists()).toBe(true)
    expect(win().find('iframe').attributes('src')).toBe('http://localhost:8123/')
    expect(win().find('a.tab').attributes('href')).toBe('http://localhost:8123/')
    // Minimize: hidden, with its icon in the dock.
    await win().find('.dot.min').trigger('click')
    expect(win().isVisible()).toBe(false)
    const slot = w.find('[data-dock-win="app:homeassistant"]')
    expect(slot.exists()).toBe(true)
    await slot.trigger('click')
    expect(win().isVisible()).toBe(true)
    w.unmount()
  })

  it('opens an app that refuses frames in a new tab', async () => {
    localStorage.setItem('access_token', adminToken)
    answers['/v2/icedash/apps/homeassistant/frame'] = { name: 'homeassistant', frameable: false }
    stubFetch()
    const opened: string[] = []
    vi.stubGlobal('open', (u: string) => (opened.push(u), null))
    const w = mount(App, { attachTo: document.body })
    await flushPromises()
    await flushPromises()
    await flushPromises()
    await w.find('a[href="http://localhost:8123/"]').trigger('click', { button: 0 })
    expect(opened).toEqual(['http://localhost:8123/'])
    expect(w.find('section.win.app').exists()).toBe(false)
    w.unmount()
  })
})
