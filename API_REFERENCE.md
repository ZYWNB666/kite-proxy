# kite-proxy API Reference

Complete API reference for kite-proxy REST endpoints.

Base URL: `http://localhost:8090` (configurable via `--port` flag)

## Table of Contents

1. [Configuration APIs](#configuration-apis)
2. [Cluster APIs](#cluster-apis)
3. [Cache Management APIs](#cache-management-apis)
4. [Sync APIs](#sync-apis)
5. [Health Check APIs](#health-check-apis)
6. [Kubernetes Proxy API](#kubernetes-proxy-api)

---

## Configuration APIs

### Get Configuration

Retrieve current kite-proxy configuration. API key is masked for security.

**Endpoint**: `GET /api/config`

**Response**:
```json
{
  "kiteURL": "http://localhost:8080",
  "apiKeyMasked": "kite****-key",
  "configured": true
}
```

**Status Codes**:
- `200 OK` - Configuration retrieved successfully

**Example**:
```powershell
curl http://localhost:8090/api/config | ConvertFrom-Json
```

---

### Set Configuration

Update kite server URL and API key. This will clear all cached kubeconfigs.

**Endpoint**: `POST /api/config`

**Request Body**:
```json
{
  "kiteURL": "https://kite.example.com",
  "apiKey": "kite123-your-api-key"
}
```

**Response**:
```json
{
  "message": "configuration saved"
}
```

**Status Codes**:
- `200 OK` - Configuration updated successfully
- `400 Bad Request` - Invalid request body

**Example**:
```powershell
$body = @{
    kiteURL = "http://localhost:8080"
    apiKey = "kite123-your-key"
} | ConvertTo-Json

curl -X POST http://localhost:8090/api/config `
  -H "Content-Type: application/json" `
  -d $body
```

---

## Cluster APIs

### List Clusters

List all clusters available for proxying based on RBAC permissions.

**Endpoint**: `GET /api/clusters`

**Response**:
```json
{
  "clusters": [
    {
      "name": "dev-cluster",
      "cached": false
    },
    {
      "name": "prod-cluster",
      "cached": true
    }
  ]
}
```

**Fields**:
- `name` (string) - Cluster name
- `cached` (boolean) - Whether kubeconfig is cached in memory

**Status Codes**:
- `200 OK` - Cluster list retrieved successfully
- `502 Bad Gateway` - Failed to connect to kite server

**Example**:
```powershell
curl http://localhost:8090/api/clusters | ConvertFrom-Json
```

---

### Get Kubeconfig

Generate a local kubeconfig file that points kubectl to kite-proxy.

**Endpoint**: `GET /api/kubeconfig`

**Response**: YAML content (Content-Type: `application/x-yaml`)

**Example Response**:
```yaml
apiVersion: v1
kind: Config
preferences: {}

clusters:
- cluster:
    server: http://localhost:8090/proxy/dev-cluster
    insecure-skip-tls-verify: true
  name: kite-proxy-dev-cluster
- cluster:
    server: http://localhost:8090/proxy/prod-cluster
    insecure-skip-tls-verify: true
  name: kite-proxy-prod-cluster

users:
- name: kite-proxy-user
  user: {}

contexts:
- context:
    cluster: kite-proxy-dev-cluster
    user: kite-proxy-user
  name: kite-proxy-dev-cluster
- context:
    cluster: kite-proxy-prod-cluster
    user: kite-proxy-user
  name: kite-proxy-prod-cluster

current-context: kite-proxy-dev-cluster
```

**Status Codes**:
- `200 OK` - Kubeconfig generated successfully
- `502 Bad Gateway` - Failed to fetch clusters from kite server

**Example**:
```powershell
# Download to file
curl http://localhost:8090/api/kubeconfig -o kubeconfig.yaml

# Use with kubectl
$env:KUBECONFIG = "kubeconfig.yaml"
kubectl get pods
```

---

## Cache Management APIs

### Clear All Cache

Remove all cached kubeconfigs from memory. Next request will fetch fresh data.

**Endpoint**: `DELETE /api/cache`

**Response**:
```json
{
  "message": "cache cleared"
}
```

**Status Codes**:
- `200 OK` - Cache cleared successfully

**Example**:
```powershell
curl -X DELETE http://localhost:8090/api/cache
```

**Use Cases**:
- After rotating credentials
- After RBAC changes in kite
- Before running tests

---

### Pre-warm Cluster Cache

Fetch and cache kubeconfig for a specific cluster before first use.

**Endpoint**: `POST /api/cache/:cluster`

**URL Parameters**:
- `cluster` (string, required) - Cluster name to pre-warm

**Response**:
```json
{
  "message": "cluster \"dev-cluster\" warmed up"
}
```

**Status Codes**:
- `200 OK` - Cluster cached successfully
- `400 Bad Request` - Missing cluster name
- `502 Bad Gateway` - Failed to fetch kubeconfig from kite

**Example**:
```powershell
curl -X POST http://localhost:8090/api/cache/dev-cluster
```

**Use Cases**:
- Warm up cache during deployment
- Pre-load frequently used clusters
- Reduce latency for first kubectl request

---

## Sync APIs

### Get Status

Retrieve kite-proxy status including sync state and cached clusters.

**Endpoint**: `GET /api/status`

**Response**:
```json
{
  "status": "ok",
  "configured": true,
  "cachedClusters": ["dev-cluster", "prod-cluster"],
  "syncEnabled": true,
  "lastSyncError": null
}
```

**Fields**:
- `status` (string) - Always "ok" if responding
- `configured` (boolean) - Whether kite URL and API key are set
- `cachedClusters` (array) - List of cached cluster names
- `syncEnabled` (boolean) - Whether auto-sync is running
- `lastSyncError` (string|null) - Error from last sync attempt, or null if successful

**Status Codes**:
- `200 OK` - Status retrieved successfully

**Example**:
```powershell
curl http://localhost:8090/api/status | ConvertFrom-Json
```

---

### Trigger Manual Sync

Manually trigger synchronization with kite server to verify connectivity.

**Endpoint**: `POST /api/sync`

**Response** (success):
```json
{
  "message": "sync completed successfully"
}
```

**Response** (failure):
```json
{
  "error": "request to kite server failed: connection refused"
}
```

**Status Codes**:
- `200 OK` - Sync completed successfully
- `500 Internal Server Error` - Syncer not initialized
- `502 Bad Gateway` - Sync failed (kite unreachable or auth failed)

**Example**:
```powershell
curl -X POST http://localhost:8090/api/sync
```

**Use Cases**:
- Test connectivity after configuration change
- Verify API key is valid
- Trigger immediate sync instead of waiting for automatic interval

---

## Health Check APIs

### Health Check

Simple health check endpoint for load balancers and monitoring.

**Endpoint**: `GET /healthz`

**Response**:
```json
{
  "status": "ok"
}
```

**Status Codes**:
- `200 OK` - Service is healthy

**Example**:
```powershell
curl http://localhost:8090/healthz
```

**Notes**:
- This endpoint always returns 200 if the service is running
- Does NOT check kite server connectivity
- Use `GET /api/status` for detailed health information

---

## Kubernetes Proxy API

### Proxy Kubernetes API Requests

Forward kubectl requests to the real Kubernetes API server.

**Endpoint**: `ANY /proxy/:cluster/*path`

**URL Parameters**:
- `cluster` (string, required) - Target cluster name
- `path` (string) - Kubernetes API path (e.g., `/api/v1/pods`)

**Request Headers**:
- All standard Kubernetes API headers are supported
- Authentication is handled by cached kubeconfig

**Response**: 
- Transparently returns the Kubernetes API response

**Status Codes**:
- `200 OK` - Request proxied successfully
- `400 Bad Request` - Missing cluster name
- `502 Bad Gateway` - Cannot connect to cluster (kubeconfig fetch failed)

**Example Requests**:

List all pods:
```bash
curl http://localhost:8090/proxy/dev-cluster/api/v1/pods
```

Get specific pod:
```bash
curl http://localhost:8090/proxy/dev-cluster/api/v1/namespaces/default/pods/nginx
```

Create deployment (POST):
```bash
curl -X POST http://localhost:8090/proxy/dev-cluster/apis/apps/v1/namespaces/default/deployments \
  -H "Content-Type: application/json" \
  -d @deployment.json
```

**Notes**:
- First request to a cluster will fetch kubeconfig (may take 1-2 seconds)
- Subsequent requests reuse cached kubeconfig (fast)
- All Kubernetes API verbs are supported: GET, POST, PUT, DELETE, PATCH, etc.
- Query parameters are preserved (e.g., `?labelSelector=app=nginx`)

---

## Error Responses

All error responses follow this format:

```json
{
  "error": "descriptive error message"
}
```

### Common Error Scenarios

#### 1. kite Server Unreachable

**Request**: Any API that requires kite connection

**Response**:
```json
{
  "error": "request to kite server failed: dial tcp: connection refused"
}
```

**Status Code**: `502 Bad Gateway`

**Solution**: Check kite server URL and network connectivity

---

#### 2. Invalid API Key

**Request**: Any API that requires kite authentication

**Response**:
```json
{
  "error": "kite server returned 401: unauthorized"
}
```

**Status Code**: `502 Bad Gateway`

**Solution**: Verify API key is correct and has `allowProxy: true`

---

#### 3. No Clusters Available

**Request**: `GET /api/clusters`

**Response**:
```json
{
  "error": "no proxy permission or no accessible clusters"
}
```

**Status Code**: `502 Bad Gateway`

**Solution**: 
- Check RBAC role has `allowProxy: true`
- Verify role's `clusters` field is not empty
- Ensure role is assigned to the API key user

---

#### 4. Configuration Not Set

**Request**: Any API requiring kite connection (if not configured)

**Response**:
```json
{
  "error": "kite server URL is not configured"
}
```

**Status Code**: `502 Bad Gateway`

**Solution**: Set configuration via `POST /api/config` or command-line flags

---

## Rate Limiting

kite-proxy does NOT implement rate limiting. Consider:

1. **Upstream limits**: kite server may have its own rate limits
2. **Kubernetes limits**: K8s API server has built-in rate limiting
3. **Network limits**: Your infrastructure may have bandwidth limits

For production deployment, consider adding a reverse proxy (nginx, Traefik) with rate limiting.

---

## Authentication

kite-proxy itself does NOT require authentication for its REST API.

Security is enforced by:
1. **Network isolation** - Deploy in a trusted network
2. **Reverse proxy** - Add authentication layer (OAuth, mTLS, etc.)
3. **kite RBAC** - Access control at the kite server level

---

## Versioning

Current API version: **v1** (implied, no version prefix in URLs)

Future breaking changes will introduce versioned paths:
- `/api/v1/...` (current, default)
- `/api/v2/...` (future)

---

## WebSocket Support

kite-proxy supports WebSocket connections for:
- `kubectl exec` (interactive shell)
- `kubectl logs -f` (streaming logs)
- `kubectl port-forward`

WebSocket connections are automatically upgraded when proxying to Kubernetes API.

**Example**:
```bash
kubectl --server=http://localhost:8090/proxy/dev-cluster \
        --insecure-skip-tls-verify \
        exec -it nginx-pod -- /bin/bash
```

---

## CORS

kite-proxy does NOT currently set CORS headers.

If you need to access the API from a browser-based application:
1. Deploy a reverse proxy (nginx) to add CORS headers
2. Or modify `server/server.go` to add Gin CORS middleware

---

## Testing

Use the provided test script:

```powershell
.\test-phase1.ps1
```

Or test individual endpoints:

```powershell
# Health check
curl http://localhost:8090/healthz

# Status
curl http://localhost:8090/api/status

# Clusters
curl http://localhost:8090/api/clusters

# Sync
curl -X POST http://localhost:8090/api/sync

# Cache
curl -X DELETE http://localhost:8090/api/cache
```

---

## Further Reading

- [QUICKSTART.md](QUICKSTART.md) - Quick start guide
- [ARCHITECTURE.md](ARCHITECTURE.md) - Internal architecture
- [VERIFICATION_PLAN.md](VERIFICATION_PLAN.md) - Testing plan
- [README.md](README.md) - Project overview
