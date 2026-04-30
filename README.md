# Stratum

## 1. Purpose

Build an enterprise-first system design workspace where teams can draw, store, version, evaluate, and export architecture designs.

The product is inspired by interview-focused tools such as Mockingly.ai and LeetSys, but the first target is not candidate interview practice. The first target is engineering teams that need a durable place to reason about AI system designs, backend architectures, product infrastructure, and platform trade-offs.

Stratum should feel like a focused Miro or Excalidraw-style canvas for system design, with built-in architecture components, structured metadata, AI analysis, version history, and exportable design language.

## 2. Document Map

- [Docs Index](docs/): public product and technical documentation.
- [UI Service](docs/ui.md): React Flow canvas, component catalog, inspectors, backend sync, and structured JSON export.
- [Backend Service](docs/backend.md): Go backend for workspaces, designs, versions, WebSocket sync, Postgres storage, and deterministic analysis.
- [Backend Production Readiness](docs/backend-production-readiness.md): Hardened areas, known production gaps, and next backend steps.
- [System Architecture](docs/system-architecture.md): High-level technical architecture and major services.
- [Platform and Tech Stack](docs/platform-and-tech-stack.md): Frontend, backend, storage, AI, and deployment tradeoffs.
- [Domain Model](docs/domain-model.md): Core workspace, design, component, connector, version, and analysis objects.
- [Structured Design Language](docs/structured-design-language.md): Machine-readable design representation used for validation, exports, diffs, and AI evaluation.
- [Evaluation Engine](docs/evaluation-engine.md): How AI analysis, scoring, evidence, and recommendations should work.
- [Evaluation Suites](docs/evaluation-suites.md): Modular suites for requirements, traffic, consistency, reliability, security, observability, cost, and AI risk.

## Local Stack

Run the persistent backend stack:

```bash
docker compose up -d backend
```

This starts the Go backend on `http://127.0.0.1:8081` and Postgres on `127.0.0.1:5432` with a named Docker volume, so workspaces and designs survive backend restarts.

## 3. Short Product Definition

An enterprise system design evaluator is a persistent architecture workspace that lets teams:

- Create system design diagrams using fixed architecture-aware components.
- Connect components with typed data, control, event, and dependency relationships.
- Add requirements, assumptions, capacity estimates, constraints, and business context.
- Attach their own AI systems, prompts, agents, or architecture proposals.
- Run AI analysis against company-specific rubrics.
- Store every design as a versioned artifact.
- Compare versions and track design maturity over time.
- Export diagrams and structured architecture definitions for reviews, docs, tickets, audits, and implementation planning.

## 4. Initial Product Bet

Most AI design tools produce text or one-off diagrams. Most whiteboards are flexible but semantically weak. Most enterprise architecture tools are heavyweight and slow.

The opportunity is a middle layer:

- Fast and visual enough for product and engineering teams.
- Structured enough for automated analysis.
- Versioned enough to become a design system of record.
- AI-native enough to evaluate designs continuously.

The first wedge should be enterprise teams building or evaluating AI-backed systems, because they already need stronger review loops around latency, cost, reliability, privacy, model risk, fallback behavior, observability, and data flow.
