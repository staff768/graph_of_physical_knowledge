import { useEffect, useState } from 'react'
import { api, type GraphNode } from '../api'
import { nodeTypes } from '../meta'

export function Search({ onSelect }: { onSelect: (slug: string) => void }) {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<GraphNode[]>([])
  const [active, setActive] = useState(0)
  const [open, setOpen] = useState(false)

  useEffect(() => {
    const q = query.trim()
    if (!q) return
    const ctrl = new AbortController()
    const t = setTimeout(() => {
      api.search(q, ctrl.signal)
        .then((r) => {
          setResults(r)
          setActive(0)
        })
        .catch(() => {})
    }, 150)
    return () => {
      clearTimeout(t)
      ctrl.abort()
    }
  }, [query])

  const choose = (n: GraphNode) => {
    onSelect(n.slug)
    setQuery('')
    setOpen(false)
  }

  return (
    <div className="search">
      <input
        type="search"
        placeholder="Поиск: закон Ома, молния, ток…"
        value={query}
        onChange={(e) => {
          setQuery(e.target.value)
          setOpen(true)
        }}
        onFocus={() => setOpen(true)}
        onBlur={() => setTimeout(() => setOpen(false), 150)}
        onKeyDown={(e) => {
          if (e.key === 'ArrowDown') {
            e.preventDefault()
            setActive((a) => Math.min(a + 1, results.length - 1))
          } else if (e.key === 'ArrowUp') {
            e.preventDefault()
            setActive((a) => Math.max(a - 1, 0))
          } else if (e.key === 'Enter' && results[active]) {
            choose(results[active])
          } else if (e.key === 'Escape') {
            setOpen(false)
          }
        }}
        aria-label="Поиск по графу"
      />
      {open && query.trim() && (
        <ul className="search-results" role="listbox">
          {results.length === 0 && <li className="muted empty">Ничего не найдено</li>}
          {results.map((n, i) => (
            <li
              key={n.slug}
              role="option"
              aria-selected={i === active}
              className={i === active ? 'active' : ''}
              onMouseDown={() => choose(n)}
              onMouseEnter={() => setActive(i)}
            >
              <span className="dot" style={{ background: nodeTypes[n.type].color }} />
              <span className="title">{n.title}</span>
              <span className="muted type">{nodeTypes[n.type].label}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
