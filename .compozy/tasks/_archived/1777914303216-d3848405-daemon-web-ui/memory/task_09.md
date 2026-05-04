# Task Memory: task_09.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Build daemon web UI shell (`__root.tsx`, `_app.tsx`), `/` dashboard route, and `/workflows` inventory route.
- Implement tab-scoped single-workspace bootstrap: zero (empty picker + `POST /api/workspaces/resolve`), one (auto-select), many (explicit selection), stale (recovery). Persist selection in `sessionStorage`; send `X-Compozy-Workspace-ID` on workspace-scoped requests.
- Render dashboard and workflow inventory through the typed `openapi-fetch` daemon client and `@compozy/ui` primitives.
- Expose shell-level operational actions that live in the dashboard/inventory surface: workflow sync (`POST /api/sync`) and archive (`POST /api/tasks/:slug/archive`).

## Important Decisions
- task_02 was marked completed but its frontend deliverables never landed in this worktree: no codegen script, no `web/src/generated/compozy-openapi.d.ts`, no `web/src/lib/api-client.ts`, no `@tanstack/*` or `openapi-fetch` deps. Backfilling those inside task_09 is the only way to satisfy the task_09 requirement of rendering via the typed daemon client. The scope drift is recorded as a shared-memory correction rather than silent expansion.
- Frontend deps added to `web/package.json` stay minimal: TanStack Router + Router plugin/devtools, TanStack Query + devtools, openapi-fetch, openapi-typescript (codegen), zustand, zod, sonner, lucide-react, plus testing-library + user-event for tests. Defer MSW/Playwright/Storybook to task_13/14.
- Codegen is a small bun script that shells `openapi-typescript` and writes `web/src/generated/compozy-openapi.d.ts`. `codegen-check` re-runs and fails if the output diverges. Root `package.json` owns the scripts so later tasks can also run `bun run codegen[-check]`.
- Double-submit CSRF: helper reads the `compozy_csrf` cookie and sets `X-Compozy-CSRF-Token` on mutating requests via an openapi-fetch middleware. The GET-issued cookie bootstraps the token; if absent, the middleware lets the server error propagate rather than inventing a token.
- Workspace persistence: zustand store seeded from `sessionStorage["compozy.web.active-workspace"]`, writes back on change, clears on stale (`workspace_context_stale`). Tab-scoped by design (not localStorage).
- Workspace selection policy: zero -> onboarding view with resolve-by-path form, one -> auto-select, many -> explicit selection UI (no default), stale -> clear selection + fall back to selection surface (banner explains).
- Tests use `globalThis.fetch` stubs instead of MSW because MSW wiring belongs to task_13. jsdom + @testing-library/react cover component tests; view-model hooks use `renderHook`.
- Following AGH convention, shell composition lives inside `web/src/systems/app-shell` rather than duplicating `@compozy/ui` primitives. `_app.tsx` is a thin route that delegates to the system.

## Learnings
- OpenAPI path `/api/ui/dashboard` returns `DashboardResponse { dashboard: DashboardPayload }` — always unwrap `dashboard` when consuming.
- Mutating browser endpoints require both `X-Compozy-Workspace-ID` (via `activeWorkspaceMiddleware`) and `X-Compozy-CSRF-Token` (via `csrfMiddleware`). The double-submit pattern uses cookie `compozy_csrf` + header of the same value.
- Stale-workspace responses come back as `412` with `TransportError.code === "workspace_context_stale"`. Browser hooks must match on `code` to trigger recovery.
- `openapi-fetch` does not parse non-2xx bodies into `data`; it returns `{ error, response }`. Error mappers should inspect `error` first, then fall back to `response.statusText`.
- `@tanstack/react-router` plugin generates `routeTree.gen.ts` lazily on dev; for tests and CI we must either precommit a placeholder or run `tsr generate`. Adding `routeTree.gen.ts` to `.gitignore` and regenerating at build/test time is the AGH pattern — keep it.

## Files / Surfaces
- Root tooling: `package.json`, `bun.lock`, `scripts/codegen.mjs` (new), `web/package.json`, `web/vite.config.ts`, `web/tsconfig.json`, `web/vitest.config.ts`.
- Web lib/helpers: `web/src/lib/api-client.ts`, `web/src/lib/query-client.ts`, `web/src/lib/csrf.ts`, `web/src/lib/router.tsx`, `web/src/lib/session-storage.ts`, `web/src/lib/utils.ts`.
- Generated: `web/src/generated/compozy-openapi.d.ts`.
- Routes: `web/src/routes/__root.tsx`, `web/src/routes/_app.tsx`, `web/src/routes/_app/index.tsx`, `web/src/routes/_app/workflows.tsx`, `web/src/routeTree.gen.ts` (generated).
- Systems: `web/src/systems/app-shell/**`, `web/src/systems/dashboard/**`, `web/src/systems/workflows/**`.
- Entry: `web/src/main.tsx` (rewired), `web/src/app.test.ts` (removed — supplanted by route tests).

## Errors / Corrections
- Shared workflow memory claimed "Root and web package codegen / codegen-check entrypoints already cover both the legacy AGH contract and the new compozy-daemon contract". That is not true for this worktree. Corrected in shared memory.

## Ready for Next Run
- Shell, workspace bootstrap, and typed client are stable landing points that subsequent task_10/task_11/task_12 slices can plug into. Expect them to reuse `useActiveWorkspace`, the `daemonApiClient`, and `apiErrorMessage` helpers instead of reinventing.

## Verification
- `make verify` — PASS (2524 Go tests, 1 skipped, build succeeded).
- `bun run --cwd web typecheck` — PASS (codegen-check clean, tsc clean).
- `bun run --cwd web test` — PASS (13 files, 55 tests, coverage 83.29% stmts / 85.71% fns / 83.25% lines / 70.93% branches).
- Manual cleanup of `~/.compozy/runs/{hooks-integration,run-job-hooks,review-hooks,artifact-hooks}` was required once to clear a `rundb: schema too new` error carried over from an earlier binary. Captured in the shared workflow memory learning.
