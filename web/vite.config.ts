/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { execSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { mock } from './dev/mock'

// `npm run dev` serves the app on this PC. Two ways to feed it:
//   npm run mock (or `make preview`)                 sample data shaped like a real box
//   ICEDASH_BOX=http://192.168.68.17 npm run dev     API calls go to a real box
// With a real box the ZimaOS token lives in the box's origin, not localhost's:
// copy it once with localStorage.setItem('access_token', '<token>').
const box = process.env.ICEDASH_BOX ?? 'http://zimaos.local'

// `npm run demo` builds the website's live demo into dist-demo/, served at
// /demo/: the API is answered in the browser (src/demo/), apps open demo/app.html.
function demoVersion() {
  try {
    return execSync('git describe --tags --abbrev=0 --match "v*"', { encoding: 'utf8' }).trim()
  } catch {
    return 'v1'
  }
}

export default defineConfig(({ mode }) => {
  const useMock = mode === 'mock' || process.env.ICEDASH_MOCK === '1'
  const demo = mode === 'demo'
  return {
    base: demo ? '/demo/' : '/modules/icedash/',
    define: demo ? { __DEMO_VERSION__: JSON.stringify(demoVersion()) } : {},
    plugins: [
      vue(),
      ...(useMock ? [mock()] : []),
      ...(demo
        ? [{
            name: 'icedash-demo-app-page',
            generateBundle() {
              this.emitFile({ type: 'asset', fileName: 'app.html', source: readFileSync('demo/app.html', 'utf8') })
            },
          }]
        : []),
    ],
    server: {
      proxy: useMock
        ? {}
        : {
            '/v1': { target: box, changeOrigin: true },
            '/v2': { target: box, changeOrigin: true },
          },
    },
    build: {
      target: 'es2022',
      assetsDir: 'assets',
      outDir: demo ? 'dist-demo' : 'dist',
    },
    test: {
      environment: 'jsdom',
      include: ['src/**/*.test.ts'],
    },
  }
})
