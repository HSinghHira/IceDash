/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// `npm run dev` serves the app on this PC and sends API calls to a real box:
//   ICEDASH_BOX=http://192.168.68.17 npm run dev
// The ZimaOS token lives in the box's origin, not localhost's: for dev, copy it
// once into this origin with localStorage.setItem('access_token', '<token>').
const box = process.env.ICEDASH_BOX ?? 'http://zimaos.local'

export default defineConfig({
  base: '/modules/icedash/',
  plugins: [vue()],
  server: {
    proxy: {
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
  },
})
