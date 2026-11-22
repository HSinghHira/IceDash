import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { CatalogApp } from '../../api/apps'
import { host } from '../../lib/host'
import { appInfo, catalogList, iceStoreSort, iceStoreView, showOtherDevices, uninstallFor, user } from '../../store'
import IceStorePanel from '../IceStorePanel.vue'

const app = (id: string, o: Partial<CatalogApp> = {}): CatalogApp => ({
  id: `com.icestore.${id}`,
  name: id,
  title: id[0]!.toUpperCase() + id.slice(1),
  tagline: `${id} does things`,
  category: 'Tools',
  categories: [],
  author: '',
  developer: '',
  architectures: ['amd64', 'arm64'],
  icon: '',
  thumbnail: '',
  version: '2',
  content_hash: '',
  ...o,
})

const apps = [
  app('alpha', { stars: 10, added: 100 }),
  app('beta', { stars: 500, added: 300, featured: true, thumbnail: 'https://x/beta.webp' }),
  app('gamma', { stars: 50, added: 200 }),
  app('intelonly', { architectures: ['amd64'] }),
]

function stubFetch() {
  vi.stubGlobal('fetch', async (path: string) => {
    const m = path.match(/\/catalog\/([^/?]+)$/)
    const a = m && apps.find((x) => x.id === decodeURIComponent(m[1]!))
    if (a) {
      const meta = { description: 'About it', screenshot_link: ['https://x/1.webp', 'https://x/2.webp', 'https://x/3.webp'] }
      return new Response(JSON.stringify({ app: a, meta }))
    }
    return new Response('{"code":"not_found"}', { status: 404 })
  })
}

const cardIds = (w: ReturnType<typeof mount>) => w.findAll('.cards > .card').map((c) => c.attributes('data-app'))

beforeEach(() => {
  stubFetch()
  host.arch = 'arm64'
  user.value = { name: 'hira', admin: true }
  catalogList.value = { updated_at: '', fetched_at: '', apps } as never
  appInfo.value = {}
  iceStoreView.value = 'all'
  iceStoreSort.value = 'az'
  showOtherDevices.value = false
  uninstallFor.value = null
})
afterEach(() => {
  vi.unstubAllGlobals()
  host.arch = ''
  document.body.innerHTML = ''
})

describe('IceStore', () => {
  it('sorts the list as chosen', async () => {
    const w = mount(IceStorePanel)
    expect(cardIds(w)).toEqual(['com.icestore.alpha', 'com.icestore.beta', 'com.icestore.gamma'])
    await w.find('.sort select').setValue('stars')
    expect(iceStoreSort.value).toBe('stars')
    expect(cardIds(w)).toEqual(['com.icestore.beta', 'com.icestore.gamma', 'com.icestore.alpha'])
    await w.find('.sort select').setValue('za')
    expect(cardIds(w)).toEqual(['com.icestore.gamma', 'com.icestore.beta', 'com.icestore.alpha'])
  })

  it('shows Featured and Recently added only on All apps without a search', async () => {
    const w = mount(IceStorePanel)
    expect(w.findAll('.featured .feat').map((c) => c.attributes('data-app'))).toEqual(['com.icestore.beta'])
    expect(w.findAll('.mini-app').map((c) => c.attributes('data-app'))).toEqual(['com.icestore.beta', 'com.icestore.gamma', 'com.icestore.alpha'])
    await w.find('.search input').setValue('alp')
    expect(w.find('.featured').exists()).toBe(false)
    expect(w.find('.newest').exists()).toBe(false)
    expect(cardIds(w)).toEqual(['com.icestore.alpha'])
  })

  it('hides apps for other CPUs until asked', async () => {
    const w = mount(IceStorePanel)
    expect(cardIds(w)).not.toContain('com.icestore.intelonly')
    expect(w.find('.other').text()).toContain('1 app for other devices is hidden.')
    await w.find('.other input').setValue(true)
    expect(cardIds(w)).toContain('com.icestore.intelonly')
    expect(w.find('[data-app="com.icestore.intelonly"]').text()).toContain('Not for this device')
  })

  it('offers Uninstall on an installed app’s page, to admins only', async () => {
    appInfo.value = { alpha: { name: 'alpha', store_id: 'com.icestore.alpha', icestore: true, version: '2', update: false, first_launch: false } }
    const w = mount(IceStorePanel, { attachTo: document.body })
    await w.find('[data-app="com.icestore.alpha"] .card-hit').trigger('click')
    await flushPromises()
    const btn = w.findAll('.page .actions button').find((b) => b.text() === 'Uninstall')
    expect(btn).toBeDefined()
    await btn!.trigger('click')
    expect(uninstallFor.value).toBe('alpha')

    user.value = { name: 'guest', admin: false }
    await flushPromises()
    expect(w.findAll('.page .actions button').some((b) => b.text() === 'Uninstall')).toBe(false)
    w.unmount()
  })

  it('shows Update all in Updates, to admins only', async () => {
    appInfo.value = { gamma: { name: 'gamma', store_id: 'com.icestore.gamma', icestore: true, version: '1', latest: '2', update: true, first_launch: false } }
    iceStoreView.value = 'updates'
    const w = mount(IceStorePanel)
    expect(cardIds(w)).toEqual(['com.icestore.gamma'])
    expect(w.find('.all-head .pill.update').text()).toBe('Update all (1)')
    user.value = { name: 'guest', admin: false }
    await flushPromises()
    expect(w.find('.all-head .pill.update').exists()).toBe(false)
  })

  it('opens screenshots full size and steps through them with the arrow keys', async () => {
    const w = mount(IceStorePanel, { attachTo: document.body })
    await w.find('[data-app="com.icestore.gamma"] .card-hit').trigger('click')
    await flushPromises()
    await w.findAll('.shots .shot')[1]!.trigger('click')
    await flushPromises()
    const viewer = document.querySelector<HTMLElement>('.viewer')!
    expect(viewer.querySelector('.count')!.textContent).toBe('2 / 3')
    viewer.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }))
    await flushPromises()
    expect(viewer.querySelector('.count')!.textContent).toBe('3 / 3')
    viewer.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }))
    await flushPromises()
    expect(viewer.querySelector('.count')!.textContent).toBe('1 / 3')
    viewer.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await flushPromises()
    expect(document.querySelector('.viewer')).toBeNull()
    w.unmount()
  })
})
