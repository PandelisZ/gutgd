import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { HashRouter } from 'react-router-dom'

import AppLayout from '../components/AppLayout'
import { navItems } from './routes'
import { useBridgeMode } from '../lib/wails'
import ClipboardView from '../views/ClipboardView'
import AgentView from '../views/AgentView'
import AgentSettingsView from '../views/AgentSettingsView'
import DiagnosticsView from '../views/DiagnosticsView'
import KeyboardView from '../views/KeyboardView'
import MouseView from '../views/MouseView'
import ScreenView from '../views/ScreenView'
import SearchView from '../views/SearchView'
import WindowsView from '../views/WindowsView'

function RoutedApp() {
  const bridgeMode = useBridgeMode()
  const location = useLocation()
  const currentItem = navItems.find((item) => item.path === location.pathname) || navItems[0]

  return (
    <AppLayout bridgeMode={bridgeMode} currentItem={currentItem}>
      <Routes>
        <Route path="/" element={<Navigate to="/diagnostics" replace />} />
        <Route path="/diagnostics" element={<DiagnosticsView bridgeMode={bridgeMode} />} />
        <Route path="/keyboard" element={<KeyboardView />} />
        <Route path="/mouse" element={<MouseView />} />
        <Route path="/screen" element={<ScreenView />} />
        <Route path="/windows" element={<WindowsView />} />
        <Route path="/search" element={<SearchView />} />
        <Route path="/clipboard" element={<ClipboardView />} />
        <Route path="/agent" element={<AgentView />} />
        <Route path="/agent-settings" element={<AgentSettingsView />} />
      </Routes>
    </AppLayout>
  )
}

export default function App() {
  return (
    <HashRouter>
      <RoutedApp />
    </HashRouter>
  )
}
