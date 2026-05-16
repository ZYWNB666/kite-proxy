import { createContext, useContext, useState, useEffect, type ReactNode } from 'react'

export type Language = 'en' | 'zh'

const translations = {
  en: {
    // Nav
    configuration: 'Configuration',
    clusters: 'Clusters',
    portForwarding: 'Port Forwarding',
    connected: 'Connected',
    notConfigured: 'Not configured',

    // Config page
    configTitle: 'Configuration',
    configDescription: 'Connect kite-proxy to your kite server. Settings are saved to your home directory.',
    kiteServerURL: 'Kite Server URL',
    apiKey: 'API Key',
    save: 'Save',
    saving: 'Saving...',
    testConnection: 'Test Connection',
    configSaved: 'Configuration saved',
    connectionSuccessful: 'Connection successful',
    connectionFailed: 'Connection failed',
    fillAllFields: 'Please fill in all fields',
    connectedTo: 'Connected to',
    appearance: 'Appearance',
    language: 'Language',
    theme: 'Theme',
    lightTheme: 'Light',
    darkTheme: 'Dark',
    english: 'English',
    chinese: '中文',
    prefsSaved: 'Preferences saved',

    // Clusters page
    clustersTitle: 'Clusters',
    clustersDescription: 'Clusters you have proxy access to via kite.',
    refresh: 'Refresh',
    clearCache: 'Clear Cache',
    cacheCleared: 'Cache cleared',
    prewarm: 'Load',
    cached: 'Cached',
    notCached: 'Not cached',
    noClusters: 'No clusters found.',
    prewarmCluster: 'Load kubeconfig into memory',
    status: 'Status',
    actions: 'Actions',

    // Usage page
    portForwardingTitle: 'Port Forwarding',
    portForwardingDescription: 'Forward ports from Kubernetes services/pods to your local machine',
    addPortMapping: 'Add Port Mapping',
    availableClusters: 'Available Clusters',
    cluster: 'Cluster',
    namespace: 'Namespace',
    resourceType: 'Resource Type',
    service: 'Service',
    pod: 'Pod',
    remotePort: 'Remote Port',
    localPort: 'Local Port',
    localPortPlaceholder: 'Random',
    localPortHint: '(empty = random)',
    connecting: 'Connecting...',
    addMapping: 'Add Mapping',
    activePortMappings: 'Active Port Mappings',
    noMappingsYet: 'No port mappings yet. Add one above to get started!',
    resource: 'Resource',
    localURL: 'Local URL',
    remotePortCol: 'Remote Port',
    copyURL: 'Copy local URL',
    openBrowser: 'Open in browser',
    start: 'Start',
    stop: 'Stop',
    remove: 'Remove',
    mappingAdded: 'Port mapping added successfully',
    mappingStarted: 'Port mapping started',
    mappingStopped: 'Port mapping stopped',
    mappingRemoved: 'Port mapping removed',
    fillAllRequired: 'Please fill all required fields',
    selectCluster: 'Select cluster...',
    selectNamespace: 'Select namespace...',
    selectResource: 'Select...',
    selectPort: 'Select port...',

    // Status badges
    statusRunning: 'running',
    statusStopped: 'stopped',
    statusError: 'error',

    // Confirm dialog
    confirm: 'Confirm',
    cancel: 'Cancel',
    confirmRemoveTitle: 'Remove Port Mapping',
    confirmRemoveMsg: 'Are you sure you want to remove this port mapping?',

    // Auth
    authExpired: 'Session expired. API key is no longer valid.',
    authInvalidOrExpired: 'API key is invalid or expired. Please reconfigure it.',
    proxyForbidden: 'The current API key does not have proxy access to the target cluster or namespace.',
    namespaceForbidden: 'The target namespace is outside the allowed proxy scope.',
    kiteUnreachable: 'Unable to connect to the Kite server.',

    // Errors
    failedToLoadClusters: 'Failed to load clusters',
    failedToLoadNamespaces: 'Failed to load namespaces',
    failedToLoadResources: 'Failed to load resources',
    failedToLoadMappings: 'Failed to load mappings',
    failedToLoad: 'Failed to load',
    failedToAdd: 'Failed to add mapping',
    failedToStart: 'Failed to start mapping',
    failedToStop: 'Failed to stop mapping',
    failedToRemove: 'Failed to remove mapping',
    failedToCopy: 'Failed to copy URL',
    desktopOnly: 'Port forwarding is only available in desktop mode',
    confirmRemove: 'Are you sure you want to remove this port mapping?',
    randomPort: 'Random',
    activeMappings: 'Active Port Mappings',
    noMappings: 'No port mappings yet. Add one above to get started!',
    openInBrowser: 'Open in browser',
  },
  zh: {
    // Nav
    configuration: '配置',
    clusters: '集群',
    portForwarding: '端口转发',
    connected: '已连接',
    notConfigured: '未配置',

    // Config page
    configTitle: '配置',
    configDescription: '将 kite-proxy 连接到你的 kite 服务器。设置将保存到家目录。',
    kiteServerURL: 'Kite 服务器地址',
    apiKey: 'API 密钥',
    save: '保存',
    saving: '保存中...',
    testConnection: '测试连接',
    configSaved: '配置已保存',
    connectionSuccessful: '连接成功',
    connectionFailed: '连接失败',
    fillAllFields: '请填写所有字段',
    connectedTo: '已连接到',
    appearance: '外观',
    language: '语言',
    theme: '主题',
    lightTheme: '浅色',
    darkTheme: '深色',
    english: 'English',
    chinese: '中文',
    prefsSaved: '偏好设置已保存',

    // Clusters page
    clustersTitle: '集群',
    clustersDescription: '通过 kite 代理可访问的集群。',
    refresh: '刷新',
    clearCache: '清除缓存',
    cacheCleared: '缓存已清除',
    prewarm: '预加载',
    cached: '已缓存',
    notCached: '未缓存',
    noClusters: '未找到集群。',
    prewarmCluster: '将 kubeconfig 加载到内存',
    status: '状态',
    actions: '操作',

    // Usage page
    portForwardingTitle: '端口转发',
    portForwardingDescription: '将 Kubernetes 服务/Pod 的端口转发到本机',
    addPortMapping: '添加端口映射',
    availableClusters: '可转发集群',
    cluster: '集群',
    namespace: '命名空间',
    resourceType: '资源类型',
    service: 'Service',
    pod: 'Pod',
    remotePort: '远程端口',
    localPort: '本地端口',
    localPortPlaceholder: '随机',
    localPortHint: '（留空随机分配）',
    connecting: '连接中...',
    addMapping: '添加映射',
    activePortMappings: '活跃端口映射',
    noMappingsYet: '暂无端口映射，请在上方添加。',
    resource: '资源',
    localURL: '本地地址',
    remotePortCol: '远程端口',
    copyURL: '复制本地地址',
    openBrowser: '在浏览器中打开',
    start: '启动',
    stop: '停止',
    remove: '删除',
    mappingAdded: '端口映射添加成功',
    mappingStarted: '端口映射已启动',
    mappingStopped: '端口映射已停止',
    mappingRemoved: '端口映射已删除',
    fillAllRequired: '请填写所有必填字段',
    selectCluster: '选择集群...',
    selectNamespace: '选择命名空间...',
    selectResource: '选择...',
    selectPort: '选择端口...',

    // Status badges
    statusRunning: '运行中',
    statusStopped: '已停止',
    statusError: '错误',

    // Confirm dialog
    confirm: '确认',
    cancel: '取消',
    confirmRemoveTitle: '删除端口映射',
    confirmRemoveMsg: '确定要删除这条端口映射吗？',

    // Auth
    authExpired: '会话已过期，API 密钥无效或已失效。',
    authInvalidOrExpired: 'API key 无效或已过期，请重新配置。',
    proxyForbidden: '当前 API key 没有访问该集群/命名空间的代理权限。',
    namespaceForbidden: '目标命名空间不在允许代理的范围内。',
    kiteUnreachable: '无法连接到 Kite 服务端。',

    // Errors
    failedToLoadClusters: '加载集群失败',
    failedToLoadNamespaces: '加载命名空间失败',
    failedToLoadResources: '加载资源失败',
    failedToLoadMappings: '加载映射列表失败',
    failedToLoad: '加载失败',
    failedToAdd: '添加映射失败',
    failedToStart: '启动映射失败',
    failedToStop: '停止映射失败',
    failedToRemove: '删除映射失败',
    failedToCopy: '复制地址失败',
    desktopOnly: '端口转发仅在桌面模式下可用',
    confirmRemove: '确定要删除这条端口映射吗？',
    randomPort: '随机',
    activeMappings: '活跃端口映射',
    noMappings: '暂无端口映射，请在上方添加。',
    openInBrowser: '在浏览器中打开',
  },
}

export type TranslationKey = keyof typeof translations.en

interface I18nContextType {
  lang: Language
  setLang: (l: Language) => void
  t: typeof translations.en
}

const I18nContext = createContext<I18nContextType>({
  lang: 'en',
  setLang: () => {},
  t: translations.en,
})

export function I18nProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<Language>('en')

  useEffect(() => {
    // 从后端加载偏好设置
    const isDesktop = typeof window !== 'undefined' && 'go' in window
    if (isDesktop) {
      // @ts-ignore
      void (window.go.desktop.App.GetUIPrefs() as Promise<{ language: string; theme: string }>).then((prefs) => {
        if (prefs.language === 'zh' || prefs.language === 'en') {
          setLangState(prefs.language)
        }
      }).catch(() => {})
    }
  }, [])

  const setLang = (l: Language) => setLangState(l)

  return (
    <I18nContext.Provider value={{ lang, setLang, t: translations[lang] }}>
      {children}
    </I18nContext.Provider>
  )
}

export function useI18n() {
  return useContext(I18nContext)
}
