// What a catalog app's button does, shared by the cards, the featured row and
// the app page: Install (setup popup), Open, Update, and Uninstall.
import { computed } from 'vue'
import * as api from '../../api/apps'
import { host } from '../../lib/host'
import { supportsArch } from '../../lib/icestore'
import { appInfo, apps, jobFor, openApp, setupFor, toast, trackJob, uninstallFor, user } from '../../store'
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
  if (job?.state === 'running') return { label: job.kind === 'update' ? strings.updatingApp : strings.installing, kind: 'busy', disabled: true }
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
