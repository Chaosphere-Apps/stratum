# Stratum backend

Go API, authorization, persistence, collaboration, and analysis service for Stratum.

## Current scope

- First-admin setup, local sessions, password reset, and Okta OIDC.
- Users, roles, access groups, and direct or group workspace/design grants.
- Workspaces, designs, exact structured documents, versions, reviews, comments, and notifications.
- WebSocket snapshots, conflict-safe design updates, and editor presence.
- PostgreSQL persistence with a stateless in-memory mode for evaluation and tests.
- Deterministic design analysis and centrally configured AI analysis/chat.
- Catalog governance, telemetry configuration, storage administration, and MCP capability settings.

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
- WebSocket endpoint: `ws://127.0.0.1:8081/ws?workspaceId=guest-workspace&designId=<design-id>`
- Health endpoint: `http://127.0.0.1:8081/healthz`
- Without `DATABASE_URL`, the backend uses in-memory storage.

Production-relevant configuration:

- `ALLOWED_ORIGINS`: comma-separated browser origins allowed for REST CORS and WebSocket upgrades.
- `AUTO_MIGRATE`: defaults to `true` for simple installations. Set it to `false` in controlled production deployments and run the image's `/migrate up` command before starting a new backend release.
- `DATABASE_MAX_CONNECTIONS`, `DATABASE_MIN_CONNECTIONS`: per-process Postgres pool bounds. Defaults are `20` and `2`.
- `DATABASE_MAX_CONN_LIFETIME`, `DATABASE_MAX_CONN_IDLE_TIME`, `DATABASE_HEALTH_CHECK_PERIOD`: pool lifecycle controls.
- `MIGRATION_LOCK_TIMEOUT`, `MIGRATION_STATEMENT_TIMEOUT`: bound migration lock waits and individual migration execution.
- `STARTUP_TIMEOUT`: bounds database connection and optional startup migration work. Defaults to two minutes.
- `PUBLIC_URL`: canonical browser origin used for password-reset and OIDC redirects. Set this explicitly outside local development; Stratum does not trust arbitrary `Host`, `Origin`, or forwarding headers when generating reset links.
- `TRUSTED_PROXY_CIDRS`: comma-separated proxy networks whose forwarding metadata may be used for secure-cookie and client-IP handling.
- `LOGIN_ATTEMPTS_PER_WINDOW`, `PASSWORD_RESET_ATTEMPTS_PER_WINDOW`, `AUTH_RATE_LIMIT_WINDOW`: authentication endpoint throttling. Local accounts are additionally locked for 15 minutes after five consecutive failed passwords.
- `MAX_REQUEST_BODY_BYTES`: maximum REST JSON request body size.
- `WEBSOCKET_READ_LIMIT_BYTES`: maximum WebSocket message size.
- `READ_HEADER_TIMEOUT`, `READ_TIMEOUT`, `WRITE_TIMEOUT`, `IDLE_TIMEOUT`: HTTP server timeout controls.
- `ALLOW_PRIVATE_AI_PROVIDER_URLS`: defaults to `false`; keep it disabled unless a deployment intentionally verifies private/self-hosted AI endpoints.
- AI provider settings are managed through the Admin console and persisted server-side. The deterministic analysis path does not require AI configuration. AI chat is limited per user and design to protect provider capacity; distributed deployments should additionally enforce provider quotas at the gateway.
- AI completion calls retry bounded transient provider failures (`429`, `502`, `503`, and `504`). Exhausted retries return a curated, non-sensitive explanation to the chat UI; raw provider bodies and arbitrary backend errors remain private.
- Supported managed AI providers are OpenAI, Anthropic, OpenRouter, and Google AI Studio (Gemini), plus a custom OpenAI-compatible endpoint. Google credentials use the Gemini API's `x-goog-api-key` header, provider verification checks that the configured Gemini model is available to the key, and secrets are never returned by the settings API after they are stored.

## REST API

- `GET /api/profile`
- `POST /api/auth/login`
- `GET /api/auth/config`
  returns the enabled sign-in methods and the backend-enforced password policy used by password-creation screens.
- `GET /api/auth/oidc/start`
- `GET /api/auth/oidc/callback`
- `POST /api/auth/logout`
- `POST /api/setup/admin-password` completes one-time password setup for pre-existing admin users created before local passwords were introduced.
- `GET /api/admin/ai-provider`
- `PATCH /api/admin/ai-provider`
- `GET /api/workspaces`
- `POST /api/workspaces`
- `DELETE /api/workspaces/{workspaceID}` deletes only empty non-guest workspaces; non-empty workspaces return `409 Conflict`.
- `GET /api/workspaces/{workspaceID}/designs`
- `POST /api/workspaces/{workspaceID}/designs`
- `GET /api/workspaces/{workspaceID}/designs/{designID}`
- `DELETE /api/workspaces/{workspaceID}/designs/{designID}`
- `PATCH /api/workspaces/{workspaceID}/designs/{designID}`
- `PUT /api/workspaces/{workspaceID}/designs/{designID}/document`
- `POST /api/workspaces/{workspaceID}/designs/{designID}/analysis` accepts an optional saved `versionId` and review `focus` (`full`, `security`, `scalability`, `reliability`, `data`, `operability`, or `cost`).
- `GET /api/workspaces/{workspaceID}/designs/{designID}/ai/conversations`
- `POST /api/workspaces/{workspaceID}/designs/{designID}/ai/conversations`
- `PATCH /api/workspaces/{workspaceID}/designs/{designID}/ai/conversations/{conversationID}`
- `GET /api/workspaces/{workspaceID}/designs/{designID}/ai/conversations/{conversationID}/messages`
- `POST /api/workspaces/{workspaceID}/designs/{designID}/ai/conversations/{conversationID}/messages`
- `GET /api/workspaces/{workspaceID}/designs/{designID}/versions`
- `GET /api/workspaces/{workspaceID}/designs/{designID}/docs`
- `POST /api/workspaces/{workspaceID}/designs/{designID}/docs`
- `GET /api/workspaces/{workspaceID}/designs/{designID}/docs/{docID}`
- `PATCH /api/workspaces/{workspaceID}/designs/{designID}/docs/{docID}`
- `DELETE /api/workspaces/{workspaceID}/designs/{designID}/docs/{docID}`

AI conversations are persisted separately from the versioned design document. New IDs use the compact `chat_…` prefix; existing `ai_conversation_…` IDs remain valid because identifiers are opaque. A conversation can be pinned to one saved version; otherwise it follows the current working design. Each conversation has an explicit `accessMode`: `read` is the default, while `read_write` is allowed only for the working design and only while the user retains design-edit permission. The backend rechecks that permission on every message. Model-proposed writes must preserve the design identity and schema, stay within request and graph limits, use unique IDs, reference valid connector endpoints, and pass optimistic revision control before they are stored. Read-only chat sends a compact architecture projection without canvas geometry, while write-enabled chat receives the canonical document required for a validated replacement. Recent history is bounded by message count and character budget so follow-ups retain continuity without unbounded prompt growth. Returned component and connector references are validated against the selected document before storage.

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
    "canvasSnapshot": {},
    "baseRevision": "sha256:..."
  }
}
```

`baseRevision` is the SHA-256 revision returned with the last synchronized design. The repository compares it atomically with the current structured document. A stale REST save returns `409 Conflict`; a stale WebSocket save returns an `error` envelope with code `design_conflict` and the current server document. Metadata and lifecycle changes do not invalidate this document revision.

The server also broadcasts design-scoped presence:

```json
{
  "type": "presence.updated",
  "payload": {
    "members": [
      {
        "userId": "user-1",
        "displayName": "Ada",
        "role": "architect",
        "designId": "design-1",
        "sessionCount": 2
      }
    ]
  }
}
```

Presence is ephemeral and does not grant access. WebSocket authentication and the same workspace/design policies used by REST remain authoritative. Presence fanout is process-local today; multi-replica deployments need shared pub/sub before active sessions can span backend instances.

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
It includes `/migrate`, a separate operator command for inspecting and applying the exact migration set embedded in that release. See [PostgreSQL Operations and Migrations](postgres-operations.md).

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
