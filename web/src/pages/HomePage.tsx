import { useRef, useState } from 'react'
import { GraphCard } from '../components/GraphCard'
import { DEMO_GRAPH_ID, graphPath, useFavorites, useGraphs } from '../graphs'
import { Icon } from '../components/Icon'
import { icons } from '../icons'
import { linkClick } from '../router'

const ACCEPT = '.pdf,.docx,.txt,.jpg,.jpeg,.png'
const MAX_SIZE = 25 * 1024 * 1024

const steps = [
  { n: '01', title: 'Загрузите материал', text: 'Печатный текст или фото записей' },
  { n: '02', title: 'Доверьте разбор ИИ', text: 'Выделим главное и найдём связи' },
  { n: '03', title: 'Исследуйте свой граф', text: 'От понятия — к формуле и источнику' },
]

export function HomePage() {
  const { graphs, error } = useGraphs()
  const { favorites, toggle } = useFavorites()

  return (
    <div className="page">
      <div className="page-context">
        <span>Рабочее пространство</span>
        <a className="context-link" href="#steps">
          <Icon src={icons.circleHelp} size={17} />
          Как это работает
        </a>
      </div>

      <header className="hero">
        <div className="eyebrow">ОТ КОНСПЕКТА К ПОНИМАНИЮ</div>
        <h1 className="hero-title">Большая картина начинается со связи.</h1>
        <p className="hero-lead">Загрузите конспект по физике — превратим записи в понятную карту знаний.</p>
      </header>

      <section className="new-graph">
        <UploadZone />
        <aside className="explainer">
          <div className="explainer-head">
            <span className="caption">ВСЁ СТАНОВИТСЯ СВЯЗАННЫМ</span>
            <Icon src={icons.moveUpRight} size={17} />
          </div>
          <div className="explainer-scheme" aria-hidden>
            <img className="explainer-links" src={icons.exampleConnections} width={256} height={67} alt="" />
            <span className="explainer-concept" style={{ left: 1, top: 18 }}>Сила</span>
            <span className="explainer-concept" style={{ left: 238, top: 18 }}>Ускорение</span>
            <span className="explainer-label">связывает</span>
            <span className="explainer-law">F = ma</span>
          </div>
          <p className="explainer-title">Не просто выучить. Понять.</p>
          <p className="explainer-text">Понятия, формулы и связи между ними — с опорой на ваш конспект.</p>
        </aside>
      </section>

      <ol className="steps" id="steps">
        {steps.map((s) => (
          <li key={s.n}>
            <span className="step-n">{s.n}</span>
            <div>
              <div className="step-title-text">{s.title}</div>
              <div className="step-text">{s.text}</div>
            </div>
          </li>
        ))}
      </ol>

      <section className="recent">
        <div className="section-head">
          <h2 className="section-title">Продолжить изучение</h2>
          <a className="accent-link" href="/graphs" onClick={linkClick('/graphs')}>
            Все графы
            <Icon src={icons.arrowRight} size={16} />
          </a>
        </div>
        {error ? (
          <div className="empty">Не удалось загрузить графы: {error}</div>
        ) : graphs && graphs.length === 0 ? (
          <div className="empty">Здесь появятся ваши графы. Загрузите первый конспект.</div>
        ) : (
          <div className="card-grid">
            {(graphs ?? []).slice(0, 3).map((meta, i) => (
              <GraphCard
                key={meta.id}
                meta={meta}
                tint={i % 2 ? 'green' : 'blue'}
                favorite={favorites.has(meta.id)}
                onToggleFavorite={() => toggle(meta.id)}
              />
            ))}
          </div>
        )}
      </section>
    </div>
  )
}

function UploadZone() {
  const inputRef = useRef<HTMLInputElement>(null)
  const [dragging, setDragging] = useState(false)
  const [notice, setNotice] = useState<{ kind: 'info' | 'error'; text: string } | null>(null)

  // Построение графа по конспекту — этап 2 плана. Пока файл никуда не отправляется.
  const accept = (file: File | undefined) => {
    if (!file) return
    const ext = file.name.split('.').pop()?.toLowerCase() ?? ''
    if (!ACCEPT.split(',').includes(`.${ext}`)) {
      setNotice({ kind: 'error', text: 'Этот формат не поддерживается. Подойдут PDF, DOCX, TXT, JPG и PNG.' })
    } else if (file.size > MAX_SIZE) {
      setNotice({ kind: 'error', text: 'Файл больше 25 МБ.' })
    } else {
      setNotice({ kind: 'info', text: `«${file.name}» выбран. Построение графа по конспекту пока в разработке — а демо-граф уже можно исследовать.` })
    }
  }

  return (
    <div
      className={dragging ? 'upload dragging' : 'upload'}
      onDragOver={(e) => {
        e.preventDefault()
        setDragging(true)
      }}
      onDragLeave={() => setDragging(false)}
      onDrop={(e) => {
        e.preventDefault()
        setDragging(false)
        accept(e.dataTransfer.files[0])
      }}
    >
      <div className="upload-symbol">
        <Icon src={icons.fileUp} size={26} />
      </div>
      <div className="upload-hint">
        <div className="upload-title">Здесь ваш следующий граф</div>
        <div className="upload-sub">Перетащите конспект сюда или выберите файл</div>
      </div>
      <button className="button primary" onClick={() => inputRef.current?.click()}>
        <Icon src={icons.plusWhite} size={18} />
        Выбрать конспект
      </button>
      <input
        ref={inputRef}
        type="file"
        accept={ACCEPT}
        hidden
        onChange={(e) => {
          accept(e.target.files?.[0])
          e.target.value = ''
        }}
      />
      {notice ? (
        <p className={`upload-notice ${notice.kind}`} role="status">
          {notice.text}{' '}
          {notice.kind === 'info' && (
            <a href={graphPath(DEMO_GRAPH_ID)} onClick={linkClick(graphPath(DEMO_GRAPH_ID))}>Открыть демо-граф</a>
          )}
        </p>
      ) : (
        <div className="upload-formats">PDF, DOCX, TXT, JPG и PNG · до 25 МБ</div>
      )}
      <div className="upload-privacy">
        <Icon src={icons.lockSmall} size={12} />
        Ваши материалы видны только вам
      </div>
    </div>
  )
}
