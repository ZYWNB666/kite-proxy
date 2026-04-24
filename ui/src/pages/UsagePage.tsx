import { useState, useEffect } from 'react'
import { toast } from 'sonner'
import { Plus, Play, Square, Trash2, RefreshCw } from 'lucide-react'

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
      toast.error(`Failed to load clusters: ${String(err)}`)
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
      toast.error(`Failed to load namespaces: ${String(err)}`)
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
      toast.error(`Failed to load resources: ${String(err)}`)
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
      toast.error(`Failed to load mappings: ${String(err)}`)
    } finally {
      setLoadingMappings(false)
    }
  }

  async function handleAddMapping() {
    if (!isDesktop) return
    if (!selectedCluster || !selectedNamespace || !selectedResource || !selectedRemotePort) {
      toast.error('Please fill all required fields')
      return
    }

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
      toast.success('Port mapping added successfully')
      void loadMappings()
      // 重置表单
      setSelectedResource('')
      setSelectedRemotePort('')
      setLocalPort('')
    } catch (err) {
      toast.error(`Failed to add mapping: ${String(err)}`)
    }
  }

  async function handleStartMapping(id: string) {
    if (!isDesktop) return
    try {
      // @ts-ignore
      await window.go.desktop.App.StartPortMapping(id)
      toast.success('Port mapping started')
      void loadMappings()
    } catch (err) {
      toast.error(`Failed to start mapping: ${String(err)}`)
    }
  }

  async function handleStopMapping(id: string) {
    if (!isDesktop) return
    try {
      // @ts-ignore
      await window.go.desktop.App.StopPortMapping(id)
      toast.success('Port mapping stopped')
      void loadMappings()
    } catch (err) {
      toast.error(`Failed to stop mapping: ${String(err)}`)
    }
  }

  async function handleRemoveMapping(id: string) {
    if (!isDesktop) return
    if (!confirm('Are you sure you want to remove this mapping?')) return
    try {
      // @ts-ignore
      await window.go.desktop.App.RemovePortMapping(id)
      toast.success('Port mapping removed')
      void loadMappings()
    } catch (err) {
      toast.error(`Failed to remove mapping: ${String(err)}`)
    }
  }

  // 获取资源列表
  const resources = resourceType === 'service' ? services : pods

  // 获取选中资源的端口列表
  const selectedResourcePorts = resources.find(r => r.name === selectedResource)?.ports || []

  if (!isDesktop) {
    return (
      <div className="p-8 text-center">
        <p className="text-gray-500">Port forwarding is only available in desktop mode</p>
      </div>
    )
  }

  return (
    <div className="p-6">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Port Forwarding</h1>
          <p className="text-sm text-gray-500 mt-1">
            Forward ports from Kubernetes services/pods to your local machine
          </p>
        </div>
        <button
          onClick={() => void loadMappings()}
          disabled={loadingMappings}
          className="flex items-center gap-2 px-3 py-2 text-sm border rounded-md hover:bg-gray-50 disabled:opacity-50"
        >
          <RefreshCw size={16} className={loadingMappings ? 'animate-spin' : ''} />
          Refresh
        </button>
      </div>

      {/* 添加端口映射表单 */}
      <div className="bg-white border rounded-lg p-6 mb-6">
        <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
          <Plus size={18} />
          Add Port Mapping
        </h2>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {/* 集群选择 */}
          <div>
            <label className="block text-sm font-medium mb-1">Cluster *</label>
            <select
              value={selectedCluster}
              onChange={(e) => setSelectedCluster(e.target.value)}
              disabled={loadingClusters}
              className="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50"
            >
              <option value="">Select cluster...</option>
              {clusters.map(c => (
                <option key={c.name} value={c.name}>{c.name}</option>
              ))}
            </select>
          </div>

          {/* Namespace 选择 */}
          <div>
            <label className="block text-sm font-medium mb-1">Namespace *</label>
            <select
              value={selectedNamespace}
              onChange={(e) => setSelectedNamespace(e.target.value)}
              disabled={!selectedCluster || loadingNamespaces}
              className="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50"
            >
              <option value="">Select namespace...</option>
              {namespaces.map(ns => (
                <option key={ns.name} value={ns.name}>{ns.name}</option>
              ))}
            </select>
          </div>

          {/* 资源类型 */}
          <div>
            <label className="block text-sm font-medium mb-1">Resource Type *</label>
            <select
              value={resourceType}
              onChange={(e) => setResourceType(e.target.value as 'service' | 'pod')}
              disabled={!selectedNamespace}
              className="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50"
            >
              <option value="service">Service</option>
              <option value="pod">Pod</option>
            </select>
          </div>

          {/* 资源选择 */}
          <div>
            <label className="block text-sm font-medium mb-1">
              {resourceType === 'service' ? 'Service' : 'Pod'} *
            </label>
            <select
              value={selectedResource}
              onChange={(e) => {
                setSelectedResource(e.target.value)
                setSelectedRemotePort('')
              }}
              disabled={!selectedNamespace || loadingResources}
              className="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50"
            >
              <option value="">Select {resourceType}...</option>
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
            <label className="block text-sm font-medium mb-1">Remote Port *</label>
            <select
              value={selectedRemotePort}
              onChange={(e) => setSelectedRemotePort(Number(e.target.value))}
              disabled={!selectedResource}
              className="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50"
            >
              <option value="">Select port...</option>
              {selectedResourcePorts.map((p, idx) => (
                <option key={idx} value={p.port}>
                  {p.port} {p.name && `(${p.name})`} - {p.protocol}
                </option>
              ))}
            </select>
          </div>

          {/* 本地端口 */}
          <div>
            <label className="block text-sm font-medium mb-1">Local Port (empty = random)</label>
            <input
              type="number"
              value={localPort}
              onChange={(e) => setLocalPort(e.target.value ? Number(e.target.value) : '')}
              placeholder="Random"
              min="1024"
              max="65535"
              className="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
        </div>

        <div className="mt-4">
          <button
            onClick={() => void handleAddMapping()}
            disabled={!selectedCluster || !selectedNamespace || !selectedResource || !selectedRemotePort}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <Plus size={16} />
            Add Mapping
          </button>
        </div>
      </div>

      {/* 端口映射列表 */}
      <div className="bg-white border rounded-lg overflow-hidden">
        <div className="px-6 py-4 border-b bg-gray-50">
          <h2 className="text-lg font-semibold">Active Port Mappings</h2>
        </div>

        {mappings.length === 0 ? (
          <div className="p-8 text-center text-gray-500">
            No port mappings yet. Add one above to get started!
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead className="bg-gray-50 border-b">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Resource</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Ports</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {mappings.map(mapping => (
                  <tr key={mapping.id} className="hover:bg-gray-50">
                    <td className="px-6 py-4">
                      <div className="text-sm font-medium">{mapping.resourceType}/{mapping.resourceName}</div>
                      <div className="text-xs text-gray-500">{mapping.cluster} / {mapping.namespace}</div>
                    </td>
                    <td className="px-6 py-4">
                      <div className="text-sm">
                        <span className="font-mono">localhost:{mapping.localPort}</span>
                        <span className="text-gray-400 mx-2">→</span>
                        <span className="font-mono">{mapping.remotePort}</span>
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                        mapping.status === 'running' ? 'bg-green-100 text-green-800' :
                        mapping.status === 'stopped' ? 'bg-gray-100 text-gray-800' :
                        'bg-red-100 text-red-800'
                      }`}>
                        {mapping.status}
                      </span>
                      {mapping.error && (
                        <div className="text-xs text-red-600 mt-1">{mapping.error}</div>
                      )}
                    </td>
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2">
                        {mapping.status === 'running' ? (
                          <button
                            onClick={() => void handleStopMapping(mapping.id)}
                            className="p-1.5 text-gray-600 hover:text-gray-900 hover:bg-gray-100 rounded"
                            title="Stop"
                          >
                            <Square size={16} />
                          </button>
                        ) : (
                          <button
                            onClick={() => void handleStartMapping(mapping.id)}
                            className="p-1.5 text-green-600 hover:text-green-900 hover:bg-green-50 rounded"
                            title="Start"
                          >
                            <Play size={16} />
                          </button>
                        )}
                        <button
                          onClick={() => void handleRemoveMapping(mapping.id)}
                          className="p-1.5 text-red-600 hover:text-red-900 hover:bg-red-50 rounded"
                          title="Remove"
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
