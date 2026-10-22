import type { EdgeType, NodeType } from './api'

export const nodeTypes: Record<NodeType, { label: string; plural: string; color: string; shape: string }> = {
  law: { label: 'Закон', plural: 'Законы', color: '#2563eb', shape: 'round-rectangle' },
  quantity: { label: 'Величина', plural: 'Величины', color: '#059669', shape: 'ellipse' },
  formula: { label: 'Формула', plural: 'Формулы', color: '#d97706', shape: 'rectangle' },
  phenomenon: { label: 'Явление', plural: 'Явления', color: '#dc2626', shape: 'hexagon' },
}

export const nodeTypeOrder: NodeType[] = ['law', 'quantity', 'formula', 'phenomenon']

// Подписи рёбер с точки зрения основания («что делает основание со следствием»).
export const edgeTypes: Record<EdgeType, { label: string; dashed: boolean }> = {
  formulates: { label: 'записывается как', dashed: false },
  used_in: { label: 'входит в', dashed: true },
  derives: { label: 'выводится в', dashed: false },
  explains: { label: 'объясняет', dashed: false },
  causes: { label: 'вызывает', dashed: false },
  defines: { label: 'вводит величину', dashed: true },
}

// Явления, с которых удобно начать знакомство.
export const featured = ['short-circuit', 'insulation-fire', 'lightning', 'induction-heating', 'incandescent-lamp']
