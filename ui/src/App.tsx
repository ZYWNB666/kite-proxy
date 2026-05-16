import { useState, useEffect } from 'react'
import { BrowserRouter, Routes, Route, NavLink } from 'react-router-dom'
import { Toaster, toast } from 'sonner'
import { Settings, BookOpen } from 'lucide-react'
import ConfigPage from './pages/ConfigPage'
import UsagePage from './pages/UsagePage'
import { getConfig } from './api/adapter'
import type { Config } from './types'
import { I18nProvider, useI18n } from './i18n'
import { ThemeProvider } from './theme'

const isDesktop = typeof window !== 'undefined' && 'go' in window

function Nav({ configured }: { configured: boolean }) {
  const { t } = useI18n()
  const linkClass = ({ isActive }: { isActive: boolean }) =>
    `flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition-colors ${
      isActive
        ? 'bg-blue-50 dark:bg-blue-900/40 text-blue-700 dark:text-blue-400'
        : 'text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
    }`

  return (
    <nav className="space-y-1">
      <NavLink to="/ui/config" className={linkClass}>
        <Settings size={16} />
        {t.configuration}
      </NavLink>
      <NavLink to="/ui/usage" className={({ isActive }) =>
        linkClass({ isActive }) + (!configured ? ' opacity-40 pointer-events-none' : '')
      }>
        <BookOpen size={16} />
        {t.portForwarding}
      </NavLink>
    </nav>
  )
}

function AppInner() {
  const { t } = useI18n()
  const [config, setConfig] = useState<Config | null>(null)
  const [authError, setAuthError] = useState<{ code: string; message: string } | null>(null)

  async function loadConfig() {
    try {
      const cfg = await getConfig()
      setConfig(cfg)
    } catch {
      // not configured yet
    }
  }

  useEffect(() => {
    void loadConfig()
  }, [])

  // 监听 auth:unauthorized 事件
  useEffect(() => {
    if (!isDesktop) return
    // @ts-ignore
    const unlistenUnauthorized = window.runtime?.EventsOn?.('auth:unauthorized', () => {
      setAuthError({ code: 'unauthorized', message: t.authInvalidOrExpired })
      toast.error(t.authInvalidOrExpired, { duration: 0, id: 'auth-error' })
    })
    // @ts-ignore
    const unlistenError = window.runtime?.EventsOn?.('auth:error', (payload) => {
      const code = payload?.code || 'kite_unreachable'
      const message = mapAuthMessage(code, payload?.message, t)
      setAuthError({ code, message })
      toast.error(message, { duration: 0, id: 'auth-error' })
    })
    return () => {
      if (typeof unlistenUnauthorized === 'function') unlistenUnauthorized()
      if (typeof unlistenError === 'function') unlistenError()
    }
  }, [t])

  const configured = config?.configured ?? false

  if (authError) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-gray-950 flex items-center justify-center">
        <div className="text-center max-w-sm p-8 bg-white dark:bg-gray-900 rounded-xl shadow border border-gray-200 dark:border-gray-700">
          <div className="text-4xl mb-4">🔒</div>
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">
            {authError.message}
          </h2>
          <button
            onClick={() => {
              setAuthError(null)
              void loadConfig()
            }}
            className="mt-4 px-4 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-700"
          >
            {t.configuration}
          </button>
        </div>
      </div>
    )
  }

  return (
    <BrowserRouter>
      <Toaster richColors position="top-right" />
      <div className="h-screen w-screen overflow-hidden bg-gray-50 dark:bg-gray-950 flex flex-col">
        {/* Header */}
        <header className="bg-white dark:bg-gray-900 border-b dark:border-gray-700 px-6 py-3 flex items-center justify-between shadow-sm flex-shrink-0">
          <div className="flex items-center gap-3">
            <div className="font-bold text-lg text-blue-700 dark:text-blue-400 tracking-tight">kite-proxy</div>
          </div>
          <div className="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
            <span
              className={`inline-block h-2 w-2 rounded-full ${
                configured ? 'bg-green-500' : 'bg-gray-300 dark:bg-gray-600'
              }`}
            />
            {configured ? t.connected : t.notConfigured}
          </div>
        </header>

        <div className="flex flex-1 overflow-hidden">
          {/* Sidebar */}
          <aside className="w-52 bg-white dark:bg-gray-900 border-r dark:border-gray-700 px-4 py-6 hidden sm:block">
            <Nav configured={configured} />
          </aside>

          {/* Main content */}
          <main className="flex-1 flex flex-col min-w-0 overflow-hidden dark:text-white">
            <Routes>
              <Route path="/" element={<ConfigPage config={config} onSaved={() => { void loadConfig() }} />} />
              <Route path="/ui" element={<ConfigPage config={config} onSaved={() => { void loadConfig() }} />} />
              <Route path="/ui/" element={<ConfigPage config={config} onSaved={() => { void loadConfig() }} />} />
              <Route path="/ui/config" element={<ConfigPage config={config} onSaved={() => { void loadConfig() }} />} />
              <Route path="/ui/usage" element={<UsagePage />} />
            </Routes>
          </main>
        </div>
      </div>
    </BrowserRouter>
  )
}

function mapAuthMessage(
  code: string,
  rawMessage: string | undefined,
  t: ReturnType<typeof useI18n>['t']
) {
  switch (code) {
    case 'unauthorized':
      return t.authInvalidOrExpired
    case 'proxy_forbidden':
      return t.proxyForbidden
    case 'namespace_forbidden':
      return t.namespaceForbidden
    case 'kite_unreachable':
      return rawMessage || t.kiteUnreachable
    default:
      return rawMessage || t.kiteUnreachable
  }
}

export default function App() {
  return (
    <ThemeProvider>
      <I18nProvider>
        <AppInner />
      </I18nProvider>
    </ThemeProvider>
  )
}
