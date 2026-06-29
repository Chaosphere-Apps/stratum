# All-in-One Image

Stratum can be packaged as a single production container image for enterprise deployments. The image runs one Go process that serves:

- the React UI static assets
- REST API routes under `/api`
- WebSocket collaboration under `/ws`
- health checks at `/healthz`

The database is intentionally external. Use Postgres as a managed service or as a separate container with persistent storage.

## Build

```bash
docker build -f Dockerfile_allinone -t stratum:allinone .
```

## Published Images

GitHub Actions builds three production container images:

- `ghcr.io/<owner>/stratum-ui`: static React UI served by unprivileged nginx.
- `ghcr.io/<owner>/stratum-backend`: Go API and WebSocket backend.
- `ghcr.io/<owner>/stratum-allinone`: Go backend serving the built UI and API from one container.

Pull the all-in-one image for the simplest self-hosted trial:

```bash
docker pull ghcr.io/chaosphere-apps/stratum-allinone:latest
```

Images are built for pull requests, but only pushed to GHCR from:

- any pushed Git tag
- manual `workflow_dispatch` runs with `publish=true`

Published tags include the Git tag, semantic version tags when the tag is semver-compatible, `sha-<commit>`, and `latest`.

Published images are multi-architecture manifests for:

- `linux/amd64`
- `linux/arm64`

Docker normally selects the matching platform automatically. To force a specific architecture:

```bash
docker pull --platform linux/arm64 ghcr.io/chaosphere-apps/stratum-allinone:latest
docker run --rm --platform linux/arm64 -p 8080:8080 ghcr.io/chaosphere-apps/stratum-allinone:latest
```

For Intel/AMD hosts:

```bash
docker pull --platform linux/amd64 ghcr.io/chaosphere-apps/stratum-allinone:latest
```

## Run

```bash
docker run --rm \
  -p 8080:8080 \
  -e DATABASE_URL='postgres://stratum:stratum_dev_password@host.docker.internal:5432/stratum?sslmode=disable' \
  ghcr.io/chaosphere-apps/stratum-allinone:latest
```

Open `http://localhost:8080`.

The all-in-one image serves UI, `/api`, `/ws`, and `/healthz` from the same origin. The browser should never need a separate backend URL for this mode.

## Runtime Settings

- `BACKEND_ADDR`: listen address, defaults to `:8080` in the all-in-one image.
- `STATIC_ASSETS_DIR`: UI asset directory, defaults to `/public` in the all-in-one image.
- `DATABASE_URL`: Postgres connection string. If omitted, the backend falls back to the in-memory repository for local experiments.
- `ALLOWED_ORIGINS`: comma-separated allowed origins for browser and WebSocket access.

## Production Notes

- Terminate TLS at an ingress, reverse proxy, or load balancer.
- Keep Postgres outside the app container and back it with persistent storage.
- Configure `ALLOWED_ORIGINS` to the deployed Stratum origin.
- For SSO, configure Okta redirect URIs using the externally visible Stratum URL.
