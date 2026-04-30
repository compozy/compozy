Goal (incl. success criteria):

- Implement stale workspace handling plus manual artifact mirror sync from the accepted plan.
- Success means: startup classifies/cleans workspaces without a periodic routine, manual workspace sync exists in API/UI, missing workspaces with catalog data are selectable read-only, DB snapshots are preferred for markdown reads, and `make verify` passes.

Constraints/Assumptions:

- Follow AGENTS/CLAUDE instructions: no destructive git commands, use required skills, run `make verify` before completion.
- Accepted plan persisted at `.codex/plans/2026-04-24-workspace-artifact-mirror.md`.
- No ticker, polling loop, global watcher, or periodic routine for workspace maintenance in this change.
- Workspaces missing on disk with catalog data stay read-only; missing workspaces without catalog data are removed on startup and manual sync.

Key decisions:

- Add explicit workspace filesystem/read-only state and stats to the daemon contract.
- Add a manual `POST /api/workspaces/sync` button-driven path instead of background periodic cleanup.
- Keep markdown files authoritative when present, but prefer `global.db` snapshots for UI reads.

State:

- Implementation complete with fresh `make verify` passing.

Done:

- Read current instructions, relevant skill docs, prior sync/workspace ledgers, and confirmed the worktree is clean.
- Persisted the accepted plan and created this ledger.
- Added migration v4 for workspace filesystem/sync state and deduplicated `artifact_bodies`.
- Extended workspace registry rows with derived read-only/catalog stats and state update/delete helpers.
- Replaced oversized artifact overflow markers with checksum-deduplicated body persistence.
- Added one-shot startup workspace refresh plus manual `POST /api/workspaces/sync` service and contract fields.
- Changed query reads to prefer artifact snapshots/bodies and keep deleted workspace files readable from `global.db`.
- Added `workspace_path_missing` for filesystem-required daemon mutations.
- Added frontend manual workspace sync action, missing/read-only workspace badges, stale-context handling, and read-only mutation guards.
- `bun run --cwd web typecheck` passed.
- Focused frontend Vitest run passed: 10 files, 55 tests.
- Focused backend Go package run passed: `internal/api/core`, `internal/api/httpapi`, `internal/daemon`, `internal/store/globaldb`.
- Full verification passed: `make verify` completed frontend lint/typecheck/test/build, Go fmt/lint/test/build, and Playwright e2e.

Now:

- Report final verification evidence.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/plans/2026-04-24-workspace-artifact-mirror.md`
- `.codex/ledger/2026-04-24-MEMORY-workspace-artifact-mirror.md`
- `internal/store/globaldb/*`
- `internal/core/sync.go`
- `internal/daemon/*`
- `internal/api/core/*`
- `internal/api/httpapi/*`
- `web/src/systems/app-shell/*`
- `web/src/systems/{workflows,reviews,memory,spec}/*`
- `openapi/compozy-daemon.json`
- `web/src/generated/compozy-openapi.d.ts`
