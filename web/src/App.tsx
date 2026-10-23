import { lazy, Suspense, useEffect } from 'react'
import { EmptyState } from './components/EmptyState'
import { Nav } from './components/Nav'
import { DEMO_GRAPH_ID, graphPath } from './graphs'
import { HomePage } from './pages/HomePage'
import { LandingPage } from './pages/LandingPage'
import { LibraryPage } from './pages/LibraryPage'
import { linkClick, navigate, useLocation } from './router'

// Страница графа тянет Cytoscape, KaTeX и Markdown — грузим её только когда она нужна.
const GraphPage = lazy(() => import('./pages/GraphPage').then((m) => ({ default: m.GraphPage })))

export default function App() {
  const { path } = useLocation()

  // Старые ссылки вида /#/n/<slug> ведут на узел демо-графа.
  useEffect(() => {
    const m = window.location.hash.match(/^#\/n\/([a-z0-9-]+)$/)
    if (m) navigate(graphPath(DEMO_GRAPH_ID, m[1]), { replace: true })
  }, [])

  const graphMatch = path.match(/^\/g\/([a-z0-9-]+)\/?$/)
  // Главная — отдельная страница без бокового меню.
  if (path === '/') return <LandingPage />
  const section = graphMatch || path.startsWith('/graphs') ? 'graphs' : 'new'

  let page
  if (graphMatch)
    page = (
      <Suspense fallback={<div className="loading-line page-loading" aria-label="Загрузка" />}>
        <GraphPage id={graphMatch[1]} />
      </Suspense>
    )
  else if (path === '/graphs' || path === '/graphs/') page = <LibraryPage />
  else if (path === '/new' || path === '/new/') page = <HomePage />
  else
    page = (
      <div className="page">
        <EmptyState
          title="Такой страницы нет"
          action={
            <a className="button primary" href="/" onClick={linkClick('/')}>
              На главную
            </a>
          }
        >
          Проверьте адрес или начните с главной.
        </EmptyState>
      </div>
    )

  return (
    <div className="shell">
      <Nav active={section} />
      <main className="shell-main">{page}</main>
    </div>
  )
}
