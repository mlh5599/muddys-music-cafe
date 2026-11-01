import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, ApiError } from '../api'
import type { Settings } from '../api'
import { Button, errorText, Loading, Page, TextArea, TextField, useLoad, useNotify } from '../ui'

const links: { key: string; label: string; help?: string }[] = [
  { key: 'radio_stream_url', label: 'Radio stream address', help: 'The address the website’s player plays. Either http:// or https:// works.' },
  { key: 'radio_status_url', label: 'Radio page address', help: 'Where “open in a new tab” goes.' },
  { key: 'club_slurl', label: 'Teleport link to Muddy’s', help: 'The Second Life link people click to visit the club.' },
  { key: 'sl_group_url', label: 'Second Life group link' },
  { key: 'facebook_url', label: 'Facebook link' },
  { key: 'flickr_url', label: 'Flickr link' },
  { key: 'youtube_url', label: 'YouTube link' },
]

export default function Details() {
  const loaded = useLoad(() => api.settings())
  const [s, setS] = useState<Settings | null>(null)
  const [fields, setFields] = useState<Record<string, string>>({})
  const [problem, setProblem] = useState('')
  const [busy, setBusy] = useState(false)
  const nav = useNavigate()
  const notify = useNotify()

  useEffect(() => { if (loaded.data) setS(loaded.data) }, [loaded.data])
  if (!s) return <Page title="Website details" back="/" backLabel="Home"><Loading error={loaded.error} /></Page>

  async function save() {
    if (!s) return
    setBusy(true)
    setProblem('')
    setFields({})
    try {
      await api.saveSettings(s)
      notify('Saved. It’s now on the website.')
      nav('/')
    } catch (e) {
      setProblem(errorText(e))
      if (e instanceof ApiError) setFields(e.fields)
      window.scrollTo({ top: 0, behavior: 'smooth' })
    } finally {
      setBusy(false)
    }
  }

  return (
    <Page title="Website details" back="/" backLabel="Home">
      {problem && <p className="notice notice--bad" role="alert">{problem}</p>}
      <p className="help">Paste web addresses in full, starting with https://</p>
      {links.map((l) => (
        <TextField key={l.key} label={l.label} help={l.help} value={s[l.key] ?? ''} error={fields[l.key]} placeholder="https://"
          onChange={(v) => setS({ ...s, [l.key]: v })} />
      ))}
      <TextArea label="Hiring message" rows={6} help="Put each point on its own line. They show as a list on the home page."
        value={s.hiring_text ?? ''} error={fields.hiring_text} onChange={(v) => setS({ ...s, hiring_text: v })} />
      <div className="actions">
        <Button busy={busy} onClick={() => void save()}>Save</Button>
        <Button kind="secondary" onClick={() => nav('/')}>Cancel</Button>
      </div>
    </Page>
  )
}
