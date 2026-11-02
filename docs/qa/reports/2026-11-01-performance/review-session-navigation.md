# Concurrent session navigation investigation

Status: the first baseline diagnostic completed, but the second, more detailed diagnostic reproduced a lost content selection on candidate `14730a705`. The intended click and authoritative window navigation succeeded while the old conversation remained mounted for over five seconds. Consuming the authoritative runtime identity directly repaired the reproduced failure: the unchanged real-browser walk passed all 64 selections, including 48 warm revisits, with all eight controlled streams complete. Source review compared the relevant navigation/row files against baseline `de5ab2f4d`; those files had no candidate diff before this correction.

## Original observation

The root's second baseline run (`browser-baseline-v5`) failed after clicking Performance 27 (`sess-62a617aea611593b`): the matching row never acquired `aria-current="true"` within the existing five-second assertion. The screenshot still shows Performance 26 in the window header. Performance 19 has the hover treatment (checkbox and actions visible); that appearance alone does not establish its selected identity. The artifact records neither the actual pointer event target nor the command/route sequence.

Inputs: 32 active sessions, eight controlled streams, 640 retained sessions, four passes over 16 rows. The first baseline run completed, and the root reports three candidate runs completed all 64 selections. These observations did not establish a candidate regression or a stable browser latency improvement. The first diagnostic below also completed; the later instrumented failure supplied the evidence for the correction.

Original evidence: `.cache/performance/browser-baseline-v5/failure.json`, `failure.png`, `daemon-failure.log`, and `teardown.json`. Both recorded teardown outcomes were fulfilled. The daemon log does not contain a per-click navigation trace.

## Static boundary review

| Boundary | Evidence | Consequence |
| --- | --- | --- |
| Row identity | `SessionListThreadList` keys roots by `thread.session.id`; row test IDs and callbacks close over that same session ID | No index-key reuse was found. |
| Sort stability | `DEFAULT_SESSION_LIST_PREFERENCES` is `created` / `none`; `sessionListSortParam` preserves created sort | Streaming timestamps do not reorder this default list. A custom last-activity preference would be a different case. |
| Background refresh | `useSessionCatalog().paging` includes placeholder, next-page and previous-page fetches, not normal refetch | The row's paging guard does not discard clicks merely because SSE invalidated a warm catalog. |
| Selection gesture | A plain row click opens unless bulk-selection mode/modifier keys request a toggle/range | The screenshot's hover styling does not prove bulk selection. |
| Window command | `userRetarget` supplies the selected session ID; the runtime queues commands and reads a fresh revision when each runs | No speculative parallelization or queue bypass is warranted. |
| Late command completion | `nextUserRetargetToken` fences older history pushes; route intents clear only when their identity matches; snapshots replace only on increasing revision | Existing defenses cover stale command completion. The failed artifact does not prove an echo escaped them. |
| Deferred identity | `useSessionWindowRuntime` passes the pathname through `useDeferredValue`; `SessionWindowView` is keyed by session ID under a view transition | The content can intentionally trail the window route while the next render/data resolves. This remains a candidate boundary for the measured post-command delay, not a proved failure cause. |
| Transition hit testing | `packages/ui/src/tokens-runtime.css` sets `::view-transition { pointer-events: none; }` | A generic snapshot-overlay interception theory is not supported by the inspected stylesheet. |

Inspected owners include `session-list-row-view.tsx`, `session-list-thread-list.tsx`, `session-list.tsx`, `session-list-group-body.tsx`, `use-session-catalog.ts`, `use-session-window-open-session.ts`, `routing-coordinator.ts`, `routing-route-decision.ts`, `window-manager-runtime-core.ts`, `window-manager-tab-commands.ts`, `window-manager-store.ts`, `window-manager-query.ts`, `os-route-sync.tsx`, `use-desktop-chrome.ts`, and the session-window runtime/resolution/view chain.

## Passive real-browser run

The root authorized one repeat using a temporary copy of its existing harness:

```sh
python3 .cache/performance/run-browser.py env GOFLAGS='-p=2 -buildvcs=false' bun .cache/performance/benchmark-navigation-debug.ts .cache/performance/baseline .cache/performance/compozy-baseline .cache/performance/browser-baseline-navigation-debug 32
```

`run-browser.py` takes the existing machine-wide E2E lock and then invokes the task's bounded runner. The run exited 0. It retained the same `row.click()`, `aria-current` assertion, timeouts, 64 selections and stream-completion checks. No retry, forced click, delay added to settle the UI, or weaker assertion was introduced. The temporary copy adds passive capture plus a pre-click state read; its timings include diagnostic overhead and are not before/after performance evidence.

The capture is limited to fixture session IDs, event targets/coordinates, modifiers, scroll offsets, row ordering/current/pressed flags, window titles, pathname, and whitelisted command fields/status/revision. It does not collect headers, credentials, prompt bodies or arbitrary response payloads. DOM events use the browser's monotonic clock; command events use Bun's monotonic clock. Their origins differ, so absolute cross-domain timestamps must not be subtracted as if synchronized.

Results from `.cache/performance/browser-baseline-navigation-debug/navigation-debug.json`:

- All **64 selections passed**; browser error list was empty; browser/runtime teardown both fulfilled.
- All **64 captured trusted clicks** targeted the expected session row; no modifier was pressed, and no row-order change occurred across the 64 pre-click snapshots.
- There were **65 command request/response pairs**: one initial open plus 64 navigations. All returned HTTP 200 and `applied=true`. Every navigation target matched the attempted session in order; no extra route-back command was recorded.
- Navigation command request-to-response intervals: median **78.28 ms**, p95 **195.25 ms**, maximum **581.70 ms**. These include browser/transport scheduling and response parsing; they are not server-only timings.
- Slowest selection: ordinal 27, Performance 26, **2868.60 ms** overall. Its expected row received pointerdown at approximately **202.2 ms** and click at **204.2 ms** after the pre-click snapshot. Its navigation command request-to-response interval was **81.86 ms**.
- The next slow selection (ordinal 43, also Performance 26) took **1626.56 ms**; click arrived at **46.0 ms**, and command request-to-response was **116.15 ms**.
- `command request time − browser click time` contains an unknown constant clock offset. Across the 64 matched pairs its range is only **34.44 ms**; the slowest selection is **2.26 ms** above the minimum. This supports prompt command dispatch in this run, rather than a multi-second command backlog. It does not synchronize the clocks or measure absolute dispatch latency.

The slow samples therefore direct the next investigation toward the period after input/command: detail/transcript reads, deferred content commit, and browser work. They do not prove which of those accounts for the delay. The existing long-task collector recorded 90 tasks (maximum 192 ms), but it stores duration without timestamps and cannot attribute a task to the slow click.

Other receipts: `.cache/performance/browser-baseline-navigation-debug/measurements.json`, `sessions.png`, `teardown.json`, and `.cache/performance/browser-baseline-navigation-debug.log`. The bounded event ring is capped at 512 entries; the 64 click entries survived in this run, but it is not an unlimited DOM history.

## Candidate diagnostic: actual content delay

The root authorized an expanded passive capture in the same temporary harness, adding a bounded DOM observer, normalized API timing, and separate click/assertion/frame stopwatches. The original click, assertions, timeouts and fixture remained unchanged. No diagnostic capture was added to production or the durable benchmark.

```sh
python3 .cache/performance/run-browser.py env GOFLAGS='-p=2 -buildvcs=false' bun .cache/performance/benchmark-navigation-debug.ts . .cache/performance/compozy-candidate-final .cache/performance/browser-candidate-navigation-debug 32
```

The exact `14730a705` candidate completed 16 selections and failed selection 17: the first warm revisit, from streaming Performance 31 to idle Performance 16 (`sess-7acc524a7a81d3ad`). The two teardown outcomes were fulfilled and the browser error list was empty. The failure is preserved in `browser-candidate-navigation-debug/navigation-debug.json`, `navigation-summary.json`, `failure.json`, `failure.png`, `daemon-failure.log` and `teardown.json`.

The failure sequence is conclusive at the input/route/content boundary:

- The trusted pointerdown, pointerup and click all targeted the expected row, with no modifiers. The list order remained unchanged; scroll moved the target into view.
- The browser click occurred at 13817.6 ms. The window's `data-instance-key` changed from the old session to the target at 13823.9 ms: **6.3 ms after the click**.
- The matching `window.navigate` returned HTTP 200, `applied=true`, revision 18 and the correct target route. Its request-to-response interval was **141.85 ms**; no later rollback command was recorded.
- The expected session's by-ID GET returned HTTP 200 in **42.29 ms** and completed its response body in **88.78 ms**. Those intervals are measured within Bun's clock. The capture contains no HTTP error status.
- At failure the URL was the intended target, but `aria-current`, the header and the original chat DOM node still belonged to Performance 31. No new chat node mounted and no target current-row mutation occurred during the **5253.0 ms** between the click and final DOM snapshot. Eighteen non-target workspace session-detail requests occurred during this interval, alongside catalog/queue refreshes.

This is real stale content, not a wrong click, DOM reordering, a rejected command, or assertion polling merely noticing an already-completed selection late. It localizes the defect to the deferred content identity/commit boundary. The evidence does **not** distinguish React background-render starvation from a pending view-transition commit, so neither internal scheduler mechanism is claimed as proved.

The 16 completed cold selections also separate observed DOM readiness from harness timing:

| Interval | Median | Range |
| --- | ---: | ---: |
| Trusted click to target `aria-current` / new chat DOM commit, browser clock | 273.10 ms | 252.50–379.70 ms |
| `locator.click()` call | 166.89 ms | 96.37–485.71 ms |
| Existing aria assertion call | 280.73 ms | 193.67–384.17 ms |
| Existing chat assertion call | 49.31 ms | 4.16–87.70 ms |
| Existing two-animation-frame wait | 114.78 ms | 34.46–168.02 ms |
| Whole measured selection | 623.61 ms | 523.60–950.33 ms |

Each pre-click browser read is bracketed by Bun timestamps. That bounds the clock-origin offset rather than assuming identical origins. Across these samples, the time between actual DOM readiness and aria assertion return lies within per-sample bounds ranging from 16.8 to 136.5 ms. The failure has no DOM-ready milestone at all. Instrumentation and polling overhead mean neither this table nor the earlier diagnostic is a production latency benchmark. API normalization intentionally drops identifiers; it also collapses static session routes into the generic family, so only the explicit `expectedSession` flag is used to attribute the selected session's reads.

## One-factor correction and owning checks

The root authorized consuming the authoritative pathname and drawer flag directly in `use-session-window-runtime.ts`. The deferred identity existed to make a session swap eligible for a crossfade. The experiment removes that deferral only: scope resolution, keyed chat providers, the `ViewTransition` boundary, command ordering and navigation reconciliation remain in place. The next session owns the loading state immediately; the old conversation cannot continue as the view's target under the new route. The accepted tradeoff is loss of the deferred crossfade at this boundary.

Invariant: after an accepted retarget, every view/read/presence identity belongs to the target session, including a target whose detail is still loading. Owner and canonical suite: OS `session-window.test.tsx`. The former test explicitly required a deferred old-session render; it now requires the stronger identity rule for both cached and loading targets. Before the production edit, both cases failed because the view saw `sess-1` and then `sess-2` (the other 27 tests passed). After the one-factor edit, all 45 tests in the existing session-window, session-window-view and session-window-sidebar suites passed. The view suite also retains the loading skeleton and User-scope runtime invariants.

Focused receipts: `.cache/performance/session-navigation-negative-control.log`, `session-navigation-after-unit.log`, and `session-navigation-format.log`. Focused Oxlint on all four changed source/test files passed with zero warnings (`session-navigation-lint.log`). The root Turbo Web build passed in 30.00 s (`session-navigation-web-build.log`), with the existing CSS `::highlight` and large-chunk warnings retained in its receipt. The experiment uses the unchanged `14730a705` daemon binary and rebuilt Web assets; the sole production behavior change is the runtime identity source. Two adjacent comments were corrected to stop claiming the identity remains deferred.

## Real-browser result after correction

```sh
python3 .cache/performance/run-browser.py env GOFLAGS='-p=2 -buildvcs=false' bun .cache/performance/benchmark-navigation-debug.ts . .cache/performance/compozy-candidate-final .cache/performance/browser-candidate-navigation-direct 32
```

The same passive harness passed **64/64 selections**, all eight controlled streams completed their 600 updates, and browser/runtime teardown both fulfilled. The browser error list and captured HTTP error list were empty. All 64 captured clicks targeted the intended rows; all 65 window commands returned HTTP 200 and `applied=true`. The final URL, current row and window instance identified the expected last session.

Every selection mounted a new chat DOM node. The observer recorded **no old chat node retained under a new window instance**. Cold targets showed their loading state and then their own chat. On all 48 warm revisits, window instance and selected-row/new-chat identity changed in the same observer batch (0.1–0.3 ms between the recorded attribute and readiness timestamps). The original failing boundary now passed: Performance 31 to Performance 16 committed the new chat **178.40 ms after the trusted click**, instead of retaining the previous chat beyond 5253 ms.

| Browser-clock click to selected row and new chat DOM | Before direct identity | After direct identity |
| --- | ---: | ---: |
| First 16, cold, median | 273.10 ms | 194.55 ms |
| First 16, cold, range | 252.50–379.70 ms | 148.10–308.00 ms |
| 48 warm revisits, median | First revisit failed; no complete distribution | 166.15 ms |
| 48 warm revisits, range | First revisit still old after 5253.0 ms | 130.70–191.50 ms |
| All 64, median / p95 | Incomplete | 167.05 / 213.30 ms |

The separately measured Playwright sequence after the correction was median **371.38 ms**, p95 **541.72 ms**, maximum **707.00 ms**. Its medians by stage were click call 249.06 ms, aria assertion 13.55 ms, chat assertion 9.58 ms and two frames 57.19 ms; medians of individual stages are not additive. This is one instrumented before/after diagnostic pair. It supports retaining the identity correction and establishes the distinction between DOM readiness and harness wall time; it is not a stable population latency improvement or live-provider INP claim.

Evidence: `.cache/performance/browser-candidate-navigation-direct/{navigation-debug.json,navigation-summary.json,measurements.json,sessions.png,teardown.json}` and the adjacent `.log`. The event ring retained all 64 clicks (453 entries); the separate DOM milestone ring held 211 entries. The failed-before artifacts remain untouched.

Durable, sanitized [diagnostic evidence](evidence/session-navigation-diagnostic.json) contains the fixture, negative-control counts, and all 17 before / 64 after selection records: trusted-target checks, command outcomes, first selected-session response timing, chat replacement, stage timing and bounded clock alignment. It contains no filesystem/URL paths, session identifiers, request/response bodies, headers or credentials. Fixture display names identify the cases.

Cross-surface impact: Web session navigation and the session drawer now consume the current window identity without a deferred old-session render. The existing profile/workspace authorization reads, retained transcript cache, loading skeleton, prompt provider identity, native tools, extension/hooks/config contracts, official Compozy skill and persisted user state are unchanged. No wire or schema migration is needed. The targeted concurrent-navigation addition to `RT-visible-session-streaming` was re-walked here; this does not claim the entire historical two-window/real-provider scenario was repeated. Root owns final combined build, current-head CI and durable benchmark tables.

## Earlier recommendation and investigation boundaries

The first investigation preserved the original failed run rather than classifying it as a tool flake or claiming the initial performance patch fixed it. The later run localized stale content and justified the narrow identity correction. Neither the assertion nor routing reconciliation was weakened, retried or redesigned.

The discriminating capture specified after the first run, and performed above, was to align clock domains and record:

1. Every change to window `data-instance-key`, row `aria-current`, and the window title, using a bounded observer with old/new identities and timestamps.
2. Request-to-response status/duration for normalized API route families, especially scoped session detail, transcript, and window-manager snapshots; no headers or arbitrary bodies.
3. The exact boundary when `row.click()` returns and when each unchanged assertion succeeds, to separate input auto-wait, content readiness and the two animation frames.

If the intended row receives the event but no matching command is sent, the owning seam is row selection/coordinator admission. If a correct command is followed by a command for the old route, the owner is URL/WM reconciliation. If the window instance is correct while content/current-row identity remains old, the owner is session-window deferred rendering/resolution. If the pointer target differs, inspect the observed scroll/layout transition before changing either product or benchmark behavior.

Canonical existing suites are `web/src/systems/os/lib/__tests__/routing-coordinator.test.ts`, `web/src/systems/os/hooks/__tests__/window-manager-runtime.test.ts`, and the session-window/sidebar suites under `web/src/systems/os/apps/session/__tests__/`. The first diagnostic was read-only; the later experiment and its owning coverage are recorded above.
