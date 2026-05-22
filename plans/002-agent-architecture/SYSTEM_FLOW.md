# System Flow

## 1. Agent Startup

```
Agent reads env: TUNN_SERVER, TUNN_AGENT_TOKEN
  → Connects WS to /ws/agent with Bearer token
  → Server validates token, resolves cluster ID from token, registers agent session
  → Agent wraps WS in yamux.Server, waits for streams
  → On disconnect: reconnect with backoff
```

## 2. Client Selects Service

```
Client fetches projects → user picks project
Client fetches services → user picks service (e.g. "postgres")
  → No cluster info shown to user
```

## 3. End-to-End Stream Flow

```
    Client                     Server                      Agent
      │                          │                           │
 App connects                    │                           │
 localhost:6060                  │                           │
      │                          │                           │
      │── yamux.Open() ─────────►│                           │
      │── "app/postgres\n" ─────►│                           │
      │                          │ resolve: cluster-a,       │
      │                          │   postgres.svc:5432       │
      │                          │                           │
      │                          │── yamux.Open() ──────────►│
      │                          │── "postgres.svc:5432\n" ─►│
      │                          │                           │── net.Dial(postgres.svc:5432)
      │                          │                           │
      │◄── data ────────────────►│◄── data ─────────────────►│◄── data ──► postgres
      │                          │                           │
      │── close ────────────────►│── close ─────────────────►│── close
```

## 4. Agent Offline

```
Client selects service on cluster-a
  → Server looks up agent for cluster-a
  → No agent connected
  → Server closes client stream (service unavailable)
  → Client shows error to user
```

## 5. Agent Reconnect

```
Agent WS drops
  → Server removes agent from registry
  → All active streams to that agent close
  → Agent reconnects with backoff
  → Server re-registers agent
  → New client connections work again
```

## 6. Data Bridge in Server

The server bridges two yamux streams — one from client, one to agent:

```go
// Server handleStream (simplified)
func handleStream(clientStream net.Conn, token, name string) {
    // 1. Read project/service from client stream
    // 2. Resolve → cluster + address
    // 3. Get agent yamux session for cluster
    // 4. Open stream to agent
    // 5. Write address to agent stream
    // 6. io.Copy both directions: clientStream ↔ agentStream
}
```

No net.Dial on the server anymore — that moves to the agent.
