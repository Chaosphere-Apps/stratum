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

## Current Capabilities

- React Flow structured canvas.
- Guest workspace and design list.
- Create and switch between designs.
- Home screen for profile, workspaces, and workspace designs.
- Small architecture component catalog.
- Click or drag-and-drop components from the catalog.
- Component inspector with basic and evaluation fields.
- Linked Design component for referencing another visible design in the same workspace.
- Redis-specific metadata for cluster mode, replication, persistence, eviction, consistency, failover, backup, QPS, memory, and hot key risk.
- Typed connector creation between semantic components.
- React Flow connector handles with smooth arrows that stay attached when components move.
- Traversal journeys that can be generated from the component graph, edited as ordered steps, and highlighted on the canvas.
- Attached design docs for written context such as assumptions, decisions, links, rollout notes, and review narrative, saved through separate backend doc endpoints.
- Sticky Note as a visible canvas component.
- Component notes with neutral, risk, decision, and question tones.
- Focus mode for a larger canvas.
- Bottom-right React Flow zoom and fit controls.
- Lightweight requirement brief.
- Local draft save/load through browser storage.
- Structured JSON export containing the semantic design model and React Flow layout metadata.

## Design Principle

The semantic architecture graph is the source of truth. React Flow renders that structured graph and stores only layout metadata on components, so evaluation can read the design document directly.
