import { useSyncExternalStore } from 'react'

// useMediaQuery — совпадает ли сейчас медиазапрос; обновляется при изменении размера окна.
export function useMediaQuery(query: string): boolean {
  return useSyncExternalStore(
    (cb) => {
      const m = window.matchMedia(query)
      m.addEventListener('change', cb)
      return () => m.removeEventListener('change', cb)
    },
    () => window.matchMedia(query).matches,
  )
}
