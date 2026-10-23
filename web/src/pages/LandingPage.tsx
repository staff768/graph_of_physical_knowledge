import { DEMO_GRAPH_ID, graphPath, useGraphs } from '../graphs'
import { Icon } from '../components/Icon'
import { icons } from '../icons'
import { nodeTypeOrder, nodeTypes } from '../meta'
import { linkClick } from '../router'
import type { NodeType } from '../api'

const demoLink = graphPath(DEMO_GRAPH_ID, 'lightning')

const steps = [
  { icon: icons.fileUp, title: 'Конспект', text: 'PDF, DOCX или фото рукописных страниц' },
  { icon: icons.sparkles, title: 'ИИ читает', text: 'Находит законы, величины, формулы и явления — с цитатами' },
  { icon: icons.network, title: 'Граф', text: 'Связывает их: от законов к тому, что мы видим в жизни' },
  { icon: icons.fileText, title: 'Разбор', text: 'Цепочка причин и рассказ простыми словами' },
]

const typeExamples: Record<NodeType, string> = {
  law: 'Закон Ома, закон Кулона',
  quantity: 'Сила тока, напряжение',
  formula: 'I = U/R, Q = I²Rt',
  phenomenon: 'Молния, короткое замыкание',
}

const reasons = [
  { title: 'Видно, откуда что следует', text: 'Нажмите на явление — граф покажет законы и формулы, из которых оно вытекает, и что следует из него дальше.' },
  { title: 'Всё по вашему конспекту', text: 'У каждого понятия — цитата из конспекта. То, что ИИ дополнил сам, отмечено пунктиром.' },
  { title: 'Ошибки не пройдут мимо', text: 'Если в конспекте неверная формула, Физграф предупредит и покажет правильную.' },
]

export function LandingPage() {
  const { graphs } = useGraphs()
  const ownGraphs = graphs?.filter((g) => !g.demo).length ?? 0
  return (
    <div className="landing">
      <header className="landing-bar">
        <a className="landing-brand" href="/" onClick={linkClick('/')}>
          <img src={icons.brand} width={24} height={26} alt="" aria-hidden />
          Физграф
        </a>
        <nav className="landing-links" aria-label="Разделы">
          <a href={demoLink} onClick={linkClick(demoLink)}>
            Пример
          </a>
          <a href="/graphs" onClick={linkClick('/graphs')}>
            Мои графы{ownGraphs > 0 && <span className="landing-count">{ownGraphs}</span>}
          </a>
          <a className="button primary small-button" href="/new" onClick={linkClick('/new')}>
            Начать
          </a>
        </nav>
      </header>

      <section className="landing-hero">
        <div className="landing-hero-text">
          <p className="landing-eyebrow">Граф физических знаний</p>
          <h1 className="landing-title">Физика — не список формул, а&nbsp;сеть причин.</h1>
          <p className="landing-lead">
            Загрузите конспект — Физграф соберёт из него граф законов, величин и явлений и покажет, как одно вытекает из другого.
          </p>
          <div className="landing-actions">
            <a className="button primary landing-cta" href="/new" onClick={linkClick('/new')}>
              Построить граф по конспекту
              <Icon src={icons.arrowRight} size={16} />
            </a>
            <a className="button landing-cta" href={demoLink} onClick={linkClick(demoLink)}>
              Посмотреть пример
            </a>
          </div>
        </div>
        <HeroGraph />
      </section>

      <section className="landing-section">
        <h2 className="landing-h2">Как это работает</h2>
        <ol className="landing-steps">
          {steps.map((s, i) => (
            <li key={s.title} className="landing-step">
              <span className="landing-step-icon">
                <Icon src={s.icon} size={20} className="accent" />
              </span>
              <span className="landing-step-n">0{i + 1}</span>
              <h3>{s.title}</h3>
              <p>{s.text}</p>
            </li>
          ))}
        </ol>
      </section>

      <section className="landing-section">
        <h2 className="landing-h2">Из чего состоит граф</h2>
        <ul className="landing-types">
          {nodeTypeOrder.map((t) => (
            <li key={t}>
              <span className="type-dot" style={{ background: nodeTypes[t].color }} />
              <div>
                <strong>{nodeTypes[t].plural}</strong>
                <span>{typeExamples[t]}</span>
              </div>
            </li>
          ))}
        </ul>
      </section>

      <section className="landing-section">
        <ul className="landing-reasons">
          {reasons.map((r) => (
            <li key={r.title}>
              <h3>{r.title}</h3>
              <p>{r.text}</p>
            </li>
          ))}
        </ul>
      </section>

      <section className="landing-final">
        <h2 className="landing-h2">Начните со своего конспекта</h2>
        <p>Около минуты на страницу. Регистрация не нужна.</p>
        <div className="landing-actions">
          <a className="button primary landing-cta" href="/new" onClick={linkClick('/new')}>
            Загрузить конспект
          </a>
          <a className="button ghost landing-cta" href={demoLink} onClick={linkClick(demoLink)}>
            или открыть пример про электричество
          </a>
        </div>
      </section>

      <footer className="landing-footer">Физграф · учебный проект по физике · 2026</footer>
    </div>
  )
}

// Иллюстрация: маленький кусок графа «почему бывает молния». Рисуется SVG, связи «прорастают».
type HeroNode = { x: number; y: number; title: string; sub: string; type: NodeType; target?: boolean }

const W = 150
const H = 50
const heroNodes: Record<string, HeroNode> = {
  coulomb: { x: 4, y: 20, title: 'Закон Кулона', sub: 'закон', type: 'law' },
  field: { x: 4, y: 120, title: 'Напряжённость', sub: 'E, В/м', type: 'quantity' },
  formula: { x: 4, y: 220, title: 'E = U / d', sub: 'формула', type: 'formula' },
  electrification: { x: 260, y: 50, title: 'Электризация тел', sub: 'явление', type: 'phenomenon' },
  breakdown: { x: 260, y: 190, title: 'Пробой воздуха', sub: 'явление', type: 'phenomenon' },
  lightning: { x: 516, y: 120, title: 'Молния', sub: 'явление', type: 'phenomenon', target: true },
}
const heroEdges: { from: string; to: string; label: string; inferred?: boolean }[] = [
  { from: 'coulomb', to: 'electrification', label: 'объясняет' },
  { from: 'field', to: 'breakdown', label: 'объясняет', inferred: true },
  { from: 'formula', to: 'breakdown', label: 'объясняет' },
  { from: 'electrification', to: 'lightning', label: 'вызывает' },
  { from: 'breakdown', to: 'lightning', label: 'вызывает' },
]

function HeroGraph() {
  return (
    <svg className="hero-graph" viewBox="0 0 670 290" role="img" aria-label="Пример графа: законы и формулы объясняют электризацию и пробой воздуха, а они вызывают молнию">
      <defs>
        <marker id="hero-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
          <path d="M0 0 L10 5 L0 10 z" className="hero-arrow" />
        </marker>
      </defs>
      {heroEdges.map((e, i) => {
        const a = heroNodes[e.from]
        const b = heroNodes[e.to]
        const x1 = a.x + W
        const y1 = a.y + H / 2
        const x2 = b.x - 2
        const y2 = b.y + H / 2
        const mx = (x1 + x2) / 2
        return (
          <g key={i} className={e.inferred ? 'hero-edge inferred' : 'hero-edge'} style={{ animationDelay: `${0.25 + i * 0.18}s` }}>
            <path d={`M${x1} ${y1} C${mx} ${y1} ${mx} ${y2} ${x2} ${y2}`} pathLength={1} markerEnd="url(#hero-arrow)" />
            <text x={mx} y={(y1 + y2) / 2 - 6} textAnchor="middle">
              {e.label}
            </text>
          </g>
        )
      })}
      {Object.entries(heroNodes).map(([id, n], i) => (
        <g key={id} className={n.target ? 'hero-node target' : 'hero-node'} style={{ animationDelay: `${i * 0.08}s` }}>
          <rect x={n.x} y={n.y} width={W} height={H} rx={8} />
          <circle cx={n.x + W - 12} cy={n.y + 12} r={3.5} style={{ fill: n.target ? 'var(--on-accent)' : nodeTypes[n.type].color }} />
          <text x={n.x + 14} y={n.y + 22} className="hero-node-title">
            {n.title}
          </text>
          <text x={n.x + 14} y={n.y + 40} className="hero-node-sub">
            {n.sub}
          </text>
        </g>
      ))}
    </svg>
  )
}
