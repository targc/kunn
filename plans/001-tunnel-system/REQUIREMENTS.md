# Local-Tunn: K8s Private Service Tunnel

> Container-based tunnel for accessing private Kubernetes services from local machines.

## Related Analysis Files

- [SYSTEM_ARCHITECTURE.md](./SYSTEM_ARCHITECTURE.md) — Architecture, components, protocol
- [SYSTEM_FLOW.md](./SYSTEM_FLOW.md) — Connection lifecycle and data flow
- [CODE_PROJECT_STRUCTURE.md](./CODE_PROJECT_STRUCTURE.md) — Go project structure and key files
- [CONFIG_SPEC.md](./CONFIG_SPEC.md) — Server YAML config and env vars
- [DOCKER.md](./DOCKER.md) — Dockerfiles and docker-compose for both sides

---

## Core Requirements

| # | Requirement |
|---|------------|
| R1 | Server runs as a K8s pod, exposes single HTTP/WS port |
| R2 | Client runs as a container on local machine |
| R3 | Client initiates outbound WebSocket connection to server |
| R4 | Server supports many simultaneous clients |
| R5 | Server config is YAML mapping token → allowed k8s services |
| R6 | Client exposes tunneled services on localhost ports |
| R7 | Raw TCP tunneling — no TLS termination or SNI routing |

## Approach

Use `hashicorp/yamux` multiplexer over a WebSocket connection. No custom binary protocol. Each yamux stream carries one TCP connection with a simple handshake (service name as first line).

~300 lines of Go total.
