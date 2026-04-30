# Evaluation Suites

## 1. Goal

Evaluation should be modular. A single generic score is useful as a summary, but architecture review is better represented as suites that inspect different concerns with different evidence requirements.

Suites can run independently, produce their own findings, and contribute to an overall readiness view.

## 2. Suite Model

Each suite should define:

- `id`
- `name`
- `description`
- `requiredInputs`
- `deterministicChecks`
- `modelChecks`
- `scoringDimensions`
- `evidenceRules`
- `outputSchema`
- `readinessCriteria`
- `defaultWeight`

Suites should report two separate numbers:

- **Readiness:** whether enough information exists to evaluate the concern.
- **Quality:** how well the design satisfies the concern.

This prevents a design from scoring well simply because important details are missing.

## 3. Default Suites

### Requirement Understanding

Purpose:

- Determine whether the system goal, use cases, actors, traffic, data, consistency, constraints, and assumptions are clear enough for evaluation.

Inputs:

- Requirement brief.
- Use cases.
- Traffic estimate.
- Data profile.
- Consistency requirements.
- Open questions.

Findings:

- Missing primary actor.
- Use case lacks success criteria.
- Traffic estimate has no peak multiplier.
- Consistency requirement is unspecified for critical data.
- Requirement conflicts with stated non-goal.

### Traffic and Capacity

Purpose:

- Evaluate whether the design can plausibly handle expected load and growth.

Inputs:

- Traffic estimate.
- Component capacity metadata.
- Connector fanout and payload size.
- Scaling strategy.
- Rate limits and quotas.

Checks:

- Peak QPS derivation.
- Read/write ratio against data store choices.
- Hot partition risk.
- Synchronous fanout latency.
- Queue throughput and backlog behavior.
- External API quota risk.
- Cache hit-rate assumptions.

Output examples:

- "Peak write traffic is estimated at 2,000 QPS, but the single SQL primary has no sharding, partitioning, or write-scaling explanation."
- "The design includes a queue but no worker concurrency or backlog drain target."

### Consistency and Data Integrity

Purpose:

- Evaluate whether data guarantees match business requirements.

Inputs:

- Data entities.
- Consistency requirements.
- Data stores.
- Replication and caching connectors.
- Write paths.
- Conflict handling.

Checks:

- Strong consistency needs backed by appropriate storage or transaction boundary.
- Cache invalidation or staleness budget defined.
- Duplicate event handling.
- Idempotency on retryable writes.
- Ordering requirements for event streams.
- Read-after-write paths for user-visible updates.

### Reliability and Failure Handling

Purpose:

- Evaluate availability, durability, failover, retries, degradation, and recovery paths.

Inputs:

- Availability targets.
- Component criticality.
- Failure modes.
- Retry policies.
- Backups and replication.
- Fallback behavior.
- Observability metadata.

Checks:

- Single points of failure.
- Missing timeout or retry policy.
- Retry storm risk.
- Queue without dead-letter path.
- Database without backup or recovery target.
- AI model without fallback.
- Regional dependency without failover plan.

### Security and Privacy

Purpose:

- Evaluate trust boundaries, authentication, authorization, sensitive data flow, encryption, retention, and auditability.

Inputs:

- Data classification.
- Connectors and auth metadata.
- Trust boundaries.
- Identity components.
- Security controls.
- Compliance needs.

Checks:

- Sensitive data crossing a trust boundary without auth or encryption.
- Internet-facing endpoint without WAF, gateway, or rate limiting.
- Admin path without stronger controls.
- LLM receives restricted data without redaction, policy checks, or audit trail.
- Secrets or keys not represented.
- Retention and deletion requirements missing for regulated data.

### Observability and Operations

Purpose:

- Evaluate whether the system can be monitored, debugged, audited, and operated.

Inputs:

- Component observability metadata.
- SLOs.
- Alerting paths.
- Logs, metrics, traces, audit events.
- Runbook and owner metadata.

Checks:

- Critical component without metrics.
- User-facing flow without latency SLI.
- No error budget or alert threshold.
- Missing audit log for sensitive action.
- No owner or escalation path.

### Cost and Efficiency

Purpose:

- Evaluate whether the design accounts for infrastructure, model, storage, and traffic costs.

Inputs:

- Traffic estimate.
- Component sizing.
- Storage growth.
- AI model usage.
- External API pricing assumptions.
- Retention.

Checks:

- LLM cost per request and monthly cost estimate.
- Storage growth over retention period.
- Cache or queue added without reason.
- Expensive synchronous model calls in high-QPS path.
- Data egress risk.

### AI and Model Risk

Purpose:

- Evaluate AI-specific architecture concerns.

Inputs:

- AI components.
- Prompts, tools, policies, datasets.
- Input and output data classification.
- Fallbacks.
- Evaluation criteria.

Checks:

- Prompt and model version ownership.
- PII handling before model call.
- Tool permission boundaries.
- Hallucination containment.
- Human review path for low confidence or high-risk actions.
- Model latency and cost budget.
- Evaluation dataset and acceptance threshold.
- Auditability of model decisions.

## 4. Suite Readiness

A suite can be:

- `ready`: enough structured data exists.
- `partial`: can run, but results need caveats.
- `blocked`: missing required inputs.
- `not_applicable`: concern does not apply to this design.

Example:

```json
{
  "suite": "traffic_capacity",
  "readiness": "partial",
  "missingInputs": ["peakQps", "averagePayloadSizeBytes"],
  "canRun": true,
  "caveat": "Capacity analysis uses estimated traffic defaults."
}
```

## 5. Suite Output Schema

```json
{
  "suiteId": "security_privacy",
  "readiness": "ready",
  "qualityScore": 72,
  "summary": "The design has basic service auth but lacks controls around restricted data sent to the LLM.",
  "findings": [],
  "questions": [],
  "evidence": [],
  "assumptionsUsed": [],
  "recommendedNextActions": []
}
```

Findings should keep the same global finding model used by analysis runs, with `suiteId` added.

## 6. Suite UI

The analysis panel should show suites as tabs or a left-side list:

- Requirements
- Traffic
- Consistency
- Reliability
- Security
- Observability
- Cost
- AI Risk

Each suite card should show:

- Readiness state.
- Quality score, if available.
- Critical findings count.
- Missing inputs.
- Main recommended action.

Users should be able to run:

- All applicable suites.
- A selected suite.
- Fast deterministic checks only.
- Deep AI analysis.

## 7. Overall Score

The overall score should be computed from suite quality scores, weighted by rubric configuration. Suites marked `blocked` should lower confidence and may lower the overall score depending on workspace policy.

The overall report should include:

- `overallScore`
- `confidence`
- `blockedSuites`
- `highestRiskFindings`
- `requirementCompleteness`
- `nextBestQuestions`

## 8. MVP Suite Order

Recommended MVP order:

1. Requirement Understanding.
2. Security and Privacy.
3. Reliability and Failure Handling.
4. Traffic and Capacity.
5. AI and Model Risk.
6. Observability and Operations.
7. Consistency and Data Integrity.
8. Cost and Efficiency.

This order supports the first enterprise AI-system scenario while still applying to general backend designs.

