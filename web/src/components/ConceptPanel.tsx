import { useEffect, useState } from 'react'
import { api, ApiError, type Explanation, type GraphNode, type Link, type NodeDetails, type Step } from '../api'
import { Icon } from './Icon'
import { icons } from '../icons'
import { featured, nodeTypes } from '../meta'
import { plural } from '../graphs'
import { Md, Tex } from './Tex'

export type Tab = 'concept' | 'chain' | 'consequences'

// Следствие понятия и его удалённость: 1 — прямое следствие.
export interface Consequence {
  node: GraphNode
  depth: number
}

interface Props {
  graphId: string
  nodes: Map<string, GraphNode>
  // Имя исходного файла — для подписи под цитатой.
  source?: string
  details: NodeDetails | null
  chain: Explanation | null
  loading: boolean
  error: string | null
  tab: Tab
  onTab: (t: Tab) => void
  onSelect: (slug: string | null) => void
  onFocus: (slug: string) => void
  // Что следует из выбранного понятия (потомки в графе).
  consequences: Consequence[]
  // На телефоне карточка — нижняя шторка: свёрнута или развёрнута.
  sheet?: { expanded: boolean; onToggle: () => void }
}

function Shell({ sheet, children }: { sheet?: Props['sheet']; children: React.ReactNode }) {
  if (!sheet) return <aside className="concept-panel">{children}</aside>
  return (
    <aside className={sheet.expanded ? 'concept-panel sheet expanded' : 'concept-panel sheet'}>
      <button className="sheet-handle" onClick={sheet.onToggle} aria-label={sheet.expanded ? 'Свернуть карточку' : 'Развернуть карточку'} aria-expanded={sheet.expanded}>
        <span />
      </button>
      {children}
    </aside>
  )
}

export function ConceptPanel(props: Props) {
  const { details, loading, error, tab, onTab, onSelect, nodes, sheet } = props
  if (error) {
    return (
      <Shell sheet={sheet}>
        <p className="error">Ошибка: {error}</p>
      </Shell>
    )
  }
  if (!details) {
    return <Shell sheet={sheet}>{loading ? <p className="muted">Загрузка…</p> : <EmptyPanel nodes={nodes} onSelect={onSelect} />}</Shell>
  }
  const n = details.node
  return (
    <Shell sheet={sheet}>
      <div className="panel-head">
        <span className="tag accent">{nodeTypes[n.type].label}</span>
        <button className="icon-button" onClick={() => onSelect(null)} aria-label="Закрыть">
          <Icon src={icons.x} size={17} className="muted" />
        </button>
      </div>

      <div className="definition">
        <h2 className="concept-title">{n.title}</h2>
        <p className="concept-summary">{n.summary}</p>
      </div>

      {n.warning && (
        <div className="warning-box" role="note">
          <span className="warning-mark" aria-hidden>
            !
          </span>
          <p>{n.warning}</p>
        </div>
      )}

      {n.formula && <FormulaBlock node={n} incoming={details.incoming} nodes={nodes} />}

      <nav className="panel-tabs" role="tablist">
        <button role="tab" aria-selected={tab === 'concept'} className={tab === 'concept' ? 'active' : ''} onClick={() => onTab('concept')}>
          Понятие
        </button>
        <button role="tab" aria-selected={tab === 'chain'} className={tab === 'chain' ? 'active' : ''} onClick={() => onTab('chain')}>
          Разбор цепочки
        </button>
        {props.consequences.length > 0 && (
          <button role="tab" aria-selected={tab === 'consequences'} className={tab === 'consequences' ? 'active' : ''} onClick={() => onTab('consequences')}>
            Что следует <span className="tab-count">{props.consequences.length}</span>
          </button>
        )}
      </nav>

      {tab === 'concept' ? (
        <ConceptTab details={details} source={props.source} onSelect={onSelect} />
      ) : tab === 'consequences' ? (
        <ConsequencesTab title={n.title} items={props.consequences} onSelect={onSelect} />
      ) : props.chain ? (
        <ChainTab {...props} chain={props.chain} />
      ) : (
        <p className="muted">Загрузка…</p>
      )}
    </Shell>
  )
}

// С чего начать: у демо-графа — подобранные явления, у остальных — явления графа, затем законы.
function suggestions(nodes: Map<string, GraphNode>): string[] {
  const demo = featured.filter((s) => nodes.has(s))
  if (demo.length > 0) return demo
  const all = [...nodes.values()]
  const byType = (t: GraphNode['type']) => all.filter((n) => n.type === t).map((n) => n.slug)
  return [...byType('phenomenon'), ...byType('law'), ...byType('formula')].slice(0, 5)
}

function EmptyPanel({ nodes, onSelect }: { nodes: Map<string, GraphNode>; onSelect: (s: string) => void }) {
  return (
    <div className="panel-empty">
      <h2 className="concept-title">Выберите понятие</h2>
      <p className="concept-summary">
        Нажмите на любое понятие на графе — увидите цепочку от фундаментальных законов до него и пошаговый разбор.
      </p>
      <div className="caption">С ЧЕГО НАЧАТЬ</div>
      <div className="tag-list">
        {suggestions(nodes).map((s) => (
          <button key={s} className="tag clickable" onClick={() => onSelect(s)}>
            {nodes.get(s)!.title}
          </button>
        ))}
      </div>
    </div>
  )
}

// Формула крупно и расшифровка входящих в неё величин — как в макете.
function FormulaBlock({ node, incoming, nodes }: { node: GraphNode; incoming: Link[]; nodes: Map<string, GraphNode> }) {
  const legend =
    node.type === 'quantity'
      ? []
      : incoming.filter((l) => l.nodeType === 'quantity').map((l) => nodes.get(l.slug)).filter((q): q is GraphNode => !!q?.formula)
  return (
    <div className="formula-block">
      <div className="formula-main">
        <Tex tex={node.formula!} block />
      </div>
      {node.type === 'quantity' && node.unit && (
        <div className="formula-legend">
          Единица измерения: <Tex tex={node.unit} />
        </div>
      )}
      {legend.length > 0 && (
        <ul className="formula-legend">
          {legend.map((q) => (
            <li key={q.slug}>
              <Tex tex={q.formula!} /> — {q.title.toLowerCase()}
              {q.unit && (
                <>
                  , <Tex tex={q.unit} />
                </>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

function ConceptTab({ details, source, onSelect }: { details: NodeDetails; source?: string; onSelect: (s: string) => void }) {
  const n = details.node
  return (
    <>
      {n.origin === 'explicit' && n.sourceQuote && (
        <section className="panel-section">
          <div className="source-head">
            <span className="caption">В ВАШЕМ КОНСПЕКТЕ</span>
            {n.sourcePage ? <span className="source-page">стр. {n.sourcePage}</span> : null}
          </div>
          <blockquote className="source-quote">«{n.sourceQuote}»</blockquote>
          {source && (
            <div className="source-file">
              <Icon src={icons.fileTextSmall} size={14} />
              {source}
            </div>
          )}
        </section>
      )}
      {n.origin === 'inferred' && (
        <section className="inferred-box">
          <div className="source-head">
            <span className="caption">ДОПОЛНЕНО ИИ</span>
            <Icon src={icons.sparkles} size={16} className="accent" />
          </div>
          <p>{n.note || 'В конспекте этого нет: понятие добавлено, чтобы граф был связным.'}</p>
        </section>
      )}
      {n.body && (
        <section className="panel-section">
          <div className="caption">ПОДРОБНЕЕ</div>
          <Md>{n.body}</Md>
        </section>
      )}
      {n.example && (
        <section className="example">
          <div className="caption">ПРИМЕР</div>
          <Md>{n.example}</Md>
        </section>
      )}
      <div className="divider" />
      <RelatedList title="ОПИРАЕТСЯ НА" links={details.incoming} onSelect={onSelect} />
      <RelatedList title="ВЕДЁТ К" links={details.outgoing} onSelect={onSelect} />
    </>
  )
}

function RelatedList({ title, links, onSelect }: { title: string; links: Link[]; onSelect: (s: string) => void }) {
  if (links.length === 0) return null
  return (
    <section className="panel-section">
      <div className="caption">{title}</div>
      <div className="tag-list">
        {links.map((l) => (
          <button key={`${l.slug}:${l.edgeType}`} className="tag clickable" onClick={() => onSelect(l.slug)} title={l.description}>
            <span className="type-dot" style={{ background: nodeTypes[l.nodeType].color }} />
            {l.title}
          </button>
        ))}
      </div>
    </section>
  )
}

function ChainTab({ graphId, chain, nodes, onSelect, onFocus }: Props & { chain: Explanation }) {
  const byLevel = new Map<number, Step[]>()
  for (const s of chain.steps) {
    byLevel.set(s.level, [...(byLevel.get(s.level) ?? []), s])
  }
  const last = chain.levels - 1
  return (
    <>
      <p className="overview">{chain.overview}</p>
      <Story key={`${graphId}/${chain.target.slug}`} graphId={graphId} slug={chain.target.slug} />
      {chain.causalPaths && chain.causalPaths.length > 0 && (
        <section className="panel-section">
          <div className="caption">{chain.causalPaths.length > 1 ? 'ПРИЧИННЫЕ ЦЕПОЧКИ' : 'ПРИЧИННАЯ ЦЕПОЧКА'}</div>
          {chain.causalPaths.map((path) => (
            <ol key={path.join('>')} className="causal">
              {path.map((s) => (
                <li key={s}>
                  <button className="tag clickable" onClick={() => onSelect(s)}>
                    {nodes.get(s)?.title ?? s}
                  </button>
                </li>
              ))}
            </ol>
          ))}
        </section>
      )}
      {chain.target.example && (
        <section className="example">
          <div className="caption">ЧИСЛОВОЙ ПРИМЕР</div>
          <Md>{chain.target.example}</Md>
        </section>
      )}
      <ol className="levels">
        {[...byLevel.entries()].map(([level, steps]) => (
          <li key={level} className={level === last ? 'level final' : 'level'}>
            <div className="caption level-title">
              {level === 0 ? 'ИСХОДНЫЕ ПОНЯТИЯ' : level === last ? 'ИТОГ' : `ШАГ ${level}`}
            </div>
            {steps.map((s) => (
              <StepCard key={s.node.slug} step={s} final={level === last} onSelect={onSelect} onFocus={onFocus} />
            ))}
          </li>
        ))}
      </ol>
    </>
  )
}

type StoryState =
  | { kind: 'checking' | 'off' | 'idle' | 'writing' }
  | { kind: 'ready'; text: string }
  | { kind: 'error'; message: string }

// Связный рассказ модели по цепочке. Уже написанный показывается сразу, новый — по кнопке.
function Story({ graphId, slug }: { graphId: string; slug: string }) {
  const [state, setState] = useState<StoryState>({ kind: 'checking' })

  useEffect(() => {
    const ctrl = new AbortController()
    api
      .story(graphId, slug, ctrl.signal)
      .then((r) => setState({ kind: 'ready', text: r.text }))
      .catch((e: unknown) => {
        if (ctrl.signal.aborted) return
        // 503 — модель не настроена на сервере: блока нет совсем.
        setState({ kind: e instanceof ApiError && e.status === 503 ? 'off' : 'idle' })
      })
    return () => ctrl.abort()
  }, [graphId, slug])

  const write = () => {
    setState({ kind: 'writing' })
    api
      .tellStory(graphId, slug)
      .then((r) => setState({ kind: 'ready', text: r.text }))
      .catch((e: unknown) => setState({ kind: 'error', message: e instanceof Error ? e.message : 'Не получилось написать рассказ.' }))
  }

  if (state.kind === 'checking' || state.kind === 'off') return null
  if (state.kind === 'ready') {
    return (
      <section className="story" aria-label="Рассказ ИИ">
        <div className="story-head">
          <Icon src={icons.sparkles} size={14} className="accent" />
          <span>Рассказ ИИ</span>
        </div>
        <Md>{state.text}</Md>
      </section>
    )
  }
  return (
    <section className="story story-cta">
      {state.kind === 'writing' ? (
        <>
          <div className="story-head">
            <Icon src={icons.sparkles} size={14} className="accent" />
            <span>Пишу рассказ по цепочке…</span>
          </div>
          <div className="loading-line" aria-label="Загрузка" />
          <p className="muted small">Обычно это занимает несколько секунд.</p>
        </>
      ) : (
        <>
          <button className="button primary story-button" onClick={write}>
            <Icon src={icons.sparkles} size={15} />
            Рассказать связно
          </button>
          <p className="muted small">
            {state.kind === 'error' ? state.message : 'ИИ объяснит цепочку простыми словами — от законов к явлению.'}
          </p>
        </>
      )}
    </section>
  )
}

// Следствия понятия, сгруппированные по удалённости.
function ConsequencesTab({ title, items, onSelect }: { title: string; items: Consequence[]; onSelect: (s: string) => void }) {
  const groups: { label: string; items: Consequence[] }[] = [
    { label: 'НАПРЯМУЮ', items: items.filter((c) => c.depth === 1) },
    { label: 'ЧЕРЕЗ ОДНО ПОНЯТИЕ', items: items.filter((c) => c.depth === 2) },
    { label: 'ДАЛЬШЕ ПО ЦЕПОЧКЕ', items: items.filter((c) => c.depth >= 3) },
  ].filter((g) => g.items.length > 0)
  return (
    <>
      <p className="overview">
        Из «{title}» следует {items.length} {plural(items.length, 'понятие', 'понятия', 'понятий')} — на графе они подсвечены.
      </p>
      {groups.map((g) => (
        <section key={g.label} className="panel-section">
          <div className="caption">{g.label}</div>
          <ul className="consequences">
            {g.items.map(({ node }) => (
              <li key={node.slug}>
                <button className="consequence" onClick={() => onSelect(node.slug)}>
                  <span className="type-dot" style={{ background: nodeTypes[node.type].color }} />
                  <span className="consequence-title">{node.title}</span>
                  <span className="consequence-kind">{nodeTypes[node.type].label}</span>
                </button>
              </li>
            ))}
          </ul>
        </section>
      ))}
    </>
  )
}

function StepCard({ step, final, onSelect, onFocus }: { step: Step; final: boolean; onSelect: (s: string) => void; onFocus: (s: string) => void }) {
  const n = step.node
  const select = (slug: string) => (e: React.MouseEvent) => {
    e.stopPropagation()
    onSelect(slug)
  }
  return (
    <article className={final ? 'step final' : 'step'} onClick={() => onFocus(n.slug)} title="Показать на графе">
      <div className="step-head">
        <span className="step-index">{String(step.index).padStart(2, '0')}</span>
        <button className="step-title" onClick={select(n.slug)}>
          {n.title}
        </button>
        <span className="step-kind">
          <span className="type-dot" style={{ background: nodeTypes[n.type].color }} />
          {nodeTypes[n.type].label}
        </span>
      </div>
      {n.formula && n.type !== 'quantity' && (
        <div className="step-formula">
          <Tex tex={n.formula} block />
        </div>
      )}
      {n.type === 'quantity' && n.formula && (
        <div className="step-symbol">
          <Tex tex={n.formula} />
          {n.unit && (
            <>
              , <Tex tex={n.unit} />
            </>
          )}
        </div>
      )}
      <p className="step-summary">{n.summary}</p>
      {step.reasons.length > 0 && (
        <ul className="reasons">
          {step.reasons.map((r) => {
            // «Использует величину «Сила тока»» — название основания делаем ссылкой на него.
            const quoted = `«${r.fromTitle}»`
            const at = r.text.lastIndexOf(quoted)
            return (
              <li key={`${r.from}:${r.type}`}>
                {at >= 0 ? (
                  <>
                    {r.text.slice(0, at)}
                    <button className="reason-link" onClick={select(r.from)}>
                      {r.fromTitle}
                    </button>
                    {r.text.slice(at + quoted.length)}
                  </>
                ) : (
                  r.text
                )}
                {r.description && <span className="reason-note">{r.description}</span>}
              </li>
            )
          })}
        </ul>
      )}
    </article>
  )
}
