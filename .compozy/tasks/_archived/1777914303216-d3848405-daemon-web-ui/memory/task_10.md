# Task Memory: task_10.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Ship the operator-facing run console: `/runs` list + `/runs/$runId` live detail, typed daemon REST + SSE, 80%+ test coverage with reconnect/overflow/cancel flows.

## Important Decisions
- Flat-route naming: `web/src/routes/_app/runs_.$runId.tsx` was chosen instead of `runs.$runId.tsx` so the detail route does NOT inherit the `/runs` list layout. TanStack Router otherwise treats the dotted sibling as a child of the list route with no `<Outlet />`, shadowing the detail view.
- SSE client abstraction lives in `web/src/systems/runs/lib/stream.ts`: a factory interface (`RunStreamFactory`) plus `setRunStreamFactoryOverrideForTests` so both the hook tests and the full route integration test can inject a deterministic fake without touching `EventSource` (jsdom lacks it).
- Stream hook (`useRunStream`) keeps cursor state in a ref so the same hook handles initial open, overflow resume, manual reconnect, and scheduled auto-reconnect after transport errors. Initial cursor hydrates from the snapshot response’s `next_cursor` once `enabled` flips true.
- Detail route disables the stream for terminal runs (`completed|succeeded|failed|canceled|cancelled`) — no live stream is opened and the cancel button is disabled.

## Learnings
- Integration tests that assert POSTs must match the method via `Request.method` (openapi-fetch uses a `Request` object, not `init.method`). The shared `matchPath` helper already does that; the local `matchUrl` in the runs integration test file must mirror that behavior.
- jsdom does not implement `EventSource` — any default factory path must be overridable for tests. We emit `handler({type: "error"})` in the default factory’s fallback so production still degrades gracefully.
- TanStack Router uses `runs_` (underscore suffix on the parent segment) as the "opt-out-of-parent-layout" convention for flat files; the generated route tree then parents the detail route directly on `/_app` instead of `/_app/runs`.
- `openapi-fetch` sets query params via `params.query`; passing `query: undefined` works, so adapters can conditionally build a query object.

## Files / Surfaces
- `web/src/systems/runs/**` (new): types, query keys, stream factory, REST adapter, hooks (`useRuns`, `useRun`, `useRunSnapshot`, `useCancelRun`, `useStartWorkflowRun`, `useRunStream`), view components (`RunsListView`, `RunDetailView`).
- `web/src/routes/_app/runs.tsx` (new): list route.
- `web/src/routes/_app/runs_.$runId.tsx` (new): detail route (flat sibling, not a child of `/runs`).
- `web/src/routes/-runs.integration.test.tsx` (new): end-to-end route integration covering inventory, filter re-query, detail snapshot + stream, cancel, overflow refresh, and manual reconnect.
- `web/src/systems/app-shell/components/app-shell-layout.tsx` (edit): adds the "Runs" nav entry under "Across workflows".
- `web/src/routeTree.gen.ts` (regenerated): registers `/_app/runs` and `/_app/runs_/$runId`.

## Errors / Corrections
- First integration pass parented the detail route under the list route, causing `/api/runs?workspace=…` list requests instead of `/api/runs/$id/snapshot` when visiting `/runs/$id`. Fixed by renaming to `runs_.$runId.tsx` and regenerating the tree.
- Initial cursor wasn’t flowing from the snapshot into the first stream open because `cursorRef` was only initialised at mount. Added a hydration branch so `cursorRef.current` picks up `initialCursor` on first `enabled` render.

## Ready for Next Run
- Task is implementation-complete: 95/95 vitest passing, lint clean, typecheck clean, codegen-check clean, `web` build succeeds, `go build ./...` succeeds.
- Downstream slices (task_11 task board/detail, task_12 reviews/spec/memory, task_13 Storybook) can reuse the `@/systems/runs` barrel (adapter + hooks + view components + stream factory override) and the `runs_` route split pattern when they expose run-linked flows.
