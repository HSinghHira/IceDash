import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from './App.vue'
import { stop } from './store'

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
    expect(text).toContain('Files')
    expect(w.find('a[href="http://localhost:8123/"]').exists()).toBe(true)
    expect(w.find('.status.stopped').exists()).toBe(true)
    expect(w.find('.avatar').text()).toBe('HI')
    w.unmount()
  })
})
