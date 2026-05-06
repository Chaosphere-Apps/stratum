# Structured Design Language

## 1. Purpose

The structured design language is the machine-readable representation of a system design. It should be friendly enough to export, diff, import, and review, while being precise enough for deterministic checks and AI evaluation.

The visual canvas is one view of the design. The structured design document is the source of truth for analysis.

## 2. Design Principles

- Use stable IDs for every requirement, component, connector, data entity, assumption, and finding reference.
- Separate user facts from system assumptions.
- Keep visual layout separate from architectural semantics.
- Make every relationship typed.
- Prefer enums and structured fields where evaluation depends on precision.
- Preserve readable names and descriptions so exports work in human documents.
- Support partial designs with explicit unknowns instead of forcing fake certainty.

## 3. Top-Level Shape

```yaml
schemaVersion: sdl/v0.1
design:
  id: customer-support-ai
  title: Customer Support AI Assistant
  version: 4
  status: draft
  summary: AI-assisted support workflow with human fallback.
context:
  workspaceId: acme
  projectId: support-platform
  ownerTeam: support-engineering
requirements:
  brief: {}
  useCases: []
  qualityAttributes: []
  traffic: {}
  dataEntities: []
  consistency: []
  assumptions: []
  openQuestions: []
architecture:
  components: []
  connectors: []
  boundaries: []
  flows: []
layout:
  nodes: []
  edges: []
analysis:
  includedRuns: []
traceability:
  links: []
```

## 4. Requirement Objects

Requirements should have IDs that can be referenced by components, connectors, flows, and findings.

```yaml
qualityAttributes:
  - id: qa-latency-chat
    type: latency
    target: p95 <= 3000ms
    scope: usecase.answer-customer-question
    priority: must
    source: user_provided

  - id: qa-availability
    type: availability
    target: 99.9% monthly
    scope: system
    priority: should
    source: assumption
```

Supported `priority` values:

- `must`
- `should`
- `could`
- `wont`

Supported `source` values:

- `user_provided`
- `estimated`
- `assumption`
- `imported`
- `model_inferred`

## 5. Component Objects

```yaml
components:
  - id: support-api
    type: compute.service
    name: Support API
    owner: support-platform
    purpose: Handles support conversation requests.
    criticality: high
    metadata:
      runtime: container
      scaling: horizontal
      expectedQps: 250
      latencyBudgetMs: 400
    data:
      reads: [account-context]
      writes: [conversation]
      classification: restricted
    reliability:
      failureMode: returns fallback response or queues for human review
      retryable: true
    observability:
      metrics: true
      logs: true
      traces: true
      auditLog: true
    supports:
      - usecase.answer-customer-question
      - qa-latency-chat
```

Component `type` should use dot notation:

- `client.web`
- `edge.api_gateway`
- `compute.service`
- `compute.worker`
- `data.sql_database`
- `data.cache`
- `messaging.queue`
- `ai.llm`
- `ai.agent`
- `design.link`
- `security.policy_filter`
- `observability.metrics`
- `external.partner_api`

Linked design components reference another design that the current user can already see. The link stores enough metadata for evaluation and display, but the target design should still be fetched through the normal design APIs when opened.

```yaml
components:
  - id: payments-subsystem-link
    type: design.link
    name: Payments Subsystem
    purpose: Points reviewers to the lower-level payment architecture.
    metadata:
      linkedDesign:
        workspaceId: workspace-payments
        designId: design-payments-subsystem
        title: Payments Subsystem
        access: workspace
        updatedAt: 2026-04-30T10:00:00Z
```

## 6. Connector Objects

```yaml
connectors:
  - id: support-api-to-llm
    from: support-api
    to: support-llm
    type: model_call
    protocol: https
    payload:
      fields: [customer_message, account_summary]
      classification: restricted
      averageSizeBytes: 12000
    auth:
      method: service_identity
      encryption: tls
    behavior:
      timeoutMs: 3000
      retryPolicy: bounded_exponential_backoff
      idempotency: required
      ordering: not_required
    consistency:
      model: read_after_write
      reason: user must see current account context
    supports:
      - usecase.answer-customer-question
```

Connector `type` values:

- `synchronous`
- `asynchronous_event`
- `batch_transfer`
- `data_replication`
- `cache_read`
- `cache_write`
- `control_plane`
- `human_approval`
- `model_call`
- `observability_signal`

## 7. Flows And Journeys

Flows and journeys represent end-to-end behavior across components and connectors. They are especially useful for AI evaluation because many risks appear across paths, not individual boxes.

In the UI, a journey is an ordered walkthrough for reviewers: each step can reference a component, a connector, or both. This gives Stratum a Figma-like prototype mode for request direction, user journeys, failure paths, and operational runbooks without relying on free-form canvas annotations.

```yaml
journeys:
  - id: journey-answer-question
    title: Answer customer question
    description: Primary support request path
    useCase: usecase.answer-customer-question
    entryComponentId: web-app
    steps:
      - id: step-web-app
        componentId: web-app
        title: Customer opens support
        description: The user starts in the web application.
      - id: step-api
        connectorId: web-to-support-api
        componentId: support-api
        title: Submit question
        description: The web app sends a synchronous request to the support API.
```

## 8. Attached Text Documents

Some design context should remain textual: decisions, rollout notes, API contracts, migration plans, threat-model notes, or links to external docs. Attached design documents are stored as their own backend resource so large docs do not inflate every structured design save.

```yaml
designDoc:
  id: doc-rollout-plan
  workspaceId: workspace-payments
  designId: design-fep-cloud
  title: Rollout plan
  body: <p>Start with one region, monitor queue depth, then expand after soak testing.</p>
  format: html
  createdAt: 2026-04-30T10:00:00Z
  updatedAt: 2026-04-30T10:00:00Z
```

Primary endpoints:

- `GET /api/workspaces/{workspaceID}/designs/{designID}/docs`
- `POST /api/workspaces/{workspaceID}/designs/{designID}/docs`
- `GET /api/workspaces/{workspaceID}/designs/{designID}/docs/{docID}`
- `PATCH /api/workspaces/{workspaceID}/designs/{designID}/docs/{docID}`
- `DELETE /api/workspaces/{workspaceID}/designs/{designID}/docs/{docID}`

## 9. Boundaries

Boundaries make security, ownership, and deployment evaluation easier.

Boundary types:

- `trust`
- `network`
- `workspace`
- `region`
- `tenant`
- `compliance`
- `ownership`
- `deployment`

```yaml
boundaries:
  - id: public-internet
    type: trust
    name: Public Internet
    contains: [web-app]
  - id: private-services
    type: network
    name: Private Service Network
    contains: [support-api, pii-filter, support-llm]
```

## 9. Unknowns

Unknowns should be explicit.

```yaml
unknowns:
  - id: unknown-traffic-peak
    field: requirements.traffic.peakQps
    impact: capacity estimates and scaling analysis may be unreliable
    severity: medium
    suggestedQuestion: What peak QPS should the system handle during launch events?
```

This avoids the product inventing confidence where the design has not supplied enough context.

## 10. Traceability Links

```yaml
traceability:
  links:
    - from: qa-latency-chat
      to: support-api
      relation: constrains
    - from: data.account-context
      to: support-api-to-llm
      relation: flows_through
    - from: assumption-human-fallback
      to: human-review-queue
      relation: implemented_by
```

Supported relations:

- `supports`
- `constrains`
- `implements`
- `depends_on`
- `flows_through`
- `mitigates`
- `violates`
- `unknown_for`

## 11. Validation Levels

The language should support staged validation:

- **Parse validation:** document is valid YAML or JSON.
- **Schema validation:** required fields and enum values are valid.
- **Reference validation:** IDs point to existing objects.
- **Graph validation:** connectors reference valid components and flows are connected.
- **Semantic validation:** component and connector metadata make architectural sense.
- **Suite readiness validation:** enough data exists to run a given analysis suite credibly.

## 12. Export Strategy

MVP exports should include:

- JSON as the canonical machine-readable format.
- YAML as the human-editable structured format.
- Markdown as a readable design brief with embedded diagrams and findings.
- Mermaid generated from the same component and connector graph.

The UI should let users inspect the structured document behind the canvas, but most users should not need to author it directly.
