import { Link } from 'react-router-dom'
import { api } from '../api'
import { Loading, useLoad } from '../ui'

const tiles = [
  { to: '/events', title: 'Events', text: 'Add or change what’s happening at Muddy’s.' },
  { to: '/people/dj', title: 'DJs', text: 'Add a DJ, change their picture or schedule.' },
  { to: '/people/host', title: 'Hosts', text: 'Add a host or update their details.' },
  { to: '/partners', title: 'Partners', text: 'The businesses we’re proud to work with.' },
  { to: '/details', title: 'Website details', text: 'Radio link, social links and the hiring message.' },
]

export default function Home() {
  const me = useLoad(() => api.me())
  return (
    <main>
      <div className="page-head">
        <h1>Muddy’s website</h1>
      </div>
      {me.error ? <Loading error={me.error} /> : <p className="muted">{me.data ? `Signed in as ${me.data.email}.` : ' '}</p>}
      <p className="lead">What would you like to change?</p>
      <div className="tiles">
        {tiles.map((t) => (
          <Link key={t.to} to={t.to} className="tile">
            <span className="tile-title">{t.title}</span>
            <span className="tile-text">{t.text}</span>
          </Link>
        ))}
      </div>
      <p className="muted">Anything you save shows up on the website straight away.</p>
    </main>
  )
}
