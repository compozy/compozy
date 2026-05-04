## Goal (incl. success criteria):

- Implement the accepted plan for grouping path-missing workspaces at the bottom of the web initial workspace picker.
- Success means path-missing workspaces are separated at the bottom, remain selectable, focused tests pass, and `make verify` passes before completion.

## Constraints/Assumptions:

- Follow workspace policy: no destructive git commands; do not touch unrelated dirty files.
- Persist accepted Plan Mode plan under `.codex/plans/`.
- `Path missing` means `workspace.filesystem_state === "missing"`.
- Missing workspaces remain selectable/read-only.
- Preserve ordering within each group.

## Key decisions:

- UI-only change in `WorkspacePicker`; no API contract or app-shell selection behavior changes.
- Mirror `WorkflowInventoryView` section label/count pattern.

## State:

- Completed after full verification.

## Done:

- Accepted plan persisted at `.codex/plans/2026-05-04-workspace-picker-path-missing.md`.
- Read relevant React, TypeScript, Tailwind, Vitest, and testing skills.
- Noted unrelated dirty worktree state, especially many `.compozy/tasks/**` deletes.
- Updated `WorkspacePicker` to render available and path-missing sections while preserving row selection behavior.
- Updated focused picker test coverage for grouping, ordering, badges, and selection.
- Focused web test passed after rerun with correct path: `bun run --cwd web test -- src/systems/app-shell/components/workspace-picker.test.tsx`.
- Web typecheck passed: `bun run --cwd web typecheck`.
- First `make verify` passed frontend lint/typecheck/tests/build, Go fmt/lint/tests/build, then failed in Playwright setup because `.compozy/tasks/daemon` is missing in the current dirty worktree.
- Investigated root cause: `PLAYWRIGHT_SOURCE_WORKFLOW_SLUGS` includes `daemon`, but `web/e2e/global.setup.ts` only had a synthetic fallback for `daemon-web-ui`.
- Added a synthetic `daemon` e2e source workflow with a completed task and pending `reviews-001/issue_001.md` so Playwright setup is robust when the source workflow is archived/missing.
- Focused e2e rerun passed: `bun run frontend:e2e` reported 5 passed.
- Final `make verify` passed: frontend lint/typecheck/tests/build, Go fmt/lint/tests/build, and Playwright e2e all completed with `All verification checks passed`.

## Now:

- Final handoff.

## Next:

- None.

## Open questions (UNCONFIRMED if needed):

- None.

## Working set (files/ids/commands):

- `.codex/plans/2026-05-04-workspace-picker-path-missing.md`
- `.codex/ledger/2026-05-04-MEMORY-workspace-picker.md`
- `web/src/systems/app-shell/components/workspace-picker.tsx`
- `web/src/systems/app-shell/components/workspace-picker.test.tsx`
- `web/e2e/global.setup.ts`
