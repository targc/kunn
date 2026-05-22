# System Flow

## 1. Client Startup

```
1. Client reads env vars: TUNN_SERVER, TUNN_TOKEN, TUNN_FORWARDS
2. Client dials WebSocket: GET /ws + Authorization: Bearer <token>
3. Server validates token → accepts or 401
4. Both sides wrap WS conn in yamux (client=yamux.Client, server=yamux.Server)
5. Client starts local TCP listeners per forward
```

## 2. TCP Connection Through Tunnel

```
App connects to localhost:5432
  → Client accepts local TCP conn
  → Client opens yamux stream
  → Client writes "postgres.app.svc.cluster.local:5432\n"
  → Server reads service name, checks token allows it
  → Server dials postgres.app.svc.cluster.local:5432
  → io.Copy both directions until either side closes
```

## 3. Disconnect & Reconnect

```
WS drops → yamux session breaks → all streams EOF
  → Client closes all local TCP connections
  → Local listeners stay open
  → Client reconnects with backoff: 1s, 2s, 4s, ... max 30s
  → New yamux session established
  → New local connections work again
```
