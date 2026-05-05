Goal (incl. success criteria):

- Implement AGH network-threads Task 11: extension Host API, SDK exports, capability gates, and bridge mapping; complete only after clean verification, tracking updates, memory updates, and local commit.

Constraints/Assumptions:

- Do not run destructive git commands (`git restore`, `git checkout`, `git reset`, `git clean`, `git rm`) without explicit permission.
- Must read workflow memory, PRD docs, ADRs, AGENTS/CLAUDE, and task_08 before editing.
- Must use required skills: cy-workflow-memory, cy-execute-task, cy-final-verify, agh-code-guidelines, golang-pro, agh-test-conventions, testing-anti-patterns.
- `make verify` is the completion and commit gate.

Key decisions:

- Provider/platform bridge `ThreadID` remains bridge routing metadata only; AGH conversation refs are carried by an explicit `conversation` mapping with `surface` plus `thread_id` or `direct_id`.
- Host API network methods reuse public network contract DTOs where practical, with Host API-specific params only for filtering/targeting.
- Direct resolve goes through the deterministic store resolver and local/remote peer lookup instead of fabricating direct-room membership.

State:

- Task complete in local commit `ac76924f`; tracking and memory files remain uncommitted by design.

Done:

- Located AGH-specific skills in `/Users/pedronauck/Dev/compozy/agh2/.agents/skills`.
- Loaded required skill instructions and AGH references.
- Read workflow memory, repo guidance, internal guidance, PRD docs, ADRs, and task_08/task_11 context.
- Added network Host API protocol/contract methods, capability gates, handlers, daemon dependency injection, bridge explicit conversation mapping, TypeScript SDK helpers/exports, Go SDK method constants, generated TS contracts, and focused Go/TS tests.
- Passing focused Go evidence before SDK finalization: `go test ./internal/extension ./internal/bridges ./internal/extension/contract ./internal/extension/protocol ./internal/daemon -count=1`.
- Passing focused evidence after SDK/test expansion: `go test ./sdk/go ./internal/extension ./internal/bridges ./internal/extension/contract ./internal/extension/protocol ./internal/daemon -count=1`, `bun run --cwd sdk/typescript typecheck`, and `bun run --cwd sdk/typescript test`.
- Coverage snapshot: `internal/bridges` 80.8%; `internal/extension` 76.1% package-wide with Task 11 network handlers/mapping covered by focused tests.
- First full `make verify` failed only on lint (`funlen`, `appendCombine`, `lll`); refactored network registration helpers and formatting.
- Final full `make verify` passed: frontend format/lint/typecheck/tests/build, Go lint `0 issues`, race tests `DONE 8361 tests in 116.211s`, and package boundaries `OK`.
- Final pre-commit `make verify` after tracking/memory updates also passed: frontend format/lint/typecheck/tests/build, Go lint `0 issues`, race tests `DONE 8361 tests in 10.774s`, and package boundaries `OK`.
- Updated task_11 status/checklists and inserted Task 11 completed row in `_tasks.md`.
- Created local commit `ac76924f feat: expose network host api` with only task-scoped code/generated files staged.
- Final post-commit `make verify` after commit-hash memory update passed: frontend format/lint/typecheck/tests/build, Go lint `0 issues`, race tests `DONE 8361 tests in 13.211s`, and package boundaries `OK`.

Now:

- Ready to report completion.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- Task repo: `/Users/pedronauck/Dev/compozy/agh2`
- Local ledger: `.codex/ledger/2026-05-05-MEMORY-extension-host-api.md`
- Workflow memory files under `/Users/pedronauck/Dev/compozy/agh2/.compozy/tasks/network-threads/memory/`
- Code surfaces: `internal/extension`, `internal/bridges`, `internal/daemon`, `sdk/go`, `sdk/typescript`.
