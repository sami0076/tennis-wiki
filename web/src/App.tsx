import { Route, Routes } from 'react-router-dom'
import { Layout } from './layout/Layout'
import { Gallery } from './routes/Gallery'
import { Home } from './routes/Home'
import { NotBuiltYet } from './routes/NotBuiltYet'
import { EmptyState, ButtonLink } from './components'

/**
 * The route table. Player, head-to-head and rankings are registered now and
 * filled by #47, #48 and #45 -- registering them here is what lets those issues
 * be a page each rather than a page plus a router change.
 */
export function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/players" element={<NotBuiltYet page="player search" issue={49} />} />
        <Route path="/players/:slug" element={<NotBuiltYet page="player page" issue={47} />} />
        <Route path="/h2h/:a/:b" element={<NotBuiltYet page="head-to-head page" issue={48} />} />
        <Route path="/rankings" element={<NotBuiltYet page="rankings page" issue={45} />} />
        <Route
          path="/simulator"
          element={
            <NotBuiltYet
              page="simulator"
              note="Nothing is built behind it yet: simulation is Phase 3, and internal/simulate is still a package comment."
            />
          }
        />
        <Route path="/_components" element={<Gallery />} />
        <Route
          path="*"
          element={
            <EmptyState
              heading="No such page"
              reason="That address does not match anything on this site."
              action={<ButtonLink to="/">Back to coverage</ButtonLink>}
            />
          }
        />
      </Routes>
    </Layout>
  )
}
