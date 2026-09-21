# Backend Agent Guide

This backend is the production core for Stratum: workspaces, designs, versions, WebSocket sync, structured storage, and deterministic analysis.

## Local Shape

- `cmd/server`: process entrypoint, HTTP server configuration, graceful shutdown.
- `internal/config`: environment-driven runtime configuration.
- `internal/domain`: stable domain types shared across storage, realtime, and HTTP.
- `internal/store`: repository interface plus Postgres and in-memory implementations.
- `internal/realtime`: WebSocket protocol, hub, and client loop.
- `internal/httpapi`: REST routes, WebSocket upgrade, CORS, request handling, provider verification.
- `internal/analysis`: deterministic analysis suites and scoring.

Large packages are organized by business capability rather than by transport
verb or storage technology:

- `internal/app/*_service.go`: use-case orchestration for identity, workspaces,
  designs, versions, docs/reviews, ACL, catalog, analysis, and AI chat.
- `internal/httpapi/*_handlers.go`: thin transport adapters grouped by the same
  capabilities; shared middleware and runtime handlers remain separate.
- `internal/store/memory_*.go` and `postgres_*.go`: behaviorally equivalent
  repository implementations, with shared normalization/scanning helpers kept
  explicit.

## Engineering Philosophy

- Keep the domain model semantic and durable. The design document should remain useful outside any one UI renderer.
- Prefer small package boundaries over a large service object. Analysis, storage, realtime, and HTTP should evolve independently.
- Treat production readiness as a default posture: explicit limits, timeouts, validation, safe errors, and observable behavior.
- Build backend behavior for a premium enterprise product: user-visible actions must be reliable, explainable, and consistent after refresh.
- Start with the product workflow before the endpoint. Save/load, ACL, review lifecycle, catalog governance, integrations, and analysis should work as coherent systems, not isolated routes.
- Keep deterministic analysis separate from future AI analysis. AI should enrich or explain, not replace the structured checks.
- Avoid premature framework gravity. Use the Go standard library where it is enough; add dependencies only when they clearly reduce risk or complexity.
- Preserve exact structured design JSON when saving versions. Do not normalize away fields that users or future analyzers may need.

## Guardrails

- Do not add authentication shortcuts that pretend to be secure. If auth is introduced, model users, workspaces, and authorization explicitly.
- Do not weaken CORS, request size limits, WebSocket limits, timeout settings, or AI-provider SSRF checks for convenience.
- Do not log API keys, provider credentials, design secrets, or full request bodies.
- Do not put long-running AI calls directly in request handlers. Introduce a worker/job boundary when that work becomes real.
- Keep Postgres as the source of truth for persisted designs; in-memory storage is for tests and local fallback only.
- If changing storage behavior, update both Postgres and memory repositories and add/adjust tests.
- Do not add dead endpoints or UI-backed routes that only partially work. Return clear errors for unsupported behavior and keep frontend-visible state truthful.
- Do not compromise reliability for speed of implementation. Data loss, inconsistent authorization, and broken version/review transitions are release blockers.
- Treat integrations as optional plug-ins. A failing telemetry, AI, MCP, or SSO integration must not break the core whiteboard, save/load, or review paths.
- Keep migrations forward-only, deterministic, and safe to apply automatically at startup.

## Quality Bar

Before handing off backend changes, run:

```bash
gofmt -w <changed-go-files>
GOCACHE=/Users/tarun/Documents/system-design-evaluator/backend/.gocache go test ./...
```

For GitHub CI parity, also expect:

```bash
go vet ./...
go build -o /tmp/stratum-backend ./cmd/server
docker build -t stratum-backend-ci .
```

## Creativity Space

Be ambitious about better analyzers, collaboration models, graph algorithms, and future Rust-backed evaluation services. The constraint is not "keep it small forever"; the constraint is "make each step understandable, testable, and safe to operate."
