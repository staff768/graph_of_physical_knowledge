import type { ReactNode } from 'react'
import { icons } from '../icons'
import { Mask } from './Icon'

// Пустой экран или ошибка: схема из пустых «понятий» в стиле превью графа,
// заголовок, объяснение и действие, которое ведёт дальше.
export function EmptyState({
  title,
  children,
  action,
  tone = 'neutral',
  compact = false,
}: {
  title: string
  children?: ReactNode
  action?: ReactNode
  tone?: 'neutral' | 'danger'
  compact?: boolean
}) {
  return (
    <div className={`empty-state ${tone}${compact ? ' compact' : ''}`}>
      {!compact && (
        <div className="empty-scheme" aria-hidden>
          <Mask className="empty-links" src={icons.previewConnections} width={230} height={107} />
          <span className="empty-node center" />
          <span className="empty-node" style={{ left: 17, top: 10 }} />
          <span className="empty-node" style={{ left: 250, top: 15 }} />
          <span className="empty-node" style={{ left: 20, top: 114 }} />
          <span className="empty-node" style={{ left: 245, top: 116 }} />
        </div>
      )}
      <h2 className="empty-title">{title}</h2>
      {children && <p className="empty-text">{children}</p>}
      {action && <div className="empty-action">{action}</div>}
    </div>
  )
}
