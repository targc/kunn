# Code Project Structure

```
local-tunn/
├── cmd/
│   ├── server/
│   │   └── main.go           # Server entrypoint
│   └── client/
│       └── main.go           # Client entrypoint
├── internal/
│   ├── wsconn/
│   │   └── wsconn.go         # net.Conn adapter for WebSocket
│   ├── server/
│   │   ├── server.go         # HTTP server, WS upgrade, yamux, stream handler
│   │   └── config.go         # YAML config loader
│   └── client/
│       └── client.go         # WS connect, yamux, local listeners, reconnect
├── config.example.yaml
├── Dockerfile.server
├── Dockerfile.client
├── docker-compose.yaml
├── go.mod
└── go.sum
```

~7 files of actual Go code. ~300 lines total.
