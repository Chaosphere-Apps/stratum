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

## Engineering Philosophy

- Keep the domain model semantic and durable. The design document should remain useful outside any one UI renderer.
- Prefer small package boundaries over a large service object. Analysis, storage, realtime, and HTTP should evolve independently.
- Treat production readiness as a default posture: explicit limits, timeouts, validation, safe errors, and observable behavior.
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
