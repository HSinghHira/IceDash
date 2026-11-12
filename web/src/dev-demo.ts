// Dev only: opens a Phase 2 screen once the desktop has loaded (see main.ts).
import { apps, firstLaunchFor, menu, openIceStore, openWindow, setupFor } from './store'

export function run(demo: string) {
  const wait = setInterval(() => {
    if (!apps.value.length) return
    clearInterval(wait)
    if (demo === 'icestore') openIceStore()
    if (demo === 'setup') {
      openIceStore('Productivity')
      setupFor.value = 'com.icestore.documenso'
    }
    if (demo === 'menu') setTimeout(() => (menu.value = { name: 'appflowy', x: 230, y: 230 }), 1500)
    if (demo === 'creds') openWindow('credentials', { name: 'appflowy', title: 'AppFlowy' })
    if (demo === 'first') firstLaunchFor.value = 'yuvomi'
  }, 200)
}
