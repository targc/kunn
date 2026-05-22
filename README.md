# local-tunn

Tunnel into private Kubernetes services from your local machine via a Docker container.

```
localhost:5432 → [client container] → WebSocket → [server pod] → postgres.app.svc:5432
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
Available services:
  1) postgres
  2) redis

Select service: 1
Local port for 'postgres': 5432

INFO tunnel established server=ws://tunn-server.example.com/ws
INFO listening local=0.0.0.0:5432 service=postgres
```

Then from your host:

```bash
psql -h localhost -p 5432 -U myuser mydb
```

## Environment Variables

### Client

| Var | Required | Example |
|-----|----------|---------|
| `TUNN_SERVER` | yes | `ws://tunn-server:8080/ws` |
| `TUNN_TOKEN` | yes | `tok_alice_abc123` |

### Server

| Var | Required | Default |
|-----|----------|---------|
| `TUNN_CONFIG` | no | `/etc/tunn/config.yaml` |
| `TUNN_ADDR` | no | from config file |
