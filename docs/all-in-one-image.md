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

## Run

```bash
docker run --rm \
  -p 8080:8080 \
  -e DATABASE_URL='postgres://stratum:stratum_dev_password@host.docker.internal:5432/stratum?sslmode=disable' \
  stratum:allinone
```

Open `http://localhost:8080`.

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
