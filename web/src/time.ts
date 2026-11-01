// Event times are typed and shown in Second Life time (Pacific by default),
// whatever time zone the person's computer is in. The server stores UTC.

export const DEFAULT_ZONE = 'America/Los_Angeles'

function wallParts(ms: number, zone: string) {
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone: zone, hourCycle: 'h23',
    year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit',
  }).formatToParts(new Date(ms))
  const get = (t: string) => Number(parts.find((p) => p.type === t)?.value)
  return { y: get('year'), mo: get('month'), d: get('day'), h: get('hour'), mi: get('minute'), s: get('second') }
}

function offsetMs(ms: number, zone: string): number {
  const p = wallParts(ms, zone)
  return Date.UTC(p.y, p.mo - 1, p.d, p.h, p.mi, p.s) - ms
}

const two = (n: number) => String(n).padStart(2, '0')

/** UTC ISO string -> "YYYY-MM-DDTHH:mm" in the zone, for a date/time picker. */
export function toPickerValue(iso: string | null, zone: string): string {
  if (!iso) return ''
  const p = wallParts(Date.parse(iso), zone)
  return `${p.y}-${two(p.mo)}-${two(p.d)}T${two(p.h)}:${two(p.mi)}`
}

/** Picker value (wall-clock time in the zone) -> UTC ISO string, or null. */
export function fromPickerValue(v: string, zone: string): string | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})/.exec(v)
  if (!m) return null
  const guess = Date.UTC(+m[1], +m[2] - 1, +m[3], +m[4], +m[5])
  let t = guess - offsetMs(guess, zone)
  t = guess - offsetMs(t, zone)
  return new Date(t).toISOString().replace(/\.\d+Z$/, 'Z')
}

/** Friendly date, e.g. "Sat, Nov 21, 7:00 PM". */
export function friendly(iso: string | null, zone: string): string {
  if (!iso) return ''
  return new Intl.DateTimeFormat('en-US', {
    timeZone: zone, weekday: 'short', month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit',
  }).format(new Date(iso))
}
