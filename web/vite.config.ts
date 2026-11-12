/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { mock } from './dev/mock'

// `npm run dev` serves the app on this PC. Two ways to feed it:
//   ICEDASH_MOCK=1 npm run dev                       measured shapes, no box needed
//   ICEDASH_BOX=http://192.168.68.17 npm run dev     API calls go to a real box
// With a real box the ZimaOS token lives in the box's origin, not localhost's:
// copy it once with localStorage.setItem('access_token', '<token>').
const useMock = process.env.ICEDASH_MOCK === '1'
const box = process.env.ICEDASH_BOX ?? 'http://zimaos.local'

export default defineConfig({
  base: '/modules/icedash/',
  plugins: [vue(), ...(useMock ? [mock()] : [])],
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
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.ts'],
  },
})
