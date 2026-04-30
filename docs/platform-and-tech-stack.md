# Platform and Tech Stack Tradeoffs

## 1. Recommendation

Build the product as a web-first enterprise SaaS, with support for private cloud or customer VPC deployment later. Do not start as an installable desktop app.

Desktop can become a companion later for:

- Offline review.
- Local file import/export.
- Air-gapped enterprise environments.
- Running local redaction or model calls before upload.

The primary product should still be a browser-based collaborative workspace because the core value depends on shared designs, review history, access control, AI analysis jobs, audit trails, and enterprise administration.

## 2. Web App vs Desktop App

### Web App Strengths

- Easier enterprise rollout through SSO, SCIM, RBAC, audit logs, and browser access.
- Better collaboration model for review meetings and async design iteration.
- Centralized versioning, analysis history, exports, and rubric management.
- Easier AI governance through a server-side model gateway.
- Faster iteration for product updates.
- Better fit for workspace administration and team-level analytics.

### Web App Weaknesses

- Canvas performance must be carefully engineered.
- Offline support is harder.
- Browser memory and rendering limits matter for very large diagrams.
- Enterprise customers may require private deployment or data residency options.

### Desktop App Strengths

- Strong local-file workflow.
- Better offline story.
- Potentially deeper OS integration.
- Useful for air-gapped customers or local-only sensitive design review.

### Desktop App Weaknesses

- Harder enterprise deployment and update management.
- More complex SSO, policy, proxy, certificate, and device-management issues.
- Collaboration still requires a backend.
- AI analysis still requires remote or customer-hosted model infrastructure.
- Larger QA matrix across macOS, Windows, and Linux.

## 3. Decision

Start with a web app. Treat desktop as a later packaging option, not the product foundation.

Recommended path:

1. Build web-first SaaS.
2. Support enterprise private deployment when security-sensitive customers require it.
3. Add a Tauri-based desktop wrapper only if offline/local workflows become a decisive sales requirement.

Electron is a viable desktop fallback, but Tauri is preferable for a companion app because it uses a web frontend, supports cross-platform apps, and is designed around a smaller, security-conscious runtime. Desktop should not own the core data model.

## 4. Frontend Stack

Recommended:

- TypeScript.
- React.
- Next.js App Router for application shell, routing, server-rendered pages, and enterprise admin surfaces.
- A dedicated client-rendered canvas workspace.
- tldraw SDK for the infinite canvas foundation if licensing and customization fit.
- React Flow as the fallback if the product should be diagram-first rather than whiteboard-first.
- Zustand or Jotai for local canvas/UI state.
- TanStack Query for server state.
- Tailwind CSS plus a restrained design system.
- Zod for client/server schema validation.

Next.js is not selected because it makes the backend "scale better." It is selected because it is a strong product shell for a React web app: routing, layouts, server-rendered enterprise/admin pages, authentication flows, loading states, and a clean way to mix server-rendered screens with client-heavy canvas screens.

The parts that scale the system are the backend service boundaries, database design, queues/workflows, worker pools, caching, collaboration service, and model gateway. Next.js should not be the home for long-running AI jobs, collaboration fanout, export rendering, or core enterprise policy logic.

### Canvas Choice

The key UI tradeoff is tldraw vs React Flow.

Use **tldraw** if the goal is closer to Miro:

- Infinite canvas.
- Whiteboard-native gestures.
- Rich annotations.
- Custom shapes and tools.
- Multiplayer canvas foundation.
- More "magic" and fluidity.

Use **React Flow** if the goal is closer to a strict architecture graph editor:

- Node and edge graph primitives.
- Easier typed connectors.
- More direct mapping to a system design graph.
- Less whiteboard surface area.

Recommendation: start with tldraw for the canvas experience, but keep the architecture graph as a separate structured model. The canvas should render and edit semantic design objects; it should not become the only source of truth.

## 5. Frontend Architecture

Use three layers:

- **Canvas layer:** shapes, arrows, selection, drag/drop, zoom, comments, badges.
- **Semantic design layer:** components, connectors, flows, boundaries, requirements, assumptions.
- **Analysis overlay layer:** findings, suite readiness, missing metadata, review comments.

The structured design language should be generated from the semantic layer, not scraped from canvas pixels or generic shapes.

For performance:

- Keep canvas interactions local and optimistic.
- Debounce persistence.
- Store large analysis payloads outside hot canvas state.
- Virtualize side panels and long findings lists.
- Avoid re-rendering the whole canvas on metadata edits.
- Use stable IDs and incremental patches.

## 6. Backend Stack

Recommended initial backend:

- Go for the first API and worker services.
- PostgreSQL as the primary system of record.
- JSONB for immutable design snapshots and structured design documents.
- Object storage for exports, attachments, images, and PDFs.
- Redis for cache, rate limits, ephemeral collaboration metadata, and job coordination.
- Temporal for durable AI analysis, export, and long-running workflow orchestration.
- WebSocket collaboration service for live canvas sessions.
- Model Gateway service for provider routing, redaction, audit, budget controls, and policy enforcement.

Keep the web UI separate from backend services. Do not put enterprise business logic inside frontend route handlers. The design service, evaluation service, export service, and model gateway should be independently deployable over time.

The current implementation starts with Go for the API, persistence, WebSocket, and deterministic analysis paths, while keeping module boundaries clear enough to introduce specialized Rust services later for formal analysis or high-performance evaluation.

## 7. Service Architecture

Core services:

- **Web App:** workspace UI, canvas, admin, review, exports.
- **API Gateway/BFF:** request shaping, session handling, coarse routing.
- **Auth and Policy Service:** SSO, RBAC, workspace policies, data access checks.
- **Design Service:** designs, versions, structured graph validation, diffs.
- **Requirement Service:** requirement brief, traffic estimates, data profile, assumptions, question generation.
- **Collaboration Service:** live sessions, presence, cursors, optimistic patches.
- **Evaluation Service:** deterministic checks, suite readiness, analysis context generation.
- **Evaluation Workers:** AI suite execution and structured output validation.
- **Model Gateway:** model routing, redaction, provider policy, cost tracking, prompt/version audit.
- **Export Service:** JSON, YAML, Markdown, Mermaid, PDF/image exports.
- **Catalog Service:** component templates, workspace patterns, deprecated components.
- **Audit Service:** append-only security and compliance events.

## 8. Data Architecture

Use PostgreSQL for:

- Workspaces.
- Projects.
- Users and membership.
- Designs.
- Immutable versions.
- Requirement briefs.
- Component catalog.
- Rubrics.
- Analysis runs.
- Findings.
- Comments.
- Review status.
- Audit references.

Use object storage for:

- Rendered image/PDF exports.
- Large attachments.
- Uploaded reference docs.
- Analysis artifacts that are too large for relational rows.

Use an event or job layer for:

- Analysis requested.
- Export requested.
- Version created.
- Finding status changed.
- Comment created.
- Rubric updated.

## 9. Collaboration Architecture

For MVP, collaboration can be staged:

1. Single-user editing with versioned saves.
2. Soft collaboration: comments, review status, analysis sharing.
3. Live collaboration: presence, cursors, simultaneous editing.

Do not make real-time multiplayer the first engineering mountain unless the first customers demand it. The semantic design model, versioning, and evaluation suites are more differentiating.

When live collaboration is added:

- Use CRDT-style patches for canvas state.
- Keep semantic graph validation server-side.
- Persist snapshots periodically.
- Store an append-only patch log for recovery.
- Separate ephemeral presence from durable design changes.

## 10. AI Architecture

AI should run server-side through a model gateway.

Reasons:

- Enterprise model policy enforcement.
- Prompt and model version tracking.
- Data redaction.
- Cost limits.
- Audit logs.
- Retries and fallbacks.
- Provider abstraction.

Evaluation jobs should be asynchronous. A user can see deterministic findings quickly, then suite-level AI analysis as workers complete.

Use a structured pipeline:

1. Normalize design graph.
2. Validate requirement completeness.
3. Run deterministic checks.
4. Build suite-specific context.
5. Redact or transform sensitive fields.
6. Call model through gateway.
7. Validate structured output.
8. Store suite result and findings.
9. Render findings on report and canvas.

## 11. Scale Architecture

Scale pressure will come from different places:

- Canvas collaboration: WebSocket fanout and patch persistence.
- AI evaluation: long-running, expensive, bursty jobs.
- Exports: CPU-heavy rendering.
- Enterprise admin/reporting: analytical queries over many designs and findings.

Recommended deployment:

- Kubernetes for independently scalable services and workers.
- Horizontal autoscaling for stateless APIs and workers.
- Separate worker pools for evaluation and export workloads.
- Queue-based backpressure for AI jobs.
- Per-workspace rate limits and budgets.
- Read replicas for reporting-heavy enterprise workspaces.
- Partition large audit/event tables by time and workspace.
- Object storage CDN for exported artifacts where policy allows.

Designs themselves are not likely to produce huge raw data volume. AI jobs, collaboration sessions, exports, and auditability are the real scaling concerns.

## 12. Enterprise Deployment Options

Offer tiers:

- **SaaS multi-tenant:** default, fastest to operate.
- **SaaS single-tenant:** isolated data plane for larger customers.
- **Customer VPC/private cloud:** for strict data residency and AI policy needs.
- **Air-gapped package:** later, only after the product stabilizes.

This gives enterprise buyers a security path without forcing every customer into desktop installation.

## 13. Suggested MVP Stack

MVP stack:

- Next.js + React + TypeScript.
- tldraw SDK for canvas.
- PostgreSQL.
- Redis.
- S3-compatible object storage.
- Fastify or NestJS services in TypeScript.
- Temporal for durable workflows.
- WebSocket service for collaboration later.
- OpenTelemetry for traces, metrics, and logs.
- Kubernetes for production deployment.

If the team is small, start as a modular monolith:

- One API service with clear internal modules.
- One worker service.
- One model gateway module.
- One PostgreSQL database.
- One Redis instance.

Keep module boundaries aligned with future services so the system can split cleanly as usage grows.

## 14. Key Tradeoffs

### Speed vs Enterprise Rigor

A pure Next.js app can ship quickly, but enterprise policy, auditability, collaboration, and AI orchestration become messy if everything lives in the web layer. Use Next.js for the product shell and maintain real backend boundaries.

### Whiteboard Flexibility vs AI-Evaluable Structure

Free-form drawing feels powerful but is hard to analyze. Strict forms are easy to analyze but feel dead. The right answer is a canvas with custom architecture-aware objects backed by structured metadata.

### Multiplayer Now vs Later

Real-time collaboration is impressive, but it can consume the roadmap. Start with versioned saves and review comments, then add live multiplayer after the semantic model is stable.

### SaaS vs Private Deployment

SaaS is operationally simpler and faster to improve. Enterprise customers may require isolation. Design from day one for workspace isolation, exportable audit logs, and configurable model policies.

### Go Backend vs Polyglot Services

Go is the recommended first backend language. Introduce Rust only where a subsystem has a clear performance, safety, parser, graph-analysis, or formal-methods reason. Introduce Python or TypeScript workers only if model SDKs or AI experimentation justify that split.

## 15. Sources Checked

Current stack guidance was checked against primary project documentation on April 29, 2026:

- [Next.js App Router documentation](https://nextjs.org/docs/app).
- [tldraw SDK](https://tldraw.dev/) and [collaboration documentation](https://tldraw.dev/docs/collaboration).
- [React Flow whiteboard](https://reactflow.dev/learn/advanced-use/whiteboard) and [collaboration examples](https://reactflow.dev/examples/interaction/collaborative).
- [Yjs shared types](https://docs.yjs.dev/getting-started/working-with-shared-types) and [awareness documentation](https://docs.yjs.dev/getting-started/adding-awareness).
- [Tauri desktop](https://tauri.app/) and [updater documentation](https://v2.tauri.app/plugin/updater/).
- [Electron auto-update documentation](https://www.electronjs.org/docs/latest/api/auto-updater).
- [PostgreSQL JSONB documentation](https://www.postgresql.org/docs/current/static/datatype-json.html).
- [Redis rate limiting documentation](https://redis.io/docs/latest/develop/use-cases/rate-limiter/).
- [Temporal durable execution documentation](https://docs.temporal.io/).
- [OpenTelemetry documentation](https://opentelemetry.io/docs/).
- [Kubernetes autoscaling documentation](https://kubernetes.io/docs/concepts/workloads/autoscaling).
