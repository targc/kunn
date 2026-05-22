#!/bin/bash
set -e

REGISTRY="k3d-registry.localhost:5000"
CLUSTER_1="kunn-cluster-1"
CLUSTER_2="kunn-cluster-2"
SERVER_CONTAINER="kunn-server-local"
WEBHOOK_CONTAINER="kunn-webhook-local"
DOCKER_NETWORK="kunn-net"
WEBHOOK_PORT=9090
SERVER_PORT=8080

# -------------------- cleanup --------------------

echo "=== Cleanup ==="

docker rm -f $WEBHOOK_CONTAINER 2>/dev/null || true
docker rm -f $SERVER_CONTAINER 2>/dev/null || true
k3d cluster delete $CLUSTER_1 2>/dev/null || true
k3d cluster delete $CLUSTER_2 2>/dev/null || true
docker network rm $DOCKER_NETWORK 2>/dev/null || true

# -------------------- docker network --------------------

echo "=== Docker Network ==="

docker network create $DOCKER_NETWORK 2>/dev/null || true

# -------------------- registry --------------------

echo "=== Registry ==="

k3d registry create registry.localhost --port 5000 2>/dev/null || true

# -------------------- build images --------------------

echo "=== Build Images ==="

docker build -f Dockerfile.server -t $REGISTRY/kunn-server:latest .
docker push $REGISTRY/kunn-server:latest

docker build -f Dockerfile.agent -t $REGISTRY/kunn-agent:latest .
docker push $REGISTRY/kunn-agent:latest

docker build -f Dockerfile.client -t $REGISTRY/kunn-client:latest .
docker push $REGISTRY/kunn-client:latest

# -------------------- mock webhook config server --------------------

echo "=== Mock Webhook Config Server ==="

docker run -d --name $WEBHOOK_CONTAINER --network $DOCKER_NETWORK -p $WEBHOOK_PORT:8000 python:3.12-slim bash -c '
pip install -q fastapi uvicorn &&
python -c "
from fastapi import FastAPI, Header, HTTPException, Query
app = FastAPI()

AGENTS = {
    \"agent_tok_1\": \"cluster-1\",
    \"agent_tok_2\": \"cluster-2\",
}

CLIENTS = {
    \"tok_dev\": {
        \"name\": \"dev\",
        \"projects\": [
            {
                \"id\": \"app\",
                \"name\": \"Main App\",
                \"services\": [
                    {\"id\": \"pg-1\", \"name\": \"PostgreSQL (cluster-1)\", \"cluster\": \"cluster-1\", \"address\": \"postgres.default.svc.cluster.local:5432\"},
                    {\"id\": \"pg-2\", \"name\": \"PostgreSQL (cluster-2)\", \"cluster\": \"cluster-2\", \"address\": \"postgres.default.svc.cluster.local:5432\"},
                    {\"id\": \"redis-1\", \"name\": \"Redis (cluster-1)\", \"cluster\": \"cluster-1\", \"address\": \"redis.default.svc.cluster.local:6379\"},
                ],
            }
        ],
    }
}

def get_token(authorization: str = Header()):
    return authorization.removeprefix(\"Bearer \")

@app.get(\"/projects\")
def projects(authorization: str = Header()):
    t = get_token(authorization)
    c = CLIENTS.get(t)
    if not c: raise HTTPException(401)
    return {\"name\": c[\"name\"], \"projects\": [{\"id\": p[\"id\"], \"name\": p[\"name\"]} for p in c[\"projects\"]]}

@app.get(\"/services\")
def services(project: str = Query(), authorization: str = Header()):
    t = get_token(authorization)
    c = CLIENTS.get(t)
    if not c: raise HTTPException(401)
    for p in c[\"projects\"]:
        if p[\"id\"] == project:
            return {\"services\": [{\"id\": s[\"id\"], \"name\": s[\"name\"]} for s in p[\"services\"]]}
    raise HTTPException(404)

@app.get(\"/resolve\")
def resolve(project: str = Query(), service: str = Query(), authorization: str = Header()):
    t = get_token(authorization)
    c = CLIENTS.get(t)
    if not c: raise HTTPException(401)
    for p in c[\"projects\"]:
        if p[\"id\"] == project:
            for s in p[\"services\"]:
                if s[\"id\"] == service:
                    return {\"cluster\": s[\"cluster\"], \"address\": s[\"address\"]}
    raise HTTPException(404)

@app.get(\"/agent-auth\")
def agent_auth(authorization: str = Header()):
    t = get_token(authorization)
    cluster = AGENTS.get(t)
    if not cluster: raise HTTPException(401)
    return {\"cluster\": cluster}

from fastapi.responses import RedirectResponse

@app.get(\"/login\")
def login(port: int = Query()):
    return RedirectResponse(f\"http://localhost:{port}/callback?token=tok_dev\")

import uvicorn
uvicorn.run(app, host=\"0.0.0.0\", port=8000)
"'

echo "Waiting for webhook server..."
until curl -sf http://localhost:$WEBHOOK_PORT/docs > /dev/null 2>&1; do sleep 1; done
echo "Webhook server ready"

# -------------------- tunnel server --------------------

echo "=== Tunnel Server ==="

docker run -d --name $SERVER_CONTAINER --network $DOCKER_NETWORK -p $SERVER_PORT:8080 \
  -e KUNN_WEBHOOK_URL=http://$WEBHOOK_CONTAINER:8000 \
  $REGISTRY/kunn-server:latest

echo "Waiting for tunnel server..."
until curl -sf http://localhost:$SERVER_PORT/projects -H "Authorization: Bearer tok_dev" > /dev/null 2>&1; do sleep 1; done
echo "Tunnel server ready"

# -------------------- k8s cluster 1 --------------------

echo "=== K8s Cluster 1 ==="

k3d cluster create $CLUSTER_1 \
  --image rancher/k3s:v1.31.11-k3s1 \
  --servers 1 \
  --agents 1 \
  --registry-use $REGISTRY \
  --k3s-arg "--disable=traefik@server:*" \
  --network $DOCKER_NETWORK

kubectl config use-context k3d-$CLUSTER_1

kubectl apply -f - <<'EOF'
apiVersion: v1
kind: Pod
metadata:
  name: postgres
  labels:
    app: postgres
spec:
  containers:
    - name: postgres
      image: postgres:16-alpine
      env:
        - name: POSTGRES_PASSWORD
          value: "testpass"
      ports:
        - containerPort: 5432
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
spec:
  selector:
    app: postgres
  ports:
    - port: 5432
---
apiVersion: v1
kind: Pod
metadata:
  name: redis
  labels:
    app: redis
spec:
  containers:
    - name: redis
      image: redis:7-alpine
      ports:
        - containerPort: 6379
---
apiVersion: v1
kind: Service
metadata:
  name: redis
spec:
  selector:
    app: redis
  ports:
    - port: 6379
EOF

kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kunn-agent
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kunn-agent
  template:
    metadata:
      labels:
        app: kunn-agent
    spec:
      containers:
        - name: kunn-agent
          image: $REGISTRY/kunn-agent:latest
          env:
            - name: KUNN_SERVER
              value: "ws://$SERVER_CONTAINER:8080/ws/agent"
            - name: KUNN_AGENT_TOKEN
              value: "agent_tok_1"
EOF

kubectl wait --for=condition=ready pod -l app=postgres --timeout=120s
kubectl wait --for=condition=ready pod -l app=redis --timeout=120s
kubectl wait --for=condition=ready pod -l app=kunn-agent --timeout=120s
echo "Cluster 1 ready"

# -------------------- k8s cluster 2 --------------------

echo "=== K8s Cluster 2 ==="

k3d cluster create $CLUSTER_2 \
  --image rancher/k3s:v1.31.11-k3s1 \
  --servers 1 \
  --agents 1 \
  --registry-use $REGISTRY \
  --k3s-arg "--disable=traefik@server:*" \
  --network $DOCKER_NETWORK

kubectl config use-context k3d-$CLUSTER_2

kubectl apply -f - <<'EOF'
apiVersion: v1
kind: Pod
metadata:
  name: postgres
  labels:
    app: postgres
spec:
  containers:
    - name: postgres
      image: postgres:16-alpine
      env:
        - name: POSTGRES_PASSWORD
          value: "testpass"
      ports:
        - containerPort: 5432
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
spec:
  selector:
    app: postgres
  ports:
    - port: 5432
EOF

kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kunn-agent
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kunn-agent
  template:
    metadata:
      labels:
        app: kunn-agent
    spec:
      containers:
        - name: kunn-agent
          image: $REGISTRY/kunn-agent:latest
          env:
            - name: KUNN_SERVER
              value: "ws://$SERVER_CONTAINER:8080/ws/agent"
            - name: KUNN_AGENT_TOKEN
              value: "agent_tok_2"
EOF

kubectl wait --for=condition=ready pod -l app=postgres --timeout=120s
kubectl wait --for=condition=ready pod -l app=kunn-agent --timeout=120s
echo "Cluster 2 ready"

# -------------------- done --------------------

echo ""
echo "===================="
echo "=== Ready ==="
echo "===================="
echo ""
echo "Components:"
echo "  Webhook:  http://localhost:$WEBHOOK_PORT"
echo "  Server:   ws://localhost:$SERVER_PORT"
echo "  Cluster1: k3d-$CLUSTER_1 (agent_tok_1)"
echo "  Cluster2: k3d-$CLUSTER_2 (agent_tok_2)"
echo ""
echo "Run client with token (docker):"
echo "  docker run -it --rm --network host \\"
echo "    -e KUNN_SERVER=ws://localhost:$SERVER_PORT/ws/client \\"
echo "    -e KUNN_TOKEN=tok_dev \\"
echo "    $REGISTRY/kunn-client:latest"
echo ""
echo "Run client with token (go):"
echo "  KUNN_SERVER=ws://localhost:$SERVER_PORT/ws/client \\"
echo "    KUNN_TOKEN=tok_dev \\"
echo "    go run ./cmd/client"
echo ""
echo "Run client with login (docker):"
echo "  docker run -it --rm --network host \\"
echo "    -e KUNN_SERVER=ws://localhost:$SERVER_PORT/ws/client \\"
echo "    -e KUNN_AUTH_URL=http://localhost:$WEBHOOK_PORT/login \\"
echo "    $REGISTRY/kunn-client:latest"
echo ""
echo "Run client with login (go):"
echo "  KUNN_SERVER=ws://localhost:$SERVER_PORT/ws/client \\"
echo "    KUNN_AUTH_URL=http://localhost:$WEBHOOK_PORT/login \\"
echo "    go run ./cmd/client"
echo ""
echo "Services available:"
echo "  pg-1    → PostgreSQL on cluster-1"
echo "  pg-2    → PostgreSQL on cluster-2"
echo "  redis-1 → Redis on cluster-1"
echo ""
echo "Test (after selecting a service):"
echo "  psql -h localhost -p 6060 -U postgres -d postgres"
echo "  redis-cli -h localhost -p 6060"
