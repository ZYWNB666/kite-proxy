package server

import "testing"

func TestExtractNamespaceFromProxyPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "pod list", path: "/api/v1/namespaces/default/pods", want: "default"},
		{name: "deployment detail", path: "/apis/apps/v1/namespaces/test/deployments/web", want: "test"},
		{name: "cluster scoped", path: "/api/v1/nodes", want: ""},
		{name: "root", path: "/", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := extractNamespaceFromProxyPath(tt.path); got != tt.want {
				t.Fatalf("extractNamespaceFromProxyPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
