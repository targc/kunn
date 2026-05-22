# Configuration Spec

## Server Config (YAML)

File: `/etc/tunn/config.yaml` (mounted via ConfigMap in k8s)

```yaml
addr: ":8080"

clients:
  - name: "dev-alice"
    token: "tok_alice_abc123"
    services:
      - "postgres.app.svc.cluster.local:5432"
      - "redis.cache.svc.cluster.local:6379"
      - "api.backend.svc.cluster.local:8080"

  - name: "dev-bob"
    token: "tok_bob_xyz789"
    services:
      - "postgres.app.svc.cluster.local:5432"
```

### Rules
- Each token must be unique
- Services are exact-match k8s DNS names with port
- Client can only open streams to services listed under their token
- Config is loaded at startup; restart server to reload (keep it simple)

## Client Config (Env Vars)

| Env Var | Required | Example | Description |
|---------|----------|---------|-------------|
| `TUNN_SERVER` | yes | `wss://tunn.example.com/ws` | Server WebSocket URL |
| `TUNN_TOKEN` | yes | `tok_alice_abc123` | Auth token |
| `TUNN_FORWARDS` | yes | `5432:postgres.app.svc.cluster.local:5432,6379:redis.cache.svc.cluster.local:6379` | Comma-separated `localPort:remoteHost:remotePort` |

### TUNN_FORWARDS format
```
<local_port>:<service_host>:<service_port>[,...]
```

Example:
```
5432:postgres.app.svc.cluster.local:5432,6379:redis.cache.svc.cluster.local:6379
```

This means:
- `localhost:5432` → tunnel to `postgres.app.svc.cluster.local:5432`
- `localhost:6379` → tunnel to `redis.cache.svc.cluster.local:6379`

## Server Env Vars

| Env Var | Required | Default | Description |
|---------|----------|---------|-------------|
| `TUNN_CONFIG` | no | `/etc/tunn/config.yaml` | Path to config YAML |
| `TUNN_ADDR` | no | from config | Override listen address |
