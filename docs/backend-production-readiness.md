# Backend Production Readiness

This note captures the backend hardening state before publishing the repository.

## Hardened Now

- HTTP server has explicit read-header, read, write, idle, and shutdown timeouts.
- REST request bodies are capped with `MAX_REQUEST_BODY_BYTES`.
- JSON request decoding rejects unknown fields and multiple JSON documents.
- CORS no longer falls back to `*`; allowed origins are configured with `ALLOWED_ORIGINS`.
- WebSocket origins use the same configured origin list.
- Panic recovery returns a generic 500 while logging the server-side failure.
- Basic security headers are emitted on HTTP responses.
- Custom AI provider verification blocks private network targets by default to reduce SSRF risk.
- AI provider configuration is centrally managed through admin-only endpoints and is never stored in browser local storage.
- Analysis returns deterministic findings even when AI synthesis fails, with AI status captured in the structured report.
- Local sign-in now uses password hashes and bearer session tokens instead of browser-side user switching.
- Postgres-backed storage preserves exact structured design JSON and creates versions on save.
- Deterministic analysis is isolated under `internal/analysis`.

## Known Gaps Before Real Enterprise Production

- Authorization is role-aware for admin endpoints, but workspace membership and design-level permissions are still not tenant-safe yet.
- Rate limiting and abuse protection are not implemented.
- Database migrations are embedded in startup code; production should use explicit migration files and release workflow.
- Secrets should move to deployment-managed secret storage.
- TLS termination is expected at an ingress/proxy layer, not inside the Go process.
- AI provider keys are currently stored in backend persistence; enterprise deployments should move them to KMS/secret-manager backed encryption and add audit trails.
- WebSocket collaboration is single-process; horizontal scale will need a shared pub/sub layer.
- Observability is basic JSON logging; metrics, tracing, and audit events should be added.

## Recommended Next Backend Steps

1. Add real users, auth sessions, and workspace membership.
2. Split database migrations into versioned migration files.
3. Add request IDs, structured audit events, and Prometheus-style metrics.
4. Add rate limiting around REST writes, analysis runs, and AI-provider verification.
5. Add an AI analysis worker interface separate from HTTP request handling.
6. Add integration tests against Postgres through Docker Compose or testcontainers.
