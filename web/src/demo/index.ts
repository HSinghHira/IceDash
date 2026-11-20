// The live demo (`npm run demo`, served at /demo/ on IceDash's website): the
// real dashboard, with its API answered in the browser by demo/backend.ts.
// Only a demo build loads this file; main.ts drops it from every other build.
import { answer } from './backend'
import { DemoSocket } from './shell'

const b64 = (o: object) => btoa(JSON.stringify(o)).replace(/=+$/, '').replace(/\+/g, '-').replace(/\//g, '_')

export function start() {
  // Signed in as the demo's admin: the token only has to parse, nothing checks it.
  localStorage.setItem('access_token', `${b64({ alg: 'none' })}.${b64({ username: 'Demo', role: 'admin', id: 1, iss: 'demo' })}.demo`)

  const realFetch = window.fetch.bind(window)
  window.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
    const req = input instanceof Request ? input : null
    const url = new URL(req ? req.url : String(input), location.href)
    if (url.origin === location.origin) {
      let body: Record<string, unknown> = {}
      const raw = init?.body ?? null
      if (typeof raw === 'string') {
        try {
          body = JSON.parse(raw) as Record<string, unknown>
        } catch {
          // not JSON
        }
      }
      const reply = await answer((init?.method ?? req?.method ?? 'GET').toUpperCase(), url, body)
      if (reply) {
        // A moment's wait, so loading states show as they do on a real server.
        await new Promise((r) => setTimeout(r, 60 + Math.random() * 120))
        return new Response(JSON.stringify(reply.body), { status: reply.status ?? 200, headers: { 'Content-Type': 'application/json' } })
      }
    }
    return realFetch(input, init)
  }

  const RealSocket = window.WebSocket
  const Socket = function (url: string | URL, protocols?: string | string[]) {
    return String(url).includes('/terminal') ? new DemoSocket(String(url)) : new RealSocket(url, protocols)
  } as unknown as typeof WebSocket
  Object.assign(Socket, { CONNECTING: 0, OPEN: 1, CLOSING: 2, CLOSED: 3 })
  window.WebSocket = Socket

  addEventListener('DOMContentLoaded', note, { once: true })
  if (document.readyState !== 'loading') note()
}

/** A small, always-visible note that this is a demo, with the way to the real thing. */
function note() {
  if (document.getElementById('demo-note')) return
  const site = new URL('..', location.href).href
  const el = document.createElement('a')
  el.id = 'demo-note'
  el.href = site
  el.innerHTML = '<b>Demo</b><span>Sample data, nothing is installed. Get IceDash</span>'
  const css = document.createElement('style')
  css.textContent = `
#demo-note{position:fixed;left:12px;bottom:12px;z-index:900;display:flex;align-items:center;gap:8px;max-width:calc(100vw - 24px);
  padding:6px 12px 6px 6px;border-radius:10px;border:1px solid var(--line);background:rgba(var(--surface-rgb),.92);
  color:var(--mut);font:500 12px/16px var(--font);text-decoration:none;backdrop-filter:var(--blur);-webkit-backdrop-filter:var(--blur);
  transition:color .15s ease,border-color .15s ease}
#demo-note b{padding:2px 8px;border-radius:6px;background:rgba(var(--ac-rgb),.16);color:var(--ac2);font-weight:500}
#demo-note:hover{color:var(--text);border-color:rgba(var(--fg-rgb),.2)}
@media(max-width:700px){#demo-note span{display:none}}`
  document.head.append(css)
  document.body.append(el)
}
