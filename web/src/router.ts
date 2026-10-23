import { useSyncExternalStore } from 'react'

// Минимальный роутер на History API: приложению хватает трёх маршрутов.
const NAV_EVENT = 'app:navigate'

function subscribe(cb: () => void) {
  window.addEventListener('popstate', cb)
  window.addEventListener(NAV_EVENT, cb)
  return () => {
    window.removeEventListener('popstate', cb)
    window.removeEventListener(NAV_EVENT, cb)
  }
}

const snapshot = () => window.location.pathname + window.location.search

export function useLocation(): { path: string; query: URLSearchParams } {
  const loc = useSyncExternalStore(subscribe, snapshot)
  const url = new URL(loc, window.location.origin)
  return { path: url.pathname, query: url.searchParams }
}

export function navigate(to: string, { replace = false } = {}) {
  if (to === snapshot()) return
  if (replace) window.history.replaceState(null, '', to)
  else window.history.pushState(null, '', to)
  window.dispatchEvent(new Event(NAV_EVENT))
}

// Обработчик для <a>: обычный клик — переход без перезагрузки, с модификаторами — как у браузера.
export function linkClick(to: string) {
  return (e: React.MouseEvent) => {
    if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return
    e.preventDefault()
    navigate(to)
  }
}
