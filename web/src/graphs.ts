import { useCallback, useEffect, useState } from 'react'
import { api, ApiError, type GraphData, type GraphMeta } from './api'

// Демо-граф всегда есть в базе (его загружает seed); на него ведут старые ссылки.
export const DEMO_GRAPH_ID = 'demo'

export const graphPath = (id: string, node?: string | null) => `/g/${id}${node ? `?n=${node}` : ''}`

export function plural(n: number, one: string, few: string, many: string) {
  const m = n % 100
  if (m >= 11 && m <= 14) return many
  switch (n % 10) {
    case 1:
      return one
    case 2:
    case 3:
    case 4:
      return few
  }
  return many
}

export const conceptCount = (n: number) => `${n} ${plural(n, 'понятие', 'понятия', 'понятий')}`
export const linkCount = (n: number) => `${n} ${plural(n, 'связь', 'связи', 'связей')}`
export const graphCount = (n: number) => `${n} ${plural(n, 'граф', 'графа', 'графов')}`

// Полный список графов запрашивается один раз и переиспользуется навигацией и страницами.
let allGraphs: Promise<GraphMeta[]> | null = null

export function invalidateGraphs() {
  allGraphs = null
}

const GRAPHS_EVENT = 'app:graphs-changed'

// Список графов изменился (например, граф удалён): все useGraphs перезапрашивают его.
export function notifyGraphsChanged() {
  allGraphs = null
  window.dispatchEvent(new Event(GRAPHS_EVENT))
}

// useGraphs возвращает графы, начиная с новых; с query — только подходящие по названию или понятиям.
// Пока грузится новый запрос, остаются предыдущие результаты, чтобы список не мигал.
export function useGraphs(query = ''): { graphs: GraphMeta[] | null; error: string | null } {
  const [state, setState] = useState<{ graphs: GraphMeta[] | null; error: string | null }>({ graphs: null, error: null })
  const [version, setVersion] = useState(0)
  useEffect(() => {
    const bump = () => setVersion((v) => v + 1)
    window.addEventListener(GRAPHS_EVENT, bump)
    return () => window.removeEventListener(GRAPHS_EVENT, bump)
  }, [])
  useEffect(() => {
    const q = query.trim()
    const ctrl = new AbortController()
    let request: Promise<GraphMeta[]>
    if (q) {
      request = api.graphs(q, ctrl.signal)
    } else {
      allGraphs ??= api.graphs()
      request = allGraphs
    }
    request
      .then((graphs) => !ctrl.signal.aborted && setState({ graphs, error: null }))
      .catch((e: Error) => {
        if (!q) allGraphs = null
        if (e.name !== 'AbortError' && !ctrl.signal.aborted) setState({ graphs: null, error: e.message })
      })
    return () => ctrl.abort()
  }, [query, version])
  return state
}

// Граф целиком загружается один раз на сессию и переиспользуется при возврате на страницу.
const graphCache = new Map<string, Promise<GraphData>>()

// Сбрасывает кэш графа: нужен, когда граф достроился и его надо перечитать.
export function reloadGraph(id: string) {
  graphCache.delete(id)
  allGraphs = null
}

export function useGraph(id: string, version = 0): { data: GraphData | null; error: string | null; notFound: boolean } {
  const [state, setState] = useState<{ id: string; data: GraphData | null; error: string | null; notFound: boolean }>({
    id: '',
    data: null,
    error: null,
    notFound: false,
  })
  useEffect(() => {
    let alive = true
    let request = graphCache.get(id)
    if (!request) {
      request = api.graph(id)
      graphCache.set(id, request)
    }
    request
      .then((data) => alive && setState({ id, data, error: null, notFound: false }))
      .catch((e: Error) => {
        graphCache.delete(id)
        if (alive) setState({ id, data: null, error: e.message, notFound: e instanceof ApiError && e.status === 404 })
      })
    return () => {
      alive = false
    }
  }, [id, version])
  return state.id === id ? state : { data: null, error: null, notFound: false }
}

const FAV_KEY = 'favorite-graphs'

function readFavorites(): Set<string> {
  try {
    return new Set(JSON.parse(localStorage.getItem(FAV_KEY) ?? '[]') as string[])
  } catch {
    return new Set()
  }
}

// Избранное хранится в браузере: аккаунтов пока нет.
export function useFavorites() {
  const [favorites, setFavorites] = useState(readFavorites)
  const toggle = useCallback((id: string) => {
    setFavorites((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      try {
        localStorage.setItem(FAV_KEY, JSON.stringify([...next]))
      } catch {
        // хранилище недоступно — избранное живёт до перезагрузки
      }
      return next
    })
  }, [])
  return { favorites, toggle }
}
