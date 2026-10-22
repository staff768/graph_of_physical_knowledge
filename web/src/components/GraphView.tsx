import cytoscape, { type Core, type ElementDefinition } from 'cytoscape'
import dagre from 'cytoscape-dagre'
import { useEffect, useRef } from 'react'
import type { Graph, NodeType } from '../api'
import { edgeTypes, nodeTypes } from '../meta'

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

function cssVar(name: string) {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
}

function buildStyle(): cytoscape.StylesheetJson {
  const text = cssVar('--text') || '#1f2937'
  const muted = cssVar('--text-muted') || '#6b7280'
  const bg = cssVar('--bg') || '#ffffff'
  return [
    {
      selector: 'node',
      style: {
        label: 'data(label)',
        'background-color': 'data(color)',
        shape: 'data(shape)' as never,
        width: 150,
        height: 46,
        color: '#ffffff',
        'font-size': 11,
        'font-weight': 600,
        'text-wrap': 'wrap',
        'text-max-width': '132px',
        'text-valign': 'center',
        'text-halign': 'center',
        'border-width': 0,
      },
    },
    { selector: 'node[type = "quantity"]', style: { width: 120 } },
    { selector: 'node[type = "phenomenon"]', style: { width: 160, height: 54 } },
    {
      selector: 'edge',
      style: {
        width: 1.4,
        'line-color': muted,
        'target-arrow-color': muted,
        'target-arrow-shape': 'triangle',
        'arrow-scale': 0.9,
        'curve-style': 'bezier',
        opacity: 0.55,
      },
    },
    { selector: 'edge[?dashed]', style: { 'line-style': 'dashed' } },
    { selector: 'edge[type = "causes"]', style: { 'line-color': '#dc2626', 'target-arrow-color': '#dc2626' } },
    { selector: '.faded', style: { opacity: 0.1 } },
    {
      selector: 'edge.in-chain',
      style: {
        width: 2.4,
        opacity: 1,
        label: 'data(label)',
        'font-size': 9,
        color: text,
        'text-background-color': bg,
        'text-background-opacity': 0.85,
        'text-background-padding': '2px',
        'text-rotation': 'autorotate',
      },
    },
    { selector: 'node.selected', style: { 'border-width': 4, 'border-color': '#facc15' } },
    { selector: 'node.flash', style: { 'border-width': 4, 'border-color': text } },
    { selector: '.hidden', style: { display: 'none' } },
  ]
}

function toElements(graph: Graph, only: Set<string> | null): ElementDefinition[] {
  const nodes = graph.nodes
    .filter((n) => !only || only.has(n.slug))
    .map((n) => ({
      data: {
        id: n.slug,
        label: n.title,
        type: n.type,
        color: nodeTypes[n.type].color,
        shape: nodeTypes[n.type].shape,
      },
    }))
  const edges = graph.edges
    .filter((e) => !only || (only.has(e.from) && only.has(e.to)))
    .map((e) => ({
      data: {
        id: `${e.from}->${e.to}:${e.type}`,
        source: e.from,
        target: e.to,
        type: e.type,
        label: edgeTypes[e.type].label,
        dashed: edgeTypes[e.type].dashed,
      },
    }))
  return [...nodes, ...edges]
}

const layoutOptions = {
  name: 'dagre',
  rankDir: 'LR',
  nodeSep: 18,
  rankSep: 90,
  edgeSep: 8,
  animate: false,
} as cytoscape.LayoutOptions

export function GraphView({ graph, selected, chain, chainOnly, hiddenTypes, focus, onSelect }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)
  const cyRef = useRef<Core | null>(null)
  const onSelectRef = useRef(onSelect)
  useEffect(() => {
    onSelectRef.current = onSelect
  }, [onSelect])

  useEffect(() => {
    const cy = cytoscape({
      container: containerRef.current,
      style: buildStyle(),
      minZoom: 0.15,
      maxZoom: 2.5,
      wheelSensitivity: 0.3,
    })
    cy.on('tap', 'node', (evt) => onSelectRef.current(evt.target.id()))
    cy.on('tap', (evt) => {
      if (evt.target === cy) onSelectRef.current(null)
    })
    cyRef.current = cy

    const scheme = window.matchMedia('(prefers-color-scheme: dark)')
    const restyle = () => cy.style(buildStyle())
    scheme.addEventListener('change', restyle)
    return () => {
      scheme.removeEventListener('change', restyle)
      cy.destroy()
      cyRef.current = null
    }
  }, [])

  // Пересобираем элементы при смене графа или режима «только цепочка».
  const only = chainOnly ? chain : null
  useEffect(() => {
    const cy = cyRef.current
    if (!cy) return
    cy.batch(() => {
      cy.elements().remove()
      cy.add(toElements(graph, only))
    })
    cy.layout(layoutOptions).run()
    cy.fit(undefined, 30)
  }, [graph, only])

  // Подсветка цепочки, выделение и фильтр по типам.
  useEffect(() => {
    const cy = cyRef.current
    if (!cy) return
    cy.batch(() => {
      cy.elements().removeClass('faded in-chain selected hidden')
      cy.nodes().forEach((n) => {
        if (hiddenTypes.has(n.data('type'))) n.addClass('hidden')
      })
      if (chain) {
        cy.nodes().forEach((n) => {
          n.addClass(chain.has(n.id()) ? 'in-chain' : 'faded')
        })
        cy.edges().forEach((e) => {
          e.addClass(chain.has(e.data('source')) && chain.has(e.data('target')) ? 'in-chain' : 'faded')
        })
      }
      if (selected) cy.getElementById(selected).addClass('selected')
    })
    const target = chain ? cy.nodes('.in-chain') : cy.nodes(':visible')
    if (target.nonempty()) {
      cy.animate({ fit: { eles: target, padding: 40 } }, { duration: 350 })
    }
  }, [chain, selected, hiddenTypes, graph, only])

  // Центрирование на узле по запросу из панели разбора.
  useEffect(() => {
    const cy = cyRef.current
    if (!cy || !focus) return
    const node = cy.getElementById(focus.slug)
    if (node.empty()) return
    cy.animate({ center: { eles: node }, zoom: Math.max(cy.zoom(), 1.1) }, { duration: 300 })
    node.addClass('flash')
    const t = setTimeout(() => node.removeClass('flash'), 1200)
    return () => clearTimeout(t)
  }, [focus])

  return <div className="graph" ref={containerRef} />
}
