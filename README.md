# local-tunn

Tunnel into private Kubernetes services from your local machine via a Docker container.

```
localhost:6060 → [client container] → WebSocket → [server pod] → postgres.app.svc:5432
```

## Server Setup (K8s)

### Config Mode 1: YAML (default)

Server maps service IDs to real k8s service addresses. Clients only need to know the ID.

```yaml
# config.yaml
addr: ":8080"

clients:
  - name: "alice"
    token: "tok_alice_abc123"
    services:
      - id: "postgres"
        name: "PostgreSQL (app)"
        address: "postgres.app.svc.cluster.local:5432"
      - id: "redis"
        name: "Redis (cache)"
        address: "redis.cache.svc.cluster.local:6379"

  - name: "bob"
    token: "tok_bob_xyz789"
    services:
      - id: "postgres"
        name: "PostgreSQL (app)"
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

Your API must implement two endpoints:

**`GET /services`** — list services for a token

```
Authorization: Bearer <client-token>
```

```json
{
  "name": "alice",
  "services": [
    { "id": "postgres", "name": "PostgreSQL (app)" },
    { "id": "redis", "name": "Redis (cache)" }
  ]
}
```

**`GET /resolve?service=<id>`** — resolve service ID to address

```
Authorization: Bearer <client-token>
```

```json
{
  "address": "postgres.app.svc.cluster.local:5432"
}
```

Return `401` for invalid tokens, or non-200 for disallowed services.

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
? Select service
> PostgreSQL (app) (postgres)
  Redis (cache) (redis)

Tunneling PostgreSQL (app) on localhost:6060
INFO tunnel established server=ws://tunn-server.example.com/ws
INFO listening local=0.0.0.0:6060 service=postgres
```

Then connect:

```bash
psql -h localhost -p 6060 -U myuser mydb
```

The client auto-assigns port 6060. If 6060 is taken, it tries 6061, 6062, etc.

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
