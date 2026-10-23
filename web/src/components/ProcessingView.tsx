import { useEffect, useState } from 'react'
import { api, type GraphMeta } from '../api'
import { icons } from '../icons'
import { linkClick } from '../router'
import { Icon } from './Icon'

const stages: { id: NonNullable<GraphMeta['stage']>; title: string }[] = [
  { id: 'reading', title: 'Читаю конспект' },
  { id: 'extracting', title: 'Нахожу понятия и формулы' },
  { id: 'linking', title: 'Строю связи между ними' },
  { id: 'checking', title: 'Проверяю формулы' },
]

// Экран, пока ИИ строит граф. Опрашивает сервер и вызывает onDone, когда обработка закончилась.
export function ProcessingView({ meta, onDone }: { meta: GraphMeta; onDone: () => void }) {
  const [current, setCurrent] = useState(meta)

  useEffect(() => {
    const ctrl = new AbortController()
    const timer = setInterval(() => {
      api
        .graph(meta.id, ctrl.signal)
        .then((g) => {
          if (g.meta.status !== 'processing') onDone()
          else setCurrent(g.meta)
        })
        .catch(() => {})
    }, 2000)
    return () => {
      clearInterval(timer)
      ctrl.abort()
    }
  }, [meta.id, onDone])

  const active = Math.max(0, stages.findIndex((s) => s.id === current.stage))
  const progress = Math.round(((active + 0.5) / stages.length) * 100)

  return (
    <div className="processing">
      <div className="processing-card">
        <div className="processing-progress" role="progressbar" aria-valuenow={progress} aria-valuemin={0} aria-valuemax={100}>
          <span style={{ width: `${progress}%` }} />
        </div>
        <p className="processing-hint">Обычно это занимает 1–2 минуты</p>
        <ol className="processing-steps">
          {stages.map((s, i) => {
            const state = i < active ? 'done' : i === active ? 'active' : 'waiting'
            return (
              <li key={s.id} className={state}>
                <span className="processing-mark" aria-hidden>
                  {state === 'done' ? '✓' : ''}
                </span>
                <div>
                  <div className="processing-step-title">{s.title}</div>
                  {state === 'active' && current.stageNote && <div className="processing-step-note">{current.stageNote}</div>}
                </div>
              </li>
            )
          })}
        </ol>
      </div>
      <div className="processing-preview" aria-hidden>
        {[0, 1, 2, 3, 4].map((i) => (
          <span key={i} className={i <= active + 1 ? 'skeleton-card shown' : 'skeleton-card'} />
        ))}
      </div>
      <p className="processing-leave">
        <Icon src={icons.cloudCheck} size={14} />
        Можно закрыть страницу — граф появится в «Моих графах».
      </p>
    </div>
  )
}

export function FailedView({ meta }: { meta: GraphMeta }) {
  return (
    <div className="processing">
      <div className="processing-card failed">
        <h2 className="concept-title">Не удалось построить граф</h2>
        <p className="concept-summary">{meta.error || 'Что-то пошло не так. Попробуйте загрузить конспект ещё раз.'}</p>
        <div className="graph-actions">
          <a className="button primary" href="/" onClick={linkClick('/')}>
            <Icon src={icons.plusWhite} size={18} />
            Загрузить другой файл
          </a>
          <a className="button" href="/graphs" onClick={linkClick('/graphs')}>
            Мои графы
          </a>
        </div>
      </div>
    </div>
  )
}
