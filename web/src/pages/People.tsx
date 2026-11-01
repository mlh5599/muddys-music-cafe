import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api } from '../api'
import type { Person } from '../api'
import { Button, errorText, LinkButton, Loading, moved, Page, useLoad, useNotify } from '../ui'

export const kindInfo = {
  dj: { one: 'DJ', many: 'DJs' },
  host: { one: 'host', many: 'hosts' },
} as const

export default function People() {
  const kind = useParams().kind === 'host' ? 'host' : 'dj'
  const info = kindInfo[kind]
  const notify = useNotify()
  const list = useLoad(() => api.people(kind), [kind])
  const [busy, setBusy] = useState(false)

  async function move(i: number, to: number) {
    const cur = list.data
    if (!cur) return
    const next = moved(cur, i, to)
    list.setData(next)
    setBusy(true)
    try {
      await api.orderPeople(kind, next.map((p) => p.id))
    } catch (e) {
      notify(errorText(e), 'bad')
      list.reload()
    } finally {
      setBusy(false)
    }
  }

  async function toggle(p: Person) {
    try {
      await api.savePerson({ ...p, active: !p.active })
      notify(p.active ? `${p.name} is now hidden on the website.` : `${p.name} is now showing on the website.`)
      list.reload()
    } catch (e) {
      notify(errorText(e), 'bad')
    }
  }

  const cap = info.many[0].toUpperCase() + info.many.slice(1)
  return (
    <Page title={cap} back="/" backLabel="Home" actions={<LinkButton to={`/people/${kind}/new`}>Add a {info.one}</LinkButton>}>
      {!list.data ? <Loading error={list.error} /> : (
        <>
          <p className="muted">The order here is the order on the website. Use the arrows to move someone.</p>
          <div className="stack">
            {list.data.map((p, i) => (
              <div key={p.id} className={`row-card row-card--static${p.active ? '' : ' row-card--off'}`}>
                <Link to={`/people/${kind}/${p.id}`} className="row-link">
                  <span className="thumb thumb--round">{p.photoUrl ? <img src={p.photoUrl} alt="" /> : null}</span>
                  <span className="row-main">
                    <span className="row-title">{p.name}</span>
                    <span className="muted">{p.active ? 'On the website' : 'Hidden'}</span>
                  </span>
                </Link>
                <span className="row-actions">
                  <button type="button" className="icon-btn" aria-label={`Move ${p.name} up`} disabled={busy || i === 0} onClick={() => void move(i, i - 1)}>↑</button>
                  <button type="button" className="icon-btn" aria-label={`Move ${p.name} down`} disabled={busy || i === list.data!.length - 1} onClick={() => void move(i, i + 1)}>↓</button>
                  <Button kind="secondary" onClick={() => void toggle(p)}>{p.active ? 'Hide' : 'Show'}</Button>
                </span>
              </div>
            ))}
          </div>
          {list.data.length === 0 && <p className="notice">Nobody here yet. Press “Add a {info.one}”.</p>}
        </>
      )}
    </Page>
  )
}
