import { createApp } from 'vue'
import App from './App.vue'
// Fonts ship inside the image: the dashboard makes no calls to other sites.
import '@fontsource-variable/inter/wght.css'
import '@fontsource/unbounded/latin-800.css'
import './styles/tokens.css'
import './styles/base.css'
import { applyRememberedTheme } from './store'
import { rememberedLang, useLanguage } from './strings'

// The last theme first, so a reload doesn't flash Ice.
applyRememberedTheme()

// The language before anything is drawn (Phase 5b); the store checks the
// user's saved choice once signed in and reloads if it differs.
void useLanguage(rememberedLang()).finally(() => createApp(App).mount('#app'))

// Dev only (`npm run dev`): ?demo=icestore|setup|menu|creds|first|notes|about|app opens that
// screen, for screenshots against the mock. Dropped from builds.
if (import.meta.env.DEV) {
  const demo = new URLSearchParams(location.search).get('demo')
  if (demo) void import('./dev-demo').then((m) => m.run(demo))
}
