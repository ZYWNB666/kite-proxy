/**
 * API 适配器 - 自动检测运行模式（Web 或 Desktop）
 * 
 * 在 Web 模式下使用 HTTP API
 * 在 Desktop 模式下使用 Wails Go 函数调用
 */

import type { Config, ClusterInfo, Status } from '../types'

// 检测是否在 Wails 桌面环境中运行
const isDesktop = typeof window !== 'undefined' && 'go' in window

// 在桌面模式下，Wails 会自动生成这些函数
// @ts-ignore - Wails 运行时注入
const go = isDesktop ? window.go : null

// Web 模式的 HTTP API 客户端
const BASE = '/api'

async function httpRequest<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    ...init,
  })
  if (!res.ok) {
    const text = await res.text()
    let msg = text
    try {
      msg = JSON.parse(text)?.error ?? text
    } catch {
      /* empty */
    }
    throw new Error(msg || `HTTP ${res.status}`)
  }
  return res.json() as Promise<T>
}

// 统一的 API 接口
export async function getConfig(): Promise<Config> {
  if (isDesktop && go) {
    // @ts-ignore
    const config = await go.desktop.App.GetConfig()
    return {
      kiteURL: config.kiteURL || '',
      apiKeyMasked: config.apiKeyMasked || '',
      configured: config.configured || false,
    }
  }
  return httpRequest<Config>('/config')
}

export async function saveConfig(kiteURL: string, apiKey: string): Promise<void> {
  if (isDesktop && go) {
    // @ts-ignore
    await go.desktop.App.SetConfig(kiteURL, apiKey)
    return
  }
  await httpRequest('/config', {
    method: 'POST',
    body: JSON.stringify({ kiteURL, apiKey }),
  })
}

export async function listClusters(): Promise<{ clusters: ClusterInfo[] }> {
  if (isDesktop && go) {
    // @ts-ignore
    const clusters = await go.desktop.App.ListClusters()
    return { clusters: clusters || [] }
  }
  return httpRequest<{ clusters: ClusterInfo[] }>('/clusters')
}

export async function getStatus(): Promise<Status> {
  if (isDesktop && go) {
    // 桌面模式使用简化的状态
    const config = await getConfig()
    // @ts-ignore
    const cachedClusters = go.desktop.App.cache?.List() || []
    return {
      status: 'ok',
      configured: config.configured,
      cachedClusters,
      syncEnabled: false, // 桌面模式不需要自动同步
      lastSyncError: null,
    }
  }
  return httpRequest<Status>('/status')
}

export async function clearCache(): Promise<void> {
  if (isDesktop && go) {
    // @ts-ignore
    await go.desktop.App.ClearCache()
    return
  }
  await httpRequest('/cache', { method: 'DELETE' })
}

export async function prewarmCluster(name: string): Promise<void> {
  if (isDesktop && go) {
    // @ts-ignore
    await go.desktop.App.PrewarmCluster(name)
    return
  }
  await httpRequest(`/cache/${encodeURIComponent(name)}`, { method: 'POST' })
}

export async function downloadKubeconfig(): Promise<string> {
  if (isDesktop && go) {
    // @ts-ignore
    return await go.desktop.App.GetKubeconfigYAML()
  }
  const res = await fetch(`${BASE}/kubeconfig`)
  return res.text()
}

export async function testConnection(): Promise<void> {
  if (isDesktop && go) {
    // @ts-ignore
    await go.desktop.App.TestConnection()
    return
  }
  // Web 模式使用 sync 端点测试连接
  await httpRequest('/sync', { method: 'POST' })
}

// 导出运行模式标识
export const runtime = {
  isDesktop,
  isWeb: !isDesktop,
  mode: isDesktop ? 'desktop' : 'web',
}

// 桌面模式下的事件监听
export function onDesktopEvent(event: string, callback: (data: any) => void) {
  if (!isDesktop) return

  // @ts-ignore - Wails 运行时
  const runtime = (window as any).runtime
  if (runtime && runtime.EventsOn) {
    runtime.EventsOn(event, callback)
  }
}

// 显示通知（仅桌面模式）
export async function showNotification(title: string, message: string) {
  if (isDesktop && go) {
    // @ts-ignore
    await go.desktop.App.ShowNotification(title, message)
  }
}
