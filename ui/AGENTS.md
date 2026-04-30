# UI Agent Guide

This UI is Stratum's system design workspace: a structured architecture canvas, requirements capture surface, inspectors, analysis panel, and backend-connected project shell.

## Local Shape

- `src/App.tsx`: main product shell, routing, workspace/design flows, inspector panels, analysis interactions.
- `src/canvas/`: React Flow canvas provider and canvas-facing types.
- `src/backendApi.ts`: REST client for profile, workspaces, designs, analysis, and AI-provider verification.
- `src/backendSync.ts`: backend domain shapes used by the UI.
- `src/catalog.ts`: supported architecture components and display metadata.
- `src/designModel.ts`: creation, normalization, and export helpers for structured design documents.
- `src/types.ts`: canonical frontend design document types.
- `src/styles.css`: product styling and layout system.

## Product Philosophy

- Stratum is a focused design tool, not a generic whiteboard. Flexibility is welcome when it preserves structured architecture meaning.
- The semantic design document is the source of truth. React Flow is the renderer/editor, not the data model.
- UI should feel mature and calm: dense enough for enterprise work, visual enough for architecture thinking, and never like a marketing landing page inside the app.
- Prefer explicit architecture controls over hidden magic. Components, connectors, requirements, and analysis output should be inspectable.
- Make the canvas enjoyable, but avoid fragile customizations that fight React Flow upgrades.

## Guardrails

- Do not reintroduce tldraw or another paid/commercially risky canvas dependency without a fresh licensing decision.
- Use React Flow primitives where possible: nodes, edges, handles, toolbars, resize controls, viewport controls, and grouping.
- Keep renderer-specific layout in component metadata; keep evaluator-facing facts in structured fields.
- Do not store API keys or provider secrets in long-lived frontend state unless the product explicitly adds secure backend storage.
- Do not make analysis-only information impossible for the backend to read. If it affects evaluation, it belongs in the structured design document.
- Keep visual polish responsive. Text must not overflow controls, panels, nodes, or cards on typical laptop and mobile widths.

## Quality Bar

Before handing off UI changes, run:

```bash
npm run lint
npm run build
```

For GitHub CI parity, also expect:

```bash
docker build -t stratum-ui-ci .
```

When changing canvas interactions, smoke-test at least:

- Create a workspace and design.
- Add components by click and drag/drop.
- Connect components.
- Move/resize components and cloud frames.
- Save, reload, and reopen the design.
- Run analysis after saving.

## Creativity Space

Push the canvas experience forward: better grouping, review overlays, flow animation, compact inspectors, design diffs, export quality, and AI-assisted drafting are all fair game. The design rule is simple: make it more powerful without making the structured model harder to trust.
