import { useCallback, useEffect, useMemo, useState } from 'react'
import { api, type Explanation, type GraphNode, type NodeDetails, type NodeType } from '../api'
import { ConceptPanel, type Tab } from '../components/ConceptPanel'
import { GraphView } from '../components/GraphView'
import { Search } from '../components/Search'
import { Tex } from '../components/Tex'
import { conceptCount, graphPath, linkCount, useGraph } from '../graphs'
import { Icon } from '../components/Icon'
import { icons } from '../icons'
import { nodeTypeOrder, nodeTypes } from '../meta'
import { linkClick, navigate, useLocation } from '../router'

type View = 'graph' | 'list'

export function GraphPage({ id }: { id: string }) {
  const { query } = useLocation()
  const selected = query.get('n')
  const { data: graph, error: graphError, notFound } = useGraph(id)
  const meta = graph?.meta

  // Данные выбранного узла; хранятся вместе со slug, чтобы не показывать устаревшие.
  const [loaded, setLoaded] = useState<{ slug: string; details: NodeDetails; chain: Explanation } | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [tab, setTab] = useState<Tab>('chain')
  const [view, setView] = useState<View>('graph')
  // На большом графе цепочку читать удобнее отдельно; весь граф — по кнопке в легенде.
  const [chainOnly, setChainOnly] = useState(true)
  const [hiddenTypes, setHiddenTypes] = useState<Set<NodeType>>(new Set())
  const [focus, setFocus] = useState<{ slug: string; seq: number } | null>(null)
  const [copied, setCopied] = useState(false)

  const select = useCallback((slug: string | null) => navigate(graphPath(id, slug), { replace: true }), [id])

  useEffect(() => {
    if (!selected) return
    const ctrl = new AbortController()
    Promise.all([api.node(id, selected, ctrl.signal), api.chain(id, selected, ctrl.signal)])
      .then(([details, chain]) => {
        setLoaded({ slug: selected, details, chain })
        setError(null)
        // Для исходных понятий разбирать нечего — сразу показываем описание.
        setTab(chain.steps.length > 1 ? 'chain' : 'concept')
      })
      .catch((e: Error) => {
        if (e.name !== 'AbortError') setError(e.message)
      })
    return () => ctrl.abort()
  }, [id, selected])

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

  const share = async () => {
    try {
      await navigator.clipboard.writeText(window.location.href)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      window.prompt('Скопируйте ссылку', window.location.href)
    }
  }

  if (notFound) {
    return (
      <div className="page">
        <h1 className="page-title">Граф не найден</h1>
        <p className="page-lead">
          Возможно, ссылка устарела. <a href="/graphs" onClick={linkClick('/graphs')}>Перейти к моим графам</a>
        </p>
      </div>
    )
  }

  return (
    <div className="graph-page">
      <header className="graph-head">
        <div className="graph-head-row">
          <nav className="breadcrumbs" aria-label="Путь">
            <a href="/graphs" onClick={linkClick('/graphs')}>Мои графы</a>
            <Icon src={icons.chevronRight} size={14} />
            <span>{meta?.title ?? '…'}</span>
          </nav>
          {meta?.demo && (
            <span className="status">
              <Icon src={icons.circleCheck} size={15} />
              Проверен вручную
            </span>
          )}
        </div>
        <div className="graph-head-row">
          <div>
            <h1 className="graph-title">{meta?.title ?? 'Загрузка…'}</h1>
            {meta && (
              <div className="graph-meta">
                {[meta.source, conceptCount(meta.nodeCount), linkCount(meta.edgeCount)].filter(Boolean).join(' · ')}
              </div>
            )}
          </div>
          <div className="graph-actions">
            <button className="button" disabled title="Исходный конспект появится у графов, построенных из загрузки">
              <Icon src={icons.fileText} size={18} />
              Источник
            </button>
            <button className="button primary" onClick={share}>
              <Icon src={icons.shareWhite} size={18} />
              {copied ? 'Ссылка скопирована' : 'Поделиться'}
            </button>
          </div>
        </div>
      </header>

      <div className="toolbar">
        <div className="view-tabs" role="tablist">
          <button role="tab" aria-selected={view === 'graph'} className={view === 'graph' ? 'active' : ''} onClick={() => setView('graph')}>
            <Icon src={icons.networkTab} size={17} />
            Граф знаний
          </button>
          <button role="tab" aria-selected={view === 'list'} className={view === 'list' ? 'active' : ''} onClick={() => setView('list')}>
            Список понятий
          </button>
        </div>
        <Search graphId={id} onSelect={select} />
      </div>

      <div className="workspace">
        <div className="workspace-main">
          {!graph ? (
            <div className="placeholder">{graphError ? `Не удалось загрузить граф: ${graphError}` : 'Загрузка графа…'}</div>
          ) : view === 'graph' ? (
            <>
              <GraphView
                graph={graph}
                selected={selected}
                chain={chainSet}
                chainOnly={chainOnly && !!chainSet}
                hiddenTypes={hiddenTypes}
                focus={focus}
                onSelect={select}
              />
              <div className="legend">
                {chainSet ? (
                  <span className="legend-item">
                    <Icon src={icons.legendDot} size={7} />
                    Выбранное понятие и его связи
                  </span>
                ) : (
                  <span className="legend-item muted">Нажмите на понятие, чтобы увидеть его цепочку</span>
                )}
                {nodeTypeOrder.map((t) => (
                  <button key={t} className={hiddenTypes.has(t) ? 'legend-toggle off' : 'legend-toggle'} onClick={() => toggleType(t)} aria-pressed={!hiddenTypes.has(t)}>
                    <span className="type-dot" style={{ background: nodeTypes[t].color }} />
                    {nodeTypes[t].plural}
                  </button>
                ))}
                {chainSet && (
                  <button className="legend-toggle on" onClick={() => setChainOnly((v) => !v)}>
                    {chainOnly ? 'Показать весь граф' : 'Только цепочка'}
                  </button>
                )}
              </div>
            </>
          ) : (
            <ConceptList nodes={graph.nodes} selected={selected} onSelect={select} />
          )}
        </div>
        <ConceptPanel
          nodes={nodesBySlug}
          details={details}
          chain={chain}
          loading={loading}
          error={graph ? error : null}
          tab={tab}
          onTab={setTab}
          onSelect={select}
          onFocus={(slug) => {
            setView('graph')
            focusNode(slug)
          }}
        />
      </div>
    </div>
  )
}

function ConceptList({ nodes, selected, onSelect }: { nodes: GraphNode[]; selected: string | null; onSelect: (s: string) => void }) {
  return (
    <div className="concept-list">
      {nodeTypeOrder.map((t) => {
        const group = nodes.filter((n) => n.type === t).sort((a, b) => a.title.localeCompare(b.title, 'ru'))
        if (group.length === 0) return null
        return (
          <section key={t}>
            <h2 className="caption">
              {nodeTypes[t].plural.toUpperCase()} · {group.length}
            </h2>
            <div className="concept-grid">
              {group.map((n) => (
                <button key={n.slug} className={n.slug === selected ? 'concept-item selected' : 'concept-item'} onClick={() => onSelect(n.slug)}>
                  <span className="concept-item-title">{n.title}</span>
                  {n.formula && n.type !== 'phenomenon' && (
                    <span className="concept-item-formula">
                      <Tex tex={n.formula} />
                    </span>
                  )}
                  <span className="concept-item-summary">{n.summary}</span>
                </button>
              ))}
            </div>
          </section>
        )
      })}
    </div>
  )
}
