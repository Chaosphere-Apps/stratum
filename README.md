# Stratum

Stratum is a self-hosted architecture workspace for teams to design, review, and govern software systems. It combines a semantic system canvas, requirements, request journeys, documentation, version review, deterministic analysis, AI-assisted architecture work, and enterprise access controls in one place.

## What you can do

- Organize architecture designs in private or shared workspaces.
- Model systems with typed components, connectors, flows, notes, and structured requirements.
- Capture functional requirements, consistency expectations, availability targets, SLA, traffic, and capacity context.
- Save immutable versions, request reviews, record comments, and track design changes.
- Run evidence-based checks across topology, scale, reliability, security, data, and operability.
- Use design-scoped AI chat in read-only mode or explicitly grant read-and-edit access for one conversation.
- Attach design documentation and export the structured design for use outside Stratum.
- Govern users, roles, groups, workspace/design permissions, catalog assets, SSO, AI providers, storage, and integrations from the Admin Console.

Access is enforced by the backend. The UI shows whether a workspace or design is view-only or editable before it is opened.

## Quick start

The simplest self-hosted deployment is the all-in-one image with an external PostgreSQL database:

```bash
docker pull ghcr.io/chaosphere-apps/stratum-allinone:latest
docker run --rm \
  -p 8080:8080 \
  -e DATABASE_URL='postgres://stratum:change-me@host.docker.internal:5432/stratum?sslmode=disable' \
  -e PUBLIC_URL='http://localhost:8080' \
  -e ALLOWED_ORIGINS='http://localhost:8080' \
  ghcr.io/chaosphere-apps/stratum-allinone:latest
```

Open `http://localhost:8080`. On a new installation, create the first account; it becomes the organization administrator. Replace the example database credentials and use TLS-verified PostgreSQL before production use.

For a source checkout, start PostgreSQL and the backend from the repository root:

```bash
docker compose up -d backend
```

Then start the UI:

```bash
cd ui
npm ci
npm run dev
```

The UI is available at `http://127.0.0.1:5173` and connects to the backend at `http://127.0.0.1:8081`.

Without `DATABASE_URL`, Stratum runs in stateless in-memory mode. That mode is suitable for evaluation only: data is lost when the backend restarts. Administrators can configure PostgreSQL from the storage settings.

## Documentation

- [User guide](docs/user-guide.md): workspaces, designs, canvas, analysis, AI chat, reviews, and permissions.
- [Administration and SSO](docs/admin-and-sso.md): identity, access, providers, integrations, and Okta configuration.
- [All-in-one deployment](docs/all-in-one-image.md): container image, runtime settings, and production notes.
- [PostgreSQL operations](docs/postgres-operations.md): migrations, rollout, roles, backups, and readiness.
- [Engineering onboarding](docs/onboarding.md): repository map and development workflow.
- [Backend reference](docs/backend.md) and [UI reference](docs/ui.md): service-specific development information.
- [Testing](docs/testing.md): local and CI verification strategy.

The complete documentation index is in [docs/README.md](docs/README.md).

## Release verification

Run the complete backend and UI test suites from the repository root:

```bash
make test
make test-e2e
```

See [Testing Stratum](docs/testing.md) for database integration tests, coverage gates, and individual commands.
