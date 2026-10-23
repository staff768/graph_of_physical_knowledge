import { useCallback, useEffect, useMemo, useState } from 'react'
import { EmptyState } from '../components/EmptyState'
import { api, type Explanation, type GraphNode, type NodeDetails, type NodeType } from '../api'
import { ConceptPanel, type Tab } from '../components/ConceptPanel'
import { GraphView } from '../components/GraphView'
import { Search } from '../components/Search'
import { Tex } from '../components/Tex'
import { FailedView, ProcessingView } from '../components/ProcessingView'
import { useMediaQuery } from '../hooks'
import { conceptCount, graphPath, linkCount, plural, reloadGraph, useGraph } from '../graphs'
import { Icon } from '../components/Icon'
import { icons } from '../icons'
import { nodeTypeOrder, nodeTypes } from '../meta'
import { linkClick, navigate, useLocation } from '../router'

type View = 'graph' | 'list'

export function GraphPage({ id }: { id: string }) {
  const { query } = useLocation()
  const selected = query.get('n')
  const [version, setVersion] = useState(0)
  const { data: graph, error: graphError, notFound } = useGraph(id, version)
  const meta = graph?.meta
  // Граф достроился — перечитываем его.
  const reload = useCallback(() => {
    reloadGraph(id)
    setVersion((v) => v + 1)
  }, [id])
  // Легенда «из конспекта / дополнено» нужна только графам, построенным по конспекту.
  const fromNotes = useMemo(() => !!graph?.nodes.some((n) => n.origin), [graph])

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
  // На телефоне карточка понятия — нижняя шторка поверх графа.
  const isPhone = useMediaQuery('(max-width: 680px)')
  const [sheetOpen, setSheetOpen] = useState(false)

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
  // Следствия выбранного понятия — обход графа вперёд по связям (ширина = удалённость).
  const consequences = useMemo(() => {
    if (!graph || !selected) return []
    const out = new Map<string, string[]>()
    for (const e of graph.edges) out.set(e.from, [...(out.get(e.from) ?? []), e.to])
    const depth = new Map<string, number>([[selected, 0]])
    const queue = [selected]
    while (queue.length > 0) {
      const cur = queue.shift()!
      for (const next of out.get(cur) ?? []) {
        if (!depth.has(next)) {
          depth.set(next, depth.get(cur)! + 1)
          queue.push(next)
        }
      }
    }
    return [...depth.entries()]
      .filter(([slug]) => slug !== selected)
      .map(([slug, d]) => ({ node: nodesBySlug.get(slug)!, depth: d }))
      .filter((c) => c.node)
      .sort((a, b) => a.depth - b.depth || a.node.title.localeCompare(b.node.title, 'ru'))
  }, [graph, selected, nodesBySlug])

  // На вкладке «Что следует» подсвечиваются следствия, на остальных — основания (цепочка).
  const chainSet = useMemo(() => {
    if (tab === 'consequences' && selected) return new Set([selected, ...consequences.map((c) => c.node.slug)])
    return chain ? new Set(chain.graph.nodes.map((n) => n.slug)) : null
  }, [tab, selected, consequences, chain])
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
        <EmptyState
          title="Граф не найден"
          action={
            <a className="button primary" href="/graphs" onClick={linkClick('/graphs')}>
              Перейти к моим графам
            </a>
          }
        >
          Возможно, ссылка устарела или граф удалили. Все графы — в библиотеке.
        </EmptyState>
      </div>
    )
  }

  return (
    <div className="graph-page">
      <header className="graph-head">
        <div className="graph-head-row">
          <nav className="breadcrumbs" aria-label="Путь">
            <a href="/graphs" onClick={linkClick('/graphs')}>Мои графы</a>
            <Icon src={icons.chevronRight} size={14} className="muted" />
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
                {[
                  meta.source,
                  meta.pages ? `${meta.pages} ${plural(meta.pages, 'страница', 'страницы', 'страниц')}` : '',
                  meta.status === 'ready' ? conceptCount(meta.nodeCount) : '',
                  meta.status === 'ready' ? linkCount(meta.edgeCount) : '',
                ]
                  .filter(Boolean)
                  .join(' · ')}
              </div>
            )}
          </div>
          <div className="graph-actions">
            <button
              className="button"
              disabled
              title={meta?.demo ? 'У демо-графа нет исходного конспекта' : 'Исходный файл удаляется сразу после построения графа, поэтому открыть его нельзя'}
            >
              <Icon src={icons.fileText} size={18} />
              Источник
            </button>
            <button className="button primary" onClick={share} disabled={meta?.status !== 'ready'}>
              <Icon src={icons.shareWhite} size={18} />
              {copied ? 'Ссылка скопирована' : 'Поделиться'}
            </button>
          </div>
        </div>
      </header>

      {meta?.status === 'processing' ? (
        <ProcessingView meta={meta} onDone={reload} />
      ) : meta?.status === 'failed' ? (
        <FailedView meta={meta} />
      ) : (
        <>
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
                        <Icon src={icons.legendDot} size={7} className="accent" />
                        Выбранное понятие и его связи
                      </span>
                    ) : (
                      <span className="legend-item muted">Нажмите на понятие, чтобы увидеть его цепочку</span>
                    )}
                    {fromNotes && (
                      <>
                        <span className="legend-item">
                          <span className="legend-line" /> Из конспекта
                        </span>
                        <span className="legend-item">
                          <span className="legend-line dashed" /> Дополнено ИИ
                        </span>
                      </>
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
              source={meta?.demo ? undefined : meta?.source}
              consequences={consequences}
              details={details}
              chain={chain}
              loading={loading}
              error={graph ? error : null}
              tab={tab}
              onTab={setTab}
              onSelect={select}
              sheet={isPhone ? { expanded: sheetOpen, onToggle: () => setSheetOpen((o) => !o) } : undefined}
              onFocus={(slug) => {
                setView('graph')
                setSheetOpen(false)
                focusNode(slug)
              }}
            />
          </div>
        </>
      )}
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
