Goal (incl. success criteria):

- Implement force archive with confirmation across daemon API and web UI.
- Success: archive endpoint supports force confirmation for locally resolvable conflicts, force path auto-completes tasks and resolves review issues locally before resync+archive, web shows confirmation dialog and retries with force, relevant tests pass, and `make verify` passes.

Constraints/Assumptions:

- Do not use destructive git commands or revert unrelated user changes.
- Existing worktree contains many unrelated task archive/deletion changes; ignore them.
- Force scope is local only: complete pending/non-terminal tasks and resolve review issues locally; do not call remote review providers.
- Active runs remain a hard archive blocker even with force.
- Accepted plan persisted at `.codex/plans/2026-05-04-force-archive.md`.
- Required skills reviewed this turn: golang-pro, systematic-debugging, no-workarounds, testing-anti-patterns, react, typescript-advanced, vitest, cy-final-verify.

Key decisions:

- First archive attempt remains normal; backend returns a structured `workflow_force_required` conflict when the workflow is forceable.
- Archive confirmation will use a shared `AlertDialog` in `packages/ui` built on `@base-ui/react/alert-dialog`.
- The backend remains the source of truth for whether force is allowed and for the confirmation counts shown in the UI.
- CLI archive surface stays unchanged in this task.

State:

- Completed after full verification.

Done:

- Explored current archive flow across core, daemon transport, API contracts, OpenAPI, client, and workflow inventory UI.
- Confirmed current archive endpoint only accepts workspace and returns `archived` boolean.
- Confirmed review/task state is file-backed and sync projects it into globaldb.
- Confirmed reusable local helpers already exist for single-task completion and review finalization, but force needs workflow-wide helpers.
- Persisted accepted plan and created this session ledger.
- Implemented backend force-archive flow across contract, core archive logic, sync path normalization, daemon transport mapping, and client request body updates.
- Added workflow-wide helpers to complete non-terminal tasks and resolve unresolved review issues locally before sync.
- Added focused backend tests covering force-required conflicts, forced task/review rewrites, review-only workflows, and the `/var` vs `/private/var` sync path normalization bug on macOS.
- Focused tests currently passing:
  - `go test ./internal/core`
  - `go test ./internal/daemon`
  - `go test ./internal/api/core ./internal/api/httpapi ./internal/api/client`
- Added shared `AlertDialog` in `packages/ui`, wired archive confirmation flow in `web/`, and updated archive success/error handling for forced archive retries.
- Updated OpenAPI schema and regenerated `web/src/generated/compozy-openapi.d.ts`.
- Fixed follow-on verification regressions:
  - refactored `internal/core/archive.go` to reduce cyclomatic complexity without changing behavior
  - corrected tests to align with terminal task alias semantics and the new archive-confirmation CLI message
  - hardened Playwright fixture bootstrap so missing `.compozy/tasks/daemon-web-ui` in a dirty workspace falls back to a synthetic smoke fixture
- Full verification passed:
  - `make verify`

Now:

- Final handoff.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `.codex/plans/2026-05-04-force-archive.md`
- `.codex/ledger/2026-05-04-MEMORY-force-archive.md`
- `internal/core/{archive.go,archive_test.go,api.go,sync.go}`
- `internal/core/model/workflow_ops.go`
- `internal/core/tasks/store.go`
- `internal/core/reviews/store.go`
- `internal/daemon/{task_transport_service.go,transport_service_test.go}`
- `internal/api/{contract/types.go,contract/errors.go,core/{handlers.go,errors.go,interfaces.go},httpapi/openapi_contract_test.go}`
- `internal/api/client/operator.go`
- `openapi/compozy-daemon.json`
- `packages/ui/src/{components/alert-dialog.tsx,index.ts}`
- `packages/ui/tests/alert-dialog.test.tsx`
- `web/src/routes/_app/workflows.tsx`
- `web/src/systems/workflows/{adapters/workflows-api.ts,components/workflow-inventory-view.tsx,components/workflow-inventory-view.test.tsx,hooks/use-workflows.ts,mocks/*,types.ts}`
- `web/e2e/global.setup.ts`
