# Agent Architecture — Adding Cluster Agents

> Change from `client → server → target` to `client → server → agent → target`

## Related Files

- [SYSTEM_ARCHITECTURE.md](./SYSTEM_ARCHITECTURE.md) — New 3-tier architecture
- [SYSTEM_FLOW.md](./SYSTEM_FLOW.md) — Connection flows for all three components
- [CHANGES.md](./CHANGES.md) — What to change in existing code

---

## Current Architecture

```
Client → (WebSocket) → Server → (net.Dial) → k8s service
```

Server directly dials backend services. Server must live inside the k8s cluster.

## New Architecture

```
Client → (WebSocket) → Server → (yamux over WebSocket) → Agent → (net.Dial) → k8s service
```

- **Server**: Public-facing, can live anywhere (cloud VM, managed k8s, etc.)
- **Agent**: Runs inside each k8s cluster, connects outbound to server, no exposed ports
- **Client**: Same as before — interactive container on local machine

## Core Requirements

| # | Requirement |
|---|------------|
| R1 | Agent runs as a pod in k8s, connects outbound to server via WebSocket |
| R2 | Agent exposes zero ports — initiates connection to server |
| R3 | Server manages multiple agents (one per cluster) |
| R4 | Server manages multiple clients simultaneously |
| R5 | Config maps each service to a cluster ID — agent auto-selected |
| R6 | Client UX unchanged — user picks project → service, doesn't know about clusters |
| R7 | If agent for a cluster is offline, service shows as unavailable |

## What This Enables

- **Multi-cluster**: One server can bridge services from many k8s clusters
- **No exposed ports on k8s**: Agents connect outbound, NAT-friendly
- **Centralized access control**: Server manages who can access what
- **Agent independence**: Agents don't know about clients or projects, they just dial what the server tells them
