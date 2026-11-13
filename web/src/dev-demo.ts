// Dev only: opens a Phase 2 screen once the desktop has loaded (see main.ts).
import { install } from './api/apps'
import { apps, firstLaunchFor, menu, notesOpen, openIceStore, openWindow, setupFor, trackJob } from './store'

export function run(demo: string) {
  const wait = setInterval(() => {
    if (!apps.value.length) return
    clearInterval(wait)
    if (demo === 'icestore') openIceStore()
    if (demo === 'setup') {
      openIceStore('Productivity')
      setupFor.value = 'com.icestore.documenso'
    }
    // The setup popup with its port taken by AppFlowy (6101).
    if (demo === 'clash') setupFor.value = 'com.icestore.clash'
    if (demo === 'installing') void install('com.icestore.documenso', { values: {}, ports: {} }).then(trackJob)
    if (demo === 'menu') setTimeout(() => (menu.value = { name: 'appflowy', x: 230, y: 230 }), 1500)
    if (demo === 'creds') openWindow('credentials', { name: 'appflowy', title: 'AppFlowy' })
    if (demo === 'first') firstLaunchFor.value = 'yuvomi'
    if (demo === 'notes') notesOpen.value = true
    if (demo === 'about') openWindow('about')
    if (demo === 'app') {
      openWindow('app', { name: 'homeassistant', title: 'Home Assistant' })
      openWindow('about')
    }
  }, 200)
}
