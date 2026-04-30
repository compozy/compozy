Goal (incl. success criteria):

- Execute AGH tools-refac Task 04: add read-only memory, observe, and bridge built-in tools using existing query surfaces; preserve redaction/visibility; add required tests; update workflow tracking; run clean `make verify`; create one local commit if gates pass.

Constraints/Assumptions:

- Must use cy-workflow-memory, cy-execute-task, and cy-final-verify.
- Must read shared/current workflow memory, AGH guidance, PRD docs, `_techspec.md`, `_tasks.md`, ADR-001, and ADR-002 before editing code. Done.
- Go production edits require AGH code guidelines and golang-pro; Go test edits require AGH test conventions and testing-anti-patterns. Done.
- Do not run destructive git commands without explicit user permission.
- Existing worktree has many unrelated modified files; do not revert or touch unrelated changes.
- Commit may be unsafe if task hunks cannot be isolated from pre-existing unrelated edits in touched files.

Key decisions:

- Implement tool-first descriptors under `internal/tools/builtin` and daemon-native handlers that reuse memory store, observe query/health, and bridge service/health APIs.
- New tool families are read-only: `agh__memory_*`, `agh__observe_*`, `agh__bridges_*`, grouped into new built-in toolsets.
- Preserve current API semantics for memory content reads; rely on existing DTO conversions/redaction for memory history, observe summaries/health, and bridge DTOs.
- Unavailable dependencies remain visible to operator projections with deterministic unavailable reasons and hidden from session projections.

State:

- Implementation, focused tests, full verification, self-review, workflow memory, and task tracking updates are complete.
- Commit is pending isolated staging review because touched tracked files contain pre-existing unrelated hunks from other work.

Done:

- Loaded required skills and docs.
- Updated stale task workflow memory to Task 04 objective before implementation.
- Added memory/observe/bridge descriptor files, IDs, toolset registrations, daemon dependency availability, native handlers, and focused tests.
- Focused tests passed: `go test ./internal/tools ./internal/tools/builtin ./internal/daemon -count=1`.
- Coverage check passed for touched packages: `internal/tools` 80.8%, `internal/tools/builtin` 94.2%, `internal/daemon` 72.1%.
- Full `make verify` passed on 2026-04-30 00:11:30 -03: frontend checks passed, `golangci-lint` 0 issues, Go tests `DONE 7049 tests`, package boundary check passed.
- Updated `.compozy/tasks/tools-refac/memory/{MEMORY.md,task_04.md}`, task_04 status/checklists, and `_tasks.md` row 04.

Now:

- Determine whether a clean local commit can be created by staging only this task's hunks.

Next:

- If hunk-isolated staging is practical, commit `feat: add memory observe bridge read tools`; otherwise report that commit is blocked to avoid staging unrelated user work.

Open questions (UNCONFIRMED if needed):

- Whether final local commit can be made safely without staging pre-existing unrelated hunks in the same files.

Working set (files/ids/commands):

- `internal/tools/builtin_ids.go`
- `internal/tools/builtin/descriptors.go`
- `internal/tools/builtin/toolsets.go`
- `internal/tools/builtin/{memory,observe,bridges}.go`
- `internal/daemon/native_tools.go`
- `internal/daemon/native_read_tools.go`
- `internal/daemon/native_tools_test.go`
- `internal/tools/builtin/builtin_test.go`
- `.compozy/tasks/tools-refac/{task_04.md,_tasks.md,memory/MEMORY.md,memory/task_04.md}`
