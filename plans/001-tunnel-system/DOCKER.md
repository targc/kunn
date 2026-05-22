# Docker & Deployment

## Dockerfile.server

```dockerfile
FROM golang:1.23-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /tunn-server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /tunn-server /usr/local/bin/tunn-server
ENTRYPOINT ["tunn-server"]
```

## Dockerfile.client

```dockerfile
FROM golang:1.23-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /tunn-client ./cmd/client

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /tunn-client /usr/local/bin/tunn-client
ENTRYPOINT ["tunn-client"]
```

## Client docker-compose.yaml (local machine)

```yaml
services:
  tunn:
    image: tunn-client:latest
    build:
      context: .
      dockerfile: Dockerfile.client
    environment:
      TUNN_SERVER: "wss://tunn.example.com/ws"
      TUNN_TOKEN: "tok_alice_abc123"
      TUNN_FORWARDS: "5432:postgres.app.svc.cluster.local:5432,6379:redis.cache.svc.cluster.local:6379"
    ports:
      - "5432:5432"
      - "6379:6379"
    restart: unless-stopped
```

Usage:
```bash
docker compose up -d
psql -h localhost -p 5432 -U myuser mydb    # connects to k8s postgres
redis-cli -h localhost -p 6379              # connects to k8s redis
```

## K8s Deployment (server)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: tunn-server-config
data:
  config.yaml: |
    addr: ":8080"
    clients:
      - name: "dev-alice"
        token: "tok_alice_abc123"
        services:
          - "postgres.app.svc.cluster.local:5432"
          - "redis.cache.svc.cluster.local:6379"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tunn-server
spec:
  replicas: 1
  selector:
    matchLabels:
      app: tunn-server
  template:
    metadata:
      labels:
        app: tunn-server
    spec:
      containers:
        - name: tunn-server
          image: tunn-server:latest
          ports:
            - containerPort: 8080
          volumeMounts:
            - name: config
              mountPath: /etc/tunn
      volumes:
        - name: config
          configMap:
            name: tunn-server-config
---
apiVersion: v1
kind: Service
metadata:
  name: tunn-server
spec:
  type: LoadBalancer  # or NodePort, or behind Ingress
  ports:
    - port: 443
      targetPort: 8080
  selector:
    app: tunn-server
```
