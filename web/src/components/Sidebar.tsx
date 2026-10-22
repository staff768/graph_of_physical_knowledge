import type { Explanation, GraphNode, Link, NodeDetails, Step } from '../api'
import { edgeTypes, featured, nodeTypes } from '../meta'
import { Md, Tex } from './Tex'

export type Tab = 'node' | 'chain'

interface Props {
  nodes: Map<string, GraphNode>
  details: NodeDetails | null
  chain: Explanation | null
  loading: boolean
  error: string | null
  tab: Tab
  onTab: (t: Tab) => void
  onSelect: (slug: string) => void
  onFocus: (slug: string) => void
}

export function Sidebar(props: Props) {
  const { details, chain, loading, error, tab, onTab } = props
  if (error) return <aside className="sidebar"><p className="error">Ошибка: {error}</p></aside>
  if (!details) {
    return <aside className="sidebar">{loading ? <p className="muted">Загрузка…</p> : <Welcome {...props} />}</aside>
  }
  const n = details.node
  return (
    <aside className="sidebar">
      <header className="node-head">
        <TypeBadge type={n.type} />
        <h2>{n.title}</h2>
        {n.formula && (
          <div className="formula">
            <Tex tex={n.formula} block />
            {n.unit && <span className="unit">[<Tex tex={n.unit} />]</span>}
          </div>
        )}
        <p className="summary">{n.summary}</p>
      </header>
      <nav className="tabs" role="tablist">
        <button role="tab" aria-selected={tab === 'chain'} className={tab === 'chain' ? 'active' : ''} onClick={() => onTab('chain')}>
          Разбор цепочки
        </button>
        <button role="tab" aria-selected={tab === 'node'} className={tab === 'node' ? 'active' : ''} onClick={() => onTab('node')}>
          Подробнее
        </button>
      </nav>
      <div className="tab-body">
        {tab === 'node' ? <NodeTab {...props} details={details} /> : chain ? <ChainTab {...props} chain={chain} /> : <p className="muted">Загрузка…</p>}
      </div>
    </aside>
  )
}

function Welcome({ nodes, onSelect }: Props) {
  return (
    <div className="welcome">
      <h2>Граф физических знаний</h2>
      <p>
        Узлы графа — законы, величины, формулы и явления. Стрелки ведут от основания к следствию.
        Выберите любой узел, и вы увидите всю цепочку: от фундаментальных законов до формул,
        которые объясняют явление, а также пошаговый разбор.
      </p>
      <h3>С чего начать</h3>
      <div className="chips">
        {featured.filter((s) => nodes.has(s)).map((s) => (
          <button key={s} className="chip" onClick={() => onSelect(s)}>
            {nodes.get(s)!.title}
          </button>
        ))}
      </div>
      <h3>Обозначения</h3>
      <ul className="legend-list">
        {Object.entries(nodeTypes).map(([t, m]) => (
          <li key={t}><span className="dot" style={{ background: m.color }} /> {m.plural}</li>
        ))}
        <li><span className="line" /> основание → следствие</li>
        <li><span className="line dashed" /> величина входит в формулу</li>
        <li><span className="line causes" /> причинно-следственная связь явлений</li>
      </ul>
    </div>
  )
}

function NodeTab({ details, onSelect }: Props & { details: NodeDetails }) {
  const n = details.node
  return (
    <>
      {n.body && <Md>{n.body}</Md>}
      {n.example && (
        <section className="example">
          <h4>Пример</h4>
          <Md>{n.example}</Md>
        </section>
      )}
      <LinkList title="Опирается на" links={details.incoming} onSelect={onSelect} incoming />
      <LinkList title="Ведёт к" links={details.outgoing} onSelect={onSelect} incoming={false} />
    </>
  )
}

function LinkList({ title, links, onSelect, incoming }: { title: string; links: Link[]; onSelect: (s: string) => void; incoming: boolean }) {
  if (links.length === 0) return null
  return (
    <section className="links">
      <h4>{title}</h4>
      <ul>
        {links.map((l) => (
          <li key={`${l.slug}:${l.edgeType}`}>
            <button className="link" onClick={() => onSelect(l.slug)}>
              <span className="dot" style={{ background: nodeTypes[l.nodeType].color }} />
              {l.title}
            </button>
            <span className="muted"> — {incoming ? edgeTypes[l.edgeType].label + ' этот узел' : 'этот узел ' + edgeTypes[l.edgeType].label}</span>
            {l.description && <div className="desc">{l.description}</div>}
          </li>
        ))}
      </ul>
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
            <span key={s}>
              {i > 0 && <span className="arrow">→</span>}
              <button className="chip phen" onClick={() => onSelect(s)}>{nodes.get(s)?.title ?? s}</button>
            </span>
          ))}
        </div>
      )}
      {chain.target.example && (
        <section className="example">
          <h4>Числовой пример</h4>
          <Md>{chain.target.example}</Md>
        </section>
      )}
      <ol className="levels">
        {[...byLevel.entries()].map(([level, steps]) => (
          <li key={level} className="level">
            <div className="level-title">
              {level === 0 ? 'Исходные понятия' : level === last ? 'Итог' : `Уровень ${level}`}
            </div>
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
        <TypeBadge type={n.type} small />
      </div>
      {n.formula && n.type !== 'quantity' && <Tex tex={n.formula} block />}
      {n.type === 'quantity' && n.formula && (
        <div className="muted">Обозначение <Tex tex={n.formula} />{n.unit && <>, единица <Tex tex={n.unit} /></>}</div>
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

function TypeBadge({ type, small }: { type: GraphNode['type']; small?: boolean }) {
  return (
    <span className={small ? 'badge small' : 'badge'} style={{ background: nodeTypes[type].color }}>
      {nodeTypes[type].label}
    </span>
  )
}
