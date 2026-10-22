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

async function get<T>(path: string, signal?: AbortSignal): Promise<T> {
  const res = await fetch(path, { signal })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error ?? `HTTP ${res.status}`)
  }
  return res.json() as Promise<T>
}

export const api = {
  graph: (signal?: AbortSignal) => get<Graph>('/api/graph', signal),
  node: (slug: string, signal?: AbortSignal) =>
    get<NodeDetails>(`/api/nodes/${encodeURIComponent(slug)}`, signal),
  chain: (slug: string, signal?: AbortSignal) =>
    get<Explanation>(`/api/nodes/${encodeURIComponent(slug)}/chain`, signal),
  search: (q: string, signal?: AbortSignal) =>
    get<GraphNode[]>(`/api/search?q=${encodeURIComponent(q)}`, signal),
}
