import { useEffect, useRef, useState } from 'react'
import { useGraphs } from '../graphs'
import { Icon } from './Icon'
import { icons } from '../icons'
import { linkClick } from '../router'
import { useTheme, type ThemeChoice } from '../theme'

type Section = 'home' | 'graphs'

const themeOptions: { id: ThemeChoice; label: string }[] = [
  { id: 'system', label: 'Авто' },
  { id: 'light', label: 'Светлая' },
  { id: 'dark', label: 'Тёмная' },
]

const COLLAPSE_KEY = 'nav-collapsed'

function readCollapsed() {
  try {
    return localStorage.getItem(COLLAPSE_KEY) === '1'
  } catch {
    return false
  }
}

export function Nav({ active }: { active: Section }) {
  const [collapsed, setCollapsed] = useState(readCollapsed)
  const { graphs } = useGraphs()
  const [theme, setTheme] = useTheme()
  const [settingsOpen, setSettingsOpen] = useState(false)
  const settingsRef = useRef<HTMLDivElement>(null)

  // Меню настроек закрывается по клику снаружи и по Escape.
  useEffect(() => {
    if (!settingsOpen) return
    const onClick = (e: MouseEvent) => {
      if (!settingsRef.current?.contains(e.target as Node)) setSettingsOpen(false)
    }
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setSettingsOpen(false)
    document.addEventListener('mousedown', onClick)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onClick)
      document.removeEventListener('keydown', onKey)
    }
  }, [settingsOpen])
  const toggle = () => {
    setCollapsed((c) => {
      try {
        localStorage.setItem(COLLAPSE_KEY, c ? '0' : '1')
      } catch {
        // настройка просто не запомнится
      }
      return !c
    })
  }

  return (
    <nav className={collapsed ? 'nav collapsed' : 'nav'} aria-label="Основная навигация">
      <div className="nav-top">
        <div className="nav-brand-row">
          <a className="nav-brand" href="/" onClick={linkClick('/')}>
            <img src={icons.brand} width={27} height={29} alt="" aria-hidden />
            <span className="nav-label">Физграф</span>
          </a>
          <button className="nav-burger" onClick={toggle} aria-label={collapsed ? 'Развернуть меню' : 'Свернуть меню'}>
            <Icon src={icons.hamburger} size={20} />
          </button>
        </div>

        <div className="nav-items">
          <a className={active === 'home' ? 'nav-item active' : 'nav-item'} href="/" onClick={linkClick('/')}>
            <Icon src={icons.house} size={19} />
            <span className="nav-label">Главная</span>
          </a>
          <a className={active === 'graphs' ? 'nav-item active' : 'nav-item'} href="/graphs" onClick={linkClick('/graphs')}>
            <Icon src={icons.network} size={19} />
            <span className="nav-label">Мои графы</span>
            {graphs && <span className="nav-count nav-label">{graphs.length}</span>}
          </a>
          <div className="nav-settings" ref={settingsRef}>
            <button
              className={settingsOpen ? 'nav-item open' : 'nav-item'}
              onClick={() => setSettingsOpen((o) => !o)}
              aria-expanded={settingsOpen}
              aria-haspopup="dialog"
            >
              <Icon src={icons.settings} size={19} />
              <span className="nav-label">Настройки</span>
            </button>
            {settingsOpen && (
              <div className="settings-popover" role="dialog" aria-label="Настройки">
                <div className="settings-title">Тема оформления</div>
                <div className="settings-segmented" role="radiogroup" aria-label="Тема оформления">
                  {themeOptions.map((o) => (
                    <button key={o.id} role="radio" aria-checked={theme === o.id} className={theme === o.id ? 'active' : ''} onClick={() => setTheme(o.id)}>
                      {o.label}
                    </button>
                  ))}
                </div>
                <p className="settings-hint">«Авто» — как в настройках системы.</p>
              </div>
            )}
          </div>
        </div>

        <div className="nav-quick nav-label">
          <div className="nav-caption">ПОД РУКОЙ</div>
          <a href="/graphs?filter=favorites" onClick={linkClick('/graphs?filter=favorites')}>
            <Icon src={icons.starMuted} size={16} />
            Избранное
          </a>
          <a href="/graphs" onClick={linkClick('/graphs')}>
            <Icon src={icons.clock} size={16} />
            Недавние
          </a>
        </div>
      </div>

      <div className="nav-bottom nav-label">
        <div className="nav-note">
          <Icon src={icons.sparkles} size={19} className="accent" />
          <p className="nav-note-quote">Понимание начинается с хорошего вопроса.</p>
          <p className="nav-note-caption">Находите связи каждый день</p>
        </div>
        <div className="nav-profile">
          <div className="avatar">Г</div>
          <div className="nav-profile-text">
            <div className="nav-profile-name">Гость</div>
            <div className="nav-profile-sub">Личное пространство</div>
          </div>
          <Icon src={icons.chevronsUpDown} size={13} className="muted" />
        </div>
      </div>
    </nav>
  )
}
