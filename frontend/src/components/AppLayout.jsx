import { Avatar, Badge, Button, Input, Text, makeStyles, mergeClasses, tokens } from '@fluentui/react-components'
import { Nav, NavCategory, NavCategoryItem, NavItem, NavSubItemGroup } from '@fluentui/react-nav'
import { useNavigate, useLocation } from 'react-router-dom'

import { navItems } from '../app/routes'

const useStyles = makeStyles({
  frame: {
    minHeight: '100vh',
    padding: '10px'
  },
  shell: {
    display: 'grid',
    gridTemplateColumns: '308px minmax(0, 1fr)',
    minHeight: 'calc(100vh - 20px)',
    overflow: 'hidden',
    borderRadius: '14px',
    border: `1px solid ${tokens.colorNeutralStroke2}`,
    background: `linear-gradient(180deg, ${tokens.colorNeutralBackground1} 0%, ${tokens.colorNeutralBackground1Hover} 100%)`,
    boxShadow: tokens.shadow64
  },
  rail: {
    display: 'flex',
    flexDirection: 'column',
    gap: '12px',
    padding: '14px 12px 16px',
    borderRight: `1px solid ${tokens.colorNeutralStroke2}`,
    background: `linear-gradient(180deg, ${tokens.colorNeutralBackground2} 0%, ${tokens.colorNeutralBackground3} 100%)`
  },
  railTop: {
    display: 'flex',
    alignItems: 'center',
    gap: '10px',
    minHeight: '34px'
  },
  profile: {
    display: 'flex',
    alignItems: 'center',
    gap: '14px',
    padding: '8px 4px 12px'
  },
  profileText: {
    display: 'grid',
    gap: '2px'
  },
  navSurface: {
    display: 'grid',
    gap: '4px',
    flex: 1,
    minHeight: 0
  },
  navRoot: {
    background: 'transparent'
  },
  navItemHeader: {
    borderRadius: '10px'
  },
  currentItem: {
    padding: '14px 12px',
    borderRadius: '16px',
    border: `1px solid ${tokens.colorNeutralStroke2}`,
    background: `linear-gradient(180deg, ${tokens.colorNeutralBackground3} 0%, ${tokens.colorNeutralBackground2} 100%)`
  },
  currentTitle: {
    display: 'flex',
    alignItems: 'center',
    gap: '10px',
    marginBottom: '4px'
  },
  currentGlyph: {
    display: 'grid',
    placeItems: 'center',
    width: '28px',
    height: '28px',
    borderRadius: '10px',
    background: tokens.colorBrandBackground2,
    color: tokens.colorBrandForeground1
  },
  footnote: {
    padding: '12px 8px 0',
    color: tokens.colorNeutralForeground3,
    lineHeight: '1.45'
  },
  content: {
    minWidth: 0,
    display: 'flex',
    flexDirection: 'column',
    padding: '10px 28px 24px'
  },
  topbar: {
    display: 'flex',
    justifyContent: 'space-between',
    gap: '16px',
    alignItems: 'center',
    padding: '2px 0 14px'
  },
  searchWrap: {
    width: 'min(560px, 100%)'
  },
  searchInput: {
    borderRadius: '999px'
  },
  statusBar: {
    display: 'flex',
    gap: '10px',
    alignItems: 'center',
    flexWrap: 'wrap'
  },
  main: {
    minWidth: 0
  },
  responsiveShell: {
    '@media (max-width: 1100px)': {
      gridTemplateColumns: '1fr'
    }
  },
  responsiveRail: {
    '@media (max-width: 1100px)': {
      borderRight: 0,
      borderBottom: `1px solid ${tokens.colorNeutralStroke2}`
    }
  },
  responsiveTopbar: {
    '@media (max-width: 840px)': {
      flexDirection: 'column',
      alignItems: 'flex-start'
    }
  }
})

export default function AppLayout({ bridgeMode, currentItem, children }) {
  const styles = useStyles()
  const navigate = useNavigate()
  const location = useLocation()

  return (
    <div className={styles.frame}>
      <div className={mergeClasses(styles.shell, styles.responsiveShell)}>
        <aside className={mergeClasses(styles.rail, styles.responsiveRail)}>
          <div className={styles.railTop}>
            <Button appearance="subtle" shape="circular" aria-label="Back">
              ←
            </Button>
            <Text weight="semibold">gutgd</Text>
          </div>

          <div className={styles.profile}>
            <Avatar name="gut graphical debugger" size={56} color="brand" />
            <div className={styles.profileText}>
              <Text as="div" weight="semibold" size={400}>gut graphical debugger</Text>
              <Text as="div" size={200} appearance="subtle">desktop verification harness</Text>
            </div>
          </div>

          <div className={styles.navSurface}>
            <Nav
              aria-label="gut feature sections"
              className={styles.navRoot}
              selectedValue={location.pathname}
              onNavItemSelect={(_, data) => {
                if (typeof data.value === 'string') {
                  navigate(data.value)
                }
              }}
            >
              <NavCategory value="features">
                <NavCategoryItem>
                  <Button appearance="transparent">Features</Button>
                </NavCategoryItem>
                <NavSubItemGroup>
                  {navItems.map((item) => (
                    <NavItem key={item.key} value={item.path}>
                      <div className={styles.navItemHeader}>
                        <span aria-hidden="true">{item.glyph}</span>
                        <span>{item.title}</span>
                      </div>
                    </NavItem>
                  ))}
                </NavSubItemGroup>
              </NavCategory>
            </Nav>

            <div className={styles.currentItem}>
              <div className={styles.currentTitle}>
                <span className={styles.currentGlyph} aria-hidden="true">{currentItem?.glyph || '⌘'}</span>
                <Text weight="semibold">{currentItem?.title || 'Diagnostics'}</Text>
              </div>
              <Text size={200} appearance="subtle">{currentItem?.description}</Text>
            </div>
          </div>

          <div className={styles.footnote}>
            This app drives the live <code>gut</code> APIs. Keyboard, mouse, clipboard, screen, and window actions affect the real host session.
          </div>
        </aside>

        <div className={styles.content}>
          <header className={mergeClasses(styles.topbar, styles.responsiveTopbar)}>
            <div className={styles.searchWrap}>
              <Input className={styles.searchInput} aria-label="Search" value="Find a setting" readOnly contentBefore="⌕" />
            </div>

            <div className={styles.statusBar}>
              <Badge color={bridgeMode === 'desktop' ? 'success' : bridgeMode === 'preview' ? 'warning' : 'danger'}>
                {bridgeMode === 'desktop' ? 'Desktop runtime connected' : bridgeMode === 'preview' ? 'Browser preview mode' : 'Desktop runtime unavailable'}
              </Badge>
            </div>
          </header>

          <main className={styles.main}>
            {children}
          </main>
        </div>
      </div>
    </div>
  )
}
