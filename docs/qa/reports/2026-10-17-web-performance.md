# Web performance optimization — 2026-10-17

Scope: frontend module loading, idle queries, query-cache subscriptions, and live transcript projections in `web/`. The isolated branch is `perf/web-performance`, based on `6de9011b6c34a39f4e0cadf7f44fd06caede54da`. There are no backend, dependency, visual-design, mobile-layout, schema, or public API changes.

The branch was subsequently rebased on `3b83024757c38bdca11ef94555fba9f6d2c6ab80`. Its existing terminal-lifecycle and stream-aware polling improvements are preserved; the performance baseline above remains the original measured commit.

## Results and limits

The desktop shell chunk measured after the initial lazy-loading split shrank from 1,895,620 to 1,348,535 bytes (28.9%), or from 596,254 to 426,856 bytes using Python gzip (28.4%). Deferred code still loads when its feature opens. This is a shell-chunk comparison, not a claim that every route downloads 29% less JavaScript. Aggregate cold-route captures had different readiness points and are excluded.

Warm immutable transcript projections became cheaper in the measured workload:

| Operation | Baseline p50, ms | Optimized p50, ms | Reduction |
| --- | ---: | ---: | ---: |
| Flatten/project transcript | 0.12729 | 0.07535 | 40.8% |
| Reconcile durable and runtime messages | 0.13862 | 0.06212 | 55.2% |
| Pending permission projection | 0.08706 | 0.01739 | 80.0% |
| Decided permission projection | 0.08745 | 0.01846 | 78.9% |
| Turn-outcome projection | 0.12515 | 0.04257 | 66.0% |

The seeded equivalence probe compared 2,000 adversarial cases against the original source, performing 40,000 comparisons without a mismatch. Eight baseline/optimized golden output hashes also match. The equivalence digest is `0150c05716418db8f797d100d8dbdd48cafad866efb10e66b95b6104b02fe67c`.

A separate cache diagnostic mounted five disabled subagent readers with 1,005 cached queries and issued 1,000 unrelated cache writes. The original hook made 10,000 linear `QueryCache.find` calls; the optimized hook made zero. The measured diagnostic took 14,362.883 ms before and 1.9745 ms after. This deliberately amplified workload is not a user-visible latency measurement.

[Raw measurements and hashes](../evidence/2026-10-17-web-performance/measurements.json) include p95/p99 results. Shared-machine scheduling and garbage collection affected tail timings; no INP, frame-rate, memory reduction, or universal startup-latency improvement is claimed. Bundle numbers precede the final route/query refinements; CI validates the final source.

## Profiling method

- Worktree: `/Users/pedronauck/Dev/compozy/_worktrees/web-performance`.
- Runtime: Bun 1.4.0. The transformation workload contains 1,200 messages, 13,816 parts, and six 200-entry transcript pages, with 50 warmups and 200 samples of ten calls per operation.
- The baseline CPU profile sampled 1,987 frames over approximately 5.17 seconds. Application self-time hotspots included `latestPermissionParts` (7.4%), `providerFailureTurnIDs` (5.3%), `eventTurnID` (4.0%), `deriveTurnOutcomes` (3.9%), and `mergeTranscriptEntries` (3.7%).
- The original source was extracted from the base commit for differential probes; current dependency versions were shared between both sides.
- Golden serialization normalizes only assistant-ui's materialization-time `createdAt`. Recorded transcript timestamps, ordering, identifiers, permission decisions, outcome precedence, and stream/page fences remain covered.
- Large CPU profiles and development logs remain ignored under `.tmp/web-performance/`; portable numeric receipts are committed alongside this report.

## Opportunity matrix and implementation

Scores are impact × confidence / effort, with each input on a 1–5 scale. Each implemented lever scores at least 2.

| Lever | Evidence | Impact | Confidence | Effort | Score |
| --- | --- | ---: | ---: | ---: | ---: |
| Build the historical failure index only when needed | 5.3% sampled self time; ordinary text never uses it | 3 | 5 | 1 | 15 |
| Reuse parsed permission facts | 7.4% sampled self time; unchanged history scanned by two consumers | 3 | 5 | 2 | 7.5 |
| Reuse parsed turn outcomes | 3.9% sampled self time; settled events reparsed on text updates | 3 | 5 | 2 | 7.5 |
| Reuse sparse provider-failure facts | Failure projection walks every old part on each update | 3 | 5 | 2 | 7.5 |
| Remove unconsumed shell attention queries | Real baseline recorded repeated pending-approval list reads; no output consumer | 4 | 5 | 1 | 20 |
| Scope roster subscriptions and use direct lookup | 10,000 linear scans for unrelated writes in the diagnostic | 4 | 5 | 2 | 10 |
| Stop workspace-setup reads while closed | Hidden shell dialog starts filesystem and default-agent queries | 3 | 5 | 2 | 7.5 |
| Load the empty-desktop composer on demand | Shell bundle included composer/runtime and Lexical before they were needed | 4 | 5 | 2 | 10 |
| Load worktree dialog boundaries on demand | Shell imported unopened worktree dialog forms | 3 | 5 | 1 | 15 |

### Transcript reconciliation and projections

`mergeSessionThreadReadModel` now constructs its durable provider-failure turn set only after an unreconciled runtime part is recognized as a possible duplicate failure. Normal text updates keep the existing ID-based merge without a second history scan.

Permission, outcome, and transcript-failure helpers keep sparse facts in `WeakMap`s keyed by immutable content/part arrays. They still recombine the currently loaded messages on every call; they do not cache a whole session result, retain discarded histories through strong global keys, or assume that message order can never change. A replacement array is parsed immediately. Role filtering remains outside the outcome cache.

The failure projection retains actionable provider diagnostics over generic failure markers, and generic markers over duplicate raw errors. Mixed messages preserve their text and other parts. The precedence of `part.turnId` over event turn identity remains unchanged.

### Shell attention reads

The shell no longer fetches an unconsumed 100-row pending-approval page every five seconds or probes two unused Loop-node existence results. Task badges still use the authoritative task dashboard; Loop request counts and rows still use the existing request-attention owner. Document visibility, stale-state handling, profile scope, and workspace aggregation remain with their existing owners.

### Subagent cache subscriptions

A roster reader now filters notifications by the workspace-qualified query hash and reads the cache by that hash instead of scanning every query. A disabled reader has no cache subscription. Reading a roster does not add a Query observer: the existing stream owner must still be present, the roster must be fresh and complete, and the last owner leaving still restores the authoritative paged HTTP list. Data and observer count remain separate external-store snapshots.

### Closed workspace setup

The existing directory-browser `enabled` control now reaches the shell's workspace setup hook. The default-agent query is also enabled only while the setup dialog is open. The dialog remains mounted so selected paths, names, extra roots, and submission errors survive close/reopen. Other consumers retain the default enabled behavior.

### Lazy desktop features

The empty desktop and its composer are loaded with React `lazy`/`Suspense`. A cold link to an app does not mount the empty composer while the daemon's window-open receipt is arriving. The router pathname is read as a boolean selector; the routing coordinator remains the only URL/window reconciliation owner. The desktop's ordinary `/` entry and existing standalone layer consumers still allow the launcher.

The five worktree dialog boundaries are also lazy imports behind their existing target conditions. The target state, create-list query, close handlers, and bulk-remove flow retain their owners.

## Behavioral equivalence by lever

| Lever | Preserved ordering and tie-breaking | Proof or owning suite |
| --- | --- | --- |
| Lazy failure index | Durable IDs win; runtime tail order and failure suppression stay unchanged | Differential golden probe; existing runtime-provider suite |
| Permission facts | First-seen request order, latest decision wins, both supported part encodings | Differential probe; existing decision-dock suite with replacement/removal/reorder cases |
| Outcome facts | Assistant-only events; verified receipts outrank markers, otherwise latest wins | Differential probe; existing turn-outcomes suite with role changes and content replacement |
| Sparse failure facts | Page merge order/fences unchanged; actionable diagnostic and turn-ID precedence retained | Differential probe; existing transcript-query suite with mixed and empty messages |
| Attention reads | No displayed result was sourced from removed queries; exact aggregate owners retained | Existing attention/Loop suites; real baseline request trace |
| Roster readers | Workspace/session key identity, completeness and stream-owner fallback unchanged | Existing session-list integration suite; unrelated-write diagnostic |
| Closed setup | Draft state and browse location retained; reads resume on reopen | Existing directory-browser integration suite with close/invalidate/reopen |
| Empty desktop | Same launcher and routing ownership; module arrives only when its destination needs it | Existing empty-desktop, routing-coordinator, and OS-shell E2E suites assigned to CI |
| Worktree dialogs | Same target conditions, forms, and handlers | Existing worktree suites assigned to CI |

Floating-point behavior and random seeds are not involved in the production changes. The adversarial diagnostic uses fixed seed `0x102026`. Each performance lever has its own commit so it can be reverted independently.

## QA coverage and validation

| Scope | Evidence | Status |
| --- | --- | --- |
| Runtime provider reconciliation and decision dock | Root Turborepo focused run, 86 tests across two existing files | PASS before final shell refinements |
| Roster integration and closed setup | Root Turborepo focused run, 35 tests across two existing files | PASS before final shell refinements |
| Differential transformations | Eight matching golden hashes; 40,000 seeded comparisons | PASS |
| Initial production code split | Root Turborepo production build | PASS; final refinements require CI build |
| React Doctor baseline | Web score 93, `ok: true` | Baseline only; final regression scan assigned to CI |
| Real isolated daemon and browser | Tasks navigation/reload, workspace registration, initial idle requests | Exploratory evidence only; used the initial build, not the final source |
| Final lint, typecheck, full suites, build, browser E2E | Pull-request CI | Required remotely |

The first broad local test attempt reported 6,857 passing tests and one new directory-browser failure while its source was being changed. The subsequent focused run of the current owning suite passed. That broad run is not passing evidence. A later `make gate` was interrupted with exit 130 at the user's explicit instruction to leave CPU-intensive gates to CI. It must not be reported as green or restarted locally.

The isolated lab manifest is `/Users/pedronauck/dev/qa-labs/compozy-web-performance-20261017-224017-475920-lab/qa-artifacts/qa/bootstrap-manifest.json`. This targeted lab has no live-provider requirement. No final-source long-history, live-streaming, attention-populated, or worktree-dialog persona walk has been claimed. Existing real-daemon Playwright suites in CI own integration validation; the scenario files below flag the specific affected legs and retain their historical evidence.

Affected scenarios: `RT-047`, `RT-visible-session-streaming`, `ET-hidden-window-live-resources`, `RT-web-attention-title-count`, `LP-web-bell-loop-requests`, `MS-web-workspace-add-directory-browser`, and `ET-web-catalog-navigation`.

## Compatibility and rollback

All changes are internal Web implementation details. Query keys, workspace/profile partitions, cursor/envelope contracts, persistence, public interfaces, native tools, configuration, hooks, extension SDKs, and official `skills/compozy/` stay unchanged. No migration or generated output is needed. The cross-surface audit is recorded once in `docs/_memory/change-impact.md`.

Rollback is the owning lever commit's revert. There is no state migration to reverse. Full current-head CI and review state are recorded in the PR; local benchmark results alone do not establish delivery readiness.

Lab hygiene: exact-manifest teardown completed with `clean=true`. The local behavioral lab remains incomplete for the final source; its strict audit records the missing final local gate, which is intentionally CI-owned under the user instruction. No lab-level PASS is claimed.

## Review follow-up: deferred-download recovery

Greptile identified that a rejected lazy import could escape Suspense and replace the whole desktop through the root boundary. The existing `OsWindowErrorBoundary` now accepts an optional fallback slot. The empty launcher and each deferred worktree dialog reuse it with existing `Empty`/`ConfirmDialog` primitives. Errors remain logged without exposing raw exception details. A failed dialog can close without affecting other windows; both feature fallbacks offer an explicit full-page Reload because React lazy retains rejected module state.

Existing real-daemon `os-shell.spec.ts` and `worktrees.spec.ts` journeys now abort the corresponding module request at the network boundary, assert contained recovery, and continue after reload to the original happy-path assertions. No test file or dependency was added, and no heavy local run was started. CI owns execution of these final recovery legs.

## Review follow-up: cached outcome ownership

CodeRabbit identified that a returned outcome value remained mutable even though its containing map is read-only. The projection now shallow-copies sparse outcome facts into each result, so a consumer cannot mutate the cache or alter later verified precedence. The existing turn-outcomes suite owns the new mutation-isolation regression. Transformation timings above precede this ownership hardening; no fresh performance timing or heavy local gate was run after the user's instruction.
