# local-tunn

Tunnel into private Kubernetes services from your local machine. Supports multiple k8s clusters through agents.

```
localhost:6060 → [client] → WebSocket → [server] → WebSocket → [agent in k8s] → postgres.svc:5432
```

## Architecture

```
┌────────────┐        ┌────────────┐        ┌─────────────────────┐
│   Client   │── WS ─►│   Server   │◄─ WS ──│  Agent (cluster-a)  │
│  (local)   │        │  (public)  │        │  ├─► postgres.svc    │
└────────────┘        │            │        │  └─► redis.svc       │
                      │            │        └─────────────────────┘
                      │            │        ┌─────────────────────┐
                      │            │◄─ WS ──│  Agent (cluster-b)  │
                      └────────────┘        │  └─► grafana.svc     │
                                            └─────────────────────┘
```

- **Server** — public-facing, manages auth and routing
- **Agent** — runs in each k8s cluster, connects outbound (no exposed ports)
- **Client** — interactive container on local machine

## Server Setup

### Config (YAML)

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

### Config (Webhook)

Set `TUNN_WEBHOOK_URL` to delegate to an external API:

| Endpoint | Response |
|----------|----------|
| `GET /projects` | `{"name":"alice","projects":[{"id":"app","name":"Main App"}]}` |
| `GET /services?project=app` | `{"services":[{"id":"postgres","name":"PostgreSQL"}]}` |
| `GET /resolve?project=app&service=postgres` | `{"cluster":"cluster-a","address":"postgres.svc:5432"}` |
| `GET /agent-auth` | `{"cluster":"cluster-a"}` |

All endpoints use `Authorization: Bearer <token>`. Return `401` for invalid tokens.

### Run Server

```bash
# YAML mode
TUNN_CONFIG=config.yaml go run ./cmd/server

# Webhook mode
TUNN_WEBHOOK_URL=https://api.example.com go run ./cmd/server
```

## Agent Setup (per k8s cluster)

Agent runs inside the cluster and connects outbound to the server.

```bash
docker build -f Dockerfile.agent -t tunn-agent .
```

```yaml
# k8s deployment
env:
  - name: TUNN_SERVER
    value: "ws://tunn-server.example.com/ws/agent"
  - name: TUNN_AGENT_TOKEN
    value: "agent_tok_cluster_a"
```

Or run locally for testing:

```bash
TUNN_SERVER=ws://localhost:8080/ws/agent \
TUNN_AGENT_TOKEN=agent_tok_cluster_a \
go run ./cmd/agent
```

## Client Usage

```bash
docker build -f Dockerfile.client -t tunn-client .

docker run -it --rm --network host \
  -e TUNN_SERVER=ws://tunn-server.example.com/ws/client \
  -e TUNN_TOKEN=tok_alice_abc123 \
  tunn-client
```

```
? Select project
> Main App (app)
  Monitoring (monitoring)

? Select service
> PostgreSQL (postgres)
  Redis (redis)

Tunneling Main App → PostgreSQL on localhost:6060
INFO tunnel established
INFO listening local=0.0.0.0:6060 service=postgres
```

```bash
psql -h localhost -p 6060 -U myuser mydb
```

Port auto-assigns starting from 6060.

## Environment Variables

### Client

| Var | Required | Example |
|-----|----------|---------|
| `TUNN_SERVER` | yes | `ws://server:8080/ws/client` |
| `TUNN_TOKEN` | yes | `tok_alice_abc123` |

### Agent

| Var | Required | Example |
|-----|----------|---------|
| `TUNN_SERVER` | yes | `ws://server:8080/ws/agent` |
| `TUNN_AGENT_TOKEN` | yes | `agent_tok_cluster_a` |

### Server

| Var | Required | Description |
|-----|----------|-------------|
| `TUNN_CONFIG` | no | Path to YAML config (default: `/etc/tunn/config.yaml`) |
| `TUNN_WEBHOOK_URL` | no | Webhook API base URL (overrides YAML mode) |
| `TUNN_ADDR` | no | Listen address (default: `:8080` or from YAML) |
