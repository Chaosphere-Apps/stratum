# Stratum UI

Frontend for Stratum, the system design workspace.

## Run

```bash
npm install
npm run dev -- --host 127.0.0.1 --port 5173
```

Open `http://127.0.0.1:5173/`.

## Docker

Build the production image:

```bash
docker build -t system-design-evaluator-ui:local .
```

Run it:

```bash
docker run --rm -p 8080:8080 system-design-evaluator-ui:local
```

Open `http://127.0.0.1:8080/`.

The image uses a multi-stage build: Node builds the Vite app, then an unprivileged nginx runtime serves only the static `dist` assets.

## Backend URL Resolution

The UI calls the backend through same-origin `/api` and `/ws` routes in production. This is required for the all-in-one image and for deployments behind a reverse proxy.

During Vite development on `5173` or preview on `4173`, the UI defaults to the local backend on `8081`. Set `VITE_BACKEND_API_URL` and `VITE_BACKEND_WS_ORIGIN` when running a split deployment that does not expose the backend through the same origin.

## Product behavior

See the [user guide](user-guide.md) for current functionality. Frontend changes should preserve these implementation boundaries:

- The backend is authoritative for identity, authorization, field limits, and durable data.
- The semantic architecture graph is the source of truth; React Flow renders it and stores layout metadata on components.
- Workspaces and designs expose effective access before navigation and disabled controls are not treated as security boundaries.
- Canvas saves use server revisions and preserve a local draft when a conflict occurs.
- AI access is scoped per conversation and defaults to read-only.

## Design principle

The semantic architecture graph is the source of truth. React Flow renders that structured graph and stores only layout metadata on components, so evaluation can read the design document directly.
