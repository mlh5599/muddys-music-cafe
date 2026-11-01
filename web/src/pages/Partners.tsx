import { useState } from 'react'
import { api } from '../api'
import type { Partner } from '../api'
import { Button, Confirm, errorText, Loading, moved, Page, useLoad, useNotify } from '../ui'

export default function Partners() {
  const list = useLoad(() => api.partners())
  const notify = useNotify()
  const [newName, setNewName] = useState('')
  const [editing, setEditing] = useState<{ id: number; name: string } | null>(null)
  const [removing, setRemoving] = useState<Partner | null>(null)
  const [busy, setBusy] = useState(false)

  async function run(fn: () => Promise<unknown>, ok?: string) {
    setBusy(true)
    try {
      await fn()
      if (ok) notify(ok)
      list.reload()
    } catch (e) {
      notify(errorText(e), 'bad')
      list.reload()
    } finally {
      setBusy(false)
    }
  }

  async function move(i: number, to: number) {
    if (!list.data) return
    const next = moved(list.data, i, to)
    list.setData(next)
    await run(() => api.orderPartners(next.map((p) => p.id)))
  }

  return (
    <Page title="Partners" back="/" backLabel="Home">
      {!list.data ? <Loading error={list.error} /> : (
        <>
          <form className="add-row" onSubmit={(e) => {
            e.preventDefault()
            if (!newName.trim()) return
            void run(async () => { await api.savePartner({ name: newName, active: true }); setNewName('') }, 'Added. It’s now on the website.')
          }}>
            <label htmlFor="new-partner">Add a partner</label>
            <div className="row">
              <input id="new-partner" value={newName} onChange={(e) => setNewName(e.target.value)} placeholder="Business name" />
              <Button busy={busy} onClick={() => {
                if (!newName.trim()) return
                void run(async () => { await api.savePartner({ name: newName, active: true }); setNewName('') }, 'Added. It’s now on the website.')
              }}>Add</Button>
            </div>
          </form>

          <p className="muted">The order here is the order on the website.</p>
          <div className="stack">
            {list.data.map((p, i) => (
              <div key={p.id} className={`row-card row-card--static${p.active ? '' : ' row-card--off'}`}>
                {editing?.id === p.id ? (
                  <form className="row grow" onSubmit={(e) => {
                    e.preventDefault()
                    void run(async () => { await api.savePartner({ ...p, name: editing.name }); setEditing(null) }, 'Saved. It’s now on the website.')
                  }}>
                    <input aria-label="Business name" value={editing.name} onChange={(e) => setEditing({ id: p.id, name: e.target.value })} autoFocus />
                    <Button busy={busy} onClick={() => void run(async () => { await api.savePartner({ ...p, name: editing.name }); setEditing(null) }, 'Saved. It’s now on the website.')}>Save</Button>
                    <Button kind="secondary" onClick={() => setEditing(null)}>Cancel</Button>
                  </form>
                ) : (
                  <>
                    <span className="row-main">
                      <span className="row-title">{p.name}</span>
                      {!p.active && <span className="muted">Hidden</span>}
                    </span>
                    <span className="row-actions">
                      <button type="button" className="icon-btn" aria-label={`Move ${p.name} up`} disabled={busy || i === 0} onClick={() => void move(i, i - 1)}>↑</button>
                      <button type="button" className="icon-btn" aria-label={`Move ${p.name} down`} disabled={busy || i === list.data!.length - 1} onClick={() => void move(i, i + 1)}>↓</button>
                      <Button kind="secondary" onClick={() => setEditing({ id: p.id, name: p.name })}>Change name</Button>
                      <Button kind="secondary" onClick={() => void run(() => api.savePartner({ ...p, active: !p.active }), p.active ? 'Hidden from the website.' : 'Now showing on the website.')}>
                        {p.active ? 'Hide' : 'Show'}
                      </Button>
                      <Button kind="danger" onClick={() => setRemoving(p)}>Delete</Button>
                    </span>
                  </>
                )}
              </div>
            ))}
          </div>
        </>
      )}
      {removing && (
        <Confirm title={`Delete ${removing.name}?`} message="It will be removed for good. If you only want it off the website for now, choose “No, keep it” and press Hide instead."
          confirmLabel="Yes, delete" onCancel={() => setRemoving(null)}
          onConfirm={() => { const r = removing; setRemoving(null); void run(() => api.deletePartner(r.id), `${r.name} has been removed.`) }} />
      )}
    </Page>
  )
}
