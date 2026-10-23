import { useEffect, useMemo, useState } from 'react'
import { GraphCard } from '../components/GraphCard'
import { graphCount, plural, useFavorites, useGraphs } from '../graphs'
import { Icon } from '../components/Icon'
import { icons } from '../icons'
import { navigate, useLocation } from '../router'

type Filter = 'all' | 'favorites' | `section:${string}`

export function LibraryPage() {
  const { query } = useLocation()
  const { favorites, toggle } = useFavorites()
  const [search, setSearch] = useState('')
  const [debounced, setDebounced] = useState('')
  useEffect(() => {
    const t = setTimeout(() => setDebounced(search.trim()), 250)
    return () => clearTimeout(t)
  }, [search])
  // Полный список — для сводки и фильтров; найденные — для сетки.
  const { graphs: all, error } = useGraphs()
  const { graphs: found } = useGraphs(debounced)
  const [newestFirst, setNewestFirst] = useState(true)
  const [layout, setLayout] = useState<'grid' | 'list'>('grid')
  const filter: Filter = (query.get('filter') as Filter | null) ?? 'all'

  const setFilter = (f: Filter) => navigate(f === 'all' ? '/graphs' : `/graphs?filter=${encodeURIComponent(f)}`, { replace: true })

  const graphList = useMemo(() => all ?? [], [all])
  const sections = [...new Set(graphList.map((g) => g.section).filter(Boolean))]
  const totalConcepts = graphList.reduce((sum, g) => sum + g.nodeCount, 0)

  const visible = useMemo(() => {
    // Поиск по названию и понятиям выполняет сервер; фильтры и порядок — здесь.
    const list = (debounced ? (found ?? []) : graphList).filter((g) => {
      if (filter === 'favorites' && !favorites.has(g.id)) return false
      if (filter.startsWith('section:') && g.section !== filter.slice('section:'.length)) return false
      return true
    })
    return newestFirst ? list : [...list].reverse()
  }, [debounced, found, graphList, filter, favorites, newestFirst])

  const chips: { id: Filter; label: string }[] = [
    { id: 'all', label: `Все графы · ${graphList.length}` },
    { id: 'favorites', label: `Избранное · ${graphList.filter((g) => favorites.has(g.id)).length}` },
    ...sections.map((s) => ({ id: `section:${s}` as Filter, label: s })),
  ]

  return (
    <div className="page">
      <div className="page-context">
        <span>Личное пространство</span>
        <span className="context-link">
          <Icon src={icons.lock} size={14} />
          Видно только вам
        </span>
      </div>

      <header className="library-head">
        <div>
          <h1 className="page-title">Мои графы</h1>
          <p className="page-lead">Всё, что вы уже связали воедино. Возвращайтесь и углубляйте знания.</p>
        </div>
        <button className="button primary" onClick={() => navigate('/')}>
          <Icon src={icons.plusWhite} size={18} />
          Создать граф
        </button>
      </header>

      <div className="summary">
        <Stat value={graphList.length} label={plural(graphList.length, 'граф', 'графа', 'графов') + ' в библиотеке'} />
        <Stat value={totalConcepts} label={plural(totalConcepts, 'понятие', 'понятия', 'понятий') + ' в графах'} />
        <Stat value={sections.length} label={plural(sections.length, 'раздел', 'раздела', 'разделов') + ' физики'} />
        <div className="summary-motto">
          <Icon src={icons.networkStat} size={22} />
          Знания складываются в систему
        </div>
      </div>

      <div className="library-tools">
        <div className="tools-row">
          <label className="search-field">
            <Icon src={icons.search} size={17} />
            <input type="search" placeholder="Поиск по названию или понятию" value={search} onChange={(e) => setSearch(e.target.value)} />
          </label>
          <button className="button" onClick={() => setNewestFirst((v) => !v)}>
            <Icon src={icons.sort} size={18} />
            {newestFirst ? 'Сначала новые' : 'Сначала старые'}
          </button>
          <div className="segmented" role="group" aria-label="Представление">
            <button className={layout === 'grid' ? 'active' : ''} onClick={() => setLayout('grid')} aria-label="Плитка" aria-pressed={layout === 'grid'}>
              <Icon src={icons.layoutGrid} size={17} />
            </button>
            <button className={layout === 'list' ? 'active' : ''} onClick={() => setLayout('list')} aria-label="Список" aria-pressed={layout === 'list'}>
              <Icon src={icons.list} size={18} />
            </button>
          </div>
        </div>
        <div className="chips-row">
          {chips.map((c) => (
            <button key={c.id} className={filter === c.id ? 'filter-chip active' : 'filter-chip'} onClick={() => setFilter(c.id)}>
              {c.label}
            </button>
          ))}
        </div>
      </div>

      {error ? (
        <div className="empty">Не удалось загрузить графы: {error}</div>
      ) : !all ? (
        <div className="empty">Загрузка…</div>
      ) : graphList.length === 0 ? (
        <div className="empty">Здесь появятся ваши графы. Загрузите первый конспект — и мы превратим его в карту знаний.</div>
      ) : visible.length > 0 ? (
        <div className={layout === 'grid' ? 'card-grid' : 'card-list'}>
          {visible.map((meta, i) => (
            <GraphCard
              key={meta.id}
              meta={meta}
              tint={i % 2 ? 'green' : 'blue'}
              favorite={favorites.has(meta.id)}
              onToggleFavorite={() => toggle(meta.id)}
              layout={layout}
            />
          ))}
        </div>
      ) : (
        <div className="empty">
          {filter === 'favorites' && !debounced
            ? 'В избранном пока пусто — отметьте граф через меню «…» на карточке.'
            : `Ничего не нашлось${debounced ? ` по запросу «${debounced}»` : ''}.`}
        </div>
      )}

      <div className="library-foot">
        <span>
          {visible.length < graphList.length
            ? `Показано ${visible.length} из ${graphList.length}`
            : graphList.length === 1
              ? 'Показан 1 граф'
              : `Показаны все ${graphCount(graphList.length)}`}
        </span>
        <span className="context-link">
          <Icon src={icons.cloudCheck} size={14} />
          Избранное сохраняется в этом браузере
        </span>
      </div>
    </div>
  )
}

function Stat({ value, label }: { value: number; label: string }) {
  return (
    <div className="stat">
      <span className="stat-value">{value}</span>
      <span className="stat-label">{label}</span>
    </div>
  )
}
