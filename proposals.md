# Stratum Product Proposals

This document captures product and architecture proposals that are not yet committed implementation plans. Proposals here should stay concrete enough to evaluate, but lightweight enough to evolve as the product direction changes.

## Enterprise Catalog Systems

### Problem

Enterprise architecture rarely reuses only individual components. Teams commonly depend on a group of services and infrastructure that operate together as a system, such as a payments platform, identity system, notification system, or data platform.

Today, Stratum can save a service or infrastructure component into the Enterprise Catalog. That is useful for tracking individual assets, but it does not fully capture larger reusable systems with internal dependencies, ownership, and operational context.

### Proposal

Add **System Assets** to the Enterprise Catalog.

A System Asset is a reusable catalog entry that can contain:

- Components
- Connectors
- Notes
- Ownership and criticality metadata
- Tags and aliases
- Optional canonical design reference
- Optional structured system document

In product language, this should be presented as a **System**, not as a generic group. Internally it may be represented as a grouped graph, but enterprise users should understand it as a reusable system boundary.

### Primary Use Cases

- Represent a reusable enterprise capability such as Payments Platform, Identity, Notification System, or Data Platform.
- Link a design to an existing enterprise system without redrawing every internal component.
- Save selected canvas components as a reusable system asset.
- Track where major systems are used across architecture designs.
- Give analysis more context about hidden dependencies inside linked systems.

### UX Direction

Enterprise Catalog should distinguish:

- **Components**: single services, databases, queues, APIs, controls, or infrastructure assets.
- **Systems**: reusable groups of components and connectors.

System creation paths:

1. Create from Enterprise Catalog.
2. Save selected canvas nodes/connectors as a catalog system.
3. Link an existing design as the canonical source for a system.

When inserted into a design, a system should support two modes:

- **Linked System**: collapsed, governed reference to the catalog system.
- **Expanded Snapshot**: copied components/connectors that can be edited independently.

Default should be **Linked System**. Expansion should be explicit.

### Canvas Behavior

A linked system node should look visually distinct from normal components and show:

- System name
- Owner/team
- Criticality
- Number of internal components
- Catalog link/provenance
- Open details action
- Expand snapshot action

The inspector should show system-level metadata only. It should not expose every internal component field unless the user opens the system detail view.

### Backend Direction

Short-term model:

- Add `kind` to catalog assets: `component | system`.
- Store system structure as a structured JSON document in catalog metadata or a dedicated system document field.
- Keep existing component assets backward-compatible by defaulting missing `kind` to `component`.

Longer-term model:

- Add versioned catalog assets.
- Store system graph versions separately from metadata.
- Support canonical design references.
- Support impact analysis across designs that reference a system.

Possible shape:

```text
catalog_assets
- id
- name
- normalized_name
- kind: component | system
- type
- owner
- criticality
- description
- tags
- metadata
- created_by
- created_at
- updated_at

catalog_asset_versions
- id
- asset_id
- version_number
- status
- document_json
- created_by
- created_at
```

### Analysis Impact

System assets improve analysis because Stratum can reason about:

- Design-level dependencies on enterprise systems
- Hidden dependencies inside linked systems
- Cross-system consistency and availability risks
- Ownership and operational boundaries
- Whether a design uses canonical approved systems or custom copies

The evaluator should eventually distinguish:

- Direct components
- Linked systems
- Expanded system snapshots
- Runtime-observed components

### Risks

- Catalog may become too complex if users must choose between component, system, design, workspace, and template too early.
- System assets can become stale without versioning.
- Expanded snapshots can diverge from the canonical catalog system.
- Duplicate detection becomes harder because system names often vary across teams.
- Nested systems can become difficult to reason about.

### Guardrails

Phase 1 should keep this constrained:

- One system level only.
- A system can contain components and connectors.
- A system cannot contain another system yet.
- Default insertion is linked/collapsed.
- Expansion is snapshot-only and clearly marked.
- Duplicate suggestions should use normalized names, aliases, owner/team, and tags.
- Creation/editing should be admin or architect controlled.

### Implementation Phases

1. **Catalog Model Base**
   - Add catalog asset `kind`.
   - Add system document metadata.
   - Update duplicate detection.
   - Add catalog filters/tabs for Components and Systems.

2. **Admin Catalog UX**
   - Add Create Component and Create System paths.
   - Show system cards differently from component assets.
   - Allow linking a canonical design later.

3. **Canvas Integration**
   - Add linked system node type.
   - Add catalog systems to the canvas catalog rail.
   - Add inspector details for linked systems.

4. **Save Selection As System**
   - Allow selecting multiple nodes/connectors.
   - Save selected graph as a catalog system.
   - Capture name, owner, description, tags, and criticality.

5. **Analysis Awareness**
   - Let analysis understand linked systems.
   - Optionally expand system documents logically during evaluation.
   - Report linked system dependencies separately from direct components.

### Recommendation

Implement this, but start with **System Asset as a linked/collapsed catalog item**. Avoid nested systems and complex catalog versioning in the first pass. This adds enterprise value without turning the catalog into a full architecture repository too early.

## Product Refinement Proposals

These are the next high-value refinements to make Stratum feel like a premium enterprise architecture product rather than a diagram editor with metadata.

### 1. Journey And Flow Authoring

#### Problem

Large architecture diagrams are hard to traverse. Static arrows show topology, but they do not explain request direction, async behavior, callbacks, decision points, retries, or operational sequences. Reviewers need a guided walkthrough that explains how the system behaves.

#### Proposal

Make Journey mode a first-class flow authoring surface.

Core UX:

- Left or right flow panel with ordered steps.
- Canvas highlights active step path.
- Non-relevant components and connectors dim in view mode.
- Each step can reference:
  - a source component
  - a target component
  - a connector
  - a message/action
  - async event, callback, retry, or failure branch metadata
- Users can insert, reorder, duplicate, and delete steps.
- Journey view mode should be read-only by default.
- Journey edit mode should make flow changes explicit and saveable.

High-level implementation:

1. Extend journey model with step type, source/target ids, connector id, message, direction, and branch metadata.
2. Add journey editor commands: add step, reorder step, remove step, duplicate step.
3. Add canvas highlighting based on active journey step.
4. Add connector visual states for active, previous, upcoming, dimmed, async, callback, and failure.
5. Add tests for journey step ordering and structured design save/load.

Why it matters:

- This makes Stratum useful for design reviews, onboarding, incident review, and architecture explanation.
- It is a core differentiator because the design becomes explainable, not just visible.

### 2. Relationship-Aware Inspector

#### Problem

The inspector currently shows selected object metadata, but it does not yet fully help users understand local graph context. In large designs, users need to answer “what calls this?”, “what does this depend on?”, “where is this reused?”, and “is this tied to catalog?” without manually tracing lines.

#### Proposal

Upgrade the inspector into a relationship-aware component detail panel.

Core UX:

- Details tab: name, type, owner, purpose, criticality, requirements.
- Connections tab:
  - inbound dependencies
  - outbound dependencies
  - connector type
  - sync/async labels
  - linked journey steps
- Catalog tab:
  - linked enterprise asset
  - canonical/deprecated/update status
  - used-in count
  - suggested catalog matches
- Analysis tab:
  - local risks
  - missing metadata
  - consistency/availability/security hints

High-level implementation:

1. Build a graph relationship helper that derives inbound/outbound connection summaries from the structured design.
2. Add inspector tabs without bloating the default view.
3. Surface enterprise catalog linkage and asset health.
4. Add tests for relationship derivation and selected-object inspector rendering.

Why it matters:

- Large diagrams become navigable.
- Catalog investment becomes visible inside normal design work.
- Reviewers can reason from local context instead of scanning the whole canvas.

### 3. Enterprise Catalog As A Governed Asset Workspace

#### Problem

Enterprise catalog should not only store reusable components. It should govern architecture assets over time. Services can be deprecated, replaced, renamed, moved to new ownership, or require consumers to migrate. Designers should be notified when they use outdated or risky catalog assets.

#### Proposal

Evolve Enterprise Catalog into a governed asset workspace.

Core capabilities:

- Asset kinds:
  - component
  - system
- Asset lifecycle:
  - active
  - deprecated
  - retired
  - proposed
- Update advisory:
  - message
  - replacement asset id
  - effective date
  - migration notes
- Designer notifications:
  - design uses deprecated asset
  - linked catalog system has an update
  - replacement is recommended
- Duplicate control:
  - normalized name
  - aliases
  - owner/team
  - tags
  - suggested existing matches

High-level implementation:

1. [x] Add catalog asset fields for `kind`, `status`, `aliases`, `replacementAssetId`, and `updateMessage`.
2. [x] Add migration defaults so existing assets become `component` and `active`.
3. [x] Update Admin Console catalog UI with filters for Components and Systems.
4. [x] Add deprecated/update badges in the canvas inspector and catalog rail.
5. [ ] Add notification jobs or lazy checks when a design opens/saves.
6. Later, add catalog asset versioning and approval workflow.

Why it matters:

- Stratum becomes a living enterprise architecture system.
- Architects can steer teams away from deprecated services.
- Designers get actionable guidance without leaving the design canvas.

## Backlog Proposals

### Collaborative Editing

Add real-time collaborative editing once the structured model and version/review lifecycle are stable.

Foundation completed:

- Authenticated, design-scoped presence with multiple-tab session counts.
- Live whole-document updates when the receiving editor has no local changes.
- SHA-256 document revisions and atomic optimistic concurrency in memory and PostgreSQL storage.
- Explicit conflict resolution that preserves local work instead of silently applying last-write-wins.
- Shared REST and WebSocket authorization boundaries.

The remaining work is operation-level collaboration rather than basic document safety.

High-level implementation:

1. Define operation-level patches for components, connectors, metadata, requirements, journeys, and docs.
2. Use WebSocket sessions for presence and operation broadcast.
3. Add conflict handling rules:
   - last-writer-wins for text fields initially
   - operation ordering for node/edge creation and deletion
   - explicit version save remains manual
4. Add user cursors, selection presence, and edit locks only where needed.
5. Persist final design documents through the existing save/version path.

Guardrail:

- Do not let collaboration bypass ACL, review state, or version lifecycle.
- Do not introduce collaborative editing before core save/load reliability is proven.

### Test Coverage Improvement

Raise confidence through targeted tests, not raw coverage chasing.

Priority areas:

- Backend authorization and ACL decisions.
- Version and review state machine transitions.
- Catalog duplicate detection, deprecation behavior, and usage tracking.
- Design save/load round trips.
- Journey ordering and flow highlighting metadata.
- Analysis report shape and deterministic scoring.
- Frontend canvas smoke tests for add/connect/save/reopen.
- Admin Console tests for integration settings and catalog governance.

High-level implementation:

1. Add backend table-driven tests for each policy/service boundary.
2. Add structured design fixture tests for backward compatibility.
3. Add frontend component tests for inspector, catalog, and lifecycle controls.
4. Add Playwright smoke tests for the critical user path.
5. Keep CI fast; heavier browser tests can run separately or on tags.

### Integration Test Environment

Okta SSO, Prometheus/OpenTelemetry topology import, MCP, Postgres migrations, and all-in-one packaging cannot be validated well with unit tests alone. They need repeatable integration environments that mimic enterprise deployment without requiring real enterprise credentials.

Recommended approach:

1. Add a dedicated integration compose file, separate from the normal local development compose.
2. Use official or widely trusted images only:
   - Postgres for persistence and migration validation.
   - Prometheus for topology query validation.
   - OpenTelemetry Collector or a small fixture exporter for service graph metrics.
   - A lightweight OIDC test provider for SSO contract tests. For Okta specifically, keep a documented manual smoke test because real Okta tenants and policies vary by enterprise.
3. Add seeded fixtures for:
   - users, groups, ACL grants, workspaces, designs, versions, reviews
   - catalog assets and deprecated/replacement catalog metadata
   - Prometheus service graph metrics
4. Add test tiers:
   - unit tests: fast, always run in CI
   - integration tests: run in CI with compose services
   - external-provider smoke tests: opt-in, require real credentials through secure CI secrets
5. Keep provider-specific code behind integration interfaces so Prometheus, future Datadog, New Relic, Okta, and MCP can be tested independently.

Why this matters:

- Enterprise buyers will test setup, persistence, SSO, and observability imports first.
- Broken integrations damage trust faster than missing features.
- Repeatable integration tests prevent regressions when storage, auth, or topology import layers evolve.

Guardrails:

- Do not commit real credentials or tenant-specific Okta settings.
- Do not make external SaaS calls in default CI.
- Keep compose-based integration tests deterministic and disposable.
- Document which tests are official CI gates and which are manual enterprise smoke tests.
