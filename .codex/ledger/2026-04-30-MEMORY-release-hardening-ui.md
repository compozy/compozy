Goal (incl. success criteria):

- Implement release-blocking workflow/run/review/sync/UI hardening from the accepted plan.
- Success means completed workflows cannot start no-work runs, review fetch reports/syncs correctly, reviews index/detail matches the new hierarchy, text overflow is fixed, generated contracts are current, and `make verify` passes.

Constraints/Assumptions:

- No destructive git commands without explicit permission.
- Persist accepted Plan mode plan under `.codex/plans/`.
- Use root-cause fixes only; no workarounds or assertion weakening.
- Must use Go, React/TanStack, design, and test skills for touched domains.
- `make verify` is the completion gate.

Key decisions:

- Completed workflows show a non-action completed/no-pending state; direct starts return a typed conflict.
- `/api/sync` remains the workflow reconciliation endpoint for dashboard/workflow sync.
- `/reviews` shows compact latest-round cards; `/reviews/$slug/$round` shows issues.
- Review title sanitization is display-only; source markdown remains unchanged.

State:

- Implementation complete; final verification passed. Unrelated untracked
  `.codex/ledger/2026-04-30-MEMORY-reviews-watch.md` and
  `.compozy/tasks/reviews-watch/` are present and intentionally untouched.

Done:

- Read relevant prior ledgers and required skills.
- Persisted accepted plan to `.codex/plans/2026-04-30-release-hardening-workflows-reviews-sync.md`.
- Added workflow task counts/actionability to the daemon contract and list/dashboard transport.
- Added a run-manager preflight guard returning `workflow_no_pending_tasks` before run creation.
- Changed review fetch post-write reconciliation to use daemon `globalDB`/`SyncWithDB` and reload the synced workflow.
- Added `/api/reviews/{slug}/rounds/{round}` to OpenAPI and regenerated frontend OpenAPI/route artifacts.
- Split `/reviews` compact round cards from new `/reviews/$slug/$round` issue inventory route.
- Made dashboard workflow rows navigable and hardened truncation for task/run/review rows.
- Preserved adapter-wrapped daemon error messages so sync errors show the daemon problem text.
- Regenerated OpenAPI and TanStack route artifacts after moving review issue detail to the
  sibling route path.
- `bun run --cwd web typecheck` passed.
- Focused frontend regression suite passed: 11 files, 81 tests.
- Focused Go regression suite passed for API/OpenAPI/sync/review/run/workflow transport paths.
- `make verify` passed end to end after fixes:
  frontend lint/typecheck/test/build, Go fmt/lint/test/build, and Playwright E2E.

Now:

- Prepare final handoff.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- Plan: `.codex/plans/2026-04-30-release-hardening-workflows-reviews-sync.md`
- Ledger: `.codex/ledger/2026-04-30-MEMORY-release-hardening-ui.md`
- Expected areas: `internal/api`, `internal/daemon`, `openapi`, `web/src`, `packages/ui/src/components`.
