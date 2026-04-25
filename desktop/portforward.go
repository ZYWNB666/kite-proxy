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
	ID           string `json:"id"`           // 唯一标识
	Cluster      string `json:"cluster"`      // 集群名称
	Namespace    string `json:"namespace"`    // 命名空间
	ResourceType string `json:"resourceType"` // 资源类型：service 或 pod
	ResourceName string `json:"resourceName"` // 资源名称
	RemotePort   int    `json:"remotePort"`   // 远程端口
	LocalPort    int    `json:"localPort"`    // 本地端口
	Status       string `json:"status"`       // 状态：running, stopped, error
	Error        string `json:"error"`        // 错误信息
	CreatedAt    string `json:"createdAt"`    // 创建时间
}

// PortForwardManager 管理所有端口转发
type PortForwardManager struct {
	mu           sync.RWMutex
	mappings     map[string]*PortMapping
	forwarders   map[string]*portforward.PortForwarder
	stopChannels map[string]chan struct{}
	app          *App
}

// NewPortForwardManager 创建端口转发管理器
func NewPortForwardManager(app *App) *PortForwardManager {
	return &PortForwardManager{
		mappings:     make(map[string]*PortMapping),
		forwarders:   make(map[string]*portforward.PortForwarder),
		stopChannels: make(map[string]chan struct{}),
		app:          app,
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
		ID:           id,
		Cluster:      cluster,
		Namespace:    namespace,
		ResourceType: resourceType,
		ResourceName: resourceName,
		RemotePort:   remotePort,
		LocalPort:    localPort,
		Status:       "stopped",
		CreatedAt:    time.Now().Format(time.RFC3339),
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

	// 停止端口转发
	m.stopForwardingLocked(id)

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

	m.stopForwardingLocked(id)
	mapping.Status = "stopped"
	mapping.Error = ""

	return nil
}

// startForwardingLocked 启动端口转发（需要持有锁）
func (m *PortForwardManager) startForwardingLocked(mapping *PortMapping) error {
	// 如果已经在运行，先停止
	if mapping.Status == "running" {
		m.stopForwardingLocked(mapping.ID)
	}

	// 获取 rest.Config
	restConfig, err := m.app.getOrFetchRestConfig(mapping.Cluster)
	if err != nil {
		return fmt.Errorf("failed to get cluster config: %w", err)
	}

	// Build target port-forward path.
	// Kubernetes only supports pod port-forward; for Service we resolve a backend pod first.
	var resourcePath string
	remotePortForPod := mapping.RemotePort
	if mapping.ResourceType == "service" {
		podName, podPort, err := m.resolveServiceToPod(restConfig, mapping.Namespace, mapping.ResourceName, mapping.RemotePort)
		if err != nil {
			return err
		}
		remotePortForPod = podPort
		resourcePath = fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", mapping.Namespace, podName)
	} else if mapping.ResourceType == "pod" {
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

	ports := []string{fmt.Sprintf("%d:%d", mapping.LocalPort, remotePortForPod)}

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
		} else {
			fwErrChan <- nil
		}
	}()

	// 等待准备就绪或出错
	select {
	case <-readyChan:
		mapping.Status = "running"
		mapping.Error = ""
		m.forwarders[mapping.ID] = forwarder
		m.stopChannels[mapping.ID] = stopChan
		klog.Infof("Port forwarding started: %s -> localhost:%d", mapping.ID, mapping.LocalPort)
		if m.app.ctx != nil {
			runtime.EventsEmit(m.app.ctx, "mapping:started", mapping)
		}
		return nil
	case fwErr := <-fwErrChan:
		close(stopChan)
		if fwErr != nil {
			return fmt.Errorf("port forward failed: %w", fwErr)
		}
		return fmt.Errorf("port forward exited unexpectedly")
	case <-time.After(15 * time.Second):
		close(stopChan)
		return fmt.Errorf("timeout waiting for port forward to be ready")
	}
}

// stopForwardingLocked 停止端口转发（需要持有锁）
func (m *PortForwardManager) stopForwardingLocked(id string) {
	if stopChan, exists := m.stopChannels[id]; exists {
		close(stopChan)
		delete(m.stopChannels, id)
	}
	delete(m.forwarders, id)
	klog.Infof("Port forwarding stopped: %s", id)
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
		m.stopForwardingLocked(id)
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
