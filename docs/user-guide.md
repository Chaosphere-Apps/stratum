# Stratum user guide

Stratum gives engineering teams one governed workspace for architecture diagrams and the context needed to review them. A design is more than a picture: its requirements, components, connections, flows, documents, versions, comments, and analysis remain attached to the same artifact.

## Home and workspaces

The home screen shows recent designs, workspace activity, items needing attention, and quick-start actions. New users without personal history see activity from workspaces they can access.

Workspaces group designs for a team or system. A new workspace is private by default and can be shared with users or access groups. Access labels show whether you can view, create, edit, review, or manage content before you open it. Search and pagination help with larger installations.

## Create a design

Only a name is required. The optional requirements section is recommended because measurable context makes reviews and analysis more useful. It captures:

- the use case and functional requirements;
- consistency, availability, and non-functional requirements;
- expected requests per second and SLA targets.

Open questions can be recorded after the design is created. Field limits and remaining-character counters are shown in the form and are also enforced by the backend.

## Model the system

The canvas uses architecture-aware components rather than unstructured drawing primitives. Add components from the catalog, connect them with typed relationships, and edit their metadata in the inspector. Components retain semantic data used by analysis and export; canvas position and size are presentation data.

Use request journeys to record an ordered path through the design. Journeys can be highlighted on the canvas to review how a request, event, or data flow crosses the system. Notes and attached documents capture decisions, assumptions, rollout context, and supporting links without crowding the diagram.

Saving uses optimistic revision checks. If another session changes the design first, Stratum preserves the local draft and asks how to resolve the conflict rather than silently overwriting work. The header shows save progress and confirmation.

## Analyze a design

Design Review combines deterministic checks with optional AI synthesis. It evaluates available evidence across requirements, topology, traffic and capacity, consistency, availability, security, and data handling.

Readiness reflects whether the inputs needed for a meaningful review exist. An empty design receives no artificial readiness credit. Findings identify the affected area, explain why it matters, and suggest a concrete next step. A review can target the working design or a saved version and can use a focused lens such as security, scalability, reliability, data, operability, or cost.

Deterministic analysis works without an AI provider. When AI is unavailable or rejects a request, Stratum keeps deterministic results and reports the synthesis failure separately.

## Use AI chat safely

An administrator must configure and verify a supported provider before AI chat is available. Stratum supports OpenAI, Anthropic, OpenRouter, Google AI Studio (Gemini), and custom OpenAI-compatible endpoints.

Each conversation has its own design access mode:

- **Read only** lets the assistant inspect a compact representation of the selected design and discuss it.
- **Read + edit** lets the assistant propose validated changes to the current working design only when you already have edit permission.

Read only is the default. Saved-version conversations cannot write. Backend authorization is rechecked for every message, and proposed edits must pass schema, graph, identity, size, and revision validation before storage. Follow-up messages retain a bounded amount of recent conversation context.

## Versions, reviews, and collaboration

Create an immutable version when a design is ready for a checkpoint or review. Reviewers can comment and move a review through its supported states. Notifications surface relevant review activity. Version history lets the team compare progress without treating every autosave as a formal milestone.

Presence shows who currently has the design open. It is informational and never grants access. Collaboration uses conflict-safe saves rather than field-level live merging.

## Permissions

Permissions may come from a direct user grant or an access group and can differ between a workspace and an individual design. Depending on the grant, a user may be able to read, create designs, edit, comment, review, or manage access.

The backend is always authoritative. Hiding a control in the browser is not treated as authorization. Contact a workspace manager or administrator if an expected action is unavailable.

## Administration

Organization administrators manage users and roles, local sign-in, SSO, access groups, workspace/design grants, catalog assets, persistent storage, telemetry, MCP capabilities, and the shared AI provider. See [Administration and SSO](admin-and-sso.md) for operational details.
