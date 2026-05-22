# local-tunn

Tunnel into private Kubernetes services from your local machine via a Docker container.

```
localhost:6060 → [client container] → WebSocket → [server pod] → postgres.app.svc:5432
```

## Server Setup (K8s)

### Config Mode 1: YAML (default)

```yaml
# config.yaml
addr: ":8080"

clients:
  - name: "alice"
    token: "tok_alice_abc123"
    projects:
      - id: "app"
        name: "Main App"
        services:
          - id: "postgres"
            name: "PostgreSQL"
            address: "postgres.app.svc.cluster.local:5432"
          - id: "redis"
            name: "Redis"
            address: "redis.app.svc.cluster.local:6379"
      - id: "monitoring"
        name: "Monitoring"
        services:
          - id: "grafana"
            name: "Grafana"
            address: "grafana.monitoring.svc.cluster.local:3000"

  - name: "bob"
    token: "tok_bob_xyz789"
    projects:
      - id: "app"
        name: "Main App"
        services:
          - id: "postgres"
            name: "PostgreSQL"
            address: "postgres.app.svc.cluster.local:5432"
```

```bash
TUNN_CONFIG=config.yaml go run ./cmd/server
```

### Config Mode 2: Webhook

Delegate auth and service resolution to an external API.

```bash
TUNN_WEBHOOK_URL=https://api.example.com go run ./cmd/server
```

Your API must implement:

**`GET /projects`** — list projects for a token

```
Authorization: Bearer <client-token>
```
```json
{
  "name": "alice",
  "projects": [
    { "id": "app", "name": "Main App" },
    { "id": "monitoring", "name": "Monitoring" }
  ]
}
```

**`GET /services?project=<id>`** — list services in a project

```json
{
  "services": [
    { "id": "postgres", "name": "PostgreSQL" },
    { "id": "redis", "name": "Redis" }
  ]
}
```

**`GET /resolve?project=<id>&service=<id>`** — resolve to address

```json
{ "address": "postgres.app.svc.cluster.local:5432" }
```

Return `401` for invalid tokens.

### Deploy

```bash
docker build -f Dockerfile.server -t tunn-server .

kubectl create configmap tunn-server-config --from-file=config.yaml
kubectl apply -f deploy/server.yaml
```

## Client Usage

### 1. Build

```bash
docker build -f Dockerfile.client -t tunn-client .
```

### 2. Run

```bash
docker run -it --rm --network host \
  -e TUNN_SERVER=ws://tunn-server.example.com/ws \
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
INFO tunnel established server=ws://tunn-server.example.com/ws
INFO listening local=0.0.0.0:6060 service=postgres
```

Then connect:

```bash
psql -h localhost -p 6060 -U myuser mydb
```

Port auto-assigns starting from 6060. If taken, tries 6061, 6062, etc.

## Environment Variables

### Client

| Var | Required | Example |
|-----|----------|---------|
| `TUNN_SERVER` | yes | `ws://tunn-server:8080/ws` |
| `TUNN_TOKEN` | yes | `tok_alice_abc123` |

### Server

| Var | Required | Description |
|-----|----------|-------------|
| `TUNN_CONFIG` | no | Path to YAML config (default: `/etc/tunn/config.yaml`) |
| `TUNN_WEBHOOK_URL` | no | Webhook API base URL (overrides YAML mode) |
| `TUNN_ADDR` | no | Listen address (default: `:8080` or from YAML) |
