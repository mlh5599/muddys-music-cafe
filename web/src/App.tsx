import { Route, Routes } from 'react-router-dom'
import Details from './pages/Details'
import EventForm from './pages/EventForm'
import Events from './pages/Events'
import Home from './pages/Home'
import Partners from './pages/Partners'
import PersonForm from './pages/PersonForm'
import People from './pages/People'
import { Page, ToastProvider } from './ui'

export default function App() {
  return (
    <ToastProvider>
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/events" element={<Events />} />
        <Route path="/events/new" element={<EventForm />} />
        <Route path="/events/:id" element={<EventForm />} />
        <Route path="/people/:kind" element={<People />} />
        <Route path="/people/:kind/new" element={<PersonForm />} />
        <Route path="/people/:kind/:id" element={<PersonForm />} />
        <Route path="/partners" element={<Partners />} />
        <Route path="/details" element={<Details />} />
        <Route path="*" element={<Page title="We couldn’t find that page" back="/" backLabel="Home"><p>Please go back to the home page.</p></Page>} />
      </Routes>
    </ToastProvider>
  )
}
