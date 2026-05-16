package desktop

import (
	"context"
	"fmt"
	"sort"

	"github.com/zxh326/kite-proxy/pkg/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

// NamespaceInfo 描述一个 namespace
type NamespaceInfo struct {
	Name string `json:"name"`
}

// ServiceInfo 描述一个 service
type ServiceInfo struct {
	Name      string     `json:"name"`
	Namespace string     `json:"namespace"`
	Ports     []PortInfo `json:"ports"`
	Type      string     `json:"type"`
}

// PodInfo 描述一个 pod
type PodInfo struct {
	Name      string     `json:"name"`
	Namespace string     `json:"namespace"`
	Ports     []PortInfo `json:"ports"`
	Status    string     `json:"status"`
}

// PortInfo 描述一个端口
type PortInfo struct {
	Name       string `json:"name"`
	Port       int32  `json:"port"`
	TargetPort int32  `json:"targetPort,omitempty"`
	Protocol   string `json:"protocol"`
}

// GetNamespaces 获取指定集群的所有 namespace
func (a *App) GetNamespaces(clusterName string) ([]NamespaceInfo, error) {
	if err := a.checkAuth(); err != nil {
		return nil, err
	}
	if clusterName == "" {
		return nil, fmt.Errorf("cluster name is required")
	}
	if a.client == nil {
		return nil, fmt.Errorf("not configured")
	}

	proxyNamespaces, err := a.client.GetProxyNamespaces(context.Background(), clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch proxy namespaces: %w", err)
	}

	for _, cluster := range proxyNamespaces {
		if cluster.Name != clusterName {
			continue
		}
		namespaces := make([]NamespaceInfo, 0, len(cluster.Namespaces))
		for _, ns := range cluster.Namespaces {
			namespaces = append(namespaces, NamespaceInfo{Name: ns})
		}
		sort.Slice(namespaces, func(i, j int) bool {
			return namespaces[i].Name < namespaces[j].Name
		})
		klog.Infof("Found %d allowed namespaces in cluster %s", len(namespaces), clusterName)
		return namespaces, nil
	}

	return []NamespaceInfo{}, nil
}

// GetServices 获取指定集群和 namespace 的所有 services
func (a *App) GetServices(clusterName, namespace string) ([]ServiceInfo, error) {
	if err := a.checkAuth(); err != nil {
		return nil, err
	}
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}

	// 获取集群的 rest.Config
	restConfig, err := a.getOrFetchRestConfig(clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster config: %w", err)
	}

	// 创建 Kubernetes 客户端
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// 获取指定 namespace 的所有 services
	ctx := context.Background()
	svcList, err := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	services := make([]ServiceInfo, 0, len(svcList.Items))
	for _, svc := range svcList.Items {
		ports := make([]PortInfo, 0, len(svc.Spec.Ports))
		for _, p := range svc.Spec.Ports {
			ports = append(ports, PortInfo{
				Name:       p.Name,
				Port:       p.Port,
				TargetPort: p.TargetPort.IntVal,
				Protocol:   string(p.Protocol),
			})
		}

		services = append(services, ServiceInfo{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			Ports:     ports,
			Type:      string(svc.Spec.Type),
		})
	}

	klog.Infof("Found %d services in namespace %s/%s", len(services), clusterName, namespace)
	return services, nil
}

// GetPods 获取指定集群和 namespace 的所有 pods
func (a *App) GetPods(clusterName, namespace string) ([]PodInfo, error) {
	if err := a.checkAuth(); err != nil {
		return nil, err
	}
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}

	// 获取集群的 rest.Config
	restConfig, err := a.getOrFetchRestConfig(clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster config: %w", err)
	}

	// 创建 Kubernetes 客户端
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// 获取指定 namespace 的所有 pods
	ctx := context.Background()
	podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	pods := make([]PodInfo, 0, len(podList.Items))
	for _, pod := range podList.Items {
		ports := make([]PortInfo, 0)
		for _, container := range pod.Spec.Containers {
			for _, p := range container.Ports {
				ports = append(ports, PortInfo{
					Name:     p.Name,
					Port:     p.ContainerPort,
					Protocol: string(p.Protocol),
				})
			}
		}

		pods = append(pods, PodInfo{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Ports:     ports,
			Status:    string(pod.Status.Phase),
		})
	}

	klog.Infof("Found %d pods in namespace %s/%s", len(pods), clusterName, namespace)
	return pods, nil
}

// getOrFetchRestConfig 获取或从 kite 获取集群的 rest.Config
func (a *App) getOrFetchRestConfig(clusterName string) (*rest.Config, error) {
	// 先从缓存获取
	if cfg, ok := a.cache.Get(clusterName); ok {
		return cfg, nil
	}

	// 缓存未命中，从 kite 服务器获取
	ctx := context.Background()
	clusterKC, err := a.client.GetClusterKubeconfig(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch kubeconfig from kite: %w", err)
	}

	// 解析 kubeconfig
	restConfig, err := api.ParseKubeconfig(clusterKC.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	// 存入缓存
	a.cache.Set(clusterName, restConfig)
	klog.Infof("Fetched and cached kubeconfig for cluster: %s", clusterName)

	return restConfig, nil
}
