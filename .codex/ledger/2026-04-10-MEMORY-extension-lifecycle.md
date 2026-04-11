Goal (incl. success criteria):

- Implement extensibility task 08 by adding the extension manager runtime lifecycle: subprocess spawn, initialize handshake, Host API routing, health checks, best-effort event forwarding, cooperative shutdown escalation, lifecycle events, and real spawn-based tests.
- Success requires task-specific deliverables, coverage >=80% for `internal/core/extension`, clean targeted tests, clean `make verify`, workflow memory/tracking updates, and one local commit.

Constraints/Assumptions:

- Follow repository `AGENTS.md` / `CLAUDE.md`, task 08, `_techspec.md`, `_protocol.md`, ADR-002, ADR-003, and ADR-006.
- Required skills in use: `cy-workflow-memory`, `cy-execute-task`, `golang-pro`, `testing-anti-patterns`, `cy-final-verify`; `no-workarounds` and `systematic-debugging` are active guardrails for runtime/test failures; `brainstorming` is satisfied by the existing approved PRD/TechSpec design and does not require a fresh user design loop.
- Keep scope tight to manager lifecycle and required event kinds/tests. Do not change unrelated dirty worktree files.
- Existing dirty workspace state includes modified task tracking files from earlier tasks plus untracked memory/meta files; do not revert or restage unrelated changes.

Key decisions:

- UNCONFIRMED: likely implement a per-extension session/client around `subprocess.Transport` to support concurrent host->extension requests plus extension->host callbacks on the same stdio channel.

State:

- Completed and verified after implementing the manager lifecycle, mock-extension test harness, tracking updates, and a clean `make verify`.

Done:

- Read repository instructions, required skill files, shared workflow memory, current task memory, and extension-related prior ledgers.
- Confirmed the workspace is dirty before edits; unrelated changes remained untouched.
- Implemented `Manager.Start`, initialize validation, Host API request routing, health probes, best-effort event forwarding, lifecycle events, and cooperative shutdown escalation.
- Added the mock extension binary under `internal/core/extension/testdata/mock_extension` and comprehensive unit/integration coverage in `internal/core/extension/manager_test.go`.
- Updated event payloads/docs, task memory, shared memory, and task 08 tracking.
- Verified `internal/core/extension` coverage at 80.4% and passed `make verify`.

Now:

- Review the final diff and create one local commit with code changes only.

Next:

- Remove this ledger once the local commit is complete, or leave it if another follow-up on task 08 is requested in the same workspace.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-10-MEMORY-extension-lifecycle.md`
- `.compozy/tasks/extensibility/task_08.md`
- `.compozy/tasks/extensibility/_tasks.md`
- `.compozy/tasks/extensibility/_techspec.md`
- `.compozy/tasks/extensibility/_protocol.md`
- `.compozy/tasks/extensibility/adrs/adr-002.md`
- `.compozy/tasks/extensibility/adrs/adr-003.md`
- `.compozy/tasks/extensibility/adrs/adr-006.md`
- `.compozy/tasks/extensibility/memory/MEMORY.md`
- `.compozy/tasks/extensibility/memory/task_08.md`
- `internal/core/extension/runtime.go`
- `internal/core/extension/dispatcher.go`
- `internal/core/extension/host_api.go`
- `internal/core/extension/host_helpers.go`
- `internal/core/subprocess/process.go`
- `internal/core/subprocess/handshake.go`
- `internal/core/subprocess/transport.go`
- `pkg/compozy/events/event.go`
- `pkg/compozy/events/kinds/extension.go`
- Commands: `rg`, `sed`, `git status --short`
