# TanStack data loading — 2026-10-28

Verdict: **PASS for the targeted loading slice**. Global delivery gates are CI-owned
at the user's request. This report does not certify a full release, provider execution,
the desktop soak, or latency percentiles.

Branch: `perf/tanstack-data-loading`. Baseline: `7ccc00670d1e278515049d6ff7ba8b3c00b7ee25`.
The clean baseline production bundle was retained before implementation. All changes
and checks ran in a separate worktree; the original checkout and user daemon were untouched.
Cross-surface ownership is recorded in `docs/_memory/change-impact.md`.

## Changes

- A shared 100 ms intent scheduler warms dock app code and canonical data, session
  rows and Kanban cards. Pointer leave, blur, unmount and changed bindings cancel
  pending work. Touch starts immediately. Speculative errors do not block activation.
- Session intent checks the hydrated profile/workspace, reads the scoped detail,
  rechecks authority, then warms only a cold transcript head. Existing history is not
  traversed. Unused speculative transcript entries receive a short GC policy.
- A caught-up live stream prevents stale mount/focus from rereading retained transcript
  pages. Explicit invalidation and disconnected fallback remain active. Replay catch-up
  does not claim live authority before its watermark.
- Agent detail/settings loaders match their actual mounted consumers and explicit
  window workspace binding. Loop run loaders warm briefing and the first roster and
  notable-timeline pages only after the scoped run is confirmed.
- Concurrent stream authorization shares listener discovery while preserving independent
  cancellation, retry after failure, and a unique remote ticket for each connection.

## Measurement

The operator reads seeded Northstar Pay records through real HTTP and production Web
assets in an isolated daemon. Twelve persisted sessions, two projects and 23 tasks are
seeded test data, not live provider execution. Before and after use fresh Chromium
headless-shell profiles against the same data. Intent is sustained for approximately
600 ms. The browser measures request start relative to click, not rendering completion.

The committed [request comparison](../evidence/2026-10-28-tanstack-data-loading/request-comparison.json)
contains one observation per version. Negative values mean the request began before
activation. These observations establish movement off the click path, not a p95 speedup.

| Real browser observation | Before | After |
| --- | --- | --- |
| Startup `GET /api/status` | 6 | 2 (health plus shared listener discovery) |
| Tasks list request starts relative to click | +84.31 ms | -527.34 ms |
| Tasks window code starts relative to click | +62.17 ms | -532.42 ms |
| Selected session transcript starts relative to click | +88.76 ms | -502.50 ms |
| Duplicate transcript/list read at the measured opening | One cold read after click | No repeated warmed read |
| Mutations before activation | 0 | 0 |
| Page errors in the paired walks | 0 | 0 |

The support session displayed the same expected answer before/after, independently
confirmed through the public transcript API. Its owner/detail/control reads remain
separate authority boundaries; the result is not “zero requests to open a session.”

Focused Query/React comparisons additionally show:

| Owning behavior | Before | After |
| --- | --- | --- |
| Stale two-page transcript, caught-up live stream, focus | 2 history reads | 0 |
| Same transcript explicitly invalidated or disconnected | 2 reads | 2 (preserved) |
| Agent detail default session shelves | 5 session reads | 2 |
| Agent Settings definition in User or explicitly bound project/User | 2 reads | 1 |
| Loop run initial projections at mount after preload | 3 cold reads | 0 duplicates |

Agent detail also performed exactly two session reads in the real browser. The 5→2
comparison applies to the default filter; a retained per-window worktree filter is
outside the route preload's inputs.

## Targeted journeys

| Journey | Outcome | Evidence in the isolated lab |
| --- | --- | --- |
| Cold dock Tasks intent/open | Data/code start before activation; 23 tasks render | `baseline-browser.json`, `after-browser.json`, screenshots |
| Session pointer selection | Matching support transcript; warmed head reused | `baseline-session.json`, `after-session.json`, `api-verification.json` |
| Brief and keyboard intent | Brief hover: zero reads; focus: detail/head; Enter reuses head | `after-extra.json` |
| Kanban selection | Selected card alone warms its head; correct session opens | `after-extra.json`, `after-board-session.png` |
| Stale stream focus and reload | No transcript read on focus; reload retains selected session | `after-extra.json` |
| Agent detail | Correct project definition, metrics and two shelves render | `after-extra.json`, `after-agent-detail.png` |
| Settings footer intent | Code and General data warm with GETs only | `after-extra.json` |
| Workspace selection/reload | Launch HQ excludes Platform session rows | `after-workspace.json`, `after-workspace.png` |

Lab: `/Users/pedronauck/dev/qa-labs/compozy-tanstack-data-loading-20261028-212132-054690-lab/qa-artifacts/qa`.
The initial shared-browser/manual Chromium driver stalled; the completed comparisons
used Playwright's Chromium headless shell. One supplementary script initially matched
hidden retained windows as well as the active window; restricting interaction to visible
controls completed the walk. Neither driver issue is treated as product failure evidence.

## Verification

All frontend commands ran through root Turborepo with `--filter=./web --only` and
one or two test workers. The explicit `--only` scope avoids global dependency gates.
The fresh worktree required the client SDK build before Web typechecking.

- Session live-tail/catalog-stream/query-option suites: **81 passed**.
- Intent scheduler, SessionList and SessionsBoard suites: **127 passed**.
- Dock and app registry suites: **121 passed**.
- Route preload and agent-session deep-link suites: **95 passed**.
- Stream auth, ticketed EventSource/WebSocket and gateway access signal: **39 passed**.
- Total: **463 focused tests**, across 14 canonical files; repeated confirmation runs
  are not counted again. Test additions state observable I/O/cache/scope invariants.
- `bunx turbo run build --filter=@compozy/client --only`: passed.
- `bunx turbo run typecheck --filter=./web --only`: passed after fixing importer and fixture types.
- Clean baseline and final `bunx turbo run build --filter=./web --only`: passed.
- React Doctor changed-file scan: **100/100, zero diagnostics**, 27 Web source/test files.
- Scoped format/lint and `git diff --check`: passed. Staged validation uses `--no-stash`
  because the user's Git safety policy forbids automatic stash operations.
- Independent read-only review: no actionable defect in session authority, auth
  cancellation, dock fences, dynamic imports, or Agent/Loop consumer parity.
- Strict targeted QA audit: passed. Manifest-owned teardown: `TEARDOWN_CLEAN=true`; no surviving lab processes.
- Current-head CI: recorded at delivery below.

The baseline already emitted CSS `::highlight`, large-chunk and ineffective terminal
import build warnings. Existing folder-menu tests emitted React `act` warnings;
new intent cases did not. These were not suppressed or represented as new clean gates.
No local global gate or full E2E suite was run.

## Current documentation and boundaries

Context7 was queried for `/tanstack/query` and `/tanstack/router`. The installed
Query 5.104.1 supports `queryClient.query` / `infiniteQuery`. Changed preloads reuse
canonical option factories and propagate AbortSignals; populated-cache preservation
uses `staleTime: "static"`, with mounted observers retaining their own freshness policy.
Router's existing intent preload and `defaultPreloadStaleTime: 0` remain appropriate.

- [Query migration guidance](https://github.com/tanstack/query/blob/main/docs/framework/react/guides/migrating-to-v5.md)
- [Query cancellation](https://github.com/tanstack/query/blob/main/docs/framework/react/guides/query-cancellation.md)
- [Router preloading](https://github.com/tanstack/router/blob/main/docs/router/guide/preloading.md)
- [Router data loading](https://github.com/tanstack/router/blob/main/docs/router/guide/data-loading.md)

No dependency upgrade, SSR migration, public contract, persisted-state migration or
permission change is included. The command palette, marketplace completeness policy,
provider usage freshness and per-window worktree filtering retain their existing owners.
Global CI and real provider/soak scenarios remain distinct from this targeted loading proof.

## Delivery

Local implementation and targeted QA are complete. Global CI evidence and its exact
head are maintained on the pull request for this branch. Local results do not replace CI.
Lab audit and `teardown.json` are retained alongside the request traces.
