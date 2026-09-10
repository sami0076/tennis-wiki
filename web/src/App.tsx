import { Route, Routes } from 'react-router-dom'
import { Layout } from './layout/Layout'
import { Gallery } from './routes/Gallery'
import { HeadToHead } from './routes/HeadToHead'
import { Home } from './routes/Home'
import { NotBuiltYet } from './routes/NotBuiltYet'
import { Player } from './routes/Player'
import { Players } from './routes/Players'
import { Simulator } from './routes/Simulator'
import { EmptyState, ButtonLink } from './components'

/**
 * The route table. /h2h is the picker and /h2h/:a/:b the comparison, which is
 * the same page: the URL is the state, so a comparison is a link.
 */
export function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/players" element={<Players />} />
        <Route path="/players/:slug" element={<Player />} />
        <Route path="/h2h" element={<HeadToHead />} />
        <Route path="/h2h/:a/:b" element={<HeadToHead />} />
        <Route path="/rankings" element={<NotBuiltYet page="rankings page" issue={45} />} />
        <Route path="/simulator" element={<Simulator />} />
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
