import { useEffect, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { api, ApiError } from '../api'
import type { EventItem } from '../api'
import { DEFAULT_ZONE, fromPickerValue, toPickerValue } from '../time'
import { Button, Confirm, errorText, Loading, Page, PhotoPicker, TextArea, TextField, Toggle, useLoad, useNotify } from '../ui'

const colours: { id: EventItem['badgeStyle']; name: string }[] = [
  { id: 'orange', name: 'Orange' },
  { id: 'purple', name: 'Purple' },
  { id: 'blue', name: 'Blue' },
  { id: 'pink', name: 'Pink' },
]

interface Draft extends Omit<EventItem, 'id' | 'startsAt' | 'endsAt'> {
  id?: number
  start: string
  end: string
}

function blank(away: boolean): Draft {
  return {
    kind: away ? 'road' : 'event', title: '', summary: '', badgeText: '', badgeStyle: 'orange',
    start: '', end: '', posterImageId: null, linkUrl: '', linkLabel: '', link2Url: '', link2Label: '',
    featured: false, published: true,
  }
}

export default function EventForm() {
  const { id } = useParams()
  const [params] = useSearchParams()
  const nav = useNavigate()
  const notify = useNotify()
  const settings = useLoad(() => api.settings())
  const zone = settings.data?.timezone || DEFAULT_ZONE
  const [draft, setDraft] = useState<Draft | null>(id ? null : blank(params.get('away') === '1'))
  const [loadError, setLoadError] = useState('')
  const [fields, setFields] = useState<Record<string, string>>({})
  const [problem, setProblem] = useState('')
  const [busy, setBusy] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)

  useEffect(() => {
    if (!id || !settings.data) return
    api.event(Number(id)).then(
      (e) => setDraft({ ...e, start: toPickerValue(e.startsAt, zone), end: toPickerValue(e.endsAt, zone) }),
      (e) => setLoadError(errorText(e)),
    )
  }, [id, settings.data, zone])

  if (!draft || !settings.data) return <Page title="Event" back="/events" backLabel="All events"><Loading error={loadError || settings.error} /></Page>

  const set = <K extends keyof Draft>(k: K, v: Draft[K]) => setDraft({ ...draft, [k]: v })
  const away = draft.kind === 'road'

  async function save() {
    if (!draft) return
    setBusy(true)
    setProblem('')
    setFields({})
    const { start, end, ...rest } = draft
    try {
      await api.saveEvent({ ...rest, startsAt: fromPickerValue(start, zone), endsAt: fromPickerValue(end, zone) })
      notify('Saved. It’s now on the website.')
      nav('/events')
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
      await api.deleteEvent(draft.id)
      notify('That event has been removed.')
      nav('/events')
    } catch (e) {
      setConfirmDelete(false)
      setProblem(errorText(e))
    }
  }

  return (
    <Page title={id ? 'Change this event' : away ? 'Add an event away from Muddy’s' : 'Add an event'} back="/events" backLabel="All events">
      {problem && <p className="notice notice--bad" role="alert">{problem}</p>}

      <TextField label="Event name" value={draft.title} onChange={(v) => set('title', v)} error={fields.title} placeholder="For example: Friday Night Live" />
      <TextArea label="What’s happening?" help="Just type it as you would tell a friend." value={draft.summary} onChange={(v) => set('summary', v)} error={fields.summary} />

      <div className="two">
        <TextField label="Starts" type="datetime-local" value={draft.start} onChange={(v) => set('start', v)} error={fields.startsAt} />
        <TextField label="Ends" type="datetime-local" value={draft.end} onChange={(v) => set('end', v)} error={fields.endsAt} />
      </div>
      <p className="help">Times are Second Life time. After the end time, the event leaves the website by itself.</p>

      <PhotoPicker label="Poster or picture" shape="wide" url={draft.posterUrl}
        onPicked={(pid, url) => setDraft({ ...draft, posterImageId: pid, posterUrl: url })}
        onRemove={() => setDraft({ ...draft, posterImageId: null, posterUrl: undefined })} />
      {fields.posterImageId && <p className="field-error" role="alert">{fields.posterImageId}</p>}

      <div className="field">
        <span className="label" id="colour-label">Label</span>
        <p className="help">A small tag shown above the event name, like “Tonight” or “Live”. You can leave it empty.</p>
        <TextField label="Label words" value={draft.badgeText} onChange={(v) => set('badgeText', v)} error={fields.badgeText} />
        <div className="swatches" role="radiogroup" aria-labelledby="colour-label">
          {colours.map((c) => (
            <label key={c.id} className={`swatch swatch--${c.id}${draft.badgeStyle === c.id ? ' swatch--on' : ''}`}>
              <input type="radio" name="colour" checked={draft.badgeStyle === c.id} onChange={() => set('badgeStyle', c.id)} />
              {c.name}
            </label>
          ))}
        </div>
        {fields.badgeStyle && <p className="field-error" role="alert">{fields.badgeStyle}</p>}
      </div>

      <h2 className="section">Buttons that link somewhere</h2>
      <p className="help">Optional. Paste a web address, then say what the button should say.</p>
      <div className="two">
        <TextField label="First button: web address" value={draft.linkUrl} onChange={(v) => set('linkUrl', v)} error={fields.linkUrl} placeholder="https://" />
        <TextField label="First button: words" value={draft.linkLabel} onChange={(v) => set('linkLabel', v)} error={fields.linkLabel} placeholder="For example: Find out more" />
        <TextField label="Second button: web address" value={draft.link2Url} onChange={(v) => set('link2Url', v)} error={fields.link2Url} placeholder="https://" />
        <TextField label="Second button: words" value={draft.link2Label} onChange={(v) => set('link2Label', v)} error={fields.link2Label} />
      </div>

      <Toggle label="Show on the website" help="Turn this off to hide the event without deleting it." checked={draft.published} onChange={(v) => set('published', v)} />
      {!away && <Toggle label="Feature this event at the top" help="Gives it a big spot at the top of the home page." checked={draft.featured} onChange={(v) => set('featured', v)} />}

      <div className="preview" aria-label="Preview">
        <p className="muted">How it will look</p>
        {draft.badgeText && <span className={`badge badge--${draft.badgeStyle}`}>{draft.badgeText}</span>}
        <p className="preview-title">{draft.title || 'Event name'}</p>
        {draft.summary && <p>{draft.summary}</p>}
      </div>

      <div className="actions">
        <Button busy={busy} onClick={() => void save()}>Save</Button>
        <Button kind="secondary" onClick={() => nav('/events')}>Cancel</Button>
        {draft.id && <Button kind="danger" onClick={() => setConfirmDelete(true)}>Delete this event</Button>}
      </div>
      {draft.id && <p className="help">Tip: to keep an event but take it off the website, turn off “Show on the website” instead of deleting it.</p>}

      {confirmDelete && (
        <Confirm title="Delete this event?" message="It will be removed for good. If you only want it off the website for now, choose “No, keep it” and turn off “Show on the website”."
          confirmLabel="Yes, delete it" onConfirm={() => void remove()} onCancel={() => setConfirmDelete(false)} />
      )}
    </Page>
  )
}
