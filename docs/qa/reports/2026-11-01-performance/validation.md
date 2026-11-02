# Integrated validation

Baseline: `de5ab2f4d5703f933f3134d2cf6844d99eb39b9f`. Branch:
`perf/responsiveness-audit-20261101`. Owner reports name the canonical suites and
their invariants; this document records integration evidence rather than duplicating
their test coverage. All runtime state is disposable and separate from the operator's
home. No live provider credentials were copied or used.

## Completed local checks

| Check | Result and boundary |
| --- | --- |
| `make codegen` then `make codegen-check` | Passed; canonical SQLC and all generated owners agree. No schema or public wire change. |
| `make go-lint` with explicit changed-package scopes and concurrency 2 | Passed, zero issues in normal and integration-tag lanes; changed-file formatter and source policies passed. Uses repository pin golangci-lint 2.15.0. |
| Root Turbo Web `lint typecheck build --filter=./web --only` | All three tasks passed. Lint: zero warnings/errors. SDK dependency built first through root Turbo. Build retains existing CSS/chunk-size/plugin-timing diagnostics; no threshold or assertion was weakened. |
| Changed concurrent Go owners | Focused race checks passed; real SQLite, real Git, real filesystem and loopback I/O are identified in each owner report. |
| Changed frontend owners | Canonical QueryObserver/component suites passed, including 63 dashboard tests, 32 Loop stream tests and 68 OS interaction tests after review/lint follow-ups. Other focused counts are in the reports; overlapping suites are not summed. |
| Real worktree lifecycle integration subset | Passed under race in 10.959 s: truthful status corruption/recovery, upstream divergence, refusal of dirty/unpushed loss, and external commit reread. |
| Independent cross-reviews | Five reports; UW-001 and RD-001 reproduced before and resolved after; L1 initial-fetch freshness hole resolved with 28 canonical and eight independent checks. Extension precedence: 4,608 independent SQLite combinations passed. |
| Source size and whitespace | Changed production sources remain at most 500 lines; final diff check passed. Test files reuse existing suites except the absent scheduler-action hook suite. |

The scope list for Go lint is `internal/api/core`, `attachments`, `command`, `daemon`,
`extension`, `filesnap`, `loop`, `marketplace`, `marketplace/pluginsource`, `observe`,
`providerusage`, `redact`, `scheduler`, `session`, `sessionprocess`, `situation`,
`skills`, `store/globaldb`, `store/sessiondb`, `terminal`, `tools` and `worktree`.
The prefix `internal/` applies to each entry.

```sh
GOMAXPROCS=2 GOFLAGS=-p=2 COMPOZY_GO_LINT_CONCURRENCY=2 \
  COMPOZY_GO_LINT_SCOPES='<the package paths above, each prefixed with ./>' make go-lint
bun x turbo run lint typecheck build --filter=./web --only
CGO_ENABLED=1 GOMAXPROCS=2 GOFLAGS=-p=2 go test -race -tags=integration \
  -count=1 -parallel=2 -timeout=5m ./internal/worktree \
  -run '^TestWorktreeLifecycleIntegration/Should_(record_real_status_corruption|report_real_upstream_divergence|refuse_dirty_and_unpushed|re-read_real_Git_evidence)'
```

The commands ran through a local serializer. The browser also acquired the repository's
machine-wide verification lock. No unrelated verification process was interrupted.

## Corrected attempts

The first published implementation was `14730a705771c0a3eb6e92947eddede632dff666`.
[CI run 41383826440](https://github.com/compozy/compozy/actions/runs/41383826440)
passed runtime/desktop E2E, three Web E2E shards, seven Go race shards, SDK race,
platform checks, codegen and static/build lanes. Three primary failures were retained
and investigated; aggregate verification jobs also failed because of those results:

- The runtime-selector helper returned as soon as the popup existed, before Motion's
  final opacity frame. A controlled 250 ms RAF delay reproduced the same visibility
  failure on baseline and candidate. Waiting for the popup's existing visibility
  contract fixed both, with original row/provider/ID assertions and default timeouts
  retained. Root Turbo passed all 114 selector/store tests. See [daemon/runtime](daemon-runtime.md).
- The worktree exit fixture supplied Git responses by invocation position and still
  expected two diff subprocesses removed by the measured clean-status fast path. The
  remote URL consequently received an empty diff response. A command-aware I/O stub
  restores the real command contract and adds a persisted local-status assertion while
  preserving the exact forge identity/count and cache assertions. The full worktree
  race suite passed; suppressing the real forge refresh in a negative overlay still
  fails. See [worktree/workspace](worktree-workspace.md).
- The Home health-loss E2E intercepted only `/api/status`, while connection consumers
  correctly continued reading `/api/status/identity`. The unchanged real-browser test
  reproduced the failure: full-status requests failed while identity returned 200.
  Injecting the intended outage into both public health endpoints repaired the existing
  test without changing production, assertions, timeouts or the real daemon restart.
  The focused E2E passed with zero retries (25.9 s test, 31.4 s run). See [session/OS](session-os.md).

These are not blanket reruns or relaxed expectations. The replacement head must run
the required CI lanes again; passing jobs on this earlier SHA are historical evidence.

- Initial browser setup stopped at onboarding; the existing onboarding helper fixed setup.
  A subsequent cross-checkout Playwright import loaded duplicate package instances; imports
  now resolve from the selected checkout's Web package. Failed attempts produced no latency
  claim, and the launched runtime's teardown receipt succeeded.
- Web typecheck before building `@compozy/client` failed on missing package output. After
  its build, it found two Bun-global annotations in the new benchmark scripts; those use
  `process.versions.bun` now. Lint also caught benchmark render mutation, cleanup error
  replacement and the bounded OS owner lookup. All were corrected, not suppressed.
- The first scoped Go lint found two function-length limits, a copied benchmark struct,
  an invalid self-comparison test idiom, an obsolete private helper and whitespace. Small
  cohesive fixes passed the owning suites and final pinned lint.
- The host golangci-lint 2.13.1 cannot decode Go 1.27.3 export data. Its failed attempts
  are not code failures or accepted lint evidence. The repository-pinned run above is
  authoritative. A separate direct integration formatter run also reported an untouched
  baseline Keychain fixture; the canonical changed-file/integration lint lanes passed.
- Early codegen encountered a second archived source tree under the repository. The frozen
  baseline moved to an external temporary directory; the local path is a symlink and the
  canonical generator/check then passed. Initial Bun 1.4.4 minifier drift was regenerated
  with the pinned toolchain; no unrelated generated change remains.

Receipts are retained under `.cache/performance/`: `codegen-integrated.log`,
`go-lint-final.log`, `web-final-lint-types-build.log`,
`worktree-integrated-real-git.log` and the per-owner directories. Exact reproduction
commands and measured fixtures are also checked in with each report.

## Follow-up focused evidence

- Session-window identity: the two cached/loading target cases failed before the
  production edit and the three canonical window/view/sidebar suites passed 45/45
  afterward. The unchanged instrumented real-app workload then passed 64/64 selections,
  eight complete streams and teardown; details are in the [navigation review](review-session-navigation.md).
- Terminal descriptor: complete metadata parity and independent mutable ownership for
  all nine terminal tools, approval policy/cancellation and the actual call boundary
  passed focused race checks in builtin and daemon. The canonical benchmark compares
  the old two-file implementation through a Go overlay against the shared narrow factory.
- React Doctor `npx --yes react-doctor@latest --verbose --diff` inspected 56 changed
  source files, reported 93/100 and no issues. Its receipt is
  `.cache/performance/react-doctor-followup.log`; the repository-pinned CI check remains
  authoritative for the delivered head.
- Follow-up root Turbo Web lint/typecheck/build passed all three tasks in 36.075 s;
  `make codegen-check` passed again. The pinned Go formatter wrapped two long failure
  messages in the added descriptor tests; those initial formatter failures remain in
  the receipts. No test expectation or production behavior changed during formatting.
- Final scoped `make go-lint` passed for `internal/tools/builtin`, `internal/daemon`
  and `internal/worktree`, with zero issues in normal and integration-tag lanes,
  all 86 changed Go files passing formatter checks, and source policies passing.
  Receipt: `.cache/performance/go-lint-followup-accepted.log`.

## External review follow-up

All 27 jobs in [CI run 41387686539](https://github.com/compozy/compozy/actions/runs/41387686539)
passed on `1665fb0c1c20c3244bd0d8ca2dc992458eeabcbc`, including eight Go race shards,
four Web E2E shards, runtime/desktop E2E, frontend checks, platform builds and recovery.
Greptile reported 5/5. That evidence predates the following review remediation;
the replacement head must pass fresh required CI before delivery.

| Review comment | Disposition and causal evidence | Owning report |
| --- | --- | --- |
| [Catalog error identity](https://github.com/compozy/compozy/pull/1141#discussion_r4438972319) | Accepted. The existing API case now requires the exact injected catalog error as well as HTTP 500 and no hydration. A wrong dependency error passed the old assertion and fails the strengthened one. | [API/tasks](api-tasks.md) |
| [Package integrity error](https://github.com/compozy/compozy/pull/1141#discussion_r4438972328) | Accepted. After concurrent readers finish, deterministic corruption must report `ErrPackageUnavailable`. A generic digest error is rejected; all original concurrent no-false-success checks remain. | [Extension/marketplace](extension-marketplace.md) |
| [Audit timestamp error](https://github.com/compozy/compozy/pull/1141#discussion_r4438972335) | Accepted. The all-profile read must expose the corrupt row's `time.ParseError`, value and context. An unrelated invalid-filter control passes the old assertion and fails the new one. | [Daemon/runtime](daemon-runtime.md) |
| [Read/read credential ordering](https://github.com/compozy/compozy/pull/1141#discussion_r4438972345) | Fixed. A newer successful read, including unchanged-file evidence, advances the guarded validation sequence so an older mismatch cannot clear its snapshot. Errors do not advance the sequence. | [Usage/dashboard](usage-dashboard.md) |
| [Credential race regression](https://github.com/compozy/compozy/pull/1141#discussion_r4438972357) | Accepted. Existing table cases reproduce read/read and cached-read ABA failures before the fix; a failed-validation control preserves mismatch handling. Race and negative controls pass. | [Usage/dashboard](usage-dashboard.md) |
| [Complete worktree benchmark](https://github.com/compozy/compozy/pull/1141#discussion_r4438972367) | Accepted. Every one of the 64 rows must retain local and forge status. Dropping only the final row's PR data fails. The equally guarded pair was remeasured: 6.292 → 2.628 ms/op, workspace reads 64 → 2 and PR reads 64 → 1. | [Worktree/workspace](worktree-workspace.md) |
| [Transient workspace lookup](https://github.com/compozy/compozy/pull/1141#discussion_r4438972378) | Fixed. Failed workspace resolution is no longer memoized as an empty repository for later rows. A real SQLite/Git fixture fails before, then retries once and shares the subsequent successful resolution. | [Worktree/workspace](worktree-workspace.md) |
| [Task query identity](https://github.com/compozy/compozy/pull/1141#discussion_r4438972387) | Accepted. The existing event test requires both exact `run_0` query keys in addition to the total count. Refetching a sibling task now fails; all 19 canonical cases pass. | [API/tasks](api-tasks.md) |
| [Attachment shared-root relist](https://github.com/compozy/compozy/pull/1141#discussion_r4438977542) | Not applied. Real filesystem interleavings confirm a wider window between independent stores, but both versions permit cross-instance count overflow. Shipped daemon wiring has one shared Store under its singleton lock; fresh pins remain authoritative. The unsupported window and lack of atomic cross-instance capacity are documented, not hidden. | [Terminal/attachments](terminal-terminal.md) |
| [Per-sweep pause sampling](https://github.com/compozy/compozy/pull/1141#discussion_r4438977546) | Not applied. Wake notifications are advisory. The transaction that claims a run rereads direct, global and ancestor pauses; the previous per-run reads also could not linearize notifications against pause. Existing inherited-pause/resume and next-sweep tests pass. | [Daemon/runtime](daemon-runtime.md) |
| [Nil-session diagnostic](https://github.com/compozy/compozy/pull/1141#discussion_r4438977550) | Accepted. The existing guard test requires its exact nil-input diagnostic while retaining cancellation and dependency-error assertions. | [Daemon/runtime](daemon-runtime.md) |
| [Pre-event Loop read](https://github.com/compozy/compozy/pull/1141#discussion_r4438977553) | Fixed. Active cached reads invalidate immediately; only initial/inactive/static reads retain a trailing obligation. Three failing-before cases cover a warm query alone and mixed prefixes with initial or disabled-prefetch siblings, including a custom query hash. All 31 canonical and eight independent controls pass. | [Loop/scheduler](loop-loops.md) |
| [Reverse credential observation order](https://github.com/compozy/compozy/pull/1141#discussion_r4439116140) | Fixed after reproducing against `56176f829`. A mismatch that collides only with a newer validation sequence now obtains fresh evidence outside the mutex; application order cannot establish observation order. Existing ownership guards, per-attempt cancellation and the exact validation error classification remain. | [Usage/dashboard](usage-dashboard.md) |
| [Credential application order](https://github.com/compozy/compozy/pull/1141#discussion_r4439135779) | The same reverse-order defect was independently reported by CodeRabbit. The shared correction above addresses both comments; no duplicate production path or test suite was added. | [Usage/dashboard](usage-dashboard.md) |
| [Retryable repository identity](https://github.com/compozy/compozy/pull/1141#discussion_r4439135789) | Fixed. A real Git start failure caused by a temporarily unavailable fixture root is not cached by `RepoKey`, but the listing had memoized its empty result. Only successful identities now enter the request-local map. Five canonical SQLite/Git cases pass, and the 64-row guard retains 2 workspace, 1 PR and 64 status reads. Stable negative caching remains unchanged. | [Worktree/workspace](worktree-workspace.md) |
| [Independent Loop progress](https://github.com/compozy/compozy/pull/1141#discussion_r4439135799) | Fixed. The global completion barrier is replaced by one read and one pending bit per existing Query object, local to the subscription. Healthy briefing/catalog queries process a second event before a blocked sibling settles. Initial/disabled reads consume both wakes in one trailing read. All 32 canonical and eight independent controls pass, preserving the 100/3/0 burst result. | [Loop/scheduler](loop-loops.md) |
| [PR-fact batch failure isolation](https://github.com/compozy/compozy/pull/1141#discussion_r4439472334) | Fixed after a real SQLite reproduction. One schema-valid but undecodable row made the batch omit all five valid siblings; the baseline retained them. Only a failed multi-key batch falls back once per key, respecting cancellation. Nine canonical cases pass under race; the normal 64-row guard still uses one PR batch. | [Worktree/workspace](worktree-workspace.md) |
| [Same-query Loop recovery](https://github.com/compozy/compozy/pull/1141#discussion_r4439472344) | Fixed. The first pending wake arms one recovery timer using the existing 15-second Loop interval. Only the captured, still-current active cached read can be canceled; its existing drain starts one trailing read. Later frames do not postpone the timer. Initial, disabled, static and replaced reads retain their boundaries. All 39 canonical cases pass. | [Loop/scheduler](loop-loops.md) |
| [Home aggregate barrier](https://github.com/compozy/compozy/pull/1141#pullrequestreview-5732161953) | Confirmed from the earlier unverified review hypothesis and fixed. A stalled overview kept the current inbox at revision 1 after later events; baseline reached revision 2. Real polling callbacks refreshed only the separately polled sibling. Home now uses the shared per-query coordinator with its existing 30-second interval; 16 canonical cases pass, including variants under one prefix and scope retirement. | [Usage/dashboard](usage-dashboard.md) |
| [Fallback query tradeoff](https://github.com/compozy/compozy/pull/1141#discussion_r4439726486) | Declined after comparing the actual fault receipts. A persistently malformed row recovers the same five valid facts with seven reads versus six on the baseline: one failed batch plus the original per-key behavior. Normal 64-row listings retain 64 to one PR read; canceled and single-key requests remain bounded. No normal-path gain supports changing shared store/Refresh error semantics or introducing recursive reads/caps. The exceptional extra batch is recorded as a tradeoff. | [Worktree/workspace](worktree-workspace.md) |
| [Malformed-row error identity](https://github.com/compozy/compozy/pull/1141#discussion_r4439726475) | Accepted in the existing canonical case. `errors.AsType` now requires `*json.UnmarshalTypeError` while preserving all existing assertions. A same-text error without the typed cause fails the new assertion and passes with only that assertion removed. The real SQLite case and four focused race cases pass; production and benchmark code are unchanged. | [Worktree/workspace](worktree-workspace.md) |

The credential follow-up preserves unrelated-profile isolation and cached-read allocation
counts; overlapping timing ranges do not establish an additional speedup. The Loop burst
still produces three reads and zero cancellations for 100 events and reaches revision 100.
A stalled pre-event warm read can be canceled immediately. A pending read retains only its
own query's reconciliation obligation; independent queries continue to process events.
The subsequent Loop/Home recovery uses one shared subscription-local helper and the existing
15-second/30-second refresh intervals. No global HTTP deadline, shared authorization cache or
cross-subscription scheduler was introduced. Error assertions and ownership controls were
strengthened within existing suites; no test timeout or acceptance threshold was relaxed.

Integrated follow-up checks passed after the production fixes:

- Scoped canonical Go lint for API, package cache, observe, provider usage, situation,
  GlobalDB and worktree reported zero issues in normal and integration-tag lanes;
  formatting passed for all 86 changed Go files and source policies passed.
  Receipt: `.cache/performance/review-go-lint.log`.
- Root Turbo Web lint, typecheck and build passed all three tasks in 35.582 s.
  Lint reported zero warnings/errors; existing build diagnostics remain recorded.
  Receipt: `.cache/performance/review-web-lint-types-build.log`.
- React Doctor inspected 57 changed source files, scored 93/100 and reported no
  issues. Receipt: `.cache/performance/react-doctor-review.log`. Its deprecated
  `--diff` spelling was honored; repository-pinned CI remains authoritative.

The subsequent reverse-order credential correction passed the complete canonical
`TestUsageService` race suite in 1.417 s. Four negative overlays separately prove
revalidation, the sequence guard, cancellation and unchanged same-owner error behavior.
The identical paired benchmark retains 2,296 B/op and 31 allocations for cached reads;
overlapping timing ranges do not support another speedup claim. An independent source
review checked both orderings, repeated collisions, ABA/cache and lifecycle/error cases
against the frozen source and the owner's receipts, with no further concrete finding.
That review did not execute another suite; see [its scope and limits](review-usage-worktree.md).

The per-query Loop correction received a separate read-only review of scopes, custom
hashes, L1, pending consumption and disposal, with no concrete finding. Its 32 canonical
and eight independent results were inspected, not rerun by that reviewer. The repository
identity correction passed five SQLite/Git cases and the complete benchmark guard under
race (6.538 s), plus existing worktree status/identity policies (1.989 s). Stable error
caching and cancellation are unchanged; the one benchmark iteration proves counts and
completeness, not a new timing improvement.

Integrated checks after those two corrections passed: scoped canonical worktree/GlobalDB
lint reported zero issues in normal and integration-tag lanes, with all 87 changed Go
files passing formatting/source checks (`repository-key-go-lint.log`). Root Turbo Web
lint/typecheck/build passed all three tasks in 70.795 s (`query-refresh-web-lint-types-build.log`).
React Doctor inspected 57 changed sources, scored 93/100 and reported no issues
(`react-doctor-query-refresh.log`). Receipts are under `.cache/performance/`.

## Runtime CI deadline and artifact retention

[CI run 41392366822](https://github.com/compozy/compozy/actions/runs/41392366822) on
`56176f8293d0954e9188041bb47f4796e10138ba` completed with 25 successful jobs and two
failed jobs. The primary failure was one runtime scenario; the E2E aggregate failed
because of it. All eight Go race shards, four Web E2E shards, frontend checks,
desktop E2E/static checks, platform builds, codegen and recovery passed.

`TestDaemonE2EHostedMCPProjectsAndCallsNonBootstrapNativeTool` reached its existing
30-second context deadline at the first ACP `noop` prompt, before the explicit hosted
MCP client/tool call. The isolated local reproduction used the same canonical case and
`56176f829` production through a Go overlay inherited by child builds. It passed:
66.206 s command, 32.027 s package, 25.11 s subtest including harness construction.
Recorded ACP startup was 451 ms and hosted binding 9 ms. This does not establish a
cause or a product fix for the CI timeout; production, assertions and timeouts remain intact.

The failed job also exposed a concrete diagnostic defect: Go 1.27 removed its artifact
directory when the child test process exited without `-artifacts`, and the uploader's
pattern did not select the reported location. The canonical integration runner now
retains artifacts in an absolute temporary `compozy-e2e-artifacts-*` directory and the
runtime uploader selects its `_artifacts` tree. The nightly uploader's broader existing
prefix remains compatible. No new tracked test or runtime behavior was added.

A temporary probe in the existing Mage suite called the real runner and canonical
manifest fixture. Before: zero retained manifests, expected failure in 11.812 s.
After: one complete manifest and daemon log survived the child process, pass in
6.136 s. Scoped canonical Mage lint passed with zero issues, including formatter/source
checks for all changed Go files. See [daemon/runtime](daemon-runtime.md) for commands,
receipts, caller/path checks and the remaining uncertainty. The next head must pass
fresh CI; an isolated local pass does not replace that acceptance.

The subsequent [runtime E2E job on `e8046c842`](https://github.com/compozy/compozy/actions/runs/41396361725/job/125118571350)
passed with the original test assertions/deadline and the artifact-retaining runner.
This is an actual CI pass for that head, not proof of the earlier timeout's root cause.
The later repository/Loop corrections still require CI on their published head.

## Browser and CI delivery

The real Chromium/daemon harness is `web/e2e/scripts/benchmark-concurrent-sessions.ts`.
It starts 32 sessions and eight 600-chunk controlled ACP streams, retains another 640
sessions, selects 16 sessions over four passes, checks selected identity and chat
visibility, and records requests, long tasks, post-GC heap, screenshot and teardown.
The first pass warms navigation; the remaining 48 selections form each run's sample.
Its elapsed interval includes Playwright actionability/assertions and two animation
frames. It is not INP, production telemetry or live-model latency. Repeated results
are recorded separately; timing variation does not justify a universal navigation claim.

Full local `make gate` / `make gate-full` were not run. The user's explicit instruction
authorizes a draft PR and CI for heavy gates to protect the shared machine. Required CI
must pass at the published head before delivery. At initial draft creation this is still
pending; the PR's live checks and final delivery record are authoritative.

Owner scenario notes preserve historical verdicts and distinguish focused evidence from
full journeys. Runtime suites cover actual SQLite/Git/ACP and Web integration in CI.
There is no existing dedicated Playwright scenario for attachment multi-drop/queued
attachments; real attachment filesystem, HTTP handler, queue persistence, dispatch and
startup-pin suites own those changed boundaries. A textual queue E2E is not counted as
attachment upload evidence.

## Bounded query recovery and batch-failure follow-up

All 27 jobs in [CI run 41398958288](https://github.com/compozy/compozy/actions/runs/41398958288)
passed on `862bf0fd0a4b37b6b902210bec48eba30e761202`, including runtime, desktop and
all four Web E2E shards. That is historical evidence for the source before this final
query-recovery and PR-batch correction; the PR delivery paragraph owns acceptance of
the subsequent published head.

The shared Loop/Home helper retains one accepted follow-up per Query object and bounds
eligible stalled reads using each domain's existing refresh interval. Its 15-second
negative control failed before the change for both active and disposed owners; the
settled/replaced/initial/disabled/static controls already passed. The final 39 Loop
cases and eight independent L1/scope/cleanup controls pass. A real QueryObserver probe
confirmed that three polling callbacks did not recover the old implementation, while
the corrected read restarts at 15,000 ms after the first pending wake and reaches
revision 2 without releasing the blocked response. The 100-event fixture still has
three reads, zero cancellations and final revision 100.

Home passed 33 focused cases (16 live-stream and 17 model cases). Five repeated paired
samples retain 300 to 6 requests and 297 to zero cancellations, with all three caches
at revision 100. Same-prefix overview variants and separate aggregate families progress
independently. The independent reviewer checked frozen hashes and receipts for both
domains, with no concrete finding; that review did not execute another test or benchmark.
Its [report](review-cache-loops.md) explicitly limits the recovery to an eligible restart,
not guaranteed network completion or global deduplication.

The real SQLite malformed-row reproduction recovered five valid PR rows after the fix.
Nine canonical cases and the 64-row completeness/count guard passed under race; normal
reads still perform two workspace resolutions, one PR batch and 64 status reads.
Single-key failure and cancellation before/during fallback do not create extra retries.
Existing worktree status and PR policies passed under race. The related Tasks triage
confirmed two identical baseline/current lifecycle limits; its source and canonical
suite remain unchanged, and [API/tasks](api-tasks.md) records the four negative probes
and four positive controls without claiming a Task lifecycle fix.

Final integrated local checks after the source freeze:

- Scoped canonical Go lint passed normal and integration-tag worktree/GlobalDB lanes
  with zero issues, plus formatting/source checks for all 87 changed Go files.
  `recovery-go-lint-after.log` supersedes an initial formatter-only failure; the
  canonical formatter changed only argument line breaks in the new test fixture.
- Root Turbo `lint typecheck build --filter=./web --only` passed all three tasks in
  37.765 s (`recovery-web-scoped-lint-types-build.log`). Lint had zero warnings/errors.
  Existing generated-CSS, chunk-size and build-timing diagnostics remain visible;
  no warning threshold or test assertion was relaxed.
- React Doctor scanned 59 changed files in 4.5 s, scored 93/100 and reported no issues
  (`react-doctor-recovery.log`). Repository-pinned CI owns its delivery acceptance.
- A preceding Turbo invocation without `--only` reached a dependency's generated
  linter check through the host Bun 1.4.4 launcher and failed before Web validation
  (`recovery-web-lint-types-build.log`). Running that exact owning `build-lint.ts --check`
  with repository-pinned Bun 1.4.0 passed (`recovery-pinned-linter-codegen.log`). No
  generated artifact or toolchain pin was changed. The scoped Web result above is
  separate from this failed dependency attempt; broad generation checks remain in CI.

All local receipts above are under `.cache/performance/`. Full local gates were not run.

The final assertion-only follow-up adds three lines to the existing malformed-PR case.
The real case passed in 0.906 s; four focused race cases passed in 4.417 s. Removing the
typed JSON cause failed the new assertion in 0.870 s, while the same control without that
assertion passed in 0.880 s. Scoped canonical GlobalDB lint passed both normal and
integration-tag lanes with zero issues (`pr-error-type-go-lint.log`). Production and
benchmark sources remain byte-identical to `bfb1cc476`; their completed focused evidence
is reused, while CI must still validate the newly published head.

## Main integration and expanded coverage

All 27 jobs in [CI run 41405892390](https://github.com/compozy/compozy/actions/runs/41405892390)
passed on `e62862f48362b9cc6da311df0078b0be914a5734`. The repository-pinned React Doctor,
catalog and marketplace-public-source workflows also passed. Greptile's summary explicitly
reviewed that head at confidence 5/5, and all 19 existing inline threads had dispositions.
These are historical receipts: main then advanced to `8c025ff8fdd3a9a79e4f0e63f54b3bd144a4773b`,
so final acceptance requires a fresh run on the integrated PR head.

The integration preserves both histories and all prior performance commits. Main adds
Stays-on agents and identity across 1,586 files. There were 25 paths changed by both branches
and two textual conflicts, both in existing tests. Both branches' assertions were retained:
session catalog scope/relations plus instance-main exclusion, and GlobalDB board benchmarks
plus pending-interaction descendants. The session registration fixture now supplies the
incoming pointer argument. No assertion or test deadline was relaxed.

The [coverage matrix](coverage.md) now enumerates **197 Go-containing directories**
(7,301 Go files) and **28 frontend systems** (3,832 tracked files). These include the new
`agentinstance` and `agentmemory` packages. All ten original owners reviewed their incoming
deltas, covering 43 Go directories and 12 systems changed by main. Each owner report records
source dispositions, focused canonical checks, and limitations. The inventory's file counts,
file-list SHA-256 digests and report links were checked against the actual tracked paths.
Source inspection, generated-file inventory and executed behavior remain distinct evidence.

The new upstream instance/memory contracts, migrations and identity dependencies remain
owned by that main commit. Static observations about its full-page hydration, identity
observers, and scope placeholders are documented in the relevant reports; this integration
does not claim to have optimized or recertified every new Stays-on journey. The original
benchmark tables remain historical. Explicit main-to-merged comparisons use an immutable
external archive and identical overlaid fixtures, with no production edits to the baseline.

Root integration checks run through the serialized two-CPU runner:

- Repository-pinned Bun 1.4.0 frozen installation passed. `make codegen-check` passed,
  including generated SQLC/contracts/SDKs, native manifests, CLI/site parity and visual
  generated assets (`main-integration-codegen-check.log`). No generated output was hand-edited.
- Root Turbo `lint typecheck build` filtered to Web, client SDK and UI with `--only` passed
  eight tasks (two cached) in 38.213 s (`main-integration-frontend-build.log`). Lint had zero warnings/errors;
  existing Vite CSS, bundle-size and plugin-timing diagnostics remain in the build receipt.
- The incoming Dashboard Activity view requires a router. Its existing eight-case window
  fixture now uses the established real memory-router provider; the same assertions pass
  without the missing-provider warning. No console diagnostic was suppressed.
- The first scoped session lint exposed `infoLocked` growing from the combined snapshot
  fields to 82 lines against the existing 80-line limit. This failed receipt is retained
  separately; the integration extracts the coherent capability snapshot block and rechecks
  behavior and allocations instead of relaxing the rule.
  The final canonical Info race check passed in 2.914 s; all four benchmark byte/allocation
  medians were unchanged. Scoped canonical session lint then passed both normal and integration
  lanes with zero issues (`session-os/main8c-info-refactor-lint.txt`).
- Root Turbo Web typecheck passed again uncached in 4.402 s after the router fixture repair
  (`main-integration-final-web-types.log`).
- The combined-source browser walk completed 64/64 selections, eight 600-chunk streams and
  both teardowns, with zero errors (`main-integration-browser.log`). The [browser report](browser.md)
  records exact inputs and its limit: it preceded the final behavior-preserving Go helper
  extraction; final-head CI owns the committed runtime/Web acceptance.

The incoming frontend CI job timeout increases from 35 to 45 minutes in main itself.
The performance delta retains that deadline; the workflow changes the runtime artifact
uploader path and now streams frontend task logs as explained below. Full local `make gate` and
`make gate-full` remain unrun under the user's explicit CI authorization. The PR delivery
record owns the exact final head, live checks and clean-worktree receipt.

## Frontend CI deadline and keyboard readiness

[CI run 41414334118](https://github.com/compozy/compozy/actions/runs/41414334118) on
`895d8a5d72840c92e01015bf313a2e2340cb084d` passed 25 jobs, including all eight Go race
shards, four Web E2E shards, runtime/desktop E2E, platform checks and code generation.
The frontend job reached its existing 45-minute deadline, and the Verify aggregate
failed because of that cancellation. This head is not a complete delivery pass.

The completed job log shows lint and the other package checks finishing; Web typecheck
finished at 08:44:03 UTC. Turbo's grouped output never flushed the Web test task before
the job was cancelled at 09:22:31 UTC. This establishes non-completion inside frontend
verification, but does not identify the hanging test or prove a production defect.
The workflow now uses `--log-order=stream` for the same typecheck/test command. It keeps
the same packages, test set, worker policy, assertions and job deadline.

A serialized diagnostic selected the 98 Web test files changed since the original
baseline. The first run stopped at an existing SessionList keyboard assertion after
312 passing tests. A separate run of the remaining 96 files passed all 1,924 tests in
210.14 seconds, with each file's queued, collected and finished events recorded. The
suspected fork-dialog collection stall did not reproduce; all 96 files finished. An
isolated AgentProfileHome import-cycle probe also passed all six cases. These focused
checks do not replace the full Web suite or identify the CI deadline's root cause.

The keyboard failure had an independent causal reproduction: the Destination menu item
was mounted while focus remained on the parent submenu trigger. Enter therefore reached
the trigger and never called `onAdopt`. In the passing control, the scheduled focus
handoff completed before Enter. The existing canonical test now waits for Destination
to own focus before sending the same Enter, retaining the foreign-parent exclusion and
the exact adoption arguments. No production behavior, manual focus, sleep, assertion or
timeout was substituted. The [store/session report](store-session.md) owns the controls
and final focused result. There is no demonstrated connection between this fixture race
and the CI non-completion.

Receipts under `.cache/performance/`: `main-integration-frontend-ci-timeout.log`,
`main-integration-web-changed-diagnostic.log`,
`main-integration-web-remaining-diagnostic.log`, and
`usage-dashboard/ci-hang-agent-profile-home.log`. Final acceptance requires CI on the
newly published head; the earlier 25 successful jobs are not substituted for that run.
