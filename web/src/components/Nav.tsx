import { useState } from 'react'
import { useGraphs } from '../graphs'
import { Icon } from './Icon'
import { icons } from '../icons'
import { linkClick } from '../router'
import { ProfileMenu } from './ProfileMenu'

type Section = 'new' | 'graphs'

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
            <span className="nav-label">Физграф</span>
          </a>
          <button className="nav-burger" onClick={toggle} aria-label={collapsed ? 'Развернуть меню' : 'Свернуть меню'}>
            <Icon src={icons.hamburger} size={20} />
          </button>
        </div>

        <div className="nav-items">
          <a className="nav-item" href="/" onClick={linkClick('/')}>
            <Icon src={icons.house} size={19} />
            <span className="nav-label">Главная</span>
          </a>
          <a className={active === 'new' ? 'nav-item active' : 'nav-item'} href="/new" onClick={linkClick('/new')}>
            <Icon src={icons.fileUp} size={19} />
            <span className="nav-label">Новый граф</span>
          </a>
          <a className={active === 'graphs' ? 'nav-item active' : 'nav-item'} href="/graphs" onClick={linkClick('/graphs')}>
            <Icon src={icons.network} size={19} />
            <span className="nav-label">Мои графы</span>
            {graphs && <span className="nav-count nav-label">{graphs.length}</span>}
          </a>
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

      <div className="nav-bottom">
        <div className="nav-note nav-label">
          <Icon src={icons.sparkles} size={19} className="accent" />
          <p className="nav-note-quote">Понимание начинается с хорошего вопроса.</p>
          <p className="nav-note-caption">Находите связи каждый день</p>
        </div>
        <ProfileMenu />
      </div>
    </nav>
  )
}
