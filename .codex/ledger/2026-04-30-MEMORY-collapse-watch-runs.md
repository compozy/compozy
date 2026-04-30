Goal (incl. success criteria):

- Fix dashboard double-count/display for `cy reviews watch` by persisting run parent-child metadata and collapsing active watch children in the dashboard.
- Success means one `review_watch` parent plus its active child `review` run appears/counts as one active dashboard run, while full run history keeps the child with `parent_run_id`.
- Full `make verify` must pass.

Constraints/Assumptions:

- Accepted plan persisted at `.codex/plans/2026-04-30-collapse-review-watch-child-runs.md`.
- Must not run destructive git commands: `git restore`, `git checkout`, `git reset`, `git clean`, `git rm`.
- Must use root-cause fix; no dashboard-only workaround without persisted parent metadata.
- Use `systematic-debugging`, `no-workarounds`, `golang-pro`, `testing-anti-patterns`, and `cy-final-verify`.
- Existing dirty worktree includes prior review-watch fix files; do not revert unrelated/user changes.

Key decisions:

- Add `parent_run_id` to global run storage and API `Run`.
- Set `parent_run_id` on review-watch child `review` runs.
- Collapse child runs only in dashboard active-run view/counts when parent is active and present; preserve full run history.

State:

- Implementation complete. Full `make verify` passed.

Done:

- Read previous review-watch ledger and relevant cross-agent ledgers.
- Read required skills.
- Confirmed root cause: daemon intentionally starts a child `review` run, but global run index/API lacks parent linkage.
- Persisted accepted plan and task ledger.
- Added `parent_run_id` to global run persistence, API contracts, OpenAPI schema, generated web types, and public run summaries.
- Persisted review-watch child runs and extension-started child runs with parent metadata.
- Collapsed dashboard-visible child runs when their parent run is already present.
- Added regression coverage for DB roundtrip, review-watch child linkage, extension child linkage, and dashboard active-run collapse.
- Passed focused checks: `go test ./internal/store/globaldb ./internal/daemon ./pkg/compozy/runs`, `go test ./internal/api/...`, and `node scripts/codegen.mjs --check`.
- First `make verify` reached Go lint and failed on `gocyclo` in `resumeExistingExecRun`; extracted parent assignment to preserve behavior and reduce function complexity.
- Passed follow-up checks: `go test ./internal/daemon` and `golangci-lint run --allow-parallel-runners ./internal/daemon`.
- Passed full `make verify`: frontend lint/typecheck/tests/build, Go fmt/lint/tests/build, and Playwright daemon UI smoke tests.

Now:

- Prepare final summary and note existing unrelated dirty files.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/plans/2026-04-30-collapse-review-watch-child-runs.md`
- `.codex/ledger/2026-04-30-MEMORY-collapse-watch-runs.md`
- `internal/store/globaldb/*`
- `internal/api/contract/types.go`
- `internal/daemon/run_manager.go`
- `internal/daemon/review_watch.go`
- `internal/daemon/query_service.go`
- `web/src/generated/compozy-openapi.d.ts`
