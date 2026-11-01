import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, ApiError } from '../api'
import type { Person } from '../api'
import { Button, Confirm, errorText, Loading, Page, PhotoPicker, TextArea, TextField, Toggle, useNotify } from '../ui'
import { kindInfo } from './People'

// Bios are stored with a blank line between paragraphs. People just press
// Enter, so each line is shown (and saved) as its own paragraph.
const toEditable = (bio: string) => bio.split(/\n\s*\n/).map((s) => s.trim()).filter(Boolean).join('\n')
const fromEditable = (text: string) => text.split('\n').map((s) => s.trim()).filter(Boolean).join('\n\n')

type Draft = Omit<Person, 'id'> & { id?: number }

export default function PersonForm() {
  const params = useParams()
  const kind = params.kind === 'host' ? 'host' : 'dj'
  const info = kindInfo[kind]
  const id = params.id
  const nav = useNavigate()
  const notify = useNotify()
  const back = `/people/${kind}`
  const [draft, setDraft] = useState<Draft | null>(
    id ? null : { kind, name: '', slName: '', bio: '', schedule: '', photoImageId: null, sortOrder: 0, active: true },
  )
  const [loadError, setLoadError] = useState('')
  const [fields, setFields] = useState<Record<string, string>>({})
  const [problem, setProblem] = useState('')
  const [busy, setBusy] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)

  useEffect(() => {
    if (!id) return
    api.person(Number(id)).then(
      (p) => setDraft({ ...p, bio: toEditable(p.bio) }),
      (e) => setLoadError(errorText(e)),
    )
  }, [id])

  if (!draft) return <Page title={info.one} back={back} backLabel="Back to the list"><Loading error={loadError} /></Page>
  const set = <K extends keyof Draft>(k: K, v: Draft[K]) => setDraft({ ...draft, [k]: v })

  async function save() {
    if (!draft) return
    setBusy(true)
    setProblem('')
    setFields({})
    try {
      await api.savePerson({ ...draft, bio: fromEditable(draft.bio) })
      notify('Saved. It’s now on the website.')
      nav(back)
    } catch (e) {
      setProblem(errorText(e))
      if (e instanceof ApiError) setFields(e.fields)
      window.scrollTo({ top: 0, behavior: 'smooth' })
    } finally {
      setBusy(false)
    }
  }

  async function remove() {
    if (!draft?.id) return
    try {
      await api.deletePerson(draft.id)
      notify(`${draft.name} has been removed.`)
      nav(back)
    } catch (e) {
      setConfirmDelete(false)
      setProblem(errorText(e))
    }
  }

  return (
    <Page title={id ? `Change ${draft.name || 'this ' + info.one}` : `Add a ${info.one}`} back={back} backLabel="Back to the list">
      {problem && <p className="notice notice--bad" role="alert">{problem}</p>}
      <TextField label="Name" value={draft.name} onChange={(v) => set('name', v)} error={fields.name}
        placeholder={kind === 'dj' ? 'For example: DJ Sam' : 'For example: Sam'} />
      <TextField label="Second Life name (optional)" value={draft.slName} onChange={(v) => set('slName', v)} error={fields.slName} />
      <PhotoPicker label="Picture" url={draft.photoUrl}
        onPicked={(pid, url) => setDraft({ ...draft, photoImageId: pid, photoUrl: url })}
        onRemove={() => setDraft({ ...draft, photoImageId: null, photoUrl: undefined })} />
      <TextArea label="About them" rows={8} help="Each time you press Enter, a new paragraph starts." value={draft.bio} onChange={(v) => set('bio', v)} error={fields.bio} />
      <TextArea label="When they play (optional)" rows={4} help="Put each day and time on its own line, for example: Mondays 4pm–6pm SLT"
        value={draft.schedule} onChange={(v) => set('schedule', v)} error={fields.schedule} />
      <Toggle label="Show on the website" help={`Turn this off to hide this ${info.one} without deleting them.`} checked={draft.active} onChange={(v) => set('active', v)} />

      <div className="actions">
        <Button busy={busy} onClick={() => void save()}>Save</Button>
        <Button kind="secondary" onClick={() => nav(back)}>Cancel</Button>
        {draft.id && <Button kind="danger" onClick={() => setConfirmDelete(true)}>Delete</Button>}
      </div>
      {confirmDelete && (
        <Confirm title={`Delete ${draft.name}?`} message="They will be removed for good. If you only want them off the website for now, choose “No, keep them” and turn off “Show on the website”."
          confirmLabel="Yes, delete" cancelLabel="No, keep them" onConfirm={() => void remove()} onCancel={() => setConfirmDelete(false)} />
      )}
    </Page>
  )
}
