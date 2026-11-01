import { Link } from 'react-router-dom'
import { api } from '../api'
import type { EventItem } from '../api'
import { DEFAULT_ZONE, friendly } from '../time'
import { LinkButton, Loading, Page, useLoad } from '../ui'

function status(e: EventItem): { text: string; cls: string } {
  if (!e.published) return { text: 'Hidden', cls: 'chip--off' }
  if (e.endsAt && Date.parse(e.endsAt) < Date.now()) return { text: 'Finished', cls: 'chip--off' }
  return { text: 'On the website', cls: 'chip--on' }
}

function EventRow({ e, zone }: { e: EventItem; zone: string }) {
  const s = status(e)
  const when = e.startsAt ? friendly(e.startsAt, zone) : ''
  return (
    <Link to={`/events/${e.id}`} className="row-card">
      <span className="thumb">{e.posterUrl ? <img src={e.posterUrl} alt="" /> : null}</span>
      <span className="row-main">
        <span className="row-title">{e.title}</span>
        <span className="muted">{when || 'No date set'}</span>
      </span>
      <span className={`chip ${s.cls}`}>{s.text}</span>
    </Link>
  )
}

export default function Events() {
  const list = useLoad(() => api.events())
  const set = useLoad(() => api.settings())
  const zone = set.data?.timezone || DEFAULT_ZONE
  const here = list.data?.filter((e) => e.kind === 'event') ?? []
  const away = list.data?.filter((e) => e.kind === 'road') ?? []
  return (
    <Page title="Events" back="/" backLabel="Home" actions={<LinkButton to="/events/new">Add an event</LinkButton>}>
      {!list.data ? <Loading error={list.error} /> : (
        <>
          {here.length === 0 && <p className="notice">There are no events yet. Press “Add an event” to create one.</p>}
          <div className="stack">{here.map((e) => <EventRow key={e.id} e={e} zone={zone} />)}</div>

          <h2 className="section">Events away from Muddy’s</h2>
          <p className="muted">For when Muddy’s is appearing somewhere else.</p>
          <div className="stack">{away.map((e) => <EventRow key={e.id} e={e} zone={zone} />)}</div>
          <p><LinkButton to="/events/new?away=1" kind="secondary">Add an event away from Muddy’s</LinkButton></p>
        </>
      )}
    </Page>
  )
}
