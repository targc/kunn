# System Architecture

## Overview

```
┌─────────────────────────────────────────────┐
│              Local Machine                   │
│                                              │
│  App ──► localhost:5432 ──► ┌──────────────┐ │
│  App ──► localhost:6379 ──► │  tunn-client  │ │
│                             │  (container)  │─┼──── WebSocket (single port) ───┐
│                             └──────────────┘ │                                 │
└─────────────────────────────────────────────┘                                 │
                                                                                │
┌───────────────────────────────────────────────────────────────────────────────┤
│              Kubernetes Cluster                                               │
│                                                                               │
│  ┌──────────────┐                                                            │
│  │  tunn-server  │◄───────────────────────────────────────────────────────────┘
│  │  (pod :8080)  │
│  │               │──── dial ──► postgres.app.svc:5432
│  │               │──── dial ──► redis.cache.svc:6379
│  └──────────────┘
└───────────────────────────────────────────────────────────────────────────────┘
```

## Protocol Stack

```
TCP data (postgres, redis, etc.)
        ↕
yamux stream (multiplexing, flow control)
        ↕
WebSocket (binary frames)
        ↕
HTTP/TLS (single public port)
```

## Stream Handshake

Each yamux stream starts with a one-line handshake:

```
Client opens yamux stream
Client writes: "postgres.app.svc.cluster.local:5432\n"
Server reads service name, validates against token's allowed services
Server dials the k8s service
Both sides: io.Copy bidirectionally
```

No custom message types. No stream IDs to manage. yamux handles it all.

## Key Design Decisions

1. **yamux over WebSocket** — yamux gives us multiplexing + flow control. WebSocket gives us a single HTTP port that works through any ingress/LB.
2. **net.Conn adapter** — Wrap the WebSocket connection to implement `net.Conn` so yamux can use it directly.
3. **Service name as first line** — Simplest possible handshake. No binary encoding.
4. **Server dials, client listens** — Server has k8s network access. Client listens on local ports.
5. **Reconnect with backoff** — Client auto-reconnects. Local listeners stay open.
