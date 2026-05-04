# Task Memory: task_13.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Add the full Storybook/MSW review harness for daemon-web-ui route and component surfaces required by task 13.
- Cover `web/` routes plus `packages/ui/` reusable components.
- Route-state scope: dashboard, workflows, runs, reviews, spec, and memory, each with representative loading, empty, success, degraded, and error states where the surface supports them.

## Important Decisions
- Mirror the AGH Storybook pattern instead of inventing new tooling structure: `web/.storybook`, `packages/ui/.storybook`, grouped web MSW handlers, and route helpers under `web/src/storybook/`.
- Keep mocked route stories close to the domains they exercise using `web/src/routes/_app/stories/` plus per-domain mock barrels under `web/src/systems/<domain>/mocks/`.
- Reuse typed payload shapes from the current component tests/OpenAPI-derived system types so stories and MSW responses stay aligned with the implemented UI.
- Include a workspace/app-shell handler group because all route stories depend on bootstrap through `/api/workspaces`.
- Validate the review harness with three layers instead of one: portable-story render tests, Storybook/MSW contract tests, and actual `storybook build` runs for both workspaces.

## Learnings
- Task 13 starts from an empty Storybook baseline in this worktree: no `.storybook/` folders, no `web/src/storybook/`, and no story files or MSW helpers currently exist.
- The current `make verify` target is Go-only, so task completion still requires explicit frontend verification commands in addition to the repo-wide gate.
- Existing component tests already define stable payload shapes and `data-testid` contracts for dashboard, workflow inventory, run list/detail, task board/detail, reviews, spec, and memory views; those are the safest fixture sources for Storybook.
- `packages/ui` cannot rely on `web` for Storybook/Vitest dependencies: `tailwindcss`, `@testing-library/react`, and `@testing-library/jest-dom` must be declared in the package workspace for preview CSS resolution and portable-story tests.
- Web-wide frontend coverage crossed the task target by exercising more route stories directly; the stable path was expanding portable-story coverage instead of narrowing the coverage include set.

## Files / Surfaces
- `package.json`
- `web/package.json`
- `packages/ui/package.json`
- `web/.storybook/`
- `packages/ui/.storybook/`
- `web/src/storybook/`
- `web/src/routes/_app/stories/`
- `web/src/systems/{app-shell,dashboard,workflows,runs,reviews,spec,memory}/`
- `packages/ui/src/components/stories/`

## Errors / Corrections
- `packages/ui` Storybook initially failed because `.storybook/preview.css` imported Tailwind without a direct `tailwindcss` dependency in that workspace; adding the package-local dev dependency fixed the build.
- `packages/ui` portable-story tests initially failed because the workspace did not declare `@testing-library/react`; adding workspace-local testing deps plus `tests/setup.ts` fixed the suite.
- The repo `make verify` gate initially failed on stale `~/.compozy/runs/{hooks-integration,run-job-hooks,review-hooks,artifact-hooks}` databases with `schema too new`; clearing those documented test-artifact directories restored the expected baseline and the rerun passed.

## Ready for Next Run
- Task 13 is complete.
- Implemented surfaces now include `web/.storybook`, `web/src/storybook/{msw,route-story}.tsx`, route stories under `web/src/routes/_app/stories/`, domain mock barrels under `web/src/systems/*/mocks/`, `packages/ui/.storybook`, and shared UI stories/tests under `packages/ui/src/components/stories/` and `packages/ui/tests/`.
- Verification evidence:
  - `bun run --cwd web test`
  - `bun run --cwd packages/ui test`
  - `bun run --cwd web build-storybook`
  - `bun run --cwd packages/ui build-storybook`
  - `bun run codegen-check && bunx vitest run --config vitest.config.ts --coverage` in `web/` (`Statements 80.24%`, `Lines 80.39%`)
  - `bun run test:coverage` in `packages/ui` (`100%` coverage)
  - `bun run --cwd web typecheck`
  - `bun run --cwd packages/ui typecheck`
  - `make verify`
