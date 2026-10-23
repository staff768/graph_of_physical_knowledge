import type { EdgeType, NodeType } from './api'

export const nodeTypes: Record<NodeType, { label: string; plural: string; color: string; shape: string }> = {
  law: { label: 'Закон', plural: 'Законы', color: '#2858e8', shape: 'round-rectangle' },
  quantity: { label: 'Величина', plural: 'Величины', color: '#4d7668', shape: 'round-rectangle' },
  formula: { label: 'Формула', plural: 'Формулы', color: '#b07d2b', shape: 'round-rectangle' },
  phenomenon: { label: 'Явление', plural: 'Явления', color: '#c0573e', shape: 'round-rectangle' },
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
