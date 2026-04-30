# Formal and Mathematical Architecture Analysis

## 1. Why This Matters

AI can review architecture well when the design is structured, but it should not be the only evaluator. Some architecture concerns can be checked with deterministic, mathematical, or formal methods.

The product should combine:

- Deterministic rules.
- Graph algorithms.
- Queueing and capacity models.
- Reliability math.
- Consistency and dependency analysis.
- Security policy checks.
- AI reasoning for ambiguous tradeoffs and recommendations.

This makes the evaluator more trustworthy. AI explains and synthesizes; deterministic analysis catches things that should not depend on model judgment.

## 2. Architecture as a Graph

Represent each design as a typed directed graph:

- Nodes are components.
- Edges are connectors.
- Node attributes include type, capacity, owner, region, criticality, data handled, and failure behavior.
- Edge attributes include protocol, latency, timeout, retry policy, payload, auth, data classification, and consistency requirement.

Once the system has a graph, many checks become mathematical graph problems.

## 3. Graph Algorithms

### Reachability

Questions:

- Can public internet traffic reach a restricted data store?
- Can restricted data reach an LLM or third-party API?
- Is every critical component reachable from an observability path?
- Does every user-facing path pass through authentication?

Useful algorithms:

- Breadth-first search.
- Depth-first search.
- Path queries over typed edges.
- Transitive closure for dependency and data-flow analysis.

### Cut Sets and Single Points of Failure

Questions:

- Which component failures disconnect a critical use case?
- Is there a single gateway, database, region, queue, or external API on every success path?

Useful algorithms:

- Articulation points.
- Minimum cut.
- Edge cut analysis.
- Dominator trees for directed request flows.

### Cycles

Questions:

- Can retry loops create recursive failure?
- Do event flows create feedback loops?
- Are control-plane dependencies circular?

Useful algorithms:

- Cycle detection.
- Strongly connected components.
- Topological sorting for acyclic workflows.

### Critical Path

Questions:

- What is the longest synchronous dependency chain?
- Does the p95 latency budget survive downstream calls?
- Which dependency dominates end-to-end latency?

Useful algorithms:

- Longest path in a DAG for known synchronous chains.
- Weighted shortest or longest path depending on flow semantics.
- Path aggregation with latency distributions.

## 4. Capacity and Queueing Models

Traffic analysis can be partly mathematical.

Inputs:

- Average QPS.
- Peak QPS.
- Read/write ratio.
- Fanout.
- Service time.
- Worker concurrency.
- Queue arrival rate.
- Queue service rate.
- Payload size.
- Storage growth.

Useful models:

- Little's Law: `L = lambda * W`
- Utilization: `rho = lambda / mu`
- M/M/1 or M/M/c queue approximations for early-stage estimates.
- Backlog drain time.
- Storage growth over retention period.
- Bandwidth requirements: `qps * payload_size * fanout`.

Example findings:

- "Peak arrival rate is 5,000 jobs/minute but worker drain capacity is 2,000 jobs/minute, so backlog grows during peak."
- "The p95 synchronous path includes four serial calls; the sum of stated p95 budgets already exceeds the 2 second user requirement."
- "Object storage grows by 18 TB/month under the stated retention policy."

These models do not need perfect precision. They are most useful for catching order-of-magnitude mistakes.

## 5. Reliability Math

Availability and durability can be estimated from component assumptions.

Useful calculations:

- Serial availability approximation: multiply dependency availability along a required path.
- Parallel redundancy approximation: `1 - product(failure probabilities)`.
- Recovery target checks: compare RTO/RPO requirements against backup, replication, and failover metadata.
- Retry amplification: estimate increased load caused by retries during partial failure.
- Blast-radius analysis: count impacted use cases, tenants, regions, or data entities for a component failure.

Example:

If a user-facing flow requires API gateway, service, database, and payment provider synchronously, the effective availability cannot exceed the weakest required dependency and is usually lower than any single component target.

The tool should present these as estimates with assumptions, not as certified reliability guarantees.

## 6. Consistency and Data Integrity Checks

Some consistency concerns can be checked structurally.

Examples:

- A use case marked `strong_consistency` should not rely on asynchronous replication before returning success.
- Retryable write connectors should define idempotency.
- Event consumers should define duplicate handling.
- Ordered event requirements should map to a queue or stream configuration that preserves ordering at the required key scope.
- Cache reads should have invalidation, TTL, or staleness budget metadata.
- Multi-writer data stores should define conflict resolution.

These are rule and graph checks. AI can explain tradeoffs, but the mismatch itself is deterministic.

## 7. Security and Policy Analysis

Security can use policy-as-code style checks.

Examples:

- Restricted data must not cross a public trust boundary without encryption.
- Internet-facing endpoints must have authentication, rate limiting, and WAF/API gateway controls.
- LLM calls with sensitive data must pass through redaction or policy-filter components.
- Admin connectors must use stronger auth.
- Secrets must not be modeled as regular payloads.
- Regulated data must have retention and deletion metadata.

Implementation options:

- Custom deterministic check engine for MVP.
- Later: policy language such as Rego/OPA-style rules for workspace-specific policies.

## 8. Formal Methods, Carefully Scoped

Full formal verification of arbitrary enterprise architecture is unrealistic for MVP. But small formal techniques are useful:

- Invariant checks: "no restricted data reaches external systems without approved control."
- Temporal checks over flows: "payment is authorized before order is confirmed."
- State-machine checks for lifecycle-heavy systems.
- SAT/SMT-style constraint solving for placement, residency, policy, or dependency constraints.

These should be applied to narrow, high-value checks. The product should not imply it can formally prove an entire architecture correct.

## 9. How This Works With AI

Recommended evaluation split:

- Deterministic engine finds objective issues.
- Mathematical engine estimates capacity, latency, reliability, and growth.
- Policy engine checks organization-specific constraints.
- AI explains findings, detects missing context, compares alternatives, and reasons about ambiguous tradeoffs.

AI should consume deterministic results as evidence. It should not independently re-infer facts that the system can compute.

Example pipeline:

1. Parse structured design language.
2. Build typed graph.
3. Run graph, capacity, reliability, consistency, and policy checks.
4. Produce evidence objects.
5. Ask AI to synthesize risks and recommendations using those evidence objects.
6. Validate AI output against schema.

## 10. MVP Formal Analysis Scope

Start with:

- Graph reachability.
- Missing controls on sensitive data paths.
- Synchronous critical path latency budget.
- Queue throughput and backlog estimate.
- Storage growth estimate.
- Single point of failure detection for named critical flows.
- Retry/idempotency mismatch.
- Cache consistency/staleness mismatch.
- Requirement-to-design coverage.

These are achievable and valuable without pretending the system is a formal verification platform.

