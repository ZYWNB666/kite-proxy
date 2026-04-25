import { useState } from 'react'
import { toast } from 'sonner'
import { Save, Eye, EyeOff } from 'lucide-react'
import { saveConfig, listClusters } from '../api/adapter'
import type { Config } from '../types'
import { useI18n, type Language } from '../i18n'
import { useTheme, type Theme } from '../theme'

interface Props {
  config: Config | null
  onSaved: () => void
}

const isDesktop = typeof window !== 'undefined' && 'go' in window

export default function ConfigPage({ config, onSaved }: Props) {
  const { t, lang, setLang } = useI18n()
  const { theme, setTheme } = useTheme()
  const [kiteURL, setKiteURL] = useState(config?.kiteURL ?? '')
  const [apiKey, setApiKey] = useState('')
  const [showKey, setShowKey] = useState(false)
  const [saving, setSaving] = useState(false)

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    if (!kiteURL.trim() || !apiKey.trim()) {
      toast.error(t.fillAllFields)
      return
    }
    setSaving(true)
    try {
      await saveConfig(kiteURL.trim(), apiKey.trim())
      toast.success(t.configSaved)
      onSaved()
    } catch (err) {
      toast.error(String(err))
    } finally {
      setSaving(false)
    }
  }

  async function handleTest() {
    if (!kiteURL.trim() || !apiKey.trim()) {
      toast.error(t.fillAllFields)
      return
    }
    setSaving(true)
    try {
      await saveConfig(kiteURL.trim(), apiKey.trim())
      const result = await listClusters()
      toast.success(`${t.connectionSuccessful} – ${result.clusters.length} cluster(s)`)
      onSaved()
    } catch (err) {
      toast.error(`${t.connectionFailed}: ${String(err)}`)
    } finally {
      setSaving(false)
    }
  }

  async function handleSavePrefs(newLang: Language, newTheme: Theme) {
    setLang(newLang)
    setTheme(newTheme)
    if (isDesktop) {
      try {
        // @ts-ignore
        await window.go.desktop.App.SetUIPrefs(newLang, newTheme)
        toast.success(t.prefsSaved)
      } catch (err) {
        toast.error(String(err))
      }
    }
  }

  return (
    <div className="flex-1 overflow-y-auto p-4 lg:p-8">
      <div className="max-w-xl mx-auto">
        <h1 className="text-2xl font-bold mb-2 dark:text-white">{t.configTitle}</h1>
      <p className="text-gray-500 dark:text-gray-400 mb-6 text-sm">{t.configDescription}</p>

      {config?.configured && (
        <div className="mb-4 rounded-md bg-green-50 dark:bg-green-900/30 border border-green-200 dark:border-green-700 px-4 py-3 text-sm text-green-700 dark:text-green-400 flex items-center gap-2">
          <span className="inline-block h-2 w-2 rounded-full bg-green-500" />
          {t.connectedTo} <strong>{config.kiteURL}</strong>
        </div>
      )}

      <form onSubmit={(e) => { void handleSave(e) }} className="space-y-4 mb-8">
        <div>
          <label className="block text-sm font-medium mb-1 dark:text-gray-200">{t.kiteServerURL}</label>
          <input
            type="url"
            required
            placeholder="https://kite.example.com"
            value={kiteURL}
            onChange={(e) => setKiteURL(e.target.value)}
            className="w-full rounded-md border dark:border-gray-600 px-3 py-2 text-sm shadow-sm
              bg-white dark:bg-gray-800 text-gray-900 dark:text-white
              focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>

        <div>
          <label className="block text-sm font-medium mb-1 dark:text-gray-200">{t.apiKey}</label>
          <div className="relative">
            <input
              type={showKey ? 'text' : 'password'}
              required
              placeholder={config?.apiKeyMasked ?? '••••••••'}
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              className="w-full rounded-md border dark:border-gray-600 px-3 py-2 pr-10 text-sm shadow-sm
                bg-white dark:bg-gray-800 text-gray-900 dark:text-white
                focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <button
              type="button"
              onClick={() => setShowKey(!showKey)}
              className="absolute right-2 top-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
            >
              {showKey ? <EyeOff size={16} /> : <Eye size={16} />}
            </button>
          </div>
        </div>

        <div className="flex gap-3">
          <button
            type="submit"
            disabled={saving}
            className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white shadow hover:bg-blue-700 disabled:opacity-50"
          >
            <Save size={16} />
            {saving ? t.saving : t.save}
          </button>
          <button
            type="button"
            onClick={() => { void handleTest() }}
            disabled={saving}
            className="inline-flex items-center gap-2 rounded-md border dark:border-gray-600 px-4 py-2 text-sm font-medium
              dark:text-gray-200 shadow hover:bg-gray-50 dark:hover:bg-gray-800 disabled:opacity-50"
          >
            {t.testConnection}
          </button>
        </div>
      </form>

      {/* 外观设置 */}
      <div className="border-t dark:border-gray-700 pt-6">
        <h2 className="text-lg font-semibold mb-4 dark:text-white">{t.appearance}</h2>
        <div className="grid grid-cols-2 gap-6">
          <div>
            <label className="block text-sm font-medium mb-2 dark:text-gray-200">{t.language}</label>
            <div className="flex gap-2">
              {(['en', 'zh'] as Language[]).map((l) => (
                <button
                  key={l}
                  onClick={() => { void handleSavePrefs(l, theme) }}
                  className={`px-3 py-1.5 text-sm rounded-md border transition-colors ${
                    lang === l
                      ? 'bg-blue-600 text-white border-blue-600'
                      : 'border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800'
                  }`}
                >
                  {l === 'en' ? t.english : t.chinese}
                </button>
              ))}
            </div>
          </div>
          <div>
            <label className="block text-sm font-medium mb-2 dark:text-gray-200">{t.theme}</label>
            <div className="flex gap-2">
              {(['light', 'dark'] as Theme[]).map((th) => (
                <button
                  key={th}
                  onClick={() => { void handleSavePrefs(lang, th) }}
                  className={`px-3 py-1.5 text-sm rounded-md border transition-colors ${
                    theme === th
                      ? 'bg-blue-600 text-white border-blue-600'
                      : 'border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800'
                  }`}
                >
                  {th === 'light' ? t.lightTheme : t.darkTheme}
                </button>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
    </div>
  )
}
