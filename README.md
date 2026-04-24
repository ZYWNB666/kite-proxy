# kite-proxy

A lightweight Kubernetes API forwarding proxy that bridges local `kubectl` to remote clusters managed by a [kite](https://github.com/zxh326/kite) server.

> 🎉 **NEW:** [Desktop Application Available!](DESKTOP_QUICKSTART.md) - Double-click to run, no browser needed!

## 🚀 Quick Start

### Desktop App (Recommended)

**For end users who want a simple double-click experience:**

```powershell
# Build the desktop app
.\build-desktop.ps1

# Run it
.\build\bin\kite-proxy.exe
```

👉 See [Desktop Quick Start Guide](DESKTOP_QUICKSTART.md) for detailed instructions.

### Web Server

**For server deployments or advanced usage:**

```bash
go run main.go --kite-url https://kite.example.com --api-key your-key
# Visit http://localhost:8090
```

👉 See [Web Quick Start Guide](QUICKSTART.md) for detailed instructions.

## Architecture

```
kubectl  ──►  kite-proxy  ──►  kite server  ──►  Kubernetes API
              (memory)         (RBAC check)       (real cluster)
```

Key design principles:
- **Kubeconfigs are never written to disk** – they are fetched from kite over the network and kept only in process memory.
- **Multiple connections to the same cluster reuse the same in-memory kubeconfig** (no redundant network calls).
- **Proxy permission is controlled in kite's RBAC** – only API-key users with `allowProxy: true` can retrieve kubeconfigs.

## Prerequisites

1. A running [kite](https://github.com/zxh326/kite) server (v0.x+).
2. An API key created in kite (`Admin → API Keys`) with a role that has `allowProxy: true`.

## Running kite-proxy

### From source

```bash
# Build the frontend first
cd ui
npm install
npm run build
cd ..

# Build and run the Go binary
go build -o kite-proxy .
./kite-proxy \
  --port 8090 \
  --kite-url https://kite.example.com \
  --api-key kite123-<your-api-key>
```

### Environment variables

| Variable      | Default | Description                             |
|---------------|---------|-----------------------------------------|
| `PORT`        | `8090`  | Port kite-proxy listens on              |
| `KITE_URL`    | –       | Base URL of the kite server             |
| `KITE_API_KEY`| –       | API key for authenticating with kite    |

CLI flags (`--port`, `--kite-url`, `--api-key`) override environment variables.

## Using the Web UI

Open `http://localhost:8090/ui/` in your browser:

1. **Configuration** – enter your kite server URL and API key.
2. **Clusters** – view clusters you have proxy access to; optionally pre-warm their kubeconfigs.
3. **Usage** – download a generated `kubeconfig` file and see `kubectl` examples.

## Using with kubectl

kite-proxy exposes each cluster at:

```
http://localhost:8090/proxy/<cluster-name>/
```

### Option A – generated kubeconfig (recommended)

```bash
# Download from the UI or via API
curl http://localhost:8090/api/kubeconfig -o kubeconfig-kite-proxy.yaml
export KUBECONFIG=kubeconfig-kite-proxy.yaml
kubectl get pods -A
```

### Option B – inline server flag

```bash
kubectl --server=http://localhost:8090/proxy/my-cluster \
        --insecure-skip-tls-verify \
        get pods -n default
```

## kite server configuration

### 1. Create an API key

In kite: **Admin → API Keys → Create**.

### 2. Create a role with proxy permission

In kite: **Admin → Roles → Create** (or update an existing role):

```json
{
  "name": "proxy-user",
  "clusters": ["*"],
  "namespaces": ["*"],
  "resources": ["*"],
  "verbs": ["get"],
  "allowProxy": true,
  "proxyNamespaces": ["default", "production"]
}
```

- `allowProxy: true` – required to allow kubeconfig fetching via `/api/v1/proxy/kubeconfig`.
- `proxyNamespaces` – optional; restricts which namespaces the user can proxy to. Falls back to `namespaces` if empty.

### 3. Assign the role to the API key user

In kite: **Admin → Roles → proxy-user → Assign → (API key username)**.

## API Reference

| Method | Path                      | Description                                         |
|--------|---------------------------|-----------------------------------------------------|
| `GET`  | `/api/config`             | Get current config (API key masked)                 |
| `POST` | `/api/config`             | Set kite URL and API key                            |
| `GET`  | `/api/clusters`           | List clusters available for proxying                |
| `GET`  | `/api/kubeconfig`         | Download generated local kubeconfig                 |
| `GET`  | `/api/status`             | Status, cached clusters, and sync status            |
| `POST` | `/api/sync`               | Manually trigger sync with kite server              |
| `DELETE`| `/api/cache`             | Clear all cached kubeconfigs from memory            |
| `POST` | `/api/cache/:cluster`     | Pre-warm kubeconfig for a specific cluster          |
| `ANY`  | `/proxy/:cluster/*path`   | Forward requests to the K8s API server              |
| `GET`  | `/healthz`                | Health check                                        |

## Features

### Automatic Synchronization

kite-proxy automatically checks connectivity to the kite server every 5 minutes:
- Verifies the kite server is reachable
- Validates API key authentication
- Updates sync status (visible in `/api/status`)

You can trigger an immediate sync via:
```bash
curl -X POST http://localhost:8090/api/sync
```

### Intelligent Retry

All API requests to the kite server include automatic retry with exponential backoff:
- **Max retries**: 3 attempts
- **Initial backoff**: 500ms
- **Max backoff**: 10 seconds
- **Multiplier**: 2.0x

This ensures resilience against transient network failures.

### In-Memory Cache

Kubeconfigs are cached in memory to minimize network calls:
- First access to a cluster fetches from kite
- Subsequent accesses reuse the cached kubeconfig
- Cache can be cleared via `/api/cache` (DELETE)
- Individual clusters can be refreshed via `/api/cache/:cluster` (POST)

## Security notes

- The API key is stored **only in process memory** – restart kite-proxy to forget it.
- The kubeconfigs fetched from kite are kept **only in process memory**.
- kite-proxy does **not** enforce additional RBAC on forwarded requests; access control is handled by the upstream K8s RBAC and by kite's RBAC (`proxyNamespaces`, etc.).
- In production, run kite-proxy behind a TLS-terminating reverse proxy or restrict access by network policy.
