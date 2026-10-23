import { useState } from 'react'
import { useGraphs } from '../graphs'
import { Icon } from './Icon'
import { icons } from '../icons'
import { linkClick } from '../router'

type Section = 'home' | 'graphs'

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
            <span className="nav-label">связь</span>
          </a>
          <button className="nav-burger" onClick={toggle} aria-label={collapsed ? 'Развернуть меню' : 'Свернуть меню'}>
            <Icon src={icons.hamburger} size={20} />
          </button>
        </div>

        <div className="nav-items">
          <a className={active === 'home' ? 'nav-item active' : 'nav-item'} href="/" onClick={linkClick('/')}>
            <Icon src={active === 'home' ? icons.houseActive : icons.house} size={19} />
            <span className="nav-label">Главная</span>
          </a>
          <a className={active === 'graphs' ? 'nav-item active' : 'nav-item'} href="/graphs" onClick={linkClick('/graphs')}>
            <Icon src={active === 'graphs' ? icons.networkActive : icons.network} size={19} />
            <span className="nav-label">Мои графы</span>
            {graphs && <span className="nav-count nav-label">{graphs.length}</span>}
          </a>
          <span className="nav-item disabled" title="Настройки появятся позже" aria-disabled="true">
            <Icon src={icons.settings} size={19} />
            <span className="nav-label">Настройки</span>
          </span>
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
          <Icon src={icons.sparkles} size={19} />
          <p className="nav-note-quote">Понимание начинается с хорошего вопроса.</p>
          <p className="nav-note-caption">Находите связи каждый день</p>
        </div>
        <div className="nav-profile">
          <div className="avatar">Г</div>
          <div className="nav-profile-text">
            <div className="nav-profile-name">Гость</div>
            <div className="nav-profile-sub">Личное пространство</div>
          </div>
          <Icon src={icons.chevronsUpDown} size={13} />
        </div>
      </div>
    </nav>
  )
}
