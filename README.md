# kunn

Kubernetes tunnel for accessing private services across multiple clusters. Agents run inside each cluster with zero exposed ports — traffic flows through a single public WebSocket server.

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
- **Client** — interactive CLI on local machine, supports browser-based login

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

Set `KUNN_WEBHOOK_URL` to delegate to an external API:

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
KUNN_CONFIG=config.yaml go run ./cmd/server

# Webhook mode
KUNN_WEBHOOK_URL=https://api.example.com go run ./cmd/server
```

## Agent Setup (per k8s cluster)

Agent runs inside the cluster and connects outbound to the server.

```yaml
# k8s deployment
containers:
  - name: kunn-agent
    image: ghcr.io/targc/kunn/agent:latest
    env:
      - name: KUNN_SERVER
        value: "ws://kunn-server.example.com/ws/agent"
      - name: KUNN_AGENT_TOKEN
        value: "agent_tok_cluster_a"
```

## Client Usage

```bash
# With token
docker run -it --rm --network host \
  -e KUNN_SERVER=ws://kunn-server.example.com/ws/client \
  -e KUNN_TOKEN=tok_alice_abc123 \
  ghcr.io/targc/kunn/client:latest

# With browser login
docker run -it --rm --network host \
  -e KUNN_SERVER=ws://kunn-server.example.com/ws/client \
  -e KUNN_AUTH_URL=https://auth.example.com/login \
  ghcr.io/targc/kunn/client:latest
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
| `KUNN_SERVER` | yes | `ws://server:8080/ws/client` |
| `KUNN_TOKEN` | no | `tok_alice_abc123` (or saved in `~/.kunn/token`) |
| `KUNN_AUTH_URL` | no | `https://auth.example.com/login` (for browser login) |

### Agent

| Var | Required | Example |
|-----|----------|---------|
| `KUNN_SERVER` | yes | `ws://server:8080/ws/agent` |
| `KUNN_AGENT_TOKEN` | yes | `agent_tok_cluster_a` |

### Server

| Var | Required | Description |
|-----|----------|-------------|
| `KUNN_CONFIG` | no | Path to YAML config (default: `/etc/kunn/config.yaml`) |
| `KUNN_WEBHOOK_URL` | no | Webhook API base URL (overrides YAML mode) |
| `KUNN_ADDR` | no | Listen address (default: `:8080` or from YAML) |
