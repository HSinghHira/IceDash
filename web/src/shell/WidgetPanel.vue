<script setup lang="ts">
// The four built-in widgets (PLAN.md): Clock, System, Network and Apps.
// Not configurable in Phase 1; Phase 4 turns them into widgets of the SDK.
import { computed } from 'vue'
import Icon from '../components/Icon.vue'
import { counts } from '../lib/apps'
import { duration, pct, speed, usedOfTotal } from '../lib/format'
import { dashboardUrl } from '../lib/host'
import { ICESTORE_SITE } from '../lib/launchers'
import { cityOf, clockParts } from '../lib/timezone'
import { appUpdates, apps, iceStoreInZimaOS, openIceStore, sys, util, zone } from '../store'
import { strings } from '../strings'

const props = defineProps<{ now: Date }>()

const clock = computed(() => clockParts(props.now, zone.value))
const city = computed(() => cityOf(zone.value))

const meters = computed(() => {
  const u = util.value
  if (!u) return []
  const memUsed = u.mem.total - u.mem.available
  const mem = usedOfTotal(memUsed, u.mem.total)
  const disk = usedOfTotal(u.sys_disk.used, u.sys_disk.size, 1000)
  return [
    {
      id: 'cpu',
      label: strings.cpu,
      value: `${Math.round(u.cpu.percent)}% · ${strings.cores(u.cpu.num)}`,
      pct: pct(u.cpu.percent, 100),
    },
    { id: 'mem', label: strings.memory, value: `${strings.of(mem.used, `${mem.total} ${mem.unit}`)}`, pct: pct(memUsed, u.mem.total) },
    {
      id: 'disk',
      label: strings.storage,
      value: `${strings.of(disk.used, `${disk.total} ${disk.unit}`)}`,
      pct: pct(u.sys_disk.used, u.sys_disk.size),
    },
  ]
})

const systemLine = computed(() => {
  const s = sys.value
  if (!s) return ''
  return [s.device, s.uptime_sec ? strings.upFor(duration(s.uptime_sec)) : ''].filter(Boolean).join(' · ')
})

// The sparkline: download as the bright line, upload as the quiet one.
const W = 280
const H = 64
const chart = computed(() => {
  const h = sys.value?.history ?? []
  if (h.length < 2) return null
  const max = Math.max(1, ...h.map((p) => Math.max(p.down, p.up)))
  const line = (key: 'down' | 'up') =>
    h
      .map((p, i) => {
        const x = (i / (h.length - 1)) * W
        const y = H - 4 - (p[key] / max) * (H - 10)
        return `${i ? 'L' : 'M'}${x.toFixed(1)} ${y.toFixed(1)}`
      })
      .join(' ')
  return { down: line('down'), up: line('up') }
})

const c = computed(() => counts(apps.value))
const storeUrl = computed(() => (iceStoreInZimaOS.value ? dashboardUrl() : ICESTORE_SITE))
</script>

<template>
  <aside class="panel" aria-label="Widgets">
    <section class="card clock">
      <p class="time" aria-live="off">
        <span class="hm">{{ clock.time }}</span><span class="ampm">{{ clock.ampm }}</span>
      </p>
      <p class="date">{{ clock.date }} · {{ city }}</p>
    </section>

    <section class="card">
      <header>
        <h2 class="kicker">{{ strings.system }}</h2>
        <span class="aside">{{ systemLine }}</span>
      </header>
      <div v-for="m in meters" :key="m.id" class="meter">
        <div class="row">
          <span>{{ m.label }}</span>
          <span class="mono">{{ m.value }}</span>
        </div>
        <div class="bar" role="meter" :aria-label="m.label" aria-valuemin="0" aria-valuemax="100" :aria-valuenow="Math.round(m.pct)">
          <span :style="{ width: `${m.pct}%` }" />
        </div>
      </div>
      <p v-if="!meters.length" class="aside">{{ strings.loading }}</p>
    </section>

    <section class="card">
      <header>
        <h2 class="kicker">{{ strings.network }}</h2>
        <span class="mono speeds">
          <Icon name="down" :size="12" /> {{ speed(sys?.down ?? 0) }} · <Icon name="up" :size="12" /> {{ speed(sys?.up ?? 0) }}
        </span>
      </header>
      <svg class="spark" :viewBox="`0 0 ${W} ${H}`" preserveAspectRatio="none" role="img" :aria-label="strings.networkChart">
        <template v-if="chart">
          <path :d="chart.up" class="up" />
          <path :d="chart.down" class="down" />
        </template>
      </svg>
    </section>

    <section class="card">
      <header>
        <h2 class="kicker">{{ strings.apps }}</h2>
      </header>
      <div class="counts">
        <div class="count run">
          <strong>{{ c.running }}</strong><span>{{ strings.running }}</span>
        </div>
        <div class="count start">
          <strong>{{ c.starting }}</strong><span>{{ strings.starting }}</span>
        </div>
        <div class="count stop">
          <strong>{{ c.stopped }}</strong><span>{{ strings.stopped }}</span>
        </div>
      </div>
      <button v-if="appUpdates.length" type="button" class="store" @click="openIceStore('updates')">
        <span>{{ strings.appUpdates(appUpdates.length) }}</span>
        <span class="go">{{ strings.review }}</span>
      </button>
      <a v-else class="store" :href="storeUrl" target="_blank" rel="noopener" @click.prevent="openIceStore()">
        <span>{{ strings.browseIceStore }}</span>
        <span class="go">{{ strings.open }}</span>
      </a>
    </section>
  </aside>
</template>

<style scoped>
.panel {
  display: grid;
  gap: 16px;
  align-content: start;
}
.card {
  position: relative;
  padding: 18px 20px;
  border-radius: var(--radius-card);
  background: linear-gradient(160deg, rgba(255, 255, 255, 0.05), rgba(255, 255, 255, 0.02));
  border: 1px solid var(--line);
  backdrop-filter: var(--blur);
  -webkit-backdrop-filter: var(--blur);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
}
header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 12px;
}
.kicker {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--ac2);
}
.aside {
  color: var(--mut);
  font-size: 12.5px;
  text-align: right;
}
.mono {
  font-family: var(--font-mono);
  font-size: 12.5px;
}
.clock {
  padding: 20px 22px;
}
.time {
  margin: 0;
  display: flex;
  align-items: baseline;
  gap: 6px;
  font-family: var(--font-display);
  font-weight: 800;
  line-height: 1;
}
.hm {
  font-size: 46px;
  letter-spacing: -0.01em;
}
.ampm {
  font-size: 18px;
  color: var(--mut);
}
.date {
  margin: 10px 0 0;
  color: var(--mut);
  font-size: 14px;
}
.meter + .meter {
  margin-top: 12px;
}
.row {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  font-size: 13.5px;
  margin-bottom: 7px;
}
.bar {
  height: 5px;
  border-radius: var(--radius-pill);
  background: rgba(255, 255, 255, 0.08);
  overflow: hidden;
}
.bar span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--ac);
  box-shadow: 0 0 10px rgba(76, 201, 240, 0.5);
  transition: width 0.6s ease;
}
.speeds {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  white-space: nowrap;
}
.spark {
  display: block;
  width: 100%;
  height: 64px;
}
.spark path {
  fill: none;
  stroke-width: 1.8;
  stroke-linejoin: round;
  vector-effect: non-scaling-stroke;
}
.spark .down {
  stroke: var(--ac);
}
.spark .up {
  stroke: rgba(165, 232, 255, 0.35);
}
.counts {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}
.count {
  display: grid;
  justify-items: center;
  gap: 2px;
  padding: 10px 4px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.04);
}
.count strong {
  font-family: var(--font-display);
  font-weight: 800;
  font-size: 24px;
  line-height: 1.1;
}
.count span {
  font-size: 12.5px;
  color: var(--mut);
}
.count.run {
  background: rgba(76, 201, 240, 0.09);
}
.count.run strong {
  color: var(--ac2);
}
.count.start {
  background: rgba(245, 184, 76, 0.09);
}
.count.start strong {
  color: var(--warn);
}
.store {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  margin-top: 12px;
  padding: 10px 14px;
  border-radius: 14px;
  border: 1px solid rgba(76, 201, 240, 0.35);
  background: none;
  color: var(--text);
  font: inherit;
  font-size: 13.5px;
  text-decoration: none;
  width: 100%;
  cursor: pointer;
}
button.store span:first-child {
  text-decoration: underline;
}
.store:hover {
  background: rgba(76, 201, 240, 0.08);
}
.go {
  color: var(--ac2);
  font-weight: 600;
}
</style>
