# ProjectFlow Backend Development Checklist

This checklist follows the MVP backend phases in `PRD_MVP.md`. BE-01 through BE-12 are implemented. Reporting and contract-document upload routes remain planned and are not part of the current API.

## BE-01 — Project foundation and configuration

- [x] Initialize Go module and backend package structure.
- [x] Add validated environment configuration for port, environment, timezone, database URL, and frontend origin.
- [x] Add Gin HTTP server with health and database-readiness endpoints.
- [x] Add PostgreSQL/GORM connection initialization.
- [x] Add structured request logging, safe error responses, CORS allowlisting, and graceful shutdown.
- [x] Add local PostgreSQL/API Docker Compose services and environment template.
- [x] Add automated tests for configuration, health/readiness, and CORS.

### Run the foundation locally

1. Copy `.env.example` to `.env` for Compose defaults.
2. Run `docker compose up --build`.
3. Check `http://localhost:4000/health` and `http://localhost:4000/ready`.
4. The `db-setup` service applies migrations and seeds demo records in non-production environments.

Run database actions through Compose so they use the configured container database:

```powershell
docker compose run --rm db-setup up
docker compose run --rm db-setup version
docker compose run --rm db-setup down
```

`down` rolls back one migration and is refused when `APP_ENV=production`. Demo data is enabled by default outside production; set `SEED_DEMO_DATA=false` to opt out or `SEED_DEMO_DATA=true` to opt in explicitly. A production seed request fails.

The demo manager account is inactive and uses an invalid password hash, so it cannot be used to log in. Sample budget transactions include posted commitment/expense records and a draft expense, which should be excluded from totals.

The Compose credentials are for local development only. Do not reuse them outside a local environment.

## BE-02 — Database migrations and seeders

- [x] Define MVP relational schema, constraints, indexes, and UUID conventions.
- [x] Add versioned PostgreSQL migrations and rollback support.
- [x] Add seed data for roles, permissions, departments, project, budgets, and sample transactions.
- [x] Gate demo seeding to non-production environments and make it repeatable.
- [x] Track seed completion so service restarts do not reset non-production data.
- [x] Verify migrations apply and roll back against a clean database.

## BE-03 — Authentication and RBAC

- [x] Implement user credentials and bcrypt password hashing.
- [x] Implement login, logout, refresh-token rotation/revocation, and current-user endpoints.
- [x] Implement signed JWT authentication middleware and database-backed role/permission checks.
- [x] Protect refresh/logout cookies with HttpOnly, SameSite, Secure-in-production, and origin validation.
- [x] Add authentication, authorization, configuration, and token lifecycle tests.

Production must set `JWT_ACCESS_SECRET` to a unique secret of at least 32 characters. Access tokens default to 15 minutes and refresh tokens to 7 days; both lifetimes are configurable. Refresh-token hashes are stored server-side and rotated on every refresh. In non-production, the demo manager account stays disabled until explicitly activated:

```powershell
$env:DEMO_USER_PASSWORD = "choose-a-local-development-password"
docker compose run --rm db-setup activate-demo-user
Remove-Item Env:DEMO_USER_PASSWORD
```

Sign in at `POST /api/v1/auth/login` with `demo.manager@projectflow.local` and the password supplied for that local activation. Demo-user activation is refused in production.

To access user and department administration locally, activate the seeded super-admin role with a password of at least 12 characters:

```powershell
$env:DEMO_ADMIN_PASSWORD = "choose-a-strong-local-admin-password"
docker compose run --build --rm db-setup activate-demo-admin
Remove-Item Env:DEMO_ADMIN_PASSWORD
```

Sign in with `demo.admin@projectflow.local` and the password supplied for activation. This grants the full `SUPER_ADMIN` role, is refused in production, and is intended only for local development.

## BE-04 — Organization and user management

- [x] Implement department and user CRUD/status endpoints.
- [x] Implement department assignment and role assignment.
- [x] Add validation, pagination, filtering, and authorization tests.

## BE-05 — Project management

- [x] Implement project CRUD, team assignment, status, progress, and archive operations.
- [x] Add concurrency-safe project code generation.
- [x] Enforce ownership and department access rules.
- [x] Add project validation, filtering, and access-scope tests; verify live CRUD/API flows against local PostgreSQL.

## BE-06 — Contract management

- [x] Implement contract CRUD, search, filters, and project relationships.
- [x] Implement dynamic expiration classification and renewal history.
- [x] Enforce owner, date, and non-negative value rules.
- [x] Add expiration-boundary, renewal-rule, and validation tests; verify live contract APIs against local PostgreSQL.

## BE-07 — Tasks and milestones

- [x] Implement task and milestone CRUD, assignment, status, and progress.
- [x] Detect overdue tasks and delayed/upcoming milestones using the configured business timezone.
- [x] Record planned-date changes in schedule history and preserve task/milestone actual dates.
- [x] Add timeline, authorization, validation, actual-date, and business-rule tests.

## BE-08 — Budget management

- [x] Implement budget categories, project allocations, summaries, and financial transaction endpoints.
- [x] Calculate actual, unspent committed, forecast, remaining, variance, and utilization from posted transactions without double-counting linked commitment expenses.
- [x] Enforce decimal precision, audited transaction state transitions, and atomic traceable corrections.
- [x] Add budget calculation, threshold, validation, and transaction-filter tests; verify local API/database flows.

## BE-09 — Dashboard aggregation

- [x] Implement project, contract, timeline, alert, and budget dashboard aggregates.
- [x] Derive dashboard values from scoped source records and posted transaction calculations; aggregate budgets separately by currency.
- [x] Add aggregation, filter, and project/contract read-scope tests.

## BE-10 — Notifications and scheduled jobs

- [x] Add Redis/Asynq workers, timezone-aware schedules, graceful shutdown, and retry handling.
- [x] Implement contract expiration, overdue task, delayed milestone, budget threshold, and project status notifications.
- [x] Add deterministic notification keys and database uniqueness for duplicate prevention.
- [x] Add user-scoped in-app notification APIs, email delivery status, optional STARTTLS SMTP, and focused unit tests.

## BE-11 — Audit trail

- [x] Record actor, action, entity, previous/new values, and timestamp for critical user, department, project, contract, task, milestone, and budget changes.
- [x] Add a permission-protected, filterable, paginated audit-log API.
- [x] Enforce append-only audit records in PostgreSQL and prevent deletion of actors with audit history.
- [x] Add audit filter tests and verify migration, audited API writes/reads, and database immutability.

## BE-12 — Testing, OpenAPI, and deployment

- [x] Add API/integration tests for implemented endpoints and document planned gaps in the shared API contract.
- [x] Publish OpenAPI 3.x documentation for currently registered endpoints.
- [x] Add production-ready Docker build and complete local Compose stack.
- [x] Add CI checks for formatting, linting, tests, and builds.
- [x] Verify health/readiness, graceful shutdown, and deployment configuration.

## MVP acceptance

- [ ] All required APIs conform to the shared API contract.
- [x] Authorization is enforced on protected resources.
- [x] Contract expiration, timeline, and budget monitoring pass business-rule tests.
- [x] Notifications are idempotent and audit events are generated.
- [x] Migrations, automated tests, OpenAPI documentation, and Docker deployment work from a clean setup.
