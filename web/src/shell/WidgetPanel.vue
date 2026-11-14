<script setup lang="ts">
// The panel (IceCards): IceDash's own cards (Clock, System, Network, Apps)
// and the user's widgets (Phase 4), each in its sandbox, in the user's order.
// Every card can be dragged to a new place, taken off, or right-clicked.
// Cards dragged from Settings → IceCards drop here. When the cards don't fit
// one column, the next column starts at the top, to the left (lib/flow): the
// panel never scrolls.
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import WidgetFrame from '../components/WidgetFrame.vue'
import Icon from '../components/Icon.vue'
import { counts } from '../lib/apps'
import { duration, pct, speed, usedOfTotal } from '../lib/format'
import { dashboardUrl } from '../lib/host'
import { ICESTORE_SITE } from '../lib/launchers'
import { flowRight, type Box } from '../lib/flow'
import { cityOf, clockParts } from '../lib/timezone'
import { cardMenu, cardTitle, panelMenu } from '../menus'
import {
  appUpdates,
  apps,
  dockRect,
  iceStoreInZimaOS,
  modsView,
  openContext,
  openIceStore,
  panelCards,
  setPanelCards,
  sys,
  util,
  zone,
  h24,
  type PanelCard,
} from '../store'
import { strings } from '../strings'

const props = defineProps<{ now: Date }>()

// ---- placing cards ----
const ADD = 'application/x-icedash-widget'
const MOVE = 'application/x-icedash-panel'
const dropping = ref(false)
const accepts = (e: DragEvent) => !!e.dataTransfer && (e.dataTransfer.types.includes(ADD) || e.dataTransfer.types.includes(MOVE))
const savePanel = (ids: string[]) => setPanelCards(ids)
/** A widget reloads by remounting its frame. */
const reloads = reactive<Record<string, number>>({})
function menuFor(e: MouseEvent | KeyboardEvent, c: PanelCard, i: number) {
  openContext(e, cardTitle(c), cardMenu(c, i, panelCards.value.length, c.builtin ? undefined : () => (reloads[c.id] = (reloads[c.id] ?? 0) + 1)))
}
function over(e: DragEvent) {
  if (!accepts(e)) return
  e.preventDefault()
  dropping.value = true
}
function drop(e: DragEvent, before?: string) {
  dropping.value = false
  if (!accepts(e)) return
  e.preventDefault()
  const id = e.dataTransfer!.getData(ADD) || e.dataTransfer!.getData(MOVE)
  if (!id) return
  const ids = modsView.value.prefs.panel.filter((x) => x !== id)
  const at = before ? ids.indexOf(before) : -1
  if (at >= 0) ids.splice(at, 0, id)
  else ids.push(id)
  void savePanel(ids)
}
function lift(e: DragEvent, id: string) {
  e.dataTransfer?.setData(MOVE, id)
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}
const takeOff = (id: string) => savePanel(modsView.value.prefs.panel.filter((x) => x !== id))

// ---- columns ----
const COL = 312
const GAP = 16
const el = ref<HTMLElement | null>(null)
const wide = ref(true)
const box = ref<Box | null>(null)
const heights = reactive<Record<string, number>>({})
let ro: ResizeObserver | undefined
function measure() {
  wide.value = window.innerWidth > 900
  const me = el.value
  const area = me?.parentElement
  if (!me || !area) return
  const r = me.getBoundingClientRect()
  const a = area.getBoundingClientRect()
  box.value = { left: r.left, right: r.right, top: r.top, bottom: a.bottom - parseFloat(getComputedStyle(area).paddingBottom || '0') }
}
function observeCards() {
  if (!ro || !el.value) return
  ro.disconnect()
  if (el.value.parentElement) ro.observe(el.value.parentElement)
  for (const c of el.value.querySelectorAll<HTMLElement>(':scope > [data-card]')) ro.observe(c)
}
onMounted(() => {
  measure()
  if (typeof ResizeObserver !== 'undefined') {
    ro = new ResizeObserver((entries) => {
      for (const e of entries) {
        const id = (e.target as HTMLElement).dataset.card
        if (id) heights[id] = (e.target as HTMLElement).offsetHeight
      }
      measure()
    })
    observeCards()
  }
  window.addEventListener('resize', measure)
})
onBeforeUnmount(() => {
  ro?.disconnect()
  window.removeEventListener('resize', measure)
})
watch(
  () => panelCards.value.map((c) => c.id).join(),
  () => nextTick(observeCards),
)
const layout = computed(() =>
  wide.value && box.value ? flowRight(panelCards.value.map((c) => heights[c.id] ?? 0), box.value, dockRect.value, COL, GAP) : null,
)
const panelStyle = computed(() => {
  const l = layout.value
  if (!l) return undefined
  const w = l.cols ? l.cols * COL + (l.cols - 1) * GAP : 0
  return { width: `${Math.max(w, dropping.value ? COL : 0)}px` }
})
const cardStyle = (i: number) => {
  const p = layout.value?.pos[i]
  return p ? { position: 'absolute' as const, right: `${p.x}px`, top: `${p.y}px`, width: `${COL}px` } : undefined
}

const clock = computed(() => clockParts(props.now, zone.value, h24.value))
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

const tally = computed(() => counts(apps.value))
const storeUrl = computed(() => (iceStoreInZimaOS.value ? dashboardUrl() : ICESTORE_SITE))
</script>

<template>
  <aside
    ref="el"
    class="panel"
    :class="{ flow: !!layout }"
    :style="panelStyle"
    :aria-label="strings.settingsWidgets"
    @dragover="over"
    @dragleave.self="dropping = false"
    @drop="drop($event)"
    @contextmenu.self="openContext($event, strings.ctxPanel, panelMenu())"
  >
    <section
      v-for="(c, i) in panelCards"
      :key="c.id"
      class="card"
      :class="[c.builtin ? `b-${c.id}` : 'wcard']"
      :data-card="c.id"
      :style="cardStyle(i)"
      draggable="true"
      :aria-label="cardTitle(c)"
      @dragstart="lift($event, c.id)"
      @drop.stop="drop($event, c.id)"
      @contextmenu="menuFor($event, c, i)"
    >
      <button type="button" class="off" :title="strings.removeFromPanel" :aria-label="`${strings.removeFromPanel}: ${cardTitle(c)}`" @click="takeOff(c.id)">
        <Icon name="close" :size="13" />
      </button>

    <template v-if="c.id === 'clock'">
      <p class="time" aria-live="off">
        <span class="hm">{{ clock.time }}</span><span v-if="clock.ampm" class="ampm">{{ clock.ampm }}</span>
      </p>
      <p class="date">{{ clock.date }} · {{ city }}</p>
    </template>

    <template v-else-if="c.id === 'system'">
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
    </template>

    <template v-else-if="c.id === 'network'">
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
    </template>

    <template v-else-if="c.id === 'apps'">
      <header>
        <h2 class="kicker">{{ strings.apps }}</h2>
      </header>
      <div class="counts">
        <div class="count run">
          <strong>{{ tally.running }}</strong><span>{{ strings.running }}</span>
        </div>
        <div class="count start">
          <strong>{{ tally.starting }}</strong><span>{{ strings.starting }}</span>
        </div>
        <div class="count stop">
          <strong>{{ tally.stopped }}</strong><span>{{ strings.stopped }}</span>
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
    </template>

    <WidgetFrame v-else-if="!c.builtin" :key="`${c.id}:${reloads[c.id] ?? 0}`" :mod="c.mod" />
    </section>


    <p v-if="dropping" class="dropzone">{{ strings.dropWidget }}</p>
  </aside>
</template>

<style scoped>
.panel {
  display: grid;
  gap: 16px;
  align-content: start;
}
/* Columns (lib/flow): every card placed by its own top and right. */
.panel.flow {
  position: relative;
  display: block;
  height: 100%;
}
.panel.flow > .card {
  transition: top 0.25s ease, right 0.25s ease;
}
.panel.flow > .dropzone {
  position: absolute;
  inset: 0 0 auto auto;
  width: 312px;
  z-index: 2;
  background: rgba(var(--surface-rgb), 0.85);
}
.card.wcard {
  padding: 0;
  overflow: hidden;
}
.off {
  position: absolute;
  top: 8px;
  right: 8px;
  z-index: 1;
  display: grid;
  place-items: center;
  width: 24px;
  height: 24px;
  border: 0;
  border-radius: 7px;
  background: rgba(var(--surface-rgb), 0.85);
  color: var(--mut);
  opacity: 0;
  cursor: pointer;
  transition: opacity 0.15s;
}
.card:hover .off,
.off:focus-visible {
  opacity: 1;
}
.card[draggable='true'] {
  cursor: grab;
}
.card[draggable='true']:active {
  cursor: grabbing;
}
.dropzone {
  margin: 0;
  padding: 18px;
  border: 2px dashed var(--ac);
  border-radius: var(--radius-card);
  color: var(--ac2);
  text-align: center;
  font-size: 13.5px;
}
.card {
  position: relative;
  padding: 18px 20px;
  border-radius: var(--radius-card);
  background: linear-gradient(160deg, rgba(var(--fg-rgb), 0.05), rgba(var(--fg-rgb), 0.02));
  border: 1px solid var(--line);
  backdrop-filter: var(--blur);
  -webkit-backdrop-filter: var(--blur);
  box-shadow: inset 0 1px 0 rgba(var(--fg-rgb), 0.04);
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
.b-clock {
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
  background: rgba(var(--fg-rgb), 0.08);
  overflow: hidden;
}
.bar span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--ac);
  box-shadow: 0 0 10px rgba(var(--ac-rgb), 0.5);
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
  stroke: rgba(var(--ac2-rgb), 0.35);
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
  background: rgba(var(--fg-rgb), 0.04);
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
  background: rgba(var(--ac-rgb), 0.09);
}
.count.run strong {
  color: var(--ac2);
}
.count.start {
  background: rgba(var(--warn-rgb), 0.09);
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
  border: 1px solid rgba(var(--ac-rgb), 0.35);
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
  background: rgba(var(--ac-rgb), 0.08);
}
.go {
  color: var(--ac2);
  font-weight: 600;
}
</style>
