import { useEffect, useRef, useState } from 'react'
import { api, type OwnerStats } from '../api'
import { plural } from '../graphs'
import { useTheme, type ThemeChoice } from '../theme'
import { Icon } from './Icon'
import { icons } from '../icons'

const themeOptions: { id: ThemeChoice; label: string }[] = [
  { id: 'system', label: 'Авто' },
  { id: 'light', label: 'Светлая' },
  { id: 'dark', label: 'Тёмная' },
]

const NAME_KEY = 'profile-name'

function readName() {
  try {
    return localStorage.getItem(NAME_KEY) ?? ''
  } catch {
    return ''
  }
}

function saveName(name: string) {
  try {
    localStorage.setItem(NAME_KEY, name)
  } catch {
    // имя просто не запомнится
  }
}

// Профиль внизу меню: имя, сводка личного пространства и тема оформления.
// Личное пространство без регистрации: графы привязаны к ключу в cookie браузера.
export function ProfileMenu() {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState(readName)
  const [stats, setStats] = useState<OwnerStats | null>(null)
  const [theme, setTheme] = useTheme()
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const ctrl = new AbortController()
    api
      .me(ctrl.signal)
      .then(setStats)
      .catch(() => {})
    const onClick = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false)
    document.addEventListener('mousedown', onClick)
    document.addEventListener('keydown', onKey)
    return () => {
      ctrl.abort()
      document.removeEventListener('mousedown', onClick)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  const shown = name.trim() || 'Гость'
  const letter = shown[0].toUpperCase()
  const top = stats?.sections[0]

  return (
    <div className="profile" ref={ref}>
      <button className={open ? 'nav-profile open' : 'nav-profile'} onClick={() => setOpen((o) => !o)} aria-expanded={open} aria-haspopup="dialog" aria-label="Профиль и настройки">
        <div className="avatar">{letter}</div>
        <div className="nav-profile-text nav-label">
          <div className="nav-profile-name">{shown}</div>
          <div className="nav-profile-sub">Личное пространство</div>
        </div>
        <Icon src={icons.chevronsUpDown} size={13} className="muted nav-label" />
      </button>

      {open && (
        <div className="profile-popover" role="dialog" aria-label="Профиль">
          <div className="profile-head">
            <div className="avatar large">{letter}</div>
            <div className="profile-head-text">
              <input
                className="profile-name-input"
                value={name}
                placeholder="Как вас зовут?"
                maxLength={40}
                aria-label="Имя"
                onChange={(e) => {
                  setName(e.target.value)
                  saveName(e.target.value)
                }}
              />
              <span className="profile-caption">Личное пространство · без регистрации</span>
            </div>
          </div>

          <div className="profile-stats">
            <Stat value={stats?.graphs} label={plural(stats?.graphs ?? 0, 'граф', 'графа', 'графов')} />
            <Stat value={stats?.concepts} label={plural(stats?.concepts ?? 0, 'понятие', 'понятия', 'понятий')} />
            <Stat value={stats?.links} label={plural(stats?.links ?? 0, 'связь', 'связи', 'связей')} />
            <Stat value={stats?.stories} label={plural(stats?.stories ?? 0, 'рассказ ИИ', 'рассказа ИИ', 'рассказов ИИ')} />
          </div>
          {top && (
            <p className="profile-line">
              Чаще всего: <strong>{top.section}</strong> · {top.graphs} {plural(top.graphs, 'граф', 'графа', 'графов')}
            </p>
          )}
          {stats && stats.graphs === 0 && <p className="profile-line">Загрузите первый конспект — здесь появится ваша статистика.</p>}

          <div className="profile-block">
            <div className="settings-title">Тема оформления</div>
            <div className="settings-segmented" role="radiogroup" aria-label="Тема оформления">
              {themeOptions.map((o) => (
                <button key={o.id} role="radio" aria-checked={theme === o.id} className={theme === o.id ? 'active' : ''} onClick={() => setTheme(o.id)}>
                  {o.label}
                </button>
              ))}
            </div>
          </div>

          <p className="settings-hint">Ваши графы видны только в этом браузере. Чтобы показать граф другому, нажмите «Поделиться» на его странице.</p>
        </div>
      )}
    </div>
  )
}

function Stat({ value, label }: { value: number | undefined; label: string }) {
  return (
    <div className="profile-stat">
      <span className="profile-stat-value">{value ?? '—'}</span>
      <span className="profile-stat-label">{label}</span>
    </div>
  )
}
