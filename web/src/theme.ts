import { useEffect, useState } from 'react'

// Оформление: как в системе (по умолчанию), светлое или тёмное. Выбор хранится в браузере.
export type ThemeChoice = 'system' | 'light' | 'dark'

const KEY = 'theme'
export const THEME_EVENT = 'app:theme'

function read(): ThemeChoice {
  try {
    const v = localStorage.getItem(KEY)
    return v === 'light' || v === 'dark' ? v : 'system'
  } catch {
    return 'system'
  }
}

export function applyTheme(choice: ThemeChoice = read()) {
  if (choice === 'system') document.documentElement.removeAttribute('data-theme')
  else document.documentElement.setAttribute('data-theme', choice)
  window.dispatchEvent(new Event(THEME_EVENT))
}

export function useTheme(): [ThemeChoice, (c: ThemeChoice) => void] {
  const [choice, setChoice] = useState(read)
  useEffect(() => {
    applyTheme(choice)
    try {
      if (choice === 'system') localStorage.removeItem(KEY)
      else localStorage.setItem(KEY, choice)
    } catch {
      // выбор просто не запомнится
    }
  }, [choice])
  return [choice, setChoice]
}

// Вызывает cb при любой смене оформления — вручную или в настройках системы.
export function onThemeChange(cb: () => void): () => void {
  const media = window.matchMedia('(prefers-color-scheme: dark)')
  media.addEventListener('change', cb)
  window.addEventListener(THEME_EVENT, cb)
  return () => {
    media.removeEventListener('change', cb)
    window.removeEventListener(THEME_EVENT, cb)
  }
}
