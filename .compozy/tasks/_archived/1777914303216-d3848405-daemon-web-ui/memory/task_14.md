# Task Memory: task_14.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Make the daemon web UI part of the repo's official verification contract by wiring Bun/bootstrap, frontend lint/typecheck/test/build, CI path coverage, and daemon-served Playwright into the existing `make build` / `make verify` flow.

## Important Decisions
- Extend the existing root `package.json`, `Makefile`, and `.github/workflows/ci.yml` surfaces instead of adding parallel scripts or sidecar CI jobs.
- Run Playwright against the built `bin/compozy` daemon and embedded `web/dist` assets only; no Vite dev-server fallback is part of the E2E contract.
- Seed one synthetic completed workflow (`archive-ready`) inside the Playwright fixture so the archive smoke path has deterministic success coverage without depending on mutable project task state.

## Learnings
- Review issue routes were still treating browser workspace IDs as filesystem paths in `RunManager.resolveWorkflowContext`, which broke daemon-served review issue listing until the resolver switched to the shared `resolveWorkspaceReference(...)` path.
- Workflow inventory could not show archived rows after a successful archive because the daemon task transport excluded archived workflows from `ListWorkflows`; `IncludeArchived: true` is required for the browser inventory contract.
- `packages/ui/tsconfig.build.json` needed explicit `include` / `exclude` overrides because the inherited test globs broke `tsc -p tsconfig.build.json` once the repo gate started running the shared UI package build.

## Files / Surfaces
- `.bun-version`
- `package.json`
- `Makefile`
- `.github/actions/setup-bun/action.yml`
- `.github/workflows/ci.yml`
- `packages/ui/tsconfig.build.json`
- `test/frontend-verification-contract.test.ts`
- `test/frontend-workspace-config.test.ts`
- `web/package.json`
- `web/playwright.config.ts`
- `web/e2e/support/daemon-fixture.ts`
- `web/e2e/global.setup.ts`
- `web/e2e/global.teardown.ts`
- `web/e2e/daemon-ui.smoke.spec.ts`
- `internal/daemon/run_manager.go`
- `internal/daemon/run_manager_test.go`
- `internal/daemon/review_exec_transport_service_test.go`
- `internal/daemon/task_transport_service.go`
- `internal/daemon/transport_service_test.go`

## Errors / Corrections
- Initial Playwright review smoke failed because review issue listing resolved `ws-...` as a relative filesystem path under the fixture workspace. Fixed by routing review/run workflow resolution through the shared workspace-reference resolver and rebuilding the daemon binary before rerunning E2E.
- Initial archive smoke failed twice for valid reasons:
  - archiving `daemon-web-ui` correctly failed because the workflow is still pending in the fixture data
  - archiving a completed synthetic workflow succeeded, but the UI never showed an archived section because the transport excluded archived rows
- An attempted `bun run --cwd web test -- --runInBand` probe used an unsupported Vitest flag; no source change was required.

## Ready for Next Run
- Fresh verification evidence after all code changes:
  - `bun run frontend:e2e`
  - `make build`
  - `make verify`
- `make verify` passed with the full frontend lane, Go lint/test/build, and daemon-served Playwright smoke coverage.
