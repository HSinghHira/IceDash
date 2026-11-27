import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { AppInfo, CatalogApp } from '../../api/apps'
import AppMenu from '../../shell/AppMenu.vue'
import GoBackDialog from '../../shell/GoBackDialog.vue'
import { appInfo, apps, catalogList, goBackFor, iceStoreView, jobs, menu, toasts, user, zone } from '../../store'
import IceStorePanel from '../IceStorePanel.vue'
import { button, savedZone } from './actions'

const app = (id: string, title: string): CatalogApp => ({
  id: `com.icestore.${id}`,
  name: id,
  title,
  tagline: '',
  category: 'Tools',
  categories: [],
  author: '',
  developer: '',
  architectures: ['amd64', 'arm64'],
  icon: '',
  thumbnail: '',
  version: '2',
  content_hash: '',
})

const info = (name: string, o: Partial<AppInfo> = {}): AppInfo => ({
  name,
  store_id: `com.icestore.${name}`,
  icestore: true,
  version: '1',
  latest: '2',
  update: true,
  first_launch: false,
  ...o,
})

let calls: { path: string; body: unknown }[] = []
let fail = ''

beforeEach(() => {
  calls = []
  fail = ''
  vi.stubGlobal('fetch', async (path: string, init?: RequestInit) => {
    const body = init?.body ? JSON.parse(String(init.body)) : undefined
    calls.push({ path, body })
    if (fail && path.includes(fail)) return new Response('{"code":"busy","error":"that app is already being installed or updated"}', { status: 409 })
    if (path.endsWith('/settings')) return new Response('{"auto_check":true,"zone":"Pacific/Auckland"}')
    if (path.endsWith('/rollback'))
      return new Response(JSON.stringify({ name: 'beta', title: 'Beta', icon: '', kind: 'rollback', task_id: '', state: 'running', phase: 'accepted', started: '' }), { status: 202 })
    if (path.endsWith('/auto')) return new Response(JSON.stringify({ name: 'x', auto: body?.on }))
    const m = path.match(/\/catalog\/([^/?]+)$/)
    if (m) return new Response(JSON.stringify({ app: catalogList.value!.apps.find((a) => a.id === decodeURIComponent(m[1]!)), meta: {} }))
    return new Response('{"code":"not_found"}', { status: 404 })
  })
  user.value = { name: 'hira', admin: true }
  zone.value = 'Pacific/Auckland'
  savedZone.value = null
  catalogList.value = { updated_at: '', fetched_at: '', apps: [app('gamma', 'Gamma'), app('beta', 'Beta'), app('alpha', 'Alpha')] }
  appInfo.value = {
    alpha: info('alpha', { auto: true }),
    beta: info('beta', { latest: '1', update: false, back: '0.9' }),
    gamma: info('gamma', { held: '2', held_by: 'night', auto: true }),
  }
  apps.value = [{ id: 'beta', name: 'Beta', icon: '', url: '', status: 'running' } as never]
  jobs.value = []
  toasts.value = []
  goBackFor.value = null
  iceStoreView.value = 'updates'
})
afterEach(() => {
  vi.unstubAllGlobals()
  document.body.innerHTML = ''
})

describe('Update at night', () => {
  it('lists every IceStore app with a switch, and Update all leaves held versions out', async () => {
    const w = mount(IceStorePanel)
    await flushPromises()
    const rows = w.findAll('.auto-row')
    expect(rows.map((r) => r.find('.title').text())).toEqual(['Alpha', 'Beta', 'Gamma'])
    expect(rows.map((r) => r.find('.auto-titles .mut').text())).toEqual([
      '2 installs tonight',
      '1',
      '2 didn’t start properly, so it won’t install by itself.',
    ])
    expect(rows[0]!.find('.switch').attributes('aria-checked')).toBe('true')
    expect(rows[0]!.find('.switch').attributes('aria-label')).toBe('Update Alpha at night')
    expect(w.find('.night .mut').text()).toContain('Auckland')
    expect(w.find('.page-head .pill.update').text()).toBe('Update all (1)')

    user.value = { name: 'guest', admin: false }
    await flushPromises()
    expect(w.find('.night').exists()).toBe(false)
  })

  it('saves the switch with the clock’s zone, and puts it back when that fails', async () => {
    const w = mount(IceStorePanel)
    await flushPromises()
    await w.findAll('.auto-row')[1]!.find('.switch').trigger('click')
    await flushPromises()
    const call = calls.find((c) => c.path.endsWith('/apps/beta/auto'))
    expect(call?.body).toEqual({ on: true, zone: 'Pacific/Auckland' })
    expect(appInfo.value.beta!.auto).toBe(true)

    fail = '/apps/alpha/auto'
    await w.findAll('.auto-row')[0]!.find('.switch').trigger('click')
    await flushPromises()
    expect(appInfo.value.alpha!.auto).toBe(true)
    expect(toasts.value.at(-1)?.title).toBe('Update at night')
  })

  it('says so when nothing waits', async () => {
    appInfo.value = { beta: info('beta', { latest: '1', update: false }) }
    const w = mount(IceStorePanel)
    await flushPromises()
    expect(w.find('.empty').text()).toBe('All your apps are up to date.')
  })
})

describe('Go back', () => {
  it('shows the previous version on the app page, to admins only', async () => {
    iceStoreView.value = 'all'
    const w = mount(IceStorePanel, { attachTo: document.body })
    await w.find('.row[data-app="com.icestore.beta"]').trigger('click')
    await flushPromises()
    expect(w.find('.upd').text()).toContain('Previous version: 0.9')
    await w.find('.upd .goback').trigger('click')
    expect(goBackFor.value).toBe('beta')

    user.value = { name: 'guest', admin: false }
    await flushPromises()
    expect(w.find('.upd').exists()).toBe(false)
    w.unmount()
  })

  it('asks first, then starts the job', async () => {
    goBackFor.value = 'beta'
    const w = mount(GoBackDialog, { props: { name: 'beta' }, attachTo: document.body })
    expect(w.text()).toContain('Take Beta back to 0.9?')
    expect(w.text()).toContain('Only the app goes back, not its data.')
    expect(w.text()).toContain('1 won’t install by itself again.')
    await w.findAll('button').find((b) => b.text() === 'Go back to 0.9')!.trigger('click')
    await flushPromises()
    expect(calls.some((c) => c.path.endsWith('/apps/beta/rollback'))).toBe(true)
    expect(goBackFor.value).toBe(null)
    expect(toasts.value.at(-1)?.title).toBe('Going back…')
    expect(button(app('beta', 'Beta')).label).toBe('Going back…')
    w.unmount()
  })

  it('keeps the dialog open with the error', async () => {
    fail = '/rollback'
    goBackFor.value = 'beta'
    const w = mount(GoBackDialog, { props: { name: 'beta' }, attachTo: document.body })
    await w.findAll('button').find((b) => b.text() === 'Go back to 0.9')!.trigger('click')
    await flushPromises()
    expect(w.find('[role="alert"]').text()).toContain('already being installed')
    expect(goBackFor.value).toBe('beta')
    w.unmount()
  })

  it('is in the right-click menu after Update, for admins with a kept version', async () => {
    menu.value = { name: 'beta', x: 10, y: 10 }
    const w = mount(AppMenu, { attachTo: document.body })
    await flushPromises()
    const items = w.findAll('[role="menuitem"]').map((b) => b.text())
    expect(items).toContain('Go back to 0.9')
    await w.findAll('[role="menuitem"]').find((b) => b.text() === 'Go back to 0.9')!.trigger('click')
    expect(goBackFor.value).toBe('beta')
    await flushPromises()
    menu.value = { name: 'beta', x: 10, y: 10 }
    await flushPromises()
    user.value = { name: 'guest', admin: false }
    await flushPromises()
    expect(w.findAll('[role="menuitem"]').map((b) => b.text())).not.toContain('Go back to 0.9')
    w.unmount()
    menu.value = null
  })
})
