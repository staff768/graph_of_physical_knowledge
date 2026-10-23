import { useEffect } from 'react'
import { Nav } from './components/Nav'
import { DEMO_GRAPH_ID, graphPath } from './graphs'
import { GraphPage } from './pages/GraphPage'
import { HomePage } from './pages/HomePage'
import { LibraryPage } from './pages/LibraryPage'
import { navigate, useLocation } from './router'

export default function App() {
  const { path } = useLocation()

  // Старые ссылки вида /#/n/<slug> ведут на узел демо-графа.
  useEffect(() => {
    const m = window.location.hash.match(/^#\/n\/([a-z0-9-]+)$/)
    if (m) navigate(graphPath(DEMO_GRAPH_ID, m[1]), { replace: true })
  }, [])

  const graphMatch = path.match(/^\/g\/([a-z0-9-]+)\/?$/)
  const section = graphMatch || path.startsWith('/graphs') ? 'graphs' : 'home'

  let page
  if (graphMatch) page = <GraphPage id={graphMatch[1]} />
  else if (path === '/graphs' || path === '/graphs/') page = <LibraryPage />
  else if (path === '/') page = <HomePage />
  else
    page = (
      <div className="page">
        <h1 className="page-title">Страница не найдена</h1>
      </div>
    )

  return (
    <div className="shell">
      <Nav active={section} />
      <main className="shell-main">{page}</main>
    </div>
  )
}
