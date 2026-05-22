# local-tunn

Tunnel into private Kubernetes services from your local machine. Server runs as a pod in k8s, client runs as a container locally.

```
localhost:5432 → [client] → WebSocket → [server pod] → postgres.app.svc:5432
localhost:6379 → [client] → WebSocket → [server pod] → redis.cache.svc:6379
```

## Server Setup (K8s)

### 1. Create config

Server maps service IDs to real k8s service addresses. Clients only need to know the ID.

```yaml
# config.yaml
addr: ":8080"

clients:
  - name: "alice"
    token: "tok_alice_abc123"
    services:
      - id: "postgres"
        address: "postgres.app.svc.cluster.local:5432"
      - id: "redis"
        address: "redis.cache.svc.cluster.local:6379"

  - name: "bob"
    token: "tok_bob_xyz789"
    services:
      - id: "postgres"
        address: "postgres.app.svc.cluster.local:5432"
```

### 2. Deploy

```bash
docker build -f Dockerfile.server -t tunn-server .

kubectl create configmap tunn-server-config --from-file=config.yaml
kubectl apply -f deploy/server.yaml
```

### Run locally (for testing)

```bash
TUNN_CONFIG=config.example.yaml go run ./cmd/server
```

## Client Setup (Local Machine)

### Option A: Docker Compose (recommended)

```yaml
# docker-compose.yaml
services:
  tunn:
    image: tunn-client:latest
    build:
      context: .
      dockerfile: Dockerfile.client
    environment:
      TUNN_SERVER: "ws://tunn-server.example.com/ws"
      TUNN_TOKEN: "tok_alice_abc123"
      TUNN_FORWARDS: "5432:postgres,6379:redis"
    ports:
      - "5432:5432"
      - "6379:6379"
    restart: unless-stopped
```

```bash
docker compose up -d
```

### Option B: Docker run

```bash
docker run -d \
  -e TUNN_SERVER=ws://tunn-server.example.com/ws \
  -e TUNN_TOKEN=tok_alice_abc123 \
  -e TUNN_FORWARDS=5432:postgres,6379:redis \
  -p 5432:5432 \
  -p 6379:6379 \
  tunn-client
```

### Option C: Binary

```bash
TUNN_SERVER=ws://tunn-server.example.com/ws \
TUNN_TOKEN=tok_alice_abc123 \
TUNN_FORWARDS=5432:postgres,6379:redis \
go run ./cmd/client
```

## Connect

```bash
psql -h localhost -p 5432 -U myuser mydb
redis-cli -h localhost -p 6379
```

## Environment Variables

### Client

| Var | Required | Example |
|-----|----------|---------|
| `TUNN_SERVER` | yes | `ws://tunn-server:8080/ws` |
| `TUNN_TOKEN` | yes | `tok_alice_abc123` |
| `TUNN_FORWARDS` | yes | `5432:postgres,6379:redis` |

`TUNN_FORWARDS` format: `localPort:serviceID` comma-separated.

### Server

| Var | Required | Default |
|-----|----------|---------|
| `TUNN_CONFIG` | no | `/etc/tunn/config.yaml` |
| `TUNN_ADDR` | no | from config file |
