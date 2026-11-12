// The clock's time zone, the way the stock ZimaOS dashboard picks it (measured
// on 1.8.0, docs/HOST-NOTES.md item 7): the zone saved in
// /v1/users/current/custom/system if it's a real zone, else the browser's own.
// Never the system zone, which can differ (Europe/London on the measured box).

export function isZone(z: string | undefined): z is string {
  if (!z) return false
  try {
    new Intl.DateTimeFormat('en-US', { timeZone: z })
    return true
  } catch {
    return false
  }
}

export function browserZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
  } catch {
    return 'UTC'
  }
}

export function pickZone(saved: string | undefined, browser: string = browserZone()): string {
  return isZone(saved) ? saved : browser
}

/** "Pacific/Auckland" → "Auckland", "America/Argentina/Buenos_Aires" → "Buenos Aires". */
export function cityOf(zone: string): string {
  const last = zone.split('/').pop() ?? zone
  return last.replace(/_/g, ' ')
}

export interface ClockParts {
  time: string // "2:06"
  ampm: string // "pm"
  date: string // "Wednesday, 11 November"
  day: string // "Wed 11 Nov"
  short: string // "Wed 11 Nov 2:06 pm"
}

export function clockParts(now: Date, zone: string): ClockParts {
  const get = (o: Intl.DateTimeFormatOptions) => new Intl.DateTimeFormat('en-NZ', { timeZone: zone, ...o })
  const tp = get({ hour: 'numeric', minute: '2-digit', hour12: true }).formatToParts(now)
  const hour = tp.find((p) => p.type === 'hour')?.value ?? ''
  const minute = tp.find((p) => p.type === 'minute')?.value ?? ''
  const ampm = (tp.find((p) => p.type === 'dayPeriod')?.value ?? '').toLowerCase().replace(/\./g, '')
  const time = `${hour}:${minute}`
  const date = get({ weekday: 'long', day: 'numeric', month: 'long' }).format(now).replace(/,(?= \d)/, ',')
  const shortDate = get({ weekday: 'short', day: 'numeric', month: 'short' }).format(now).replace(/,/g, '')
  return { time, ampm, date, day: shortDate, short: `${shortDate} ${time} ${ampm}` }
}
