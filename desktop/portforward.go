package desktop

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/klog/v2"
)

// PortMapping 描述一个端口映射
type PortMapping struct {
	ID                 string `json:"id"`             // 唯一标识
	Cluster            string `json:"cluster"`        // 集群名称
	Namespace          string `json:"namespace"`      // 命名空间
	ResourceType       string `json:"resourceType"`   // 资源类型：service 或 pod
	ResourceName       string `json:"resourceName"`   // 资源名称
	RemotePort         int    `json:"remotePort"`     // 远程端口
	LocalPort          int    `json:"localPort"`      // 本地端口
	Status             string `json:"status"`         // 状态：running, stopped, error
	Error              string `json:"error"`          // 错误信息
	CreatedAt          string `json:"createdAt"`      // 创建时间
	CurrentPodName     string `json:"currentPodName"` // 当前转发的 Pod 名（用于 Service 类型自动切换）
	TotalBytesSent     uint64 `json:"totalBytesSent"`
	TotalBytesReceived uint64 `json:"totalBytesReceived"`
	CurrentSpeedSent   uint64 `json:"currentSpeedSent"`
	CurrentSpeedRecv   uint64 `json:"currentSpeedRecv"`
	prevBytesSent      uint64 `json:"-"`
	prevBytesRecv      uint64 `json:"-"`
}

// PortForwardManager 管理所有端口转发
type PortForwardManager struct {
	mu                   sync.RWMutex
	mappings             map[string]*PortMapping
	forwarders           map[string]*portforward.PortForwarder
	stopChannels         map[string]chan struct{}
	localListeners       map[string]net.Listener
	retryCtxCancels      map[string]context.CancelFunc
	endpointMonitorCalls map[string]context.CancelFunc // 用于取消 Service 监听
	app                  *App
}

// NewPortForwardManager 创建端口转发管理器
func NewPortForwardManager(app *App) *PortForwardManager {
	pm := &PortForwardManager{
		mappings:             make(map[string]*PortMapping),
		forwarders:           make(map[string]*portforward.PortForwarder),
		stopChannels:         make(map[string]chan struct{}),
		localListeners:       make(map[string]net.Listener),
		retryCtxCancels:      make(map[string]context.CancelFunc),
		endpointMonitorCalls: make(map[string]context.CancelFunc),
		app:                  app,
	}

	// 启动定期更新网速的协程
	go pm.startSpeedCalculator()

	return pm
}

func (m *PortForwardManager) startSpeedCalculator() {
	ticker := time.NewTicker(3 * time.Second)
	for range ticker.C {
		m.mu.Lock()
		for _, mapping := range m.mappings {
			if mapping.Status == "running" {
				sent := atomic.LoadUint64(&mapping.TotalBytesSent)
				recv := atomic.LoadUint64(&mapping.TotalBytesReceived)

				mapping.CurrentSpeedSent = (sent - mapping.prevBytesSent) / 3
				mapping.CurrentSpeedRecv = (recv - mapping.prevBytesRecv) / 3

				mapping.prevBytesSent = sent
				mapping.prevBytesRecv = recv
			} else if mapping.CurrentSpeedSent > 0 || mapping.CurrentSpeedRecv > 0 {
				mapping.CurrentSpeedSent = 0
				mapping.CurrentSpeedRecv = 0
			}
		}
		m.mu.Unlock()
	}
}

// AddMapping 添加端口映射
func (m *PortForwardManager) AddMapping(cluster, namespace, resourceType, resourceName string, remotePort, localPort int) (*PortMapping, error) {
	// 预加载集群 kubeconfig（在持有锁之前，避免阻塞其他操作）
	if err := m.app.PrewarmCluster(cluster); err != nil {
		klog.Warningf("Failed to prewarm cluster %s: %v", cluster, err)
		// 不返回错误，继续尝试添加映射（getOrFetchRestConfig 会重试）
	}

	return m.addMappingInternal(cluster, namespace, resourceType, resourceName, remotePort, localPort, true)
}

// addMappingWithoutAutoStart 添加端口映射但不自动启动（用于恢复持久化的映射）
func (m *PortForwardManager) addMappingWithoutAutoStart(cluster, namespace, resourceType, resourceName string, remotePort, localPort int) (*PortMapping, error) {
	return m.addMappingInternal(cluster, namespace, resourceType, resourceName, remotePort, localPort, false)
}

// addMappingInternal 内部方法：添加端口映射
func (m *PortForwardManager) addMappingInternal(cluster, namespace, resourceType, resourceName string, remotePort, localPort int, autoStart bool) (*PortMapping, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 如果本地端口为 0，自动分配
	if localPort == 0 {
		port, err := m.findAvailablePort()
		if err != nil {
			return nil, fmt.Errorf("failed to find available port: %w", err)
		}
		localPort = port
	}

	// 检查本地端口是否已被使用
	if m.isPortInUse(localPort) {
		return nil, fmt.Errorf("local port %d is already in use", localPort)
	}

	// 生成唯一 ID
	id := fmt.Sprintf("%s-%s-%s-%s-%d", cluster, namespace, resourceType, resourceName, remotePort)

	// 检查是否已存在
	if _, exists := m.mappings[id]; exists {
		return nil, fmt.Errorf("mapping already exists")
	}

	// 创建映射
	mapping := &PortMapping{
		ID:                 id,
		Cluster:            cluster,
		Namespace:          namespace,
		ResourceType:       resourceType,
		ResourceName:       resourceName,
		RemotePort:         remotePort,
		LocalPort:          localPort,
		Status:             "stopped",
		CreatedAt:          time.Now().Format(time.RFC3339),
		TotalBytesSent:     0,
		TotalBytesReceived: 0,
		CurrentSpeedSent:   0,
		CurrentSpeedRecv:   0,
		prevBytesSent:      0,
		prevBytesRecv:      0,
	}

	m.mappings[id] = mapping

	// 如果需要，自动启动端口转发
	if autoStart {
		if err := m.startForwardingLocked(mapping); err != nil {
			mapping.Status = "error"
			mapping.Error = err.Error()
			klog.Errorf("Failed to start port forwarding: %v", err)
		}
	}

	return mapping, nil
}

// RemoveMapping 删除端口映射
func (m *PortForwardManager) RemoveMapping(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.mappings[id]
	if !exists {
		return fmt.Errorf("mapping not found")
	}

	// 停止端口转发（并取消自动重连）
	m.stopForwardingLocked(id, true)

	// 删除映射
	delete(m.mappings, id)

	klog.Infof("Removed port mapping: %s", id)
	return nil
}

// ListMappings 列出所有端口映射
func (m *PortForwardManager) ListMappings() []*PortMapping {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mappings := make([]*PortMapping, 0, len(m.mappings))
	for _, mapping := range m.mappings {
		mappings = append(mappings, mapping)
	}

	sort.Slice(mappings, func(i, j int) bool {
		return mappings[i].CreatedAt < mappings[j].CreatedAt
	})

	return mappings
}

// StartMapping 启动端口映射
func (m *PortForwardManager) StartMapping(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mapping, exists := m.mappings[id]
	if !exists {
		return fmt.Errorf("mapping not found")
	}

	return m.startForwardingLocked(mapping)
}

// StopMapping 停止端口映射
func (m *PortForwardManager) StopMapping(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mapping, exists := m.mappings[id]
	if !exists {
		return fmt.Errorf("mapping not found")
	}

	m.stopForwardingLocked(id, true)
	mapping.Status = "stopped"
	mapping.Error = ""
	mapping.CurrentSpeedSent = 0
	mapping.CurrentSpeedRecv = 0

	return nil
}

// startForwardingLocked 启动端口转发（需要持有锁）
func (m *PortForwardManager) startForwardingLocked(mapping *PortMapping) error {
	// 如果已经在运行或处于错误状态，先清理旧资源（不取消重连 ctx，因为可能是重连调用）
	if mapping.Status == "running" || mapping.Status == "error" {
		m.stopForwardingLocked(mapping.ID, false)
	}

	// 获取 rest.Config
	restConfig, err := m.app.getOrFetchRestConfig(mapping.Cluster)
	if err != nil {
		return fmt.Errorf("failed to get cluster config: %w", err)
	}

	// Build target port-forward path.
	// Kubernetes only supports pod port-forward; for Service we resolve a backend pod first.
	var resourcePath string
	var resolvedPodName string // 记录 Service 解析出来的 Pod 名
	remotePortForPod := mapping.RemotePort
	if mapping.ResourceType == "service" {
		podName, podPort, err := m.resolveServiceToPod(restConfig, mapping.Namespace, mapping.ResourceName, mapping.RemotePort)
		if err != nil {
			return err
		}
		resolvedPodName = podName
		remotePortForPod = podPort
		resourcePath = fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", mapping.Namespace, podName)
	} else if mapping.ResourceType == "pod" {
		resolvedPodName = mapping.ResourceName
		resourcePath = fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", mapping.Namespace, mapping.ResourceName)
	} else {
		return fmt.Errorf("unsupported resource type: %s", mapping.ResourceType)
	}

	// 创建端口转发
	transport, upgrader, err := spdy.RoundTripperFor(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create round tripper: %w", err)
	}

	serverURL, err := url.Parse(restConfig.Host)
	if err != nil {
		return fmt.Errorf("failed to parse host URL: %w", err)
	}
	serverURL.Path = resourcePath

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, serverURL)

	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{}, 1)

	// 生成一个内部端口
	internalPort, err := m.findAvailablePort()
	if err != nil {
		return fmt.Errorf("failed to find internal port: %w", err)
	}

	// 启动本地代理监听器（绑定 0.0.0.0 使局域网内其他机器也可访问）
	proxyListener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", mapping.LocalPort))
	if err != nil {
		return fmt.Errorf("failed to bind local port %d: %w", mapping.LocalPort, err)
	}
	m.localListeners[mapping.ID] = proxyListener

	ports := []string{fmt.Sprintf("%d:%d", internalPort, remotePortForPod)}

	out, errOut := io.Discard, io.Discard
	if klog.V(2).Enabled() {
		out = &klogWriter{}
		errOut = &klogWriter{}
	}

	forwarder, err := portforward.New(dialer, ports, stopChan, readyChan, out, errOut)
	if err != nil {
		return fmt.Errorf("failed to create port forwarder: %w", err)
	}

	// 用于捕获 ForwardPorts() 的早期错误
	fwErrChan := make(chan error, 1)

	// 在后台启动端口转发
	go func() {
		if err := forwarder.ForwardPorts(); err != nil {
			klog.Errorf("Port forwarding failed for %s: %v", mapping.ID, err)
			fwErrChan <- err
			m.mu.Lock()
			mapping.Status = "error"
			mapping.Error = err.Error()
			m.mu.Unlock()
			if m.app.ctx != nil {
				runtime.EventsEmit(m.app.ctx, "mapping:error", mapping)
			}
			// Pod 断开后自动重连（service 会重新解析 pod）
			go m.scheduleReconnect(mapping.ID)
		} else {
			fwErrChan <- nil
		}
	}()

	// 等待准备就绪或出错
	select {
	case <-readyChan:
		mapping.Status = "running"
		mapping.Error = ""
		mapping.CurrentPodName = resolvedPodName // 记录当前 Pod 名
		m.forwarders[mapping.ID] = forwarder
		m.stopChannels[mapping.ID] = stopChan

		// 启动本地代理接收连接
		go m.acceptAndProxy(proxyListener, mapping, internalPort)

		// 对于 Service 类型，启动后台沐听 Service 的后端 Pod 变化，不然 Pod 重启就断
		if mapping.ResourceType == "service" {
			go m.monitorServiceEndpoints(mapping.ID, restConfig)
		}

		klog.Infof("Port forwarding started: %s -> 0.0.0.0:%d (internal: %d)", mapping.ID, mapping.LocalPort, internalPort)
		if m.app.ctx != nil {
			runtime.EventsEmit(m.app.ctx, "mapping:started", mapping)
		}
		return nil
	case fwErr := <-fwErrChan:
		close(stopChan)
		proxyListener.Close()
		delete(m.localListeners, mapping.ID)
		if fwErr != nil {
			return fmt.Errorf("port forward failed: %w", fwErr)
		}
		return fmt.Errorf("port forward exited unexpectedly")
	case <-time.After(15 * time.Second):
		close(stopChan)
		proxyListener.Close()
		delete(m.localListeners, mapping.ID)
		return fmt.Errorf("timeout waiting for port forward to be ready")
	}
}

// stopForwardingLocked 停止端口转发（需要持有锁）。
// cancelReconnect=true 时同时取消正在等待中的自动重连和 Service 监听（用户主动停止/删除时使用）。
func (m *PortForwardManager) stopForwardingLocked(id string, cancelReconnect bool) {
	if cancelReconnect {
		if cancel, exists := m.retryCtxCancels[id]; exists {
			cancel()
			delete(m.retryCtxCancels, id)
		}
		if cancel, exists := m.endpointMonitorCalls[id]; exists {
			cancel()
			delete(m.endpointMonitorCalls, id)
		}
	}
	if listener, exists := m.localListeners[id]; exists {
		listener.Close()
		delete(m.localListeners, id)
	}
	if stopChan, exists := m.stopChannels[id]; exists {
		close(stopChan)
		delete(m.stopChannels, id)
	}
	delete(m.forwarders, id)
	klog.Infof("Port forwarding stopped: %s", id)
}

// scheduleReconnect 在 pod 断连后以指数退避策略自动重连。
// 若用户主动调用 StopMapping/RemoveMapping 则取消重连。
func (m *PortForwardManager) scheduleReconnect(id string) {
	ctx, cancel := context.WithCancel(context.Background())

	m.mu.Lock()
	// 取消上一个还在等待中的重连（如快速连续失败）
	if oldCancel, exists := m.retryCtxCancels[id]; exists {
		oldCancel()
	}
	m.retryCtxCancels[id] = cancel
	m.mu.Unlock()

	go func() {
		defer cancel()

		backoff := 3 * time.Second
		const maxBackoff = 60 * time.Second

		for attempt := 1; attempt <= 20; attempt++ {
			select {
			case <-ctx.Done():
				klog.Infof("Auto-reconnect cancelled for %s", id)
				return
			case <-time.After(backoff):
			}

			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}

			m.mu.Lock()
			mapping, exists := m.mappings[id]
			if !exists || mapping.Status == "stopped" {
				m.mu.Unlock()
				return
			}
			klog.Infof("Auto-reconnecting port forwarding for %s (attempt %d/%d)...", id, attempt, 20)
			err := m.startForwardingLocked(mapping)
			m.mu.Unlock()

			if err == nil {
				klog.Infof("Auto-reconnect succeeded for %s after %d attempt(s)", id, attempt)
				return
			}
			klog.Errorf("Auto-reconnect attempt %d failed for %s: %v", attempt, id, err)
		}
		klog.Errorf("Auto-reconnect exhausted (20 attempts) for %s", id)
	}()
}

// monitorServiceEndpoints 监听 Service 的后端 Pod 列表变化。
// 当后端 Pod 被替换（Pod 重启/滚动更新）时，自动切换转发到新的可用 Pod。
// 这样 Service 转发就永不断裂，对用户完全透明。
func (m *PortForwardManager) monitorServiceEndpoints(id string, restConfig *rest.Config) {
	ctx, cancel := context.WithCancel(context.Background())

	m.mu.Lock()
	if oldCancel, exists := m.endpointMonitorCalls[id]; exists {
		oldCancel()
	}
	m.endpointMonitorCalls[id] = cancel
	m.mu.Unlock()

	ticker := time.NewTicker(5 * time.Second) // 每 5 秒检查一次后端 Pod 列表
	defer ticker.Stop()
	defer func() {
		m.mu.Lock()
		delete(m.endpointMonitorCalls, id)
		m.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		m.mu.Lock()
		mapping, exists := m.mappings[id]
		if !exists || mapping.Status == "stopped" || mapping.ResourceType != "service" {
			m.mu.Unlock()
			return
		}

		// 获取 Service 的后端 Pod 列表
		pods, err := m.getServiceBackendPods(restConfig, mapping.Namespace, mapping.ResourceName)
		m.mu.Unlock()

		if err != nil {
			klog.Warningf("Failed to get backend pods for service %s/%s: %v", mapping.Namespace, mapping.ResourceName, err)
			continue
		}

		if len(pods) == 0 {
			klog.Warningf("No available pods for service %s/%s", mapping.Namespace, mapping.ResourceName)
			continue
		}

		// 检查当前 Pod 是否还在后端 Pod 列表中
		m.mu.Lock()
		mapping, exists = m.mappings[id]
		if !exists || mapping.Status == "stopped" {
			m.mu.Unlock()
			return
		}

		currentPodFound := false
		for _, pod := range pods {
			if pod.Name == mapping.CurrentPodName && pod.Status.Phase == corev1.PodRunning {
				currentPodFound = true
				break
			}
		}

		// 如果当前 Pod 已经不在可用列表中（被替换/重启），需要切换到新 Pod
		if !currentPodFound && mapping.Status == "running" {
			klog.Infof("Backend pod %s for service %s/%s is no longer available, switching to new pod",
				mapping.CurrentPodName, mapping.Namespace, mapping.ResourceName)

			// 停止当前转发（不取消监听，继续监听）
			m.stopForwardingLocked(id, false)
			mapping.Status = "reconnecting"
			mapping.Error = "Backend pod changed, reconnecting..."

			if m.app.ctx != nil {
				runtime.EventsEmit(m.app.ctx, "mapping:reconnecting", mapping)
			}

			// 重新建立转发到新 Pod
			err := m.startForwardingLocked(mapping)
			if err != nil {
				mapping.Status = "error"
				mapping.Error = fmt.Sprintf("Failed to switch to new backend pod: %v", err)
				klog.Errorf("Failed to reconnect to new backend pod for service %s/%s: %v",
					mapping.Namespace, mapping.ResourceName, err)
				if m.app.ctx != nil {
					runtime.EventsEmit(m.app.ctx, "mapping:error", mapping)
				}
			}
		}
		m.mu.Unlock()
	}
}

// getServiceBackendPods 获取 Service 的后端 Pod 列表
func (m *PortForwardManager) getServiceBackendPods(restConfig *rest.Config, namespace, serviceName string) ([]corev1.Pod, error) {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	ctx := context.Background()
	svc, err := clientset.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get service %s/%s: %w", namespace, serviceName, err)
	}

	if len(svc.Spec.Selector) == 0 {
		return nil, fmt.Errorf("service %s/%s has no selector", namespace, serviceName)
	}

	podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(svc.Spec.Selector).String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods for service %s/%s: %w", namespace, serviceName, err)
	}

	return podList.Items, nil
}

func (m *PortForwardManager) acceptAndProxy(listener net.Listener, mapping *PortMapping, internalPort int) {
	for {
		clientConn, err := listener.Accept()
		if err != nil {
			// listener closed
			return
		}

		go func(clientConn net.Conn) {
			defer clientConn.Close()

			k8sConn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", internalPort))
			if err != nil {
				klog.Errorf("Failed to dial internal port: %v", err)
				return
			}
			defer k8sConn.Close()

			var wg sync.WaitGroup
			wg.Add(2)

			go func() {
				defer wg.Done()
				// 从客户端读，发送到k8s (Sent)
				_, _ = io.Copy(k8sConn, &countingReader{Reader: clientConn, count: &mapping.TotalBytesSent})
			}()

			go func() {
				defer wg.Done()
				// 从k8s读，发送到客户端 (Received)
				_, _ = io.Copy(clientConn, &countingReader{Reader: k8sConn, count: &mapping.TotalBytesReceived})
			}()

			wg.Wait()
		}(clientConn)
	}
}

// countingReader counts bytes read
type countingReader struct {
	io.Reader
	count *uint64
}

func (c *countingReader) Read(p []byte) (n int, err error) {
	n, err = c.Reader.Read(p)
	if n > 0 {
		atomic.AddUint64(c.count, uint64(n))
	}
	return
}

// findAvailablePort 查找可用的本地端口
func (m *PortForwardManager) findAvailablePort() (int, error) {
	// 尝试随机端口 10 次
	for i := 0; i < 10; i++ {
		port := rand.Intn(65535-10000) + 10000
		if !m.isPortInUse(port) {
			// 双重检查端口是否真的可用
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			if err == nil {
				listener.Close()
				return port, nil
			}
		}
	}

	return 0, fmt.Errorf("failed to find available port after 10 attempts")
}

// isPortInUse 检查端口是否已被当前映射使用
func (m *PortForwardManager) isPortInUse(port int) bool {
	for _, mapping := range m.mappings {
		if mapping.LocalPort == port {
			return true
		}
	}
	return false
}

// StopAll 停止所有端口转发
func (m *PortForwardManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id := range m.mappings {
		m.stopForwardingLocked(id, true)
	}

	klog.Info("All port forwardings stopped")
}

// klogWriter 实现 io.Writer，将输出写入 klog
type klogWriter struct{}

func (w *klogWriter) Write(p []byte) (n int, err error) {
	klog.V(2).Info(string(p))
	return len(p), nil
}

func (m *PortForwardManager) resolveServiceToPod(restConfig *rest.Config, namespace, serviceName string, requestedPort int) (string, int, error) {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	ctx := context.Background()
	svc, err := clientset.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return "", 0, fmt.Errorf("failed to get service %s/%s: %w", namespace, serviceName, err)
	}

	if len(svc.Spec.Selector) == 0 {
		return "", 0, fmt.Errorf("service %s/%s has no selector, cannot resolve backend pod for port-forward", namespace, serviceName)
	}

	podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(svc.Spec.Selector).String(),
	})
	if err != nil {
		return "", 0, fmt.Errorf("failed to list pods for service %s/%s: %w", namespace, serviceName, err)
	}
	if len(podList.Items) == 0 {
		return "", 0, fmt.Errorf("no pods found for service %s/%s selector", namespace, serviceName)
	}

	pod := pickPortForwardPod(podList.Items)
	targetPort, err := resolveServiceTargetPort(svc, pod, requestedPort)
	if err != nil {
		return "", 0, err
	}

	klog.Infof("Resolved service %s/%s:%d to pod %s:%d for port-forward", namespace, serviceName, requestedPort, pod.Name, targetPort)
	return pod.Name, targetPort, nil
}

func pickPortForwardPod(pods []corev1.Pod) *corev1.Pod {
	for i := range pods {
		pod := &pods[i]
		if pod.DeletionTimestamp == nil && pod.Status.Phase == corev1.PodRunning {
			return pod
		}
	}
	return &pods[0]
}

func resolveServiceTargetPort(svc *corev1.Service, pod *corev1.Pod, requestedPort int) (int, error) {
	for _, port := range svc.Spec.Ports {
		if int(port.Port) != requestedPort {
			continue
		}

		switch port.TargetPort.Type {
		case intstr.Int:
			if port.TargetPort.IntVal > 0 {
				return int(port.TargetPort.IntVal), nil
			}
			return int(port.Port), nil
		case intstr.String:
			name := port.TargetPort.StrVal
			if name == "" {
				return int(port.Port), nil
			}
			if p, ok := findContainerPortByName(pod, name); ok {
				return p, nil
			}
			return 0, fmt.Errorf("service targetPort %q was not found on pod %s", name, pod.Name)
		default:
			return int(port.Port), nil
		}
	}

	return 0, fmt.Errorf("service port %d was not found on service %s/%s", requestedPort, svc.Namespace, svc.Name)
}

func findContainerPortByName(pod *corev1.Pod, targetName string) (int, bool) {
	for _, c := range pod.Spec.Containers {
		for _, p := range c.Ports {
			if p.Name == targetName {
				return int(p.ContainerPort), true
			}
		}
	}
	return 0, false
}
