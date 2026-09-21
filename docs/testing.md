# Testing Stratum

Stratum uses a risk-based test strategy. Coverage is a release gate, but the purpose of each test is to protect a user-visible behavior or an operational invariant.

## Quality goals

- Coverage cannot fall below the checked-in CI floors. The floors are ratchets, not completion targets: backend is currently 43% statements with a separate 50% HTTP feature boundary; UI is 26% statements, 31% branches, 28% functions, and 27% lines.
- The long-term production target is 90% for testable business modules. Large orchestration shells are additionally protected by browser workflows rather than line coverage alone.
- Every authorization decision, lifecycle transition, persistence operation, and destructive canvas action has a behavioral test.
- Tests are deterministic, parallel-safe, and do not depend on developer data.
- External systems are exercised through protocol-compatible local fakes; PostgreSQL is exercised against a real database.
- A failing optional integration must not break core design, save, review, or ACL workflows.

Coverage does not replace scenario quality. Tests that only execute lines without asserting outcomes are not accepted.

## Test layers

### Backend unit tests

Run pure domain, policy, lifecycle, analysis, integration-mapping, memory-store, and application-service tests:

```bash
make test-backend-unit
```

These tests use Go's standard `testing`, `httptest`, race detector, and coverage tools.

### Backend integration tests

Run repository and migration behavior against an ephemeral PostgreSQL 17 instance:

```bash
make test-backend-integration
```

The test compose file uses `tmpfs`; no developer database or named volume is reused. Integration tests require the `integration` build tag and `TEST_DATABASE_URL`.

Okta/OIDC, Prometheus, AI providers, and MCP protocol behavior should use local HTTP/WebSocket test servers with representative payloads. They must not call vendor services from CI.

### Frontend unit and component tests

Run Vitest with jsdom and Testing Library:

```bash
make test-ui-unit
```

Tests should prefer accessible queries and user interactions. Pure graph transformations, routing, lifecycle, API error mapping, and structured document helpers remain fast unit tests. Components are tested through rendered behavior rather than internal state.

### Browser workflows

Playwright protects high-value cross-surface workflows. The currently automated workflow covers first-admin setup, private workspace creation, name-only design creation, component creation, save, and reload persistence.

The following workflows remain required additions before they can be described as fully covered:

- local sign-in after setup and password reset;
- workspace sharing with direct and group grants;
- connect, move, resize, and delete canvas components;
- create a version, request review, review it, and promote the reviewed version;
- enforce workspace/design ACLs for another user;
- configure stateless/database mode and verify persistence warnings.

Run with:

```bash
make test-e2e
```

Browser tests use a disposable all-in-one test stack and never production credentials.

## Complete local verification

```bash
make test
```

This runs formatting/type checks, unit tests, integration tests, race detection, coverage gates, production builds, and the critical browser suite.

Coverage output is written under `artifacts/coverage/`. Generated coverage and browser artifacts are ignored by Git and uploaded by CI on failure.

## Coverage policy

Coverage is enforced in CI and ratcheted upward as scenarios are added. It is not currently accurate to claim 90% repository coverage. Critical packages are held to a stronger behavioral standard:

| Surface | Required behavior |
| --- | --- |
| Policy and ACL | Every role, direct grant, group grant, inheritance, denial, and malformed grant |
| Versions and reviews | Every allowed and rejected transition, reviewer state, live-version uniqueness |
| Storage | Memory/PostgreSQL parity, migrations, rollback, concurrency, pagination, exact JSON preservation |
| HTTP/WebSocket | Authentication, authorization, limits, errors, disconnects, retries, malformed payloads |
| Canvas model | Create, move, resize, connect, reconnect, copy, duplicate, delete, grouping, save/load |
| Integrations | Disabled mode, timeouts, malformed upstream data, authentication, filtering, mapping |

Coverage thresholds are ratcheted upward and may never be lowered to make a change pass. Generated code, type-only declarations, and process entrypoints may be excluded; product behavior may not.

## Test data and secrets

- Use factories/builders for users, workspaces, designs, components, and reviews.
- Use clearly fake domains and tokens.
- Never record real Okta tokens, API keys, design documents, email addresses, or database URLs in snapshots or logs.
- CI secrets are not available to pull-request tests.

## Bug protocol

When a new test exposes an existing product bug, document the failing scenario and expected behavior before changing production code. The bug fix and regression test should be reviewed together.

## Tool licensing

- Go test, coverage, race detector: Go project BSD-style license.
- Vitest and Testing Library: MIT.
- jsdom: MIT.
- Playwright: Apache-2.0.
- PostgreSQL Docker Official Image: PostgreSQL license and bundled component licenses.

No commercial test runner or hosted test dependency is required.
