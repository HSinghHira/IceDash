// Runs fn now and every ms while the tab is visible; pauses while it's hidden
// and runs again as soon as it's shown (PLAN.md, "Live data").
export interface Poller {
  stop(): void
  /** Run now (e.g. after an action) and restart the interval. */
  now(): void
  /** Change the interval (e.g. faster while an update installs). */
  every(ms: number): void
}

export function poll(fn: () => unknown, ms: number, doc: Document = document): Poller {
  let timer: ReturnType<typeof setInterval> | undefined
  let interval = ms
  let stopped = false

  const run = () => {
    try {
      const r = fn()
      if (r instanceof Promise) r.catch(() => undefined)
    } catch {
      // the next tick tries again
    }
  }
  const start = () => {
    clear()
    if (stopped || doc.visibilityState === 'hidden') return
    run()
    timer = setInterval(run, interval)
  }
  const clear = () => {
    if (timer !== undefined) clearInterval(timer)
    timer = undefined
  }
  const onVisibility = () => (doc.visibilityState === 'hidden' ? clear() : start())

  doc.addEventListener('visibilitychange', onVisibility)
  start()
  return {
    stop() {
      stopped = true
      clear()
      doc.removeEventListener('visibilitychange', onVisibility)
    },
    now: start,
    every(next: number) {
      if (next === interval) return
      interval = next
      start()
    },
  }
}
