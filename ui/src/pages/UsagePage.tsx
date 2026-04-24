import { useState, useEffect } from 'react'
import { toast } from 'sonner'
import { Plus, Play, Square, Trash2, RefreshCw, Copy, Check, Loader2, ExternalLink } from 'lucide-react'
import { useI18n } from '../i18n'
import ConfirmDialog from '../components/ConfirmDialog'

// 端口映射信息
interface PortMapping {
  id: string
  cluster: string
  namespace: string
  resourceType: string
  resourceName: string
  remotePort: number
  localPort: number
  status: string
  error: string
  createdAt: string
}

// Namespace 信息
interface NamespaceInfo {
  name: string
}

// Service 信息
interface ServiceInfo {
  name: string
  namespace: string
  ports: Array<{ name: string; port: number; protocol: string }>
  type: string
}

// Pod 信息
interface PodInfo {
  name: string
  namespace: string
  ports: Array<{ name: string; port: number; protocol: string }>
  status: string
}

export default function UsagePage() {
  const { t } = useI18n()
  // 表单状态
  const [selectedCluster, setSelectedCluster] = useState('')
  const [selectedNamespace, setSelectedNamespace] = useState('')
  const [resourceType, setResourceType] = useState<'service' | 'pod'>('service')
  const [selectedResource, setSelectedResource] = useState('')
  const [selectedRemotePort, setSelectedRemotePort] = useState<number | ''>('')
  const [localPort, setLocalPort] = useState<number | ''>('')

  // 数据列表
  const [clusters, setClusters] = useState<Array<{ name: string }>>([])
  const [namespaces, setNamespaces] = useState<NamespaceInfo[]>([])
  const [services, setServices] = useState<ServiceInfo[]>([])
  const [pods, setPods] = useState<PodInfo[]>([])
  const [mappings, setMappings] = useState<PortMapping[]>([])

  // 加载状态
  const [loadingClusters, setLoadingClusters] = useState(false)
  const [loadingNamespaces, setLoadingNamespaces] = useState(false)
  const [loadingResources, setLoadingResources] = useState(false)
  const [loadingMappings, setLoadingMappings] = useState(false)
  const [addingMapping, setAddingMapping] = useState(false)
  const [copiedId, setCopiedId] = useState<string | null>(null)
  const [confirmRemoveId, setConfirmRemoveId] = useState<string | null>(null)

  // 检测是否在桌面环境
  const isDesktop = typeof window !== 'undefined' && 'go' in window

  // 加载集群列表
  useEffect(() => {
    if (!isDesktop) return
    void loadClusters()
    void loadMappings()
  }, [])

  // 当选择集群后，加载 namespace 列表
  useEffect(() => {
    if (selectedCluster && isDesktop) {
      void loadNamespaces()
    } else {
      setNamespaces([])
      setSelectedNamespace('')
    }
  }, [selectedCluster])

  // 当选择 namespace 后，加载资源列表
  useEffect(() => {
    if (selectedCluster && selectedNamespace && isDesktop) {
      void loadResources()
    } else {
      setServices([])
      setPods([])
      setSelectedResource('')
    }
  }, [selectedCluster, selectedNamespace, resourceType])

  async function loadClusters() {
    if (!isDesktop) return
    setLoadingClusters(true)
    try {
      // @ts-ignore
      const result = await window.go.desktop.App.ListClusters()
      setClusters(result || [])
    } catch (err) {
      toast.error(`${t.failedToLoad}: ${String(err)}`)
    } finally {
      setLoadingClusters(false)
    }
  }

  async function loadNamespaces() {
    if (!isDesktop || !selectedCluster) return
    setLoadingNamespaces(true)
    try {
      // @ts-ignore
      const result = await window.go.desktop.App.GetNamespaces(selectedCluster)
      setNamespaces(result || [])
    } catch (err) {
      toast.error(`${t.failedToLoad}: ${String(err)}`)
    } finally {
      setLoadingNamespaces(false)
    }
  }

  async function loadResources() {
    if (!isDesktop || !selectedCluster || !selectedNamespace) return
    setLoadingResources(true)
    try {
      if (resourceType === 'service') {
        // @ts-ignore
        const result = await window.go.desktop.App.GetServices(selectedCluster, selectedNamespace)
        setServices(result || [])
        setPods([])
      } else {
        // @ts-ignore
        const result = await window.go.desktop.App.GetPods(selectedCluster, selectedNamespace)
        setPods(result || [])
        setServices([])
      }
    } catch (err) {
      toast.error(`${t.failedToLoad}: ${String(err)}`)
    } finally {
      setLoadingResources(false)
    }
  }

  async function loadMappings() {
    if (!isDesktop) return
    setLoadingMappings(true)
    try {
      // @ts-ignore
      const result = await window.go.desktop.App.ListPortMappings()
      setMappings(result || [])
    } catch (err) {
      toast.error(`${t.failedToLoad}: ${String(err)}`)
    } finally {
      setLoadingMappings(false)
    }
  }

  async function handleAddMapping() {
    if (!isDesktop) return
    if (!selectedCluster || !selectedNamespace || !selectedResource || !selectedRemotePort) {
      toast.error(t.fillAllFields)
      return
    }

    setAddingMapping(true)
    try {
      // @ts-ignore
      await window.go.desktop.App.AddPortMapping(
        selectedCluster,
        selectedNamespace,
        resourceType,
        selectedResource,
        selectedRemotePort,
        localPort || 0
      )
      toast.success(t.mappingAdded)
      void loadMappings()
      setSelectedResource('')
      setSelectedRemotePort('')
      setLocalPort('')
    } catch (err) {
      toast.error(`${t.failedToAdd}: ${String(err)}`)
    } finally {
      setAddingMapping(false)
    }
  }

  async function handleCopyURL(mapping: PortMapping) {
    const url = `http://localhost:${mapping.localPort}`
    try {
      await navigator.clipboard.writeText(url)
      setCopiedId(mapping.id)
      setTimeout(() => setCopiedId(null), 2000)
    } catch {
      toast.error(t.failedToCopy)
    }
  }

  function handleOpenBrowser(url: string) {
    if (isDesktop) {
      // @ts-ignore
      void window.go.desktop.App.OpenBrowser(url)
    } else {
      window.open(url, '_blank', 'noreferrer')
    }
  }

  async function handleStartMapping(id: string) {
    if (!isDesktop) return
    try {
      // @ts-ignore
      await window.go.desktop.App.StartPortMapping(id)
      toast.success(t.mappingStarted)
      void loadMappings()
    } catch (err) {
      toast.error(`${t.failedToStart}: ${String(err)}`)
    }
  }

  async function handleStopMapping(id: string) {
    if (!isDesktop) return
    try {
      // @ts-ignore
      await window.go.desktop.App.StopPortMapping(id)
      toast.success(t.mappingStopped)
      void loadMappings()
    } catch (err) {
      toast.error(`${t.failedToStop}: ${String(err)}`)
    }
  }

  async function handleRemoveMapping(id: string) {
    if (!isDesktop) return
    try {
      // @ts-ignore
      await window.go.desktop.App.RemovePortMapping(id)
      toast.success(t.mappingRemoved)
      void loadMappings()
    } catch (err) {
      toast.error(`${t.failedToRemove}: ${String(err)}`)
    }
  }

  // 获取资源列表
  const resources = resourceType === 'service' ? services : pods

  // 获取选中资源的端口列表
  const selectedResourcePorts = resources.find(r => r.name === selectedResource)?.ports || []

  if (!isDesktop) {
    return (
      <div className="p-8 text-center">
        <p className="text-gray-500">{t.desktopOnly}</p>
      </div>
    )
  }

  return (
    <div className="p-6">
      <ConfirmDialog
        open={confirmRemoveId !== null}
        message={t.confirmRemove}
        danger
        onConfirm={() => {
          if (confirmRemoveId) void handleRemoveMapping(confirmRemoveId)
          setConfirmRemoveId(null)
        }}
        onCancel={() => setConfirmRemoveId(null)}
      />

      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold dark:text-white">{t.portForwarding}</h1>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
            {t.portForwardingDescription}
          </p>
        </div>
        <button
          onClick={() => void loadMappings()}
          disabled={loadingMappings}
          className="flex items-center gap-2 px-3 py-2 text-sm border dark:border-gray-600
            dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-800 disabled:opacity-50"
        >
          <RefreshCw size={16} className={loadingMappings ? 'animate-spin' : ''} />
          {t.refresh}
        </button>
      </div>

      {/* 添加端口映射表单 */}
      <div className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-lg p-6 mb-6">
        <h2 className="text-lg font-semibold mb-4 flex items-center gap-2 dark:text-white">
          <Plus size={18} />
          {t.addMapping}
        </h2>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {/* 集群选择 */}
          <div>
            <label className="block text-sm font-medium mb-1 dark:text-gray-200">{t.cluster} *</label>
            <select
              value={selectedCluster}
              onChange={(e) => setSelectedCluster(e.target.value)}
              disabled={loadingClusters}
              className="w-full px-3 py-2 border dark:border-gray-600 rounded-md bg-white dark:bg-gray-700
                text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50"
            >
              <option value="">{t.selectCluster}</option>
              {clusters.map(c => (
                <option key={c.name} value={c.name}>{c.name}</option>
              ))}
            </select>
          </div>

          {/* Namespace 选择 */}
          <div>
            <label className="block text-sm font-medium mb-1 dark:text-gray-200">{t.namespace} *</label>
            <select
              value={selectedNamespace}
              onChange={(e) => setSelectedNamespace(e.target.value)}
              disabled={!selectedCluster || loadingNamespaces}
              className="w-full px-3 py-2 border dark:border-gray-600 rounded-md bg-white dark:bg-gray-700
                text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50"
            >
              <option value="">{t.selectNamespace}</option>
              {namespaces.map(ns => (
                <option key={ns.name} value={ns.name}>{ns.name}</option>
              ))}
            </select>
          </div>

          {/* 资源类型 */}
          <div>
            <label className="block text-sm font-medium mb-1 dark:text-gray-200">{t.resourceType} *</label>
            <select
              value={resourceType}
              onChange={(e) => setResourceType(e.target.value as 'service' | 'pod')}
              disabled={!selectedNamespace}
              className="w-full px-3 py-2 border dark:border-gray-600 rounded-md bg-white dark:bg-gray-700
                text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50"
            >
              <option value="service">Service</option>
              <option value="pod">Pod</option>
            </select>
          </div>

          {/* 资源选择 */}
          <div>
            <label className="block text-sm font-medium mb-1 dark:text-gray-200">
              {resourceType === 'service' ? 'Service' : 'Pod'} *
            </label>
            <select
              value={selectedResource}
              onChange={(e) => {
                setSelectedResource(e.target.value)
                setSelectedRemotePort('')
              }}
              disabled={!selectedNamespace || loadingResources}
              className="w-full px-3 py-2 border dark:border-gray-600 rounded-md bg-white dark:bg-gray-700
                text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50"
            >
              <option value="">{t.selectResource}</option>
              {resources.map(r => (
                <option key={r.name} value={r.name}>
                  {r.name}
                  {resourceType === 'pod' && ` (${(r as PodInfo).status})`}
                </option>
              ))}
            </select>
          </div>

          {/* 远程端口 */}
          <div>
            <label className="block text-sm font-medium mb-1 dark:text-gray-200">{t.remotePort} *</label>
            <select
              value={selectedRemotePort}
              onChange={(e) => setSelectedRemotePort(Number(e.target.value))}
              disabled={!selectedResource}
              className="w-full px-3 py-2 border dark:border-gray-600 rounded-md bg-white dark:bg-gray-700
                text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50"
            >
              <option value="">{t.selectPort}</option>
              {selectedResourcePorts.map((p, idx) => (
                <option key={idx} value={p.port}>
                  {p.port} {p.name && `(${p.name})`} - {p.protocol}
                </option>
              ))}
            </select>
          </div>

          {/* 本地端口 */}
          <div>
            <label className="block text-sm font-medium mb-1 dark:text-gray-200">{t.localPort}</label>
            <input
              type="number"
              value={localPort}
              onChange={(e) => setLocalPort(e.target.value ? Number(e.target.value) : '')}
              placeholder={t.randomPort}
              min="1024"
              max="65535"
              className="w-full px-3 py-2 border dark:border-gray-600 rounded-md bg-white dark:bg-gray-700
                text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
        </div>

        <div className="mt-4">
          <button
            onClick={() => void handleAddMapping()}
            disabled={!selectedCluster || !selectedNamespace || !selectedResource || !selectedRemotePort || addingMapping}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-md
              hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {addingMapping ? (
              <Loader2 size={16} className="animate-spin" />
            ) : (
              <Plus size={16} />
            )}
            {addingMapping ? t.connecting : t.addMapping}
          </button>
        </div>
      </div>

      {/* 端口映射列表 */}
      <div className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-lg overflow-hidden">
        <div className="px-6 py-4 border-b dark:border-gray-700 bg-gray-50 dark:bg-gray-900/50">
          <h2 className="text-lg font-semibold dark:text-white">{t.activeMappings}</h2>
        </div>

        {mappings.length === 0 ? (
          <div className="p-8 text-center text-gray-500 dark:text-gray-400">
            {t.noMappings}
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead className="bg-gray-50 dark:bg-gray-900/50 border-b dark:border-gray-700">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">{t.resource}</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">{t.localURL}</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">{t.remotePort}</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">{t.status}</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">{t.actions}</th>
                </tr>
              </thead>
              <tbody className="divide-y dark:divide-gray-700">
                {mappings.map(mapping => (
                  <tr key={mapping.id} className="hover:bg-gray-50 dark:hover:bg-gray-700/50">
                    <td className="px-6 py-4">
                      <div className="text-sm font-medium dark:text-white">{mapping.resourceType}/{mapping.resourceName}</div>
                      <div className="text-xs text-gray-500 dark:text-gray-400">{mapping.cluster} / {mapping.namespace}</div>
                    </td>
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2">
                        <span className="font-mono text-sm dark:text-gray-200">http://localhost:{mapping.localPort}</span>
                        <button
                          onClick={() => void handleCopyURL(mapping)}
                          className="p-1 text-gray-400 hover:text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/30 rounded transition-colors"
                          title={t.copyURL}
                        >
                          {copiedId === mapping.id ? <Check size={14} className="text-green-600" /> : <Copy size={14} />}
                        </button>
                        <button
                          onClick={() => handleOpenBrowser(`http://localhost:${mapping.localPort}`)}
                          className="p-1 text-gray-400 hover:text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/30 rounded transition-colors"
                          title={t.openInBrowser}
                        >
                          <ExternalLink size={14} />
                        </button>
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      <span className="font-mono text-sm dark:text-gray-200">{mapping.remotePort}</span>
                    </td>
                    <td className="px-6 py-4">
                      <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                        mapping.status === 'running' ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400' :
                        mapping.status === 'stopped' ? 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300' :
                        'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400'
                      }`}>
                        {mapping.status}
                      </span>
                      {mapping.error && (
                        <div className="text-xs text-red-600 dark:text-red-400 mt-1">{mapping.error}</div>
                      )}
                    </td>
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2">
                        {mapping.status === 'running' ? (
                          <button
                            onClick={() => void handleStopMapping(mapping.id)}
                            className="p-1.5 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-gray-700 rounded"
                            title={t.stop}
                          >
                            <Square size={16} />
                          </button>
                        ) : (
                          <button
                            onClick={() => void handleStartMapping(mapping.id)}
                            className="p-1.5 text-green-600 dark:text-green-400 hover:text-green-900 dark:hover:text-green-300 hover:bg-green-50 dark:hover:bg-green-900/30 rounded"
                            title={t.start}
                          >
                            <Play size={16} />
                          </button>
                        )}
                        <button
                          onClick={() => setConfirmRemoveId(mapping.id)}
                          className="p-1.5 text-red-600 dark:text-red-400 hover:text-red-900 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/30 rounded"
                          title={t.remove}
                        >
                          <Trash2 size={16} />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
