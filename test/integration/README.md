# Integration Test Database Modes

## Overview

Integration tests now default to the in-memory SQLite driver via the shared helper `helpers.SetupTestDatabase`. This removes the dependency on PostgreSQL testcontainers for suites that are database-agnostic and dramatically simplifies local execution, while keeping PostgreSQL-only journeys explicitly scoped.

## Migrated Suites (SQLite)

- `test/integration/store/operations_test.go`
- `test/integration/repo/*.go`
- `test/integration/worker/**` (via updated `DatabaseHelper` and `helpers.SetupTestRepos`)
- `test/integration/server/executions_integration_test.go`
- `test/integration/tool/helpers.go` (ancillary helper now provisions SQLite)

## PostgreSQL Exceptions

These suites continue to exercise PostgreSQL because they rely on dialect-specific features:

- `test/integration/store/migrations_test.go` — validates PostgreSQL schema migrations, index metadata, and `information_schema` state.
- `test/integration/standalone` helpers (memory mode env) — run embedded services end-to-end with full infrastructure wiring.
- `engine/infra/server/dependencies_integration_test.go` — covers server dependency bootstrapping with PostgreSQL.

## Key Helper Changes

- `helpers.SetupTestRepos` now returns repositories backed by SQLite by default; pass `"postgres"` when PostgreSQL behaviour is required.
- `test/integration/worker/helpers/DatabaseHelper` exposes the shared repository provider instead of raw pgx pools.
- `test/helpers/server/server.go` provisions SQLite providers and surfaces them via `ServerHarness.RepoProvider`.

## Performance Snapshot

- Before migration (`time make test`): **~71.6s**
- After migration (`time make test`): **~88.9s** _(current run still provisions PostgreSQL for the Temporal and transaction-concurrency suites; see notes in the task summary)_

Use the logged timings to monitor future regressions. When new suites are added, prefer `helpers.SetupTestDatabase` unless PostgreSQL-specific features are under test.
