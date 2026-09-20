# UI Workflows

## 1. Product Shape

The product should feel like a system design pair workspace. The primary screen is still the canvas, but the user should always have access to structured requirement context, AI suggestions, suite readiness, and findings.

The UI should avoid becoming a long form. It should collect structure through focused panels, inline prompts, quick chips, and editable tables.

## 2. Primary Workspace Layout

Recommended layout:

- Left rail: component catalog, patterns, saved templates.
- Center: canvas.
- Right inspector: selected object metadata, requirements, findings, or suite details.
- Bottom or side timeline: versions, branches, and analysis runs.
- Top bar: design title, status, save version, run analysis, export.

The right inspector should be context-sensitive. Selecting a component shows component metadata. Selecting a connector shows flow metadata. Selecting empty canvas shows design-level requirement brief and suite readiness.

## 3. Start Design Flow

The first screen for a new design should ask for a short problem statement, not an empty diagram.

Flow:

1. User enters a prompt such as "Design a customer support AI assistant."
2. System extracts draft use cases, actors, data, traffic unknowns, and risks.
3. System presents a requirement brief with editable fields.
4. System asks the top three clarification questions.
5. User answers, skips, or accepts default assumptions.
6. System opens the canvas with a requirement checklist and optional starter architecture.

The user should be able to bypass this and start with a blank canvas, but the product should make the guided flow feel faster.

## 4. Requirement Panel

Tabs:

- **Brief:** problem statement, actors, use cases, non-goals.
- **Traffic:** quick estimator and normalized traffic table.
- **Data:** entities, classification, retention, access patterns.
- **Consistency:** requirements by use case or data entity.
- **Assumptions:** approved assumptions and open questions.
- **Coverage:** mapping from requirements to components, connectors, and flows.

Useful interactions:

- "Mark as assumption."
- "Ask reviewer."
- "Apply default."
- "Link to component."
- "Create component from data entity."
- "Create finding from missing context."

## 5. Canvas Interaction

The canvas should remain familiar:

- Drag architecture components from the catalog.
- Connect components with typed connectors.
- Draw boundaries for trust, network, region, tenant, or ownership.
- Select a flow path and name it as a use case flow.
- Use badges for findings, missing metadata, and suite-specific risks.

Inline badges:

- `?` missing required metadata.
- `!` active finding.
- shield icon for security concern.
- pulse icon for observability concern.
- database icon for data or consistency concern.

Badges should open the relevant inspector section directly.

## 6. Architecture Copilot

The Architecture Copilot is a persistent, design-scoped side sheet. It is not a blank general-purpose chat: every answer is grounded in the selected design graph, requirements, and deterministic analysis evidence.

Conversations may follow the working design or be pinned to an immutable saved version. Responses can cite components and connectors; selecting a citation focuses that object on the canvas. Copilot suggestions never modify the design without an explicit human action.

Useful questions include:

Example actions:

- "Estimate traffic from current assumptions."
- "Suggest data stores for these entities."
- "Find missing trust boundaries."
- "Generate a first-pass reliability review."
- "Explain why this queue exists."
- "What should I ask the product team?"

The centrally managed AI provider must be enabled before chat is available. Conversation messages are stored separately from the versioned design document.

## 7. Analysis Suite Panel

The suite panel should make analysis feel actionable:

- A suite list shows readiness, score, and risk count.
- Clicking a suite shows missing inputs, findings, evidence, and next actions.
- A finding can highlight related components and connectors on the canvas.
- A missing input can jump to the exact requirement or metadata field.
- A recommendation can be converted into a design task or reviewer comment.

The user should see when analysis is based on assumptions.

Design Review can target either the working design or an older saved version. Review lenses let users concentrate the synthesis on security, scalability, reliability, data, operability, or cost while deterministic checks remain the evidence baseline. The selected version and lens must remain visible throughout the review.

## 8. Structured Language Inspector

Advanced users should be able to inspect the structured design language behind the canvas.

Views:

- Read-only JSON.
- Read-only YAML.
- Diff against previous version.
- Validation errors.
- References from selected object.

Direct editing can come later. For MVP, the structured view mainly builds trust and helps users export machine-readable designs.

## 9. Review Meeting Mode

Review mode should support a design review conversation:

- Freeze the current version.
- Show requirement brief and top assumptions.
- Walk through named flows.
- Show suite readiness and highest-risk findings.
- Record decisions and comments.
- Mark findings accepted, dismissed, or resolved.
- Save approved version or create follow-up tasks.

This mode turns the product from a drawing tool into a review artifact.

## 10. Concurrent Editing

Stratum shows authenticated users and tab counts for the currently open design. Presence is informational; it never replaces ACL enforcement.

Document updates use optimistic concurrency against a server-issued document revision. Autosaves are serialized so one browser cannot race its own prior save. If another tab or user saves a different document first, Stratum preserves the local draft, pauses canvas editing, and asks the user to either load the server document or explicitly keep the local document. It must never silently discard either side.

This is conflict-safe collaborative editing, not field-level or CRDT merging. Shared cursors, selections, and operation-level merging can build on the same design-scoped session model later without weakening the current save guarantees.

## 11. MVP Screens

Minimum viable screens:

- Project design list.
- New design intake.
- Canvas editor.
- Requirement inspector.
- Component and connector inspector.
- Analysis suite report.
- Findings overlay.
- Version timeline.
- Structured export view.

The first screen after creating a design should be the usable workspace, not a marketing or documentation page.
