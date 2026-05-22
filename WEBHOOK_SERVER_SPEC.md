# Webhook Server Spec

Implement an HTTP API that kunn calls for auth and config. All requests include `Authorization: Bearer <token>` header.

---

## `GET /projects`

**Request:** `Authorization: Bearer <client_token>`

**Response `200`:**
```json
{
  "name": "alice",
  "projects": [
    { "id": "app", "name": "Main App" }
  ]
}
```

**Response `401`:** invalid client token.

---

## `GET /services?project=<project_id>`

**Request:** `Authorization: Bearer <client_token>`

**Response `200`:**
```json
{
  "services": [
    { "id": "postgres", "name": "PostgreSQL" },
    { "id": "redis", "name": "Redis" }
  ]
}
```

**Response `401`:** invalid client token. **Response `404`:** project not found.

---

## `GET /resolve?project=<project_id>&service=<service_id>`

**Request:** `Authorization: Bearer <client_token>`

**Response `200`:**
```json
{
  "cluster": "cluster-a",
  "address": "postgres.app.svc.cluster.local:5432"
}
```

- `cluster` must match the value returned by `/agent-auth` for the corresponding agent.
- `address` is the internal k8s service address (`host:port`) the agent will dial.

**Response `401`:** invalid client token. **Response `404`:** project or service not found.

---

## `GET /agent-auth`

**Request:** `Authorization: Bearer <agent_token>`

**Response `200`:**
```json
{
  "cluster": "cluster-a"
}
```

**Response `401`:** invalid agent token.

---

## Notes

- Agent tokens and client tokens are separate — they must not overlap.
- `cluster` is the link between agents and services. The value from `/agent-auth` must match the `cluster` values from `/resolve`.
