// What a catalog app's button does, shared by the cards, the featured row and
// the app page: Install (setup popup), Open, Update, and Uninstall.
import { computed, ref } from 'vue'
import * as api from '../../api/apps'
import { getSettings } from '../../api/icedash'
import { host } from '../../lib/host'
import { supportsArch } from '../../lib/icestore'
import { cityOf } from '../../lib/timezone'
import { appInfo, apps, goBackFor, jobFor, openApp, setupFor, toast, trackJob, uninstallFor, user, zone } from '../../store'
import { strings } from '../../strings'

export const admin = computed(() => !!user.value?.admin)

/** The installed copy of a catalog app, by its store id. */
export const installedOf = (a: api.CatalogApp) => Object.values(appInfo.value).find((i) => i.store_id === a.id)

/** Whether a runs on this box's CPU. */
export const fitsHere = (a: api.CatalogApp) => supportsArch(a, host.arch)

export type Btn = { label: string; kind: 'install' | 'open' | 'update' | 'busy'; disabled?: boolean; title?: string }

export function button(a: api.CatalogApp): Btn {
  const inst = installedOf(a)
  const job = jobFor(a.name)
  if (job?.state === 'running')
    return { label: job.kind === 'rollback' ? strings.goingBack : job.kind === 'update' ? strings.updatingApp : strings.installing, kind: 'busy', disabled: true }
  if (inst?.update) return { label: strings.update, kind: 'update', disabled: !admin.value, title: admin.value ? undefined : strings.adminInstallOnly }
  if (inst) return { label: strings.openApp, kind: 'open' }
  return { label: strings.install, kind: 'install', disabled: !admin.value, title: admin.value ? undefined : strings.adminInstallOnly }
}

export async function act(a: api.CatalogApp) {
  const b = button(a)
  if (b.kind === 'install') setupFor.value = a.id
  else if (b.kind === 'open') {
    const grid = apps.value.find((x) => x.id === installedOf(a)?.name)
    if (grid) openApp(grid)
  } else if (b.kind === 'update') {
    const inst = installedOf(a)
    if (!inst) return
    try {
      trackJob(await api.updateApp(inst.name))
      toast({ title: strings.updatingApp, body: a.title })
    } catch (e) {
      toast({ title: strings.update, body: e instanceof Error ? e.message : String(e) })
    }
  }
}

/** Opens the uninstall popup for a's installed copy (admins only). */
export function uninstall(a: api.CatalogApp) {
  const inst = installedOf(a)
  if (inst && admin.value) uninstallFor.value = inst.name
}

/** A Unix time as "20 Aug 2026" in the UI's language. */
export function day(t: number, locale: string): string {
  return new Date(t * 1000).toLocaleDateString(locale, { day: 'numeric', month: 'short', year: 'numeric' })
}

// ---- updates at night and going back ----

/** The server's saved zone: null until read, "" when none is saved. */
export const savedZone = ref<string | null>(null)
export async function loadSavedZone() {
  try {
    savedZone.value = (await getSettings()).zone ?? ''
  } catch {
    savedZone.value = ''
  }
}
/** The city the night counts in, as Settings words it. */
export const nightCity = computed(() => cityOf(savedZone.value || zone.value))

/** Whether the version waiting is one that doesn't install by itself. */
export const heldNow = (i: api.AppInfo) => !!i.held && i.held === i.latest

/** Turns "Update at night" on or off; the switch goes back if that fails. */
export async function setAuto(name: string, on: boolean) {
  const before = appInfo.value[name]
  if (!before) return
  appInfo.value = { ...appInfo.value, [name]: { ...before, auto: on } }
  try {
    await api.setAutoUpdate(name, on, zone.value)
    if (on) savedZone.value = zone.value
  } catch (e) {
    const cur = appInfo.value[name]
    if (cur) appInfo.value = { ...appInfo.value, [name]: { ...cur, auto: before.auto } }
    toast({ title: strings.autoApp, body: e instanceof Error ? e.message : String(e) })
  }
}

/** Opens the "Go back" popup (admins, with a kept version, no job running). */
export function goBack(name: string) {
  const i = appInfo.value[name]
  if (i?.back && admin.value && jobFor(name)?.state !== 'running') goBackFor.value = name
}

/** The muted line under an app: its hold, else last night's skip. */
export function holdLine(i: api.AppInfo): string {
  if (i.held) return i.held_by === 'night' ? strings.heldNightNote(i.held) : strings.heldNote(i.held)
  return ''
}
export function skipLine(i: api.AppInfo): string {
  switch (i.night_skip) {
    case 'stopped':
      return strings.nightSkippedStopped
    case 'unhealthy':
      return strings.nightSkippedUnhealthy
    case 'no_copy':
      return strings.nightSkippedNoCopy
    case 'cant_change':
      return strings.nightSkippedCantChange
  }
  return ''
}
