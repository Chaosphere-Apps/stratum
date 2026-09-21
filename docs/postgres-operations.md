# PostgreSQL Operations and Migrations

Stratum ships forward-only PostgreSQL migrations with every backend release. The server and the migration command embed the same SQL files, so operators cannot accidentally apply a schema from a different release.

## Migration Guarantees

- Files execute in lexical order using the `NNN_description.sql` convention.
- Each migration and its `schema_migrations` record commit in one transaction.
- A PostgreSQL advisory lock allows only one migrator to run at a time.
- Lock and statement timeouts prevent an indefinitely blocked rollout.
- SHA-256 checksums detect edits to migrations that have already been applied.
- `status` is read-only. It does not initialize or alter an empty database.
- `check` is read-only and exits unsuccessfully when migrations are pending or unknown, making it suitable for deployment gates.
- The application refuses to start with `AUTO_MIGRATE=false` when its schema is behind or ahead of the running binary.

Applied migration files are immutable. Fix a released migration with a new migration; never edit the old file.

## Commands

From a source checkout:

```bash
MIGRATION_DATABASE_URL='postgres://stratum_migrator:...@db.example.com:5432/stratum?sslmode=verify-full' make migrate-status
MIGRATION_DATABASE_URL='postgres://stratum_migrator:...@db.example.com:5432/stratum?sslmode=verify-full' make migrate-check
MIGRATION_DATABASE_URL='postgres://stratum_migrator:...@db.example.com:5432/stratum?sslmode=verify-full' make migrate-up
```

Using the backend image:

```bash
docker run --rm \
  --network stratum-network \
  -e MIGRATION_DATABASE_URL='postgres://stratum_migrator:...@postgres:5432/stratum?sslmode=require' \
  --entrypoint /migrate \
  ghcr.io/chaosphere-apps/stratum-backend:<release> status

docker run --rm \
  --network stratum-network \
  -e MIGRATION_DATABASE_URL='postgres://stratum_migrator:...@postgres:5432/stratum?sslmode=require' \
  --entrypoint /migrate \
  ghcr.io/chaosphere-apps/stratum-backend:<release> check

docker run --rm \
  --network stratum-network \
  -e MIGRATION_DATABASE_URL='postgres://stratum_migrator:...@postgres:5432/stratum?sslmode=require' \
  --entrypoint /migrate \
  ghcr.io/chaosphere-apps/stratum-backend:<release> up
```

The all-in-one image also contains `/migrate`. Pass `-lock-timeout` or `-statement-timeout` only when a reviewed migration needs different bounds.

## Production Rollout

1. Take and verify a database snapshot or point-in-time recovery checkpoint.
2. Stop writes for migrations whose release notes require a maintenance window. Current migrations are transactional, but large-table `ALTER`, index creation, and backfills can still hold locks.
3. Run `status` using the new release image. Investigate checksum mismatches or unknown migrations; do not bypass them.
4. Run `up` once as a deployment job using the migration database role.
5. Run `check` and require a successful exit before deploying application instances.
6. Deploy the server with the same image tag and `AUTO_MIGRATE=false`.
7. Require `/healthz` for liveness and `/readyz` for traffic readiness.
8. Smoke-test login, workspace listing, design load/save, version creation, and review assignment.

Migrations are forward-only. Application rollback is allowed only when the prior release is schema-compatible. Otherwise restore the pre-deployment snapshot or ship a corrective forward migration.

## Database Roles

Use separate credentials in production:

- `stratum_migrator`: owns the schema and can create or alter objects. Supply it only to the migration job.
- `stratum_app`: receives connection, schema usage, and table DML permissions. Supply it to `DATABASE_URL`.

Configure default privileges so tables created by future migrations are usable by the application role. The runtime role also needs `SELECT` on `schema_migrations` for startup compatibility checks. Store both URLs in the deployment secret manager, never in source control or browser configuration.

## Baseline for 150 Employees

A 150-person organization does not require 150 database connections. Start with:

- one Stratum backend replica, 2 vCPU and 2-4 GB memory;
- managed PostgreSQL 16 or 17, 2-4 vCPU and 4-8 GB memory;
- `DATABASE_MAX_CONNECTIONS=20` and `DATABASE_MIN_CONNECTIONS=2`;
- daily backups plus point-in-time recovery, with a documented restore drill;
- TLS-verified database connections, encrypted storage, and deployment-managed secrets;
- alerts for connection saturation, query latency, locks, storage growth, backup failures, HTTP error rate, and readiness failures.

Size PostgreSQL `max_connections` above the sum of every replica's pool maximum, migration/maintenance connections, and a 20% operational reserve. Add PgBouncer only when multiple services or replicas make connection pressure measurable.

Stratum collaboration presence and WebSocket fanout are currently process-local. A single backend replica is the supported topology for complete collaboration behavior at this scale. Before adding replicas for high availability, introduce shared pub/sub and validate reconnect, presence, and conflict behavior across nodes. PostgreSQL itself should use managed high availability independently of the application replica count.

## Preflight Checklist

- Production is not using `sslmode=disable`.
- `AUTO_MIGRATE=false` is set on application containers.
- The migration job uses the exact application release tag.
- Backups and restore procedures have been tested.
- The database role shown in the Admin console is not a superuser.
- Pool totals fit the database connection budget.
- `/readyz` is removed from load balancing when PostgreSQL is unavailable.
- Migration output is retained with deployment audit records, without logging database URLs.
