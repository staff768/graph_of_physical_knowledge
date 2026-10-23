import type { Explanation, GraphNode, Link, NodeDetails, Step } from '../api'
import { Icon } from './Icon'
import { icons } from '../icons'
import { featured, nodeTypes } from '../meta'
import { Md, Tex } from './Tex'

export type Tab = 'concept' | 'chain'

interface Props {
  nodes: Map<string, GraphNode>
  details: NodeDetails | null
  chain: Explanation | null
  loading: boolean
  error: string | null
  tab: Tab
  onTab: (t: Tab) => void
  onSelect: (slug: string | null) => void
  onFocus: (slug: string) => void
}

export function ConceptPanel(props: Props) {
  const { details, loading, error, tab, onTab, onSelect, nodes } = props
  if (error) {
    return (
      <aside className="concept-panel">
        <p className="error">Ошибка: {error}</p>
      </aside>
    )
  }
  if (!details) {
    return <aside className="concept-panel">{loading ? <p className="muted">Загрузка…</p> : <EmptyPanel nodes={nodes} onSelect={onSelect} />}</aside>
  }
  const n = details.node
  return (
    <aside className="concept-panel">
      <div className="panel-head">
        <span className="tag accent">{nodeTypes[n.type].label}</span>
        <button className="icon-button" onClick={() => onSelect(null)} aria-label="Закрыть">
          <Icon src={icons.x} size={17} />
        </button>
      </div>

      <div className="definition">
        <h2 className="concept-title">{n.title}</h2>
        <p className="concept-summary">{n.summary}</p>
      </div>

      {n.formula && <FormulaBlock node={n} incoming={details.incoming} nodes={nodes} />}

      <nav className="panel-tabs" role="tablist">
        <button role="tab" aria-selected={tab === 'concept'} className={tab === 'concept' ? 'active' : ''} onClick={() => onTab('concept')}>
          Понятие
        </button>
        <button role="tab" aria-selected={tab === 'chain'} className={tab === 'chain' ? 'active' : ''} onClick={() => onTab('chain')}>
          Разбор цепочки
        </button>
      </nav>

      {tab === 'concept' ? (
        <ConceptTab details={details} onSelect={onSelect} />
      ) : props.chain ? (
        <ChainTab {...props} chain={props.chain} />
      ) : (
        <p className="muted">Загрузка…</p>
      )}
    </aside>
  )
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
        {featured.filter((s) => nodes.has(s)).map((s) => (
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

function ConceptTab({ details, onSelect }: { details: NodeDetails; onSelect: (s: string) => void }) {
  const n = details.node
  return (
    <>
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

function ChainTab({ chain, nodes, onSelect, onFocus }: Props & { chain: Explanation }) {
  const byLevel = new Map<number, Step[]>()
  for (const s of chain.steps) {
    byLevel.set(s.level, [...(byLevel.get(s.level) ?? []), s])
  }
  const last = chain.levels - 1
  return (
    <>
      <p className="overview">{chain.overview}</p>
      {chain.causalChain && (
        <div className="causal">
          {chain.causalChain.map((s, i) => (
            <span key={s} className="causal-item">
              {i > 0 && <span className="arrow">→</span>}
              <button className="tag clickable" onClick={() => onSelect(s)}>
                {nodes.get(s)?.title ?? s}
              </button>
            </span>
          ))}
        </div>
      )}
      {chain.target.example && (
        <section className="example">
          <div className="caption">ЧИСЛОВОЙ ПРИМЕР</div>
          <Md>{chain.target.example}</Md>
        </section>
      )}
      <ol className="levels">
        {[...byLevel.entries()].map(([level, steps]) => (
          <li key={level} className="level">
            <div className="caption level-title">{level === 0 ? 'ИСХОДНЫЕ ПОНЯТИЯ' : level === last ? 'ИТОГ' : `УРОВЕНЬ ${level}`}</div>
            {steps.map((s) => (
              <StepCard key={s.node.slug} step={s} onSelect={onSelect} onFocus={onFocus} />
            ))}
          </li>
        ))}
      </ol>
    </>
  )
}

function StepCard({ step, onSelect, onFocus }: { step: Step; onSelect: (s: string) => void; onFocus: (s: string) => void }) {
  const n = step.node
  return (
    <article className="step" style={{ borderLeftColor: nodeTypes[n.type].color }} onClick={() => onFocus(n.slug)} title="Показать на графе">
      <div className="step-head">
        <span className="step-index">{step.index}</span>
        <button
          className="link step-title"
          onClick={(e) => {
            e.stopPropagation()
            onSelect(n.slug)
          }}
        >
          {n.title}
        </button>
        <span className="step-kind" style={{ color: nodeTypes[n.type].color }}>
          {nodeTypes[n.type].label}
        </span>
      </div>
      {n.formula && n.type !== 'quantity' && <Tex tex={n.formula} block />}
      {n.type === 'quantity' && n.formula && (
        <div className="muted small">
          Обозначение <Tex tex={n.formula} />
          {n.unit && (
            <>
              , единица <Tex tex={n.unit} />
            </>
          )}
        </div>
      )}
      <p className="step-summary">{n.summary}</p>
      {step.reasons.length > 0 && (
        <ul className="reasons">
          {step.reasons.map((r) => (
            <li key={`${r.from}:${r.type}`}>
              {r.text}
              {r.description && <span className="muted"> — {r.description}</span>}
            </li>
          ))}
        </ul>
      )}
    </article>
  )
}
