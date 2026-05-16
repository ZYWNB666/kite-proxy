package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchProxyNamespaces_SendsAuthAndClusterQuery(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "kite123-secret" {
			t.Fatalf("unexpected Authorization header: %q", got)
		}
		if got := r.URL.Query().Get("cluster"); got != "yunqiao" {
			t.Fatalf("unexpected cluster query: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"clusters":[{"name":"yunqiao","namespaces":["default","test"]}]}`))
	}))
	defer server.Close()

	clusters, err := FetchProxyNamespaces(server.URL, "kite123-secret", "yunqiao")
	if err != nil {
		t.Fatalf("FetchProxyNamespaces returned error: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	if clusters[0].Name != "yunqiao" {
		t.Fatalf("unexpected cluster name: %q", clusters[0].Name)
	}
	if len(clusters[0].Namespaces) != 2 || clusters[0].Namespaces[0] != "default" || clusters[0].Namespaces[1] != "test" {
		t.Fatalf("unexpected namespaces: %#v", clusters[0].Namespaces)
	}
}

func TestFetchProxyNamespaces_ErrorMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		body       string
		targetErr  error
		wantCode   string
	}{
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			body:       `{"error":"Invalid API key"}`,
			targetErr:  ErrUnauthorized,
			wantCode:   "unauthorized",
		},
		{
			name:       "proxy forbidden",
			statusCode: http.StatusForbidden,
			body:       `{"error":"no clusters available for proxy or proxy not permitted","code":"proxy_forbidden"}`,
			targetErr:  ErrProxyForbidden,
			wantCode:   "proxy_forbidden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			_, err := FetchProxyNamespaces(server.URL, "kite123-secret", "")
			if !errors.Is(err, tt.targetErr) {
				t.Fatalf("expected error %v, got %v", tt.targetErr, err)
			}
			if code := ErrorCode(err); code != tt.wantCode {
				t.Fatalf("expected code %q, got %q", tt.wantCode, code)
			}
			if status := ErrorStatus(err); status != tt.statusCode {
				t.Fatalf("expected status %d, got %d", tt.statusCode, status)
			}
		})
	}
}
