# Changes from Current Codebase

## New Files

| File | Purpose |
|------|---------|
| `internal/agent/agent.go` | Agent: WS connect to server, yamux.Server, accept streams, dial + proxy |
| `cmd/agent/main.go` | Agent entrypoint |
| `Dockerfile.agent` | Agent container image |

## Modified Files

### `internal/server/server.go`
- Split `/ws` into `/ws/client` and `/ws/agent`
- Add agent registry: `agents map[string]*yamux.Session` with `sync.RWMutex`
- `handleWS` → `handleClientWS` (yamux.Server for client streams)
- New `handleAgentWS` (validate agent token, register yamux.Client session)
- `handleStream`: no more `net.Dial` — instead look up agent session, open stream to agent, write address, bridge two streams

### `internal/server/config.go`
- Add `ServiceRoute` struct: `{Cluster, Address}`
- Change `ResolveService` return type: `(string, error)` → `(ServiceRoute, error)`
- Add `ValidAgentToken(token) (clusterID, bool)` to `IConfig`
- YAML: add `agents` section and `cluster` field on services

### `internal/server/config_webhook.go`
- Update `ResolveService` to return `ServiceRoute`
- Add `ValidAgentToken` → calls new webhook endpoint `GET /agent-auth`
- Update `/resolve` response to include cluster

New webhook endpoint:

**`GET /agent-auth`** — validate agent token, return cluster ID

```
Authorization: Bearer <agent-token>
```
```json
{ "cluster": "cluster-a" }
```
Return `401` for invalid agent tokens.

### `internal/client/client.go`
- No changes needed

### `cmd/client/main.go`
- No changes needed

### `cmd/server/main.go`
- No changes needed (server.New signature unchanged)

## New Project Structure

```
local-tunn/
├── cmd/
│   ├── server/main.go
│   ├── client/main.go
│   └── agent/main.go          ← NEW
├── internal/
│   ├── wsconn/wsconn.go
│   ├── server/
│   │   ├── server.go          ← MODIFIED (agent registry, stream bridging)
│   │   ├── config.go          ← MODIFIED (ServiceRoute, agent auth)
│   │   └── config_webhook.go  ← MODIFIED
│   ├── client/client.go
│   └── agent/
│       └── agent.go           ← NEW
├── Dockerfile.server
├── Dockerfile.client
├── Dockerfile.agent            ← NEW
├── config.example.yaml         ← MODIFIED (agents section)
└── docker-compose.yaml
```

## Config Example (Updated)

```yaml
addr: ":8080"

agents:
  - cluster: "cluster-a"
    token: "agent_tok_cluster_a"
  - cluster: "cluster-b"
    token: "agent_tok_cluster_b"

clients:
  - name: "alice"
    token: "tok_alice_abc123"
    projects:
      - id: "app"
        name: "Main App"
        services:
          - id: "postgres"
            name: "PostgreSQL"
            cluster: "cluster-a"
            address: "postgres.app.svc.cluster.local:5432"
          - id: "redis"
            name: "Redis"
            cluster: "cluster-a"
            address: "redis.app.svc.cluster.local:6379"
      - id: "monitoring"
        name: "Monitoring"
        services:
          - id: "grafana"
            name: "Grafana"
            cluster: "cluster-b"
            address: "grafana.monitoring.svc.cluster.local:3000"
```

## Agent Env Vars

| Var | Required | Example |
|-----|----------|---------|
| `TUNN_SERVER` | yes | `ws://tunn-server:8080/ws/agent` |
| `TUNN_AGENT_TOKEN` | yes | `agent_tok_cluster_a` |
