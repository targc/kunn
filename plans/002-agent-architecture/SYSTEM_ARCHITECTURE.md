# System Architecture

## Overview

```
┌──────────────┐          ┌──────────────┐          ┌──────────────────────────┐
│ Local Machine │          │   Server     │          │  K8s Cluster A           │
│              │          │  (public)    │          │                          │
│  Client ─────┼── WS ──►│              │◄── WS ──┼── Agent (cluster-a)      │
│  localhost:   │          │  /ws/client  │          │      │                   │
│    6060       │          │  /ws/agent   │          │      ├─► postgres.svc    │
│              │          │  /projects   │          │      └─► redis.svc       │
└──────────────┘          │  /services   │          └──────────────────────────┘
                          │              │
                          │              │          ┌──────────────────────────┐
                          │              │          │  K8s Cluster B           │
                          │              │◄── WS ──┼── Agent (cluster-b)      │
                          │              │          │      │                   │
                          └──────────────┘          │      └─► grafana.svc     │
                                                    └──────────────────────────┘
```

## Three Components

### Server (public, single instance)

- Exposes HTTP/WS on one port
- Two WebSocket endpoints:
  - `/ws/agent` — agents connect here (bearer token + `X-Cluster-ID` header)
  - `/ws/client` — clients connect here (bearer token)
- REST endpoints: `/projects`, `/services?project=<id>`
- Routes client streams to the correct agent based on service → cluster mapping
- Manages agent registry: `map[clusterID]*agentSession` (yamux sessions)

### Agent (per k8s cluster, no exposed ports)

- Connects outbound to server via WebSocket
- Identifies itself with cluster ID
- Receives yamux sessions from server
- For each stream: reads target service address, dials it, proxies data
- Auto-reconnects on disconnect

### Client (local machine, interactive)

- Same as before — no changes to client UX
- Connects to server, picks project → service
- Server transparently routes to the correct agent

## Protocol

### Agent ↔ Server

```
Agent connects: GET /ws/agent
  Authorization: Bearer <agent-token>
  X-Cluster-ID: cluster-a

Server wraps WS in yamux.Client (server initiates streams TO agent)
Agent wraps WS in yamux.Server (agent accepts streams FROM server)
```

When client opens a stream for a service on cluster-a:
1. Server looks up agent for cluster-a
2. Server opens a yamux stream to that agent
3. Server writes service address as first line: `postgres.app.svc.cluster.local:5432\n`
4. Agent reads address, dials it, proxies bidirectionally

### Client ↔ Server

Same as current — client opens yamux stream, writes `projectID/serviceID\n`, server resolves to cluster + address, then bridges to agent.

## Config Changes

### YAML — add `cluster` field to services

```yaml
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
          - id: "grafana"
            name: "Grafana"
            cluster: "cluster-b"
            address: "grafana.monitoring.svc.cluster.local:3000"

agents:
  - cluster: "cluster-a"
    token: "agent_tok_cluster_a"
  - cluster: "cluster-b"
    token: "agent_tok_cluster_b"
```

Each agent has a unique token. Server resolves token → cluster ID, so agents don't need to send `X-Cluster-ID` header.

### IConfig changes

```go
type IConfig interface {
    // existing
    ValidToken(token string) bool
    ClientName(token string) string
    ClientProjects(token string) ([]ProjectInfo, error)
    ClientServices(token, projectID string) ([]ServiceInfo, error)

    // changed — returns cluster + address
    ResolveService(token, projectID, serviceID string) (ServiceRoute, error)

    // new — agent auth (token → cluster ID)
    ValidAgentToken(token string) (clusterID string, ok bool)
}

type ServiceRoute struct {
    Cluster string
    Address string
}
```

## Key Design Decision: Who Initiates Streams?

**Server opens streams to agent** (not the reverse). This means:
- Server = yamux.Client toward agent (opens streams)
- Agent = yamux.Server (accepts streams)

This is the opposite of the client↔server direction:
- Client = yamux.Client toward server (opens streams)
- Server = yamux.Server toward client (accepts streams)

```
Client ──yamux.Open()──► Server ──yamux.Open()──► Agent
         (client side)            (server side)
```

The server is yamux.Server for clients and yamux.Client for agents.
