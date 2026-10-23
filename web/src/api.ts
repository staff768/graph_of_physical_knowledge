export type NodeType = 'law' | 'quantity' | 'formula' | 'phenomenon'

export type EdgeType = 'formulates' | 'used_in' | 'derives' | 'explains' | 'causes' | 'defines'

export interface GraphNode {
  slug: string
  type: NodeType
  title: string
  section: string
  summary: string
  formula?: string
  unit?: string
  body?: string
  example?: string
}

export interface GraphEdge {
  from: string
  to: string
  type: EdgeType
  description?: string
}

export interface Graph {
  nodes: GraphNode[]
  edges: GraphEdge[]
}

export interface Link {
  slug: string
  title: string
  nodeType: NodeType
  edgeType: EdgeType
  description?: string
}

export interface NodeDetails {
  node: GraphNode
  incoming: Link[]
  outgoing: Link[]
}

export interface Reason {
  from: string
  fromTitle: string
  type: EdgeType
  text: string
  description?: string
}

export interface Step {
  index: number
  level: number
  node: GraphNode
  reasons: Reason[]
}

export interface Explanation {
  target: GraphNode
  overview: string
  causalChain: string[] | null
  steps: Step[]
  levels: number
  graph: Graph
}

export interface GraphMeta {
  id: string
  title: string
  section: string
  source: string
  demo: boolean
  createdAt: string
  nodeCount: number
  edgeCount: number
  // Самые связанные понятия — для превью карточки (только в списке графов).
  preview?: string[]
}

export interface GraphData extends Graph {
  meta: GraphMeta
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function get<T>(path: string, signal?: AbortSignal): Promise<T> {
  const res = await fetch(path, { signal })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new ApiError(res.status, body.error ?? `HTTP ${res.status}`)
  }
  return res.json() as Promise<T>
}

const enc = encodeURIComponent
const graphBase = (id: string) => `/api/graphs/${enc(id)}`

export const api = {
  graphs: (q = '', signal?: AbortSignal) => get<GraphMeta[]>(`/api/graphs${q ? `?q=${enc(q)}` : ''}`, signal),
  graph: (id: string, signal?: AbortSignal) => get<GraphData>(graphBase(id), signal),
  node: (id: string, slug: string, signal?: AbortSignal) => get<NodeDetails>(`${graphBase(id)}/nodes/${enc(slug)}`, signal),
  chain: (id: string, slug: string, signal?: AbortSignal) =>
    get<Explanation>(`${graphBase(id)}/nodes/${enc(slug)}/chain`, signal),
  search: (id: string, q: string, signal?: AbortSignal) => get<GraphNode[]>(`${graphBase(id)}/search?q=${enc(q)}`, signal),
}
