# All projects tasks backend evidence

Slice A2 implements AP BR-12–15 and safety invariants 4–5. The owning cross-surface impact audit is
`.compozy/tasks/all-projects/_impact-entry.md`, consolidated by the delivery coordinator.

## Behavior

- Shared `WorkspaceBreadth` parsing normalizes repeated project filters and rejects conflicting,
  empty, and reserved selectors (T-01–T-05).
- Task catalog and task dashboard keep omission populations and envelopes. Explicit all-workspaces
  reads can narrow to project IDs and `@user`; SQL applies that selection before catalog counts and
  paging and before dashboard aggregation (T-11–T-13).
- Catalog cursor identity includes explicit breadth, normalized project selection, existing filters,
  and profile lens. Legacy query fingerprints omit the new zero-valued fields.
- Overview omission remains global tasks; explicit all selects every task. Overview does not expose
  a project multi-filter. Its attention acknowledgement population uses the same scope resolver.
- Operator guards reject ordinary agent aggregate requests. App middleware preserves the task route
  access decisions and denies overview (T-16–T-17). U5 owns the granted-agent replacement.
- `task list --all-workspaces` conflicts with `--workspace` and bounded `--scope`. Human output joins
  project names and labels User rows; JSON and JSONL retain the original envelope and workspace IDs.
- Native task tools remain pinned by their existing authority rules. No new native tools, hooks,
  configuration, external services, or runtime state directories are introduced.

## Query plan

The actual catalog SQL, including its derived canonical status/activity CTEs, was explained for both
profile lenses. Before the change, both plans contained `SCAN tasks`. Generated migration
`00144_schema.sql` adds `idx_tasks_profile ON tasks(profile_id)` without changing rows.

After migration:

```text
one profile:  SEARCH tasks USING INDEX idx_tasks_profile (profile_id=?)
all profiles: SCAN tasks
both lenses: USE TEMP B-TREE FOR ORDER BY
```

All-profiles reads genuinely select all tasks. The final order derives activity from task, run and
event timestamps; a task timestamp index cannot satisfy that order. No redundant sort index was added.
Existing indexed task-history seeks remain intact.

## Validation

Targeted suites use real SQLite for catalog populations, pagination, dashboard aggregation, migration
application, and schema equivalence. Handler tests verify envelopes and errors. The actual app-token
middleware is exercised against the production route access registry. CLI tests cover flag resolution
and the PROJECT/User table output. Heavy gates and E2E are reserved for CI by the slice brief.

The test-shape checker accepts the new cases; its findings in `global_db_task_test.go` and
`observe/tasks_test.go` are pre-existing direct-assertion test functions outside this change.

Completed checks:

- Focused Go suites in `internal/api/core`, `internal/api/httpapi`, `internal/api/spec`,
  `internal/cli`, and `internal/observe`: passed.
- Real SQLite catalog breadth, query plans, production migration streams, schema equivalence,
  migration application and atlas checksum rejection: passed.
- `go build ./...` and `go vet` on the touched backend packages: passed.
- `make codegen` and `make codegen-check`: passed; OpenAPI, Go/TypeScript clients, contracts and CLI docs regenerated.
- Repository pre-commit checks via `lint-staged --no-stash --no-hide-partially-staged`: passed.
  The regular hook is disabled for this commit because its default stash conflicts with the
  explicit no-stash instruction; the same configured checks were run directly.

## Integration seams

- U5 replaces aggregate `requireOperatorSurface` calls with the granted-agent helper.
- U3 owns migration-created `global` name aliases. The route-agnostic parser currently rejects the
  literal `global`; alias resolution must precede that reserved-name rejection when integrating U3.
- Frontend consumers, the Web journey and consolidated operator skill/release documentation belong
  to their assigned slices. Task/overview query contracts and generated clients are co-shipped here.
