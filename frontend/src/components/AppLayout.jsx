import { Avatar, Badge, Input, Text, makeStyles, mergeClasses, tokens } from '@fluentui/react-components'
import { Nav, NavItem } from '@fluentui/react-nav'
import { useNavigate, useLocation } from 'react-router-dom'

import { navItems } from '../app/routes'

const useStyles = makeStyles({
  frame: {
    height: '100vh',
    width: '100vw',
    overflow: 'hidden'
  },
  shell: {
    display: 'grid',
    gridTemplateColumns: '286px minmax(0, 1fr)',
    height: '100vh',
    overflow: 'hidden',
    minHeight: 0,
    background: `linear-gradient(180deg, ${tokens.colorNeutralBackground1} 0%, ${tokens.colorNeutralBackground1Hover} 100%)`,
    boxShadow: tokens.shadow4
  },
  rail: {
    display: 'flex',
    flexDirection: 'column',
    gap: '14px',
    padding: '20px 14px 16px',
    minHeight: 0,
    overflow: 'auto',
    borderRight: `1px solid ${tokens.colorNeutralStroke2}`,
    background: `linear-gradient(180deg, ${tokens.colorNeutralBackground2} 0%, rgba(255,255,255,0.01) 100%)`
  },
  railHeader: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'flex-start',
    gap: '14px',
    padding: '0 8px'
  },
  profile: {
    display: 'flex',
    alignItems: 'flex-start',
    gap: '14px',
    width: '100%'
  },
  profileText: {
    display: 'grid',
    gap: '4px',
    minWidth: 0
  },
  appEyebrow: {
    color: tokens.colorNeutralForeground3,
    textTransform: 'uppercase',
    letterSpacing: '0.08em'
  },
  navSurface: {
    display: 'grid',
    gap: '8px',
    flex: 1,
    minHeight: 0
  },
  navLabel: {
    padding: '0 12px',
    color: tokens.colorNeutralForeground3
  },
  navRoot: {
    background: 'transparent',
    minWidth: 0
  },
  navItem: {
    display: 'grid',
    gridTemplateColumns: '18px minmax(0, 1fr)',
    gap: '14px',
    alignItems: 'center',
    width: '100%',
    minWidth: 0,
    minHeight: '24px'
  },
  navGlyph: {
    color: tokens.colorBrandForeground1,
    textAlign: 'center'
  },
  navCopy: {
    minWidth: 0
  },
  navTitle: {
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    lineHeight: '1.25'
  },
  footnote: {
    padding: '12px 14px',
    borderRadius: '16px',
    border: `1px solid ${tokens.colorNeutralStroke2}`,
    background: `linear-gradient(180deg, rgba(255,255,255,0.03) 0%, rgba(255,255,255,0.01) 100%)`,
    color: tokens.colorNeutralForeground3,
    lineHeight: '1.45'
  },
  content: {
    minWidth: 0,
    minHeight: 0,
    display: 'flex',
    flexDirection: 'column',
    padding: '18px 28px 24px'
  },
  topbar: {
    display: 'flex',
    justifyContent: 'space-between',
    gap: '16px',
    alignItems: 'center',
    minHeight: '44px',
    padding: '0 0 14px'
  },
  topbarLeading: {
    minWidth: 0,
    display: 'flex',
    alignItems: 'center',
    gap: '12px'
  },
  topbarTitle: {
    color: tokens.colorNeutralForeground3
  },
  searchWrap: {
    width: 'min(480px, 100%)',
    marginLeft: 'auto',
    marginRight: 'auto'
  },
  searchInput: {
    borderRadius: '999px',
    backgroundColor: 'rgba(255,255,255,0.04)'
  },
  statusBar: {
    display: 'flex',
    gap: '10px',
    alignItems: 'center',
    flexWrap: 'wrap'
  },
  main: {
    minWidth: 0,
    minHeight: 0,
    overflow: 'auto',
    paddingRight: '6px'
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
      alignItems: 'stretch'
    }
  },
  responsiveSearchWrap: {
    '@media (max-width: 840px)': {
      width: '100%',
      marginLeft: 0,
      marginRight: 0
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
          <div className={styles.railHeader}>
            <Text size={100} className={styles.appEyebrow}>Navigation</Text>
            <div className={styles.profile}>
              <Avatar name="gut graphical debugger" size={48} color="brand" />
              <div className={styles.profileText}>
                <Text as="div" weight="semibold" size={400}>gutgd</Text>
                <Text as="div" size={300}>gut graphical debugger</Text>
                <Text as="div" size={200} appearance="subtle">Desktop verification harness</Text>
              </div>
            </div>
          </div>

          <div className={styles.navSurface}>
            <Text size={200} weight="semibold" className={styles.navLabel}>Main areas</Text>
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
              {navItems.map((item) => (
                <NavItem key={item.key} value={item.path}>
                  <div className={styles.navItem}>
                    <span className={styles.navGlyph} aria-hidden="true">{item.glyph}</span>
                    <span className={styles.navCopy}>
                      <Text className={styles.navTitle}>{item.title}</Text>
                    </span>
                  </div>
                </NavItem>
              ))}
            </Nav>
          </div>

          <div className={styles.footnote}>
            This app drives the live <code>gut</code> APIs. Keyboard, mouse, clipboard, screen, and window actions affect the real host session.
          </div>
        </aside>

        <div className={styles.content}>
          <header className={mergeClasses(styles.topbar, styles.responsiveTopbar)}>
            <div className={styles.topbarLeading}>
              <Text size={200} className={styles.topbarTitle}>Settings-style debugger</Text>
            </div>

            <div className={mergeClasses(styles.searchWrap, styles.responsiveSearchWrap)}>
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
