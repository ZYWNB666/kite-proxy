package api

import (
	"fmt"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ParseKubeconfig converts a kubeconfig YAML string into a rest.Config.
// The raw YAML is not retained – only the parsed config is returned.
// This ensures credentials are not stored as plain text in memory longer than necessary.
func ParseKubeconfig(kubeconfigYAML string) (*rest.Config, error) {
	if kubeconfigYAML == "" {
		return nil, fmt.Errorf("kubeconfig is empty")
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfigYAML))
	if err != nil {
		return nil, fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	return restConfig, nil
}

// BuildKubeconfigYAML generates a kubeconfig YAML that points kubectl to kite-proxy.
// This is used by the /api/kubeconfig endpoint to generate a local config file.
func BuildKubeconfigYAML(clusters []string, proxyBaseURL string) string {
	var sb stringBuilder
	sb.line("apiVersion: v1")
	sb.line("kind: Config")
	sb.line("preferences: {}")
	sb.line("")
	sb.line("clusters:")
	for _, clusterName := range clusters {
		sb.linef("- cluster:")
		sb.linef("    server: %s/proxy/%s", proxyBaseURL, clusterName)
		sb.linef("    insecure-skip-tls-verify: true")
		sb.linef("  name: kite-proxy-%s", clusterName)
	}
	sb.line("")
	sb.line("users:")
	sb.line("- name: kite-proxy-user")
	sb.line("  user: {}")
	sb.line("")
	sb.line("contexts:")
	for _, clusterName := range clusters {
		sb.linef("- context:")
		sb.linef("    cluster: kite-proxy-%s", clusterName)
		sb.linef("    user: kite-proxy-user")
		sb.linef("  name: kite-proxy-%s", clusterName)
	}
	if len(clusters) > 0 {
		sb.line("")
		sb.linef("current-context: kite-proxy-%s", clusters[0])
	}

	return sb.String()
}

// stringBuilder is a helper for building multi-line strings.
type stringBuilder struct {
	buf []byte
}

func (s *stringBuilder) line(text string) {
	s.buf = append(s.buf, text...)
	s.buf = append(s.buf, '\n')
}

func (s *stringBuilder) linef(format string, args ...interface{}) {
	s.line(fmt.Sprintf(format, args...))
}

func (s *stringBuilder) String() string {
	return string(s.buf)
}
