import { useEffect, useRef, useState } from 'react'
import { api, type GraphNode } from '../api'
import { Icon } from './Icon'
import { icons } from '../icons'
import { nodeTypes } from '../meta'

const isMac = /Mac|iPhone|iPad/.test(navigator.platform)

export function Search({ graphId, onSelect }: { graphId: string; onSelect: (slug: string) => void }) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<GraphNode[]>([])
  const [active, setActive] = useState(0)
  const [open, setOpen] = useState(false)

  // ⌘K / Ctrl+K — быстрый переход к поиску.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        inputRef.current?.focus()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  useEffect(() => {
    const q = query.trim()
    if (!q) return
    const ctrl = new AbortController()
    const t = setTimeout(() => {
      api
        .search(graphId, q, ctrl.signal)
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
  }, [graphId, query])

  const choose = (n: GraphNode) => {
    onSelect(n.slug)
    setQuery('')
    setOpen(false)
    inputRef.current?.blur()
  }

  return (
    <div className="graph-search">
      <Icon src={icons.searchSmall} size={16} className="muted" />
      <input
        ref={inputRef}
        type="search"
        placeholder="Найти понятие в графе"
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
            inputRef.current?.blur()
          }
        }}
        aria-label="Найти понятие в графе"
      />
      <kbd>{isMac ? '⌘ K' : 'Ctrl K'}</kbd>
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
              <span className="type-dot" style={{ background: nodeTypes[n.type].color }} />
              <span className="title">{n.title}</span>
              <span className="muted type">{nodeTypes[n.type].label}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
