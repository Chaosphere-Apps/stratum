# Engineering Onboarding

Stratum is a self-hosted architecture workspace. The React UI edits a semantic design document; the Go backend owns identity, authorization, persistence, versioning, collaboration, and analysis. PostgreSQL is the production source of truth, while the in-memory repository supports tests and local fallback.

## Repository map

- `ui/src/App.tsx`: application shell and workflow orchestration.
- `ui/src/components/`: product surfaces, including authentication and administration.
- `ui/src/canvas/`: React Flow rendering and interactions.
- `ui/src/backendApi.ts`: browser-to-backend contract.
- `backend/cmd/server/`: server entry point.
- `backend/internal/httpapi/`: HTTP and WebSocket transport adapters.
- `backend/internal/app/`: use-case services.
- `backend/internal/store/`: repository contract plus memory and PostgreSQL implementations.
- `backend/internal/domain/`: durable shared domain types.
- `backend/internal/analysis/`: deterministic and AI-assisted analysis.
- `docs/`: user, administration, operations, service, and testing references.

Read `backend/AGENTS.md` and `ui/AGENTS.md` before changing either service. The detailed service guides are `docs/backend.md` and `docs/ui.md`.

## Local development

Start the persistent stack from the repository root:

```bash
docker compose up -d backend
```

For direct development, run the backend from `backend/` with `go run ./cmd/server`, and the UI from `ui/` with `npm run dev`. The UI expects the backend at `http://127.0.0.1:8081` in the standard local configuration.

On an empty store, the browser presents first-admin setup. Authentication settings and SSO are managed in the Admin Console. Password creation rules come from the public backend auth configuration and must remain enforced by the backend; UI checks are guidance, not a security boundary.

## Change checklist

1. Trace the workflow across UI, API handler, application service, repository contract, and both repository implementations.
2. Preserve authorization, request limits, CORS/CSRF protections, secret redaction, and exact structured design JSON.
3. Add tests at the narrowest useful layer and contract tests where frontend/backend shapes change.
4. Run `GOCACHE="$PWD/.gocache" go test ./...` from `backend/`.
5. Run `npm test`, `npm run lint`, and `npm run build` from `ui/`.

For production and migration details, see `docs/all-in-one-image.md`, `docs/postgres-operations.md`, and `docs/testing.md`.
