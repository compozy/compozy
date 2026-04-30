Goal (incl. success criteria):

- Implement the accepted plan to fix daemon web task detail failures caused by stale registered workspace roots and align synced task IDs with task document basenames.
- Success means stale workspace roots return `workspace_context_stale`, the web shell clears stale active workspaces, task sync emits `task_01` style IDs, browser regression passes, and `make verify` passes.

Constraints/Assumptions:

- Follow AGENTS/CLAUDE instructions, including no destructive git commands.
- Preserve unrelated local edits, currently observed in `web/src/systems/app-shell/components/app-shell-layout.tsx`, `workspace-onboarding.tsx`, and `workspace-picker.tsx`.
- Accepted plan is persisted at `.codex/plans/2026-04-24-fix-daemon-task-detail-stale-workspace.md`.

Key decisions:

- Validate active workspace root liveness at the browser active-workspace boundary.
- Do not delete or mutate stale workspace registry rows during request handling.
- Canonical task IDs should come from the task filename basename, e.g. `task_01.md` -> `task_01`.

State:

- Implementation and verification complete.

Done:

- Reproduced and diagnosed stale `daemon-improvs` workspace path via in-app browser and daemon APIs.
- Read required skills: `browser-use:browser`, `systematic-debugging`, `no-workarounds`, `golang-pro`, `testing-anti-patterns`, React/TypeScript/TanStack guidance, and `cy-final-verify`.
- Scanned relevant existing ledgers for daemon web and canonical daemon contract context.
- Persisted accepted plan under `.codex/plans/`.
- Added active workspace root validation and stale workspace problem mapping.
- Changed task sync to emit task IDs from task document basenames.
- Added app-shell query/mutation cache stale-workspace recovery.
- Added focused backend/frontend regression tests.
- Focused Go tests passed for `./internal/api/httpapi`, `./internal/core`, `./internal/daemon`, `./internal/store/globaldb`, and `./internal/api/core`.
- Focused frontend tests passed for app-shell, reviews stale recovery, spec/memory stale recovery, and workflow task routes/adapters.
- `env -u NO_COLOR -u FORCE_COLOR make verify` passed cleanly: frontend lint/typecheck/test/build, Go fmt/lint/test/build, and Playwright e2e all succeeded.
- Browser regression passed against isolated daemon on port `61531`: selecting deleted `stale-smoke` returned to workspace picker with stale warning; selecting `looper-smoke` and opening `/workflows/daemon-improvs/tasks/task_01` rendered task detail.
- Isolated smoke daemon was stopped.

Now:

- Prepare final response.

Next:

- User should restart the existing daemon to run the updated binary in their active browser session.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/plans/2026-04-24-fix-daemon-task-detail-stale-workspace.md`
- `.codex/ledger/2026-04-24-MEMORY-daemon-task-detail.md`
- `internal/api/httpapi/browser_middleware.go`
- `internal/api/httpapi/browser_middleware_test.go`
- `internal/core/sync.go`
- `internal/core/sync_test.go`
- `web/src/lib/query-client.ts`
- `web/src/systems/app-shell/components/app-shell-container.tsx`
- `web/src/systems/app-shell/components/app-shell-container.test.tsx`
