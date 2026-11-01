import { createContext, useCallback, useContext, useEffect, useId, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { api, ApiError } from './api'

// ---- messages ("Saved. It's now on the website.") ----

type Notify = (msg: string, kind?: 'good' | 'bad') => void
const ToastContext = createContext<Notify>(() => {})
export const useNotify = () => useContext(ToastContext)

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toast, setToast] = useState<{ msg: string; kind: 'good' | 'bad' } | null>(null)
  const timer = useRef<number>(0)
  const notify = useCallback<Notify>((msg, kind = 'good') => {
    setToast({ msg, kind })
    window.clearTimeout(timer.current)
    timer.current = window.setTimeout(() => setToast(null), kind === 'good' ? 5000 : 9000)
  }, [])
  return (
    <ToastContext.Provider value={notify}>
      {children}
      <div className="toast-area" role="status" aria-live="polite">
        {toast && <div className={`toast toast--${toast.kind}`}>{toast.msg}</div>}
      </div>
    </ToastContext.Provider>
  )
}

export const errorText = (e: unknown) => (e instanceof ApiError ? e.message : "Sorry, that didn't work. Please try again.")

// ---- loading data ----

export function useLoad<T>(load: () => Promise<T>, deps: unknown[] = []) {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState('')
  const [tick, setTick] = useState(0)
  useEffect(() => {
    let live = true
    load().then(
      (d) => live && setData(d),
      (e) => live && setError(errorText(e)),
    )
    return () => {
      live = false
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, tick])
  return { data, setData, error, reload: () => setTick((t) => t + 1) }
}

export function Loading({ error }: { error?: string }) {
  if (error) return <p className="notice notice--bad">{error}</p>
  return <p className="muted">One moment…</p>
}

// ---- layout ----

export function Page({ title, back, backLabel, children, actions }: {
  title: string
  back?: string
  backLabel?: string
  children: ReactNode
  actions?: ReactNode
}) {
  return (
    <main>
      <nav className="topbar" aria-label="Main">
        <Link to="/" className="brand">Muddy's website</Link>
        {back && <Link to={back} className="backlink">← {backLabel ?? 'Back'}</Link>}
      </nav>
      <div className="page-head">
        <h1>{title}</h1>
        {actions}
      </div>
      {children}
    </main>
  )
}

// ---- buttons ----

export function Button({ kind = 'primary', busy, children, ...rest }: {
  kind?: 'primary' | 'secondary' | 'danger'
  busy?: boolean
  children: ReactNode
} & React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button type="button" className={`btn btn--${kind}`} disabled={busy || rest.disabled} {...rest}>
      {children}
    </button>
  )
}

export function LinkButton({ to, children, kind = 'primary' }: { to: string; children: ReactNode; kind?: 'primary' | 'secondary' }) {
  return <Link to={to} className={`btn btn--${kind}`}>{children}</Link>
}

// ---- form fields ----

export function Field({ label, help, error, children, htmlFor }: {
  label: string
  help?: string
  error?: string
  children: ReactNode
  htmlFor: string
}) {
  return (
    <div className={`field${error ? ' field--bad' : ''}`}>
      <label htmlFor={htmlFor}>{label}</label>
      {help && <p className="help" id={`${htmlFor}-help`}>{help}</p>}
      {children}
      {error && <p className="field-error" role="alert">{error}</p>}
    </div>
  )
}

export function TextField({ label, help, error, value, onChange, type = 'text', placeholder }: {
  label: string
  help?: string
  error?: string
  value: string
  onChange: (v: string) => void
  type?: string
  placeholder?: string
}) {
  const id = useId()
  return (
    <Field label={label} help={help} error={error} htmlFor={id}>
      <input id={id} type={type} value={value} placeholder={placeholder} aria-describedby={help ? `${id}-help` : undefined}
        onChange={(e) => onChange(e.target.value)} />
    </Field>
  )
}

export function TextArea({ label, help, error, value, onChange, rows = 5 }: {
  label: string
  help?: string
  error?: string
  value: string
  onChange: (v: string) => void
  rows?: number
}) {
  const id = useId()
  return (
    <Field label={label} help={help} error={error} htmlFor={id}>
      <textarea id={id} rows={rows} value={value} aria-describedby={help ? `${id}-help` : undefined}
        onChange={(e) => onChange(e.target.value)} />
    </Field>
  )
}

export function Toggle({ label, help, checked, onChange }: {
  label: string
  help?: string
  checked: boolean
  onChange: (v: boolean) => void
}) {
  const id = useId()
  return (
    <div className="toggle">
      <input id={id} type="checkbox" checked={checked} onChange={(e) => onChange(e.target.checked)} />
      <label htmlFor={id}>
        <span className="toggle-label">{label}</span>
        {help && <span className="help">{help}</span>}
      </label>
    </div>
  )
}

// ---- pictures ----

export function PhotoPicker({ label, url, onPicked, onRemove, shape = 'square' }: {
  label: string
  url?: string
  onPicked: (id: number, url: string) => void
  onRemove: () => void
  shape?: 'square' | 'wide'
}) {
  const input = useRef<HTMLInputElement>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [over, setOver] = useState(false)

  async function upload(file: File | undefined) {
    if (!file) return
    setBusy(true)
    setError('')
    try {
      const img = await api.uploadImage(file)
      onPicked(img.id, img.url)
    } catch (e) {
      setError(errorText(e))
    } finally {
      setBusy(false)
      if (input.current) input.current.value = ''
    }
  }

  return (
    <div className="field">
      <span className="label">{label}</span>
      <div
        className={`photo photo--${shape}${over ? ' photo--over' : ''}`}
        onDragOver={(e) => { e.preventDefault(); setOver(true) }}
        onDragLeave={() => setOver(false)}
        onDrop={(e) => { e.preventDefault(); setOver(false); void upload(e.dataTransfer.files[0]) }}
      >
        {url ? <img src={url} alt="" /> : <p className="muted">No picture yet</p>}
      </div>
      <div className="row">
        <Button kind="secondary" busy={busy} onClick={() => input.current?.click()}>
          {busy ? 'Uploading…' : url ? 'Choose a different picture' : 'Choose a picture'}
        </Button>
        {url && <Button kind="secondary" onClick={onRemove}>Remove picture</Button>}
      </div>
      <input ref={input} type="file" accept="image/*" hidden onChange={(e) => void upload(e.target.files?.[0])} />
      <p className="help">You can also drag a picture onto the box above.</p>
      {error && <p className="field-error" role="alert">{error}</p>}
    </div>
  )
}

// ---- confirm before doing something permanent ----

export function Confirm({ title, message, confirmLabel, cancelLabel = 'No, keep it', onConfirm, onCancel }: {
  title: string
  message: string
  confirmLabel: string
  cancelLabel?: string
  onConfirm: () => void
  onCancel: () => void
}) {
  const cancel = useRef<HTMLButtonElement>(null)
  useEffect(() => cancel.current?.focus(), [])
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onCancel()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onCancel])
  return (
    <div className="overlay" role="presentation">
      <div className="dialog" role="alertdialog" aria-modal="true" aria-labelledby="confirm-title">
        <h2 id="confirm-title">{title}</h2>
        <p>{message}</p>
        <div className="row">
          <button ref={cancel} type="button" className="btn btn--primary" onClick={onCancel}>{cancelLabel}</button>
          <Button kind="danger" onClick={onConfirm}>{confirmLabel}</Button>
        </div>
      </div>
    </div>
  )
}

// ---- reorder helper ----

export function moved<T>(list: T[], from: number, to: number): T[] {
  if (to < 0 || to >= list.length) return list
  const next = list.slice()
  const [item] = next.splice(from, 1)
  next.splice(to, 0, item)
  return next
}
