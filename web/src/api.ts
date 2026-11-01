// Thin client for the admin API. Every message shown to people is plain
// language; technical details never reach the screen.

export interface EventItem {
  id: number
  kind: 'event' | 'road'
  title: string
  summary: string
  badgeText: string
  badgeStyle: 'orange' | 'purple' | 'blue' | 'pink'
  startsAt: string | null
  endsAt: string | null
  posterImageId: number | null
  posterUrl?: string
  linkUrl: string
  linkLabel: string
  link2Url: string
  link2Label: string
  featured: boolean
  published: boolean
}

export interface Person {
  id: number
  kind: 'dj' | 'host'
  name: string
  slName: string
  bio: string
  schedule: string
  photoImageId: number | null
  photoUrl?: string
  sortOrder: number
  active: boolean
}

export interface Partner {
  id: number
  name: string
  sortOrder: number
  active: boolean
}

export type Settings = Record<string, string>

export interface UploadedImage {
  id: number
  url: string
  width: number
  height: number
}

export class ApiError extends Error {
  status: number
  fields: Record<string, string>
  constructor(message: string, status: number, fields: Record<string, string> = {}) {
    super(message)
    this.status = status
    this.fields = fields
  }
}

const GENERIC = "Sorry, that didn't work. Please check your internet connection and try again."

async function request<T>(method: string, path: string, body?: BodyInit | null, json = true): Promise<T> {
  const headers: Record<string, string> = { 'X-Muddys-Admin': '1' }
  if (json && body) headers['Content-Type'] = 'application/json'
  let res: Response
  try {
    res = await fetch('/api/admin' + path, { method, headers, body, credentials: 'same-origin' })
  } catch {
    throw new ApiError(GENERIC, 0)
  }
  if (res.status === 401) {
    throw new ApiError('Your sign-in has timed out. Please reload this page to sign in again.', 401)
  }
  if (res.status === 204) return undefined as T
  let data: unknown = null
  try {
    data = await res.json()
  } catch {
    /* not JSON */
  }
  if (!res.ok) {
    const d = (data ?? {}) as { error?: string; fields?: Record<string, string> }
    throw new ApiError(d.error || GENERIC, res.status, d.fields)
  }
  return data as T
}

const send = (method: string, path: string, value?: unknown) =>
  request<never>(method, path, value === undefined ? null : JSON.stringify(value))

export const api = {
  me: () => request<{ email: string }>('GET', '/me'),
  events: () => request<EventItem[]>('GET', '/events'),
  event: (id: number) => request<EventItem>('GET', `/events/${id}`),
  saveEvent: (e: Partial<EventItem> & { id?: number }) =>
    e.id ? (send('PUT', `/events/${e.id}`, e) as Promise<EventItem>) : (send('POST', '/events', e) as Promise<EventItem>),
  deleteEvent: (id: number) => send('DELETE', `/events/${id}`) as Promise<void>,

  people: (kind: string) => request<Person[]>('GET', `/staff?kind=${kind}`),
  person: (id: number) => request<Person>('GET', `/staff/${id}`),
  savePerson: (p: Partial<Person> & { id?: number }) =>
    p.id ? (send('PUT', `/staff/${p.id}`, p) as Promise<Person>) : (send('POST', '/staff', p) as Promise<Person>),
  deletePerson: (id: number) => send('DELETE', `/staff/${id}`) as Promise<void>,
  orderPeople: (kind: string, ids: number[]) => send('PUT', '/staff/order', { kind, ids }) as Promise<void>,

  partners: () => request<Partner[]>('GET', '/partners'),
  savePartner: (p: Partial<Partner> & { id?: number }) =>
    p.id ? (send('PUT', `/partners/${p.id}`, p) as Promise<Partner>) : (send('POST', '/partners', p) as Promise<Partner>),
  deletePartner: (id: number) => send('DELETE', `/partners/${id}`) as Promise<void>,
  orderPartners: (ids: number[]) => send('PUT', '/partners/order', { ids }) as Promise<void>,

  settings: () => request<Settings>('GET', '/settings'),
  saveSettings: (s: Settings) => send('PUT', '/settings', s) as Promise<Settings>,

  uploadImage: (file: File) => {
    const fd = new FormData()
    fd.append('file', file)
    return request<UploadedImage>('POST', '/images', fd, false)
  },
}
