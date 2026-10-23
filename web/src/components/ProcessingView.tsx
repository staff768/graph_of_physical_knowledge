import { useEffect, useState } from 'react'
import { api, type GraphMeta } from '../api'
import { icons } from '../icons'
import { linkClick } from '../router'
import { nodeTypes } from '../meta'
import { EmptyState } from './EmptyState'
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
      <GrowingGraph stage={active} />
      <p className="processing-leave">
        <Icon src={icons.cloudCheck} size={14} />
        Можно закрыть страницу — граф появится в «Моих графах».
      </p>
    </div>
  )
}

// Мини-граф, который собирается по этапам: понятия → связи → проверка формулы.
// Колонки повторяют раскладку настоящего графа: законы и величины → формулы → явление.
const sketchNodes = [
  { x: 0, y: 24, kind: 'law' },
  { x: 0, y: 214, kind: 'quantity' },
  { x: 160, y: 24, kind: 'formula' },
  { x: 160, y: 119, kind: 'formula' },
  { x: 160, y: 214, kind: 'formula' },
  { x: 320, y: 119, kind: 'phenomenon' },
] as const
const sketchEdges = [
  [0, 2], [0, 3], [1, 4], [1, 3], [2, 5], [3, 5], [4, 5],
] as const
const NODE_W = 120
const NODE_H = 46

function GrowingGraph({ stage }: { stage: number }) {
  // reading — пусто; extracting — понятия; linking — связи; checking — формула проверяется.
  const nodesShown = stage === 0 ? 0 : stage === 1 ? 5 : sketchNodes.length
  const edgesShown = stage >= 2
  return (
    <div className="growing" aria-hidden>
      <svg className="growing-edges" viewBox={`0 0 ${320 + NODE_W} ${214 + NODE_H + 24}`}>
        {sketchEdges.map(([a, b]) => {
          const from = sketchNodes[a]
          const to = sketchNodes[b]
          const x1 = from.x + NODE_W
          const y1 = from.y + NODE_H / 2
          const x2 = to.x
          const y2 = to.y + NODE_H / 2
          const mid = (x1 + x2) / 2
          return (
            <path
              key={`${a}-${b}`}
              className={edgesShown ? 'growing-edge shown' : 'growing-edge'}
              d={`M${x1},${y1} C${mid},${y1} ${mid},${y2} ${x2},${y2}`}
              pathLength={1}
            />
          )
        })}
      </svg>
      {sketchNodes.map((n, i) => (
        <span
          key={i}
          className={[
            'growing-node',
            i < nodesShown ? 'shown' : '',
            stage === 3 && i === 3 ? 'checking' : '',
          ].join(' ')}
          style={{ left: n.x, top: n.y, transitionDelay: i < nodesShown ? `${i * 90}ms` : '0ms' }}
        >
          <i style={{ background: nodeTypes[n.kind].color }} />
          <b />
          <b className="short" />
        </span>
      ))}
    </div>
  )
}

export function FailedView({ meta }: { meta: GraphMeta }) {
  return (
    <div className="processing">
      <div className="processing-failed">
        <EmptyState
          tone="danger"
          title="Граф не построился"
          action={
            <>
              <a className="button primary" href="/" onClick={linkClick('/')}>
                <Icon src={icons.plusWhite} size={18} />
                Загрузить другой файл
              </a>
              <a className="button" href="/graphs" onClick={linkClick('/graphs')}>
                Мои графы
              </a>
            </>
          }
        >
          {meta.error || 'Попробуйте загрузить конспект ещё раз.'}
        </EmptyState>
      </div>
    </div>
  )
}
