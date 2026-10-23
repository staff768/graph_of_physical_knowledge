import cytoscape, { type Core, type ElementDefinition } from 'cytoscape'
import dagre from 'cytoscape-dagre'
import katex from 'katex'
import { useEffect, useRef, useState } from 'react'
import type { Graph, GraphNode, NodeType } from '../api'
import { Icon } from './Icon'
import { icons } from '../icons'
import { edgeTypes, nodeTypes } from '../meta'
import { onThemeChange } from '../theme'

cytoscape.use(dagre)

interface Props {
  graph: Graph
  selected: string | null
  // Узлы цепочки выбранного узла; null — ничего не подсвечивать.
  chain: Set<string> | null
  chainOnly: boolean
  hiddenTypes: Set<NodeType>
  focus: { slug: string; seq: number } | null
  onSelect: (slug: string | null) => void
}

// Размер карточки понятия. Cytoscape отвечает за раскладку, рёбра и события,
// а сами карточки — HTML поверх холста: так формулы рендерятся KaTeX.
const CARD_W = 216
const CARD_H = 64

function cssVar(name: string, fallback: string) {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback
}

function buildStyle(): cytoscape.StylesheetJson {
  const edge = cssVar('--edge', '#c5c8c2')
  const accent = cssVar('--accent', '#2858e8')
  const bg = cssVar('--bg', '#f7f6f2')
  return [
    {
      selector: 'node',
      style: { width: CARD_W, height: CARD_H, shape: 'round-rectangle', 'background-opacity': 0, 'border-width': 0 },
    },
    {
      selector: 'edge',
      style: {
        width: 1.2,
        'line-color': edge,
        'target-arrow-color': edge,
        'target-arrow-shape': 'triangle',
        'arrow-scale': 0.8,
        'curve-style': 'bezier',
      },
    },
    // Пунктир — связь дополнена моделью, в конспекте её нет.
    { selector: 'edge[?inferred]', style: { 'line-style': 'dashed', 'line-dash-pattern': [5, 4] } },
    { selector: '.faded', style: { opacity: 0.15 } },
    {
      selector: 'edge.in-chain',
      style: {
        width: 1.6,
        'line-color': accent,
        'target-arrow-color': accent,
        label: 'data(label)',
        'font-family': 'Inter, system-ui, sans-serif',
        'font-size': 10,
        color: accent,
        'text-background-color': bg,
        'text-background-opacity': 1,
        'text-background-padding': '2px',
      },
    },
    { selector: '.hidden', style: { display: 'none' } },
  ]
}

function toElements(graph: Graph, only: Set<string> | null): ElementDefinition[] {
  const nodes = graph.nodes
    .filter((n) => !only || only.has(n.slug))
    .map((n) => ({ data: { id: n.slug, type: n.type } }))
  const edges = graph.edges
    .filter((e) => !only || (only.has(e.from) && only.has(e.to)))
    .map((e) => ({
      data: {
        id: `${e.from}->${e.to}:${e.type}`,
        source: e.from,
        target: e.to,
        label: edgeTypes[e.type].label + (e.origin === 'inferred' ? ' · дополнено' : ''),
        inferred: e.origin === 'inferred',
      },
    }))
  return [...nodes, ...edges]
}

// В карточке формула пишется в строку, как в макете: \frac{a}{b} → a/b.
function linearize(tex: string): string {
  const wrap = (x: string) => (/[+-]/.test(x.trim()) ? `(${x})` : x)
  // Аргумент может содержать один уровень вложенных скобок, например \mathcal{E}.
  const arg = '((?:[^{}]|\\{[^{}]*\\})*)'
  const re = new RegExp(`\\\\d?frac\\{${arg}\\}\\{${arg}\\}`, 'g')
  // Фигурные скобки не дают аргументу слипнуться с командой: \rho{l}/{S}, а не \rhol/S.
  return tex.replace(re, (_, a: string, b: string) => `{${wrap(a)}}/{${wrap(b)}}`)
}

// Вторая строка карточки: формула, обозначение величины или тип понятия.
function subtitleHTML(n: GraphNode): string {
  const tex =
    n.type === 'quantity' && n.formula ? `${n.formula}${n.unit ? `,\\ ${n.unit}` : ''}` : n.type !== 'phenomenon' ? n.formula : ''
  if (!tex) return `<span class="gcard-kind">${nodeTypes[n.type].label.toLowerCase()}</span>`
  return katex.renderToString(linearize(tex), { throwOnError: false })
}

function createCard(n: GraphNode): HTMLDivElement {
  const el = document.createElement('div')
  el.className = n.origin === 'inferred' ? 'gcard inferred' : 'gcard'
  el.style.setProperty('--type-color', nodeTypes[n.type].color)
  const warning = n.warning ? '<span class="gcard-warning" title="Возможна ошибка в конспекте">!</span>' : ''
  el.innerHTML = `${warning}<span class="gcard-dot"></span><div class="gcard-title"></div><div class="gcard-sub">${subtitleHTML(n)}</div>`
  el.querySelector('.gcard-title')!.textContent = n.title
  return el
}

const layoutOptions = {
  name: 'dagre',
  rankDir: 'LR',
  nodeSep: 22,
  rankSep: 110,
  edgeSep: 10,
  animate: false,
} as cytoscape.LayoutOptions

export function GraphView({ graph, selected, chain, chainOnly, hiddenTypes, focus, onSelect }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)
  const overlayRef = useRef<HTMLDivElement>(null)
  const cyRef = useRef<Core | null>(null)
  const cardsRef = useRef(new Map<string, HTMLDivElement>())
  const onSelectRef = useRef(onSelect)
  const [zoom, setZoom] = useState(1)
  useEffect(() => {
    onSelectRef.current = onSelect
  }, [onSelect])

  useEffect(() => {
    const container = containerRef.current!
    const cy = cytoscape({ container, style: buildStyle(), minZoom: 0.15, maxZoom: 2.5, wheelSensitivity: 0.3 })
    cy.on('tap', 'node', (evt) => onSelectRef.current(evt.target.id()))
    cy.on('tap', (evt) => {
      if (evt.target === cy) onSelectRef.current(null)
    })
    cy.on('mouseover', 'node', () => (container.style.cursor = 'pointer'))
    cy.on('mouseout', 'node', () => (container.style.cursor = ''))
    cy.on('zoom', () => setZoom(cy.zoom()))

    // Синхронизация HTML-карточек с положением узлов — не чаще раза за кадр.
    let frame = 0
    const sync = () => {
      frame = 0
      const z = cy.zoom()
      cy.nodes().forEach((node) => {
        const card = cardsRef.current.get(node.id())
        if (!card) return
        const p = node.renderedPosition()
        card.style.transform = `translate(${p.x - (CARD_W / 2) * z}px, ${p.y - (CARD_H / 2) * z}px) scale(${z})`
      })
    }
    cy.on('render', () => {
      if (!frame) frame = requestAnimationFrame(sync)
    })
    cyRef.current = cy
    // Цвета рёбер берутся из CSS-переменных — при смене темы стили пересчитываются.
    const offTheme = onThemeChange(() => cy.style(buildStyle()))
    return () => {
      offTheme()
      cancelAnimationFrame(frame)
      cy.destroy()
      cyRef.current = null
    }
  }, [])

  // Пересобираем элементы и карточки при смене графа или режима «только цепочка».
  const only = chainOnly ? chain : null
  useEffect(() => {
    const cy = cyRef.current
    const overlay = overlayRef.current
    if (!cy || !overlay) return
    cy.batch(() => {
      cy.elements().remove()
      cy.add(toElements(graph, only))
    })
    overlay.replaceChildren()
    cardsRef.current.clear()
    for (const n of graph.nodes) {
      if (only && !only.has(n.slug)) continue
      const card = createCard(n)
      overlay.appendChild(card)
      cardsRef.current.set(n.slug, card)
    }
    cy.stop(true, false)
    cy.layout(layoutOptions).run()
    cy.fit(undefined, 40)
  }, [graph, only])

  // Подсветка цепочки, выделение и фильтр по типам.
  useEffect(() => {
    const cy = cyRef.current
    if (!cy) return
    cy.batch(() => {
      cy.elements().removeClass('faded in-chain hidden')
      cy.nodes().forEach((n) => {
        const hidden = hiddenTypes.has(n.data('type'))
        const faded = !!chain && !chain.has(n.id())
        if (hidden) n.addClass('hidden')
        if (faded) n.addClass('faded')
        else if (chain) n.addClass('in-chain')
        const card = cardsRef.current.get(n.id())
        card?.classList.toggle('hidden', hidden)
        card?.classList.toggle('faded', faded)
        card?.classList.toggle('selected', n.id() === selected)
      })
      if (chain) {
        cy.edges().forEach((e) => {
          e.addClass(chain.has(e.data('source')) && chain.has(e.data('target')) ? 'in-chain' : 'faded')
        })
      }
    })
    // Без анимации: анимации Cytoscape встают в очередь, и вид «доезжал» бы до цели с задержкой.
    const target = chain ? cy.nodes('.in-chain') : cy.nodes(':visible')
    cy.stop(true, false)
    if (target.nonempty()) {
      cy.fit(target, 48)
      // Маленькую цепочку не раздуваем: крупнее 100% карточки выглядят неестественно.
      if (cy.zoom() > 1) cy.zoom({ level: 1, renderedPosition: { x: cy.width() / 2, y: cy.height() / 2 } })
      if (cy.zoom() === 1) cy.center(target)
    }
  }, [chain, selected, hiddenTypes, graph, only])

  // Центрирование на узле по запросу из панели разбора.
  useEffect(() => {
    const cy = cyRef.current
    if (!cy || !focus) return
    const node = cy.getElementById(focus.slug)
    const card = cardsRef.current.get(focus.slug)
    if (node.empty()) return
    cy.stop(true, false)
    cy.animate({ center: { eles: node }, zoom: Math.max(cy.zoom(), 1) }, { duration: 300 })
    card?.classList.add('flash')
    const t = setTimeout(() => card?.classList.remove('flash'), 1200)
    return () => clearTimeout(t)
  }, [focus])

  const zoomBy = (factor: number) => {
    const cy = cyRef.current
    if (!cy) return
    cy.stop(true, false)
    cy.animate(
      { zoom: { level: cy.zoom() * factor, renderedPosition: { x: cy.width() / 2, y: cy.height() / 2 } } },
      { duration: 200 },
    )
  }

  return (
    <div className="graph">
      <div className="graph-canvas" ref={containerRef} />
      <div className="graph-overlay" ref={overlayRef} aria-hidden />
      <div className="zoom-control">
        <button onClick={() => zoomBy(1 / 1.25)} aria-label="Уменьшить">
          <Icon src={icons.minus} size={15} />
        </button>
        <span className="zoom-value">{Math.round(zoom * 100)}%</span>
        <button onClick={() => zoomBy(1.25)} aria-label="Увеличить">
          <Icon src={icons.plus} size={15} />
        </button>
        <span className="zoom-divider" />
        <button onClick={() => cyRef.current?.animate({ fit: { eles: cyRef.current.nodes(':visible'), padding: 40 } }, { duration: 300 })} aria-label="Показать весь граф">
          <Icon src={icons.maximize} size={15} />
        </button>
      </div>
    </div>
  )
}
