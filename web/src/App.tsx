import { Route, Routes } from 'react-router-dom'

function Home() {
  return (
    <main>
      <h1>Muddy's website admin</h1>
      <p>Coming soon: events, DJs, hosts and more.</p>
    </main>
  )
}

export default function App() {
  return (
    <Routes>
      <Route path="*" element={<Home />} />
    </Routes>
  )
}
