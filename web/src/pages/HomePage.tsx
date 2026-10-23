import { useRef, useState } from 'react'
import { EmptyState } from '../components/EmptyState'
import { GraphCard } from '../components/GraphCard'
import { api } from '../api'
import { graphPath, invalidateGraphs, plural, useFavorites, useGraphs } from '../graphs'
import { Icon, Mask } from '../components/Icon'
import { icons } from '../icons'
import { linkClick, navigate } from '../router'

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
            <Mask className="explainer-links" src={icons.exampleConnections} width={256} height={67} />
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
          <EmptyState compact tone="danger" title="Графы не загрузились">
            Сервер не ответил: {error}. Обновите страницу.
          </EmptyState>
        ) : graphs && graphs.length === 0 ? (
          <EmptyState compact title="Здесь появятся ваши графы">
            Загрузите первый конспект — и граф появится в этом списке.
          </EmptyState>
        ) : (
          <div className="card-grid">
            {(graphs ?? []).filter((g) => g.status === 'ready').slice(0, 3).map((meta, i) => (
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

const MAX_FILES = 10
const isImage = (f: File) => /\.(jpe?g|png)$/i.test(f.name)
const extOk = (f: File) => ACCEPT.split(',').includes(`.${f.name.split('.').pop()?.toLowerCase() ?? ''}`)
// Фото с телефона называются по порядку (IMG_0412, IMG_0413…) — сортируем по имени с учётом чисел.
const byName = (a: File, b: File) => a.name.localeCompare(b.name, 'ru', { numeric: true })

function UploadZone() {
  const inputRef = useRef<HTMLInputElement>(null)
  const [dragging, setDragging] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  // Фото страниц, которые ждут отправки: их можно дополнить или убрать перед построением.
  const [pages, setPages] = useState<File[]>([])

  const send = async (files: File[]) => {
    setError(null)
    setUploading(true)
    try {
      const { id } = await api.upload(files)
      invalidateGraphs()
      navigate(graphPath(id))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Не удалось загрузить файл.')
      setUploading(false)
    }
  }

  const accept = (list: FileList | null) => {
    const files = Array.from(list ?? [])
    if (files.length === 0 || uploading) return
    if (files.some((f) => !extOk(f))) {
      setError('Этот формат не поддерживается. Подойдут PDF, DOCX, TXT, JPG и PNG.')
      return
    }
    const docs = files.filter((f) => !isImage(f))
    if (docs.length > 0) {
      // Документ (PDF, DOCX, TXT) отправляется сразу и только по одному.
      if (files.length > 1 || pages.length > 0) {
        setError('Документ загружайте отдельно. Несколько файлов можно, только если это фото страниц.')
        return
      }
      if (docs[0].size > MAX_SIZE) {
        setError('Файл больше 25 МБ.')
        return
      }
      void send(docs)
      return
    }
    const next = [...pages, ...files.filter((f) => !pages.some((p) => p.name === f.name && p.size === f.size))].sort(byName)
    if (next.length > MAX_FILES) {
      setError(`Можно загрузить не больше ${MAX_FILES} фото за раз.`)
      return
    }
    if (next.reduce((sum, f) => sum + f.size, 0) > MAX_SIZE) {
      setError('Фото вместе больше 25 МБ.')
      return
    }
    setError(null)
    setPages(next)
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
        accept(e.dataTransfer.files)
      }}
      aria-busy={uploading}
    >
      {pages.length > 0 ? (
        <PagesPreview
          pages={pages}
          disabled={uploading}
          onRemove={(f) => {
            forgetThumbs([f])
            setPages(pages.filter((p) => p !== f))
          }}
        />
      ) : (
        <>
          <div className="upload-symbol">
            <Icon src={icons.fileUp} size={26} className="accent" />
          </div>
          <div className="upload-hint">
            <div className="upload-title">{uploading ? 'Загружаем конспект…' : 'Здесь ваш следующий граф'}</div>
            <div className="upload-sub">Перетащите конспект или фото страниц сюда либо выберите файлы</div>
          </div>
        </>
      )}

      {pages.length > 0 ? (
        <div className="upload-actions">
          <button className="button primary" onClick={() => void send(pages)} disabled={uploading}>
            {uploading ? 'Загружаем…' : `Построить граф · ${pages.length} ${plural(pages.length, 'страница', 'страницы', 'страниц')}`}
          </button>
          <button className="button" onClick={() => inputRef.current?.click()} disabled={uploading || pages.length >= MAX_FILES}>
            <Icon src={icons.plus} size={15} />
            Добавить страницы
          </button>
          <button
            className="button ghost"
            onClick={() => {
              forgetThumbs(pages)
              setPages([])
            }}
            disabled={uploading}
          >
            Очистить
          </button>
        </div>
      ) : (
        <button className="button primary" onClick={() => inputRef.current?.click()} disabled={uploading}>
          <Icon src={icons.plusWhite} size={18} />
          Выбрать конспект
        </button>
      )}
      <input
        ref={inputRef}
        type="file"
        accept={ACCEPT}
        multiple
        hidden
        onChange={(e) => {
          accept(e.target.files)
          e.target.value = ''
        }}
      />
      {error ? (
        <p className="upload-notice error" role="alert">
          {error}
        </p>
      ) : (
        <div className="upload-formats">
          {pages.length > 0
            ? 'Порядок страниц — по именам файлов. Проверьте, что страницы идут по порядку.'
            : 'PDF, DOCX или TXT — один файл; фото страниц JPG и PNG — до 10 штук · до 25 МБ'}
        </div>
      )}
      <div className="upload-privacy">
        <Icon src={icons.lockSmall} size={12} />
        Файлы удаляются сразу после построения графа
      </div>
    </div>
  )
}

// Ссылки на миниатюры создаются один раз на файл и освобождаются, когда страницу убирают.
const thumbs = new WeakMap<File, string>()
function thumbUrl(f: File): string {
  let url = thumbs.get(f)
  if (!url) {
    url = URL.createObjectURL(f)
    thumbs.set(f, url)
  }
  return url
}
function forgetThumbs(files: File[]) {
  for (const f of files) {
    const url = thumbs.get(f)
    if (url) URL.revokeObjectURL(url)
    thumbs.delete(f)
  }
}

// Превью выбранных фото с номерами страниц.
function PagesPreview({ pages, disabled, onRemove }: { pages: File[]; disabled: boolean; onRemove: (f: File) => void }) {
  return (
    <ol className="pages-preview" aria-label="Страницы конспекта">
      {pages.map((f, i) => (
        <li key={`${f.name}:${f.size}`} className="page-thumb">
          <img src={thumbUrl(f)} alt={`Страница ${i + 1}`} />
          <span className="page-number">{i + 1}</span>
          <button className="page-remove" onClick={() => onRemove(f)} disabled={disabled} aria-label={`Убрать страницу ${i + 1}`}>
            <Icon src={icons.x} size={12} />
          </button>
          <span className="page-name" title={f.name}>
            {f.name}
          </span>
        </li>
      ))}
    </ol>
  )
}
