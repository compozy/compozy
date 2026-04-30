Goal (incl. success criteria):

- Implement accepted plan for sync pruning of deleted workflow folders and review-only archive eligibility.
- Success: stale active workflow DB rows are pruned on root sync, review-only resolved workflows archive successfully, web inventory uses archive eligibility for Completed grouping, focused tests and `make verify` pass.

Constraints/Assumptions:

- Do not run destructive git commands (`restore`, `checkout`, `reset`, `clean`, `rm`) without explicit user permission.
- Existing dirty files are user/concurrent work; preserve and layer changes carefully.
- Required skills in use: systematic-debugging, no-workarounds, golang-pro, testing-anti-patterns, React/TypeScript/TanStack/Vitest, cy-final-verify.
- Accepted Plan Mode plan persisted at `.codex/plans/2026-04-30-sync-prune-review-only-archive.md`.

Key decisions:

- Root sync prunes stale active workflow catalog rows after present workflows sync successfully.
- Review-only workflows are archivable only when they have at least one review issue and zero unresolved review issues.
- Workflow summary exposes archive eligibility separately from task-run startability.

State:

- Implementation complete; focused tests and full `make verify` passed.

Done:

- Read prior ledgers and relevant code paths for sync, archive, globaldb, daemon transport, and web inventory.
- Confirmed `../agh` has stale `delete-session` DB rows and `badges-design` review-only shape.
- Persisted accepted plan and created this session ledger.
- Added sync pruning result fields through core/API/daemon/web generated types.
- Added globaldb stale active workflow pruning with active-run skip warnings.
- Changed archive eligibility so review-only resolved workflows are archivable and unresolved reviews block.
- Populated workflow summary archive eligibility and changed web inventory Completed grouping to use it.
- Added regression tests for globaldb pruning, root vs single sync pruning, review-only archive, daemon eligibility, frontend grouping, and sync summary messaging.
- Updated CLI sync text output to include stale workflow prune count.
- Focused verification passed:
  - `go test ./internal/store/globaldb ./internal/core ./internal/daemon`
  - `bun run --cwd web vitest run --config vitest.config.ts src/systems/workflows/components/workflow-inventory-view.test.tsx src/systems/workflows/lib/sync-summary.test.ts`
- Full verification passed with `make verify`: frontend lint/typecheck/test/build, Go fmt/lint/race tests/build, and frontend e2e.

Now:

- Final handoff.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/plans/2026-04-30-sync-prune-review-only-archive.md`
- `.codex/ledger/2026-04-30-MEMORY-sync-prune-archive.md`
- `internal/core/{sync.go,sync_test.go,archive_test.go,model/workflow_ops.go}`
- `internal/store/globaldb/{sync.go,sync_test.go,archive.go,archive_test.go}`
- `internal/daemon/{transport_mappers.go,task_transport_service.go,query_service.go,workspace_refresh.go,transport_service_test.go}`
- `internal/api/contract/types.go`, `internal/cli/commands_simple.go`
- `openapi/compozy-daemon.json`, `web/src/systems/workflows/**/*`, `web/src/routes/_app/{index.tsx,workflows.tsx}`, `web/src/generated/compozy-openapi.d.ts`
