# System Design Evaluator Backend

Production-oriented Go backend for the system design evaluator.

## Current Scope

- Guest workspace only.
- Guest profile.
- Multiple workspaces.
- Designs under workspaces.
- Design versions created on save.
- WebSocket API for workspace snapshot and design upsert.
- Postgres repository with exact structured JSON document storage.
- Separate design-doc storage for rich written context so large docs do not travel with every canvas save.
- Structured design analysis across requirements, topology, traffic, consistency, availability, security, and data.
- In-memory fallback repository for tests and quick local runs.
- Modular package boundaries for later Rust analyzers, AI workers, and collaboration services.

## Run

```bash
go run ./cmd/server
```

For persistent storage, run Postgres from the repository root and start the backend with `DATABASE_URL`:

```bash
docker compose up -d postgres
DATABASE_URL='postgres://stratum:stratum_dev_password@127.0.0.1:5432/stratum?sslmode=disable' go run ./cmd/server
```

Or run the backend and database together:

```bash
docker compose up -d backend
```

Defaults:

- HTTP/WebSocket address: `:8081`
- WebSocket endpoint: `ws://127.0.0.1:8081/ws?workspaceId=guest-workspace`
- Health endpoint: `http://127.0.0.1:8081/healthz`
- Without `DATABASE_URL`, the backend uses in-memory storage.

Production-relevant configuration:

- `ALLOWED_ORIGINS`: comma-separated browser origins allowed for REST CORS and WebSocket upgrades.
- `MAX_REQUEST_BODY_BYTES`: maximum REST JSON request body size.
- `WEBSOCKET_READ_LIMIT_BYTES`: maximum WebSocket message size.
- `READ_HEADER_TIMEOUT`, `READ_TIMEOUT`, `WRITE_TIMEOUT`, `IDLE_TIMEOUT`: HTTP server timeout controls.
- `ALLOW_PRIVATE_AI_PROVIDER_URLS`: defaults to `false`; keep it disabled unless a deployment intentionally verifies private/self-hosted AI endpoints.

## REST API

- `GET /api/profile`
- `GET /api/workspaces`
- `POST /api/workspaces`
- `DELETE /api/workspaces/{workspaceID}` deletes only empty non-guest workspaces; non-empty workspaces return `409 Conflict`.
- `GET /api/workspaces/{workspaceID}/designs`
- `POST /api/workspaces/{workspaceID}/designs`
- `GET /api/workspaces/{workspaceID}/designs/{designID}`
- `DELETE /api/workspaces/{workspaceID}/designs/{designID}`
- `PATCH /api/workspaces/{workspaceID}/designs/{designID}`
- `PUT /api/workspaces/{workspaceID}/designs/{designID}/document`
- `POST /api/workspaces/{workspaceID}/designs/{designID}/analysis`
- `GET /api/workspaces/{workspaceID}/designs/{designID}/versions`
- `GET /api/workspaces/{workspaceID}/designs/{designID}/docs`
- `POST /api/workspaces/{workspaceID}/designs/{designID}/docs`
- `GET /api/workspaces/{workspaceID}/designs/{designID}/docs/{docID}`
- `PATCH /api/workspaces/{workspaceID}/designs/{designID}/docs/{docID}`
- `DELETE /api/workspaces/{workspaceID}/designs/{designID}/docs/{docID}`

## Test

```bash
go test ./...
```

## WebSocket Protocol

Server sends on connect:

```json
{
  "type": "workspace.snapshot",
  "payload": {
    "snapshot": {
      "workspace": {},
      "designs": []
    }
  }
}
```

Client upserts a design:

```json
{
  "type": "design.upsert",
  "requestId": "request-id",
  "payload": {
    "design": {},
    "canvasSnapshot": {}
  }
}
```

Server broadcasts:

```json
{
  "type": "design.updated",
  "requestId": "request-id",
  "payload": {
    "design": {}
  }
}
```

## Docker

```bash
docker build -t system-design-evaluator-backend:local .
docker run --rm -p 8081:8081 system-design-evaluator-backend:local
```

The image uses a multi-stage build and distroless non-root runtime.

## Modular Boundaries

- `cmd/server`: process entrypoint.
- `internal/config`: runtime configuration.
- `internal/domain`: workspace and design domain models.
- `internal/store`: repository interfaces, in-memory implementation, and Postgres implementation.
- `internal/realtime`: WebSocket protocol, client loop, and workspace hub.
- `internal/httpapi`: HTTP routes and WebSocket upgrade handling.
- `internal/analysis`: deterministic structured analysis suites and scoring.

## Production Hardening

- HTTP server timeouts are configured explicitly.
- REST request bodies are size-limited and decoded with unknown-field rejection.
- CORS does not fall back to wildcard origins.
- Basic browser security headers are emitted by the API.
- Panic recovery returns a generic 500 and logs the failure.
- Custom AI provider verification blocks private network targets by default to reduce SSRF risk.

Rust can be introduced later behind interfaces, most likely for:

- Graph analysis.
- Constraint solving.
- Formal checks.
- High-performance evaluation engines.
