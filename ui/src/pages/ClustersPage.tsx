import { useState, useEffect } from 'react'
import { toast } from 'sonner'
import { RefreshCw, Zap, Trash2 } from 'lucide-react'
import { listClusters, prewarmCluster, clearCache } from '../api/adapter'
import type { ClusterInfo } from '../types'
import { useI18n } from '../i18n'

export default function ClustersPage() {
  const { t } = useI18n()
  const [clusters, setClusters] = useState<ClusterInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [actionCluster, setActionCluster] = useState<string | null>(null)

  async function load() {
    setLoading(true)
    try {
      const res = await listClusters()
      setClusters(res.clusters ?? [])
    } catch (err) {
      toast.error(String(err))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [])

  async function handlePrewarm(name: string) {
    setActionCluster(name)
    try {
      await prewarmCluster(name)
      toast.success(`"${name}" ${t.cached}`)
      await load()
    } catch (err) {
      toast.error(String(err))
    } finally {
      setActionCluster(null)
    }
  }

  async function handleClearCache() {
    try {
      await clearCache()
      toast.success(t.cacheCleared)
      void load()
    } catch (err) {
      toast.error(String(err))
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold dark:text-white">{t.clustersTitle}</h1>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
            {t.clustersDescription}
          </p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => { void load() }}
            disabled={loading}
            className="inline-flex items-center gap-1 rounded-md border dark:border-gray-600 px-3 py-1.5 text-sm
              dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800 disabled:opacity-50"
          >
            <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
            {t.refresh}
          </button>
          <button
            onClick={() => { void handleClearCache() }}
            className="inline-flex items-center gap-1 rounded-md border border-red-200 dark:border-red-800 px-3 py-1.5
              text-sm text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/30"
          >
            <Trash2 size={14} />
            {t.clearCache}
          </button>
        </div>
      </div>

      {clusters.length === 0 && !loading && (
        <div className="rounded-md border dark:border-gray-700 border-dashed p-8 text-center text-sm text-gray-400">
          {t.noClusters}
        </div>
      )}

      <div className="space-y-3">
        {clusters.map((cl) => (
          <div
            key={cl.name}
            className="flex items-center justify-between rounded-lg border dark:border-gray-700
              bg-white dark:bg-gray-800 px-4 py-3 shadow-sm"
          >
            <div className="flex items-center gap-3">
              <span
                className={`inline-block h-2.5 w-2.5 rounded-full ${
                  cl.cached ? 'bg-green-500' : 'bg-gray-300 dark:bg-gray-600'
                }`}
              />
              <div>
                <p className="font-medium text-sm dark:text-white">{cl.name}</p>
                <p className="text-xs text-gray-400">
                  {cl.cached ? t.cached : t.notCached}
                </p>
              </div>
            </div>
            <button
              onClick={() => { void handlePrewarm(cl.name) }}
              disabled={actionCluster === cl.name}
              className="inline-flex items-center gap-1 rounded-md border dark:border-gray-600 px-3 py-1.5
                text-xs dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50"
            >
              <Zap size={12} />
              {actionCluster === cl.name ? '...' : t.prewarm}
            </button>
          </div>
        ))}
      </div>
    </div>
  )
}
