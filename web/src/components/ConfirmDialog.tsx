import { useEffect, useRef, type ReactNode } from 'react'

// Окно подтверждения необратимого действия: Escape и клик по фону — отмена.
export function ConfirmDialog({
  title,
  children,
  confirmLabel,
  busy,
  error,
  onConfirm,
  onCancel,
}: {
  title: string
  children: ReactNode
  confirmLabel: string
  busy?: boolean
  error?: string | null
  onConfirm: () => void
  onCancel: () => void
}) {
  const cancelRef = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    // Фокус на «Отмене»: случайный Enter не должен удалять.
    cancelRef.current?.focus()
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && !busy && onCancel()
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [busy, onCancel])

  return (
    <div className="modal-backdrop" onMouseDown={(e) => e.target === e.currentTarget && !busy && onCancel()}>
      <div className="modal" role="alertdialog" aria-modal="true" aria-labelledby="confirm-title">
        <h2 id="confirm-title" className="modal-title">
          {title}
        </h2>
        <div className="modal-text">{children}</div>
        {error && (
          <p className="modal-error" role="alert">
            {error}
          </p>
        )}
        <div className="modal-actions">
          <button ref={cancelRef} className="button" onClick={onCancel} disabled={busy}>
            Отмена
          </button>
          <button className="button danger" onClick={onConfirm} disabled={busy}>
            {busy ? 'Удаляем…' : confirmLabel}
          </button>
        </div>
      </div>
    </div>
  )
}
