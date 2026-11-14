// The system IceDash runs on (PLAN.md, Phase 3a). The backend says which in
// GET /v2/icedash/health, which needs no sign-in, so it's known before the
// sign-in screen. Until then, ZimaOS.
//
// The stock dashboard is at "/" on ZimaOS (IceDash sits beside it) and at
// "/casaos/" on CasaOS (IceDash took over "/"). Both are hash-routed single
// pages; CasaOS's Files, App Store and Settings are popups on its home page.
import { reactive } from 'vue'

export interface HostInfo {
  id: 'zimaos' | 'casaos'
  name: string
  stock: string
}

export const host = reactive<HostInfo>({ id: 'zimaos', name: 'ZimaOS', stock: '/' })

export function setHost(h: { host?: string; host_name?: string; stock_url?: string }) {
  if (h.host === 'casaos' || h.host === 'zimaos') host.id = h.host
  if (h.host_name) host.name = h.host_name
  if (h.stock_url && h.stock_url.startsWith('/')) host.stock = h.stock_url
}

/** The stock dashboard's home page. */
export const dashboardUrl = () => `${host.stock}#/`
/** Its sign-in page. */
export const loginUrl = () => `${host.stock}#/login`
