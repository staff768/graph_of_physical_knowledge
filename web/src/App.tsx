import { useCallback, useEffect, useMemo, useState } from 'react'
import { api, type Explanation, type Graph, type NodeDetails, type NodeType } from './api'
import { GraphView } from './components/GraphView'
import { Search } from './components/Search'
import { Sidebar, type Tab } from './components/Sidebar'
import { nodeTypeOrder, nodeTypes } from './meta'

// Выбранный узел хранится в hash (#/n/<slug>), чтобы ссылкой можно было поделиться.
function slugFromHash(): string | null {
  const m = window.location.hash.match(/^#\/n\/([a-z0-9-]+)$/)
  return m ? m[1] : null
}

export default function App() {
  const [graph, setGraph] = useState<Graph | null>(null)
  const [selected, setSelected] = useState<string | null>(slugFromHash)
  // Данные выбранного узла; хранятся вместе со slug, чтобы не показывать устаревшие.
  const [loaded, setLoaded] = useState<{ slug: string; details: NodeDetails; chain: Explanation } | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [tab, setTab] = useState<Tab>('chain')
  const [chainOnly, setChainOnly] = useState(false)
  const [hiddenTypes, setHiddenTypes] = useState<Set<NodeType>>(new Set())
  const [focus, setFocus] = useState<{ slug: string; seq: number } | null>(null)

  useEffect(() => {
    api.graph().then(setGraph).catch((e: Error) => setError(e.message))
  }, [])

  useEffect(() => {
    const onHash = () => setSelected(slugFromHash())
    window.addEventListener('hashchange', onHash)
    return () => window.removeEventListener('hashchange', onHash)
  }, [])

  const select = useCallback((slug: string | null) => {
    window.location.hash = slug ? `/n/${slug}` : ''
    setSelected(slug)
  }, [])

  useEffect(() => {
    if (!selected) return
    const ctrl = new AbortController()
    Promise.all([api.node(selected, ctrl.signal), api.chain(selected, ctrl.signal)])
      .then(([details, chain]) => {
        setLoaded({ slug: selected, details, chain })
        setError(null)
        // Для исходных понятий разбирать нечего — сразу показываем описание.
        setTab(chain.steps.length > 1 ? 'chain' : 'node')
      })
      .catch((e: Error) => {
        if (e.name !== 'AbortError') setError(e.message)
      })
    return () => ctrl.abort()
  }, [selected])

  const current = selected && loaded?.slug === selected ? loaded : null
  const details = current?.details ?? null
  const chain = current?.chain ?? null
  const loading = !!selected && !current && !error

  const nodesBySlug = useMemo(() => new Map((graph?.nodes ?? []).map((n) => [n.slug, n])), [graph])
  const chainSet = useMemo(() => (chain ? new Set(chain.graph.nodes.map((n) => n.slug)) : null), [chain])
  const focusNode = useCallback((slug: string) => setFocus((f) => ({ slug, seq: (f?.seq ?? 0) + 1 })), [])

  const toggleType = (t: NodeType) =>
    setHiddenTypes((prev) => {
      const next = new Set(prev)
      if (next.has(t)) next.delete(t)
      else next.add(t)
      return next
    })

  return (
    <div className="app">
      <header className="topbar">
        <h1 onClick={() => select(null)}>Граф физических знаний</h1>
        <Search onSelect={select} />
        <div className="filters">
          {nodeTypeOrder.map((t) => (
            <label key={t} className={hiddenTypes.has(t) ? 'filter off' : 'filter'}>
              <input type="checkbox" checked={!hiddenTypes.has(t)} onChange={() => toggleType(t)} />
              <span className="dot" style={{ background: nodeTypes[t].color }} />
              {nodeTypes[t].plural}
            </label>
          ))}
          <label className={chainSet ? 'filter' : 'filter disabled'}>
            <input type="checkbox" checked={chainOnly} disabled={!chainSet} onChange={(e) => setChainOnly(e.target.checked)} />
            Только цепочка
          </label>
        </div>
      </header>
      <main className="main">
        {graph ? (
          <GraphView
            graph={graph}
            selected={selected}
            chain={chainSet}
            chainOnly={chainOnly && !!chainSet}
            hiddenTypes={hiddenTypes}
            focus={focus}
            onSelect={select}
          />
        ) : (
          <div className="graph placeholder">{error ? `Не удалось загрузить граф: ${error}` : 'Загрузка графа…'}</div>
        )}
        <Sidebar
          nodes={nodesBySlug}
          details={details}
          chain={chain}
          loading={loading}
          error={graph ? error : null}
          tab={tab}
          onTab={setTab}
          onSelect={select}
          onFocus={focusNode}
        />
      </main>
    </div>
  )
}
