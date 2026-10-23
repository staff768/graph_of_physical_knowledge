import { useState } from 'react'
import type { GraphMeta } from '../api'
import { api } from '../api'
import { conceptCount, graphPath, linkCount, notifyGraphsChanged } from '../graphs'
import { ConfirmDialog } from './ConfirmDialog'
import { Icon, Mask } from './Icon'
import { icons } from '../icons'
import { linkClick } from '../router'

interface Props {
  meta: GraphMeta
  tint: 'blue' | 'green'
  favorite: boolean
  onToggleFavorite: () => void
  layout?: 'grid' | 'list'
}

// Позиции понятий вокруг центра превью — как в макете.
const satellites = [
  { left: 17, top: 10, width: 104 },
  { left: 250, top: 15, width: 100 },
  { left: 20, top: 114, width: 108 },
  { left: 245, top: 116, width: 110 },
]

export function GraphCard({ meta, tint, favorite, onToggleFavorite, layout = 'grid' }: Props) {
  const [menuOpen, setMenuOpen] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  const remove = async () => {
    setDeleting(true)
    setDeleteError(null)
    try {
      await api.deleteGraph(meta.id)
      setConfirming(false)
      notifyGraphsChanged()
    } catch (e) {
      setDeleteError(e instanceof Error ? e.message : 'Не удалось удалить граф.')
    } finally {
      setDeleting(false)
    }
  }
  const href = graphPath(meta.id)
  const concepts = (meta.preview ?? []).slice(0, satellites.length)
  const when = meta.demo ? 'демо' : formatDate(meta.createdAt)
  const stats =
    meta.status === 'processing'
      ? ['ИИ строит граф…', when]
      : meta.status === 'failed'
        ? ['не удалось построить', when]
        : [conceptCount(meta.nodeCount), linkCount(meta.edgeCount), when]

  return (
    <article className={`graph-card ${layout}`}>
      <a className={`graph-card-preview ${tint}`} href={href} onClick={linkClick(href)} aria-label={`Открыть граф «${meta.title}»`}>
        <div className="preview-scheme">
          <Mask className="preview-links" src={icons.previewConnections} width={230} height={107} />
          <span className={meta.status === 'ready' ? 'preview-center' : 'preview-center pending'}>
            <span>{meta.status === 'ready' ? meta.section || 'Физика' : meta.status === 'processing' ? 'Строится…' : 'Ошибка'}</span>
          </span>
          {concepts.map((title, i) => (
            <span key={title} className="preview-concept" style={satellites[i]}>
              <span>{title}</span>
            </span>
          ))}
        </div>
      </a>
      <div className="graph-card-info">
        <div className="graph-card-row">
          {meta.status === 'processing' ? (
            <span className="tag accent">Строится</span>
          ) : meta.status === 'failed' ? (
            <span className="tag danger">Не удалось</span>
          ) : (
            <span className="tag">{meta.section || 'Физика'}</span>
          )}
          <div className="graph-card-tools">
            {favorite && (
              <button className="icon-button" onClick={onToggleFavorite} aria-label="Убрать из избранного" title="Убрать из избранного">
                <Icon src={icons.starAccent} size={17} className="accent" />
              </button>
            )}
            <div className="menu-anchor">
              <button className="icon-button" onClick={() => setMenuOpen((o) => !o)} onBlur={() => setTimeout(() => setMenuOpen(false), 150)} aria-label="Действия" aria-expanded={menuOpen}>
                <Icon src={icons.ellipsis} size={17} className="muted" />
              </button>
              {menuOpen && (
                <div className="menu" role="menu">
                  <a role="menuitem" href={href} onClick={linkClick(href)}>Открыть</a>
                  <button role="menuitem" onMouseDown={onToggleFavorite}>
                    {favorite ? 'Убрать из избранного' : 'В избранное'}
                  </button>
                  {meta.mine && (
                    <button role="menuitem" className="danger" onMouseDown={() => setConfirming(true)}>
                      Удалить
                    </button>
                  )}
                </div>
              )}
            </div>
          </div>
        </div>
        <a className="graph-card-title" href={href} onClick={linkClick(href)}>{meta.title}</a>
        <div className="graph-card-meta">{stats.join(' · ')}</div>
      </div>
      {confirming && (
        <ConfirmDialog
          title={`Удалить граф «${meta.title}»?`}
          confirmLabel="Удалить"
          busy={deleting}
          error={deleteError}
          onConfirm={() => void remove()}
          onCancel={() => {
            setConfirming(false)
            setDeleteError(null)
          }}
        >
          Граф и все его понятия удалятся без возможности восстановления.
        </ConfirmDialog>
      )}
    </article>
  )
}

// «сегодня», «вчера» или «20 октября» — как в макете.
function formatDate(iso: string) {
  const d = new Date(iso)
  const days = Math.round((startOfDay(new Date()) - startOfDay(d)) / 86_400_000)
  if (days === 0) return 'сегодня'
  if (days === 1) return 'вчера'
  return d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' })
}

const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
