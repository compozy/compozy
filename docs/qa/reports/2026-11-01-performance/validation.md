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
| Changed frontend owners | Canonical QueryObserver/component suites passed, including 63 dashboard tests, 31 Loop stream tests and 68 OS interaction tests after review/lint follow-ups. Other focused counts are in the reports; overlapping suites are not summed. |
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

The credential follow-up preserves unrelated-profile isolation and cached-read allocation
counts; overlapping timing ranges do not establish an additional speedup. The Loop burst
still produces three reads and zero cancellations for 100 events and reaches revision 100.
A stalled pre-event warm read can now be canceled once; initial/inactive or newly started
reads still keep their coalesced round open until settlement. No arbitrary timeout or
additional coordinator was introduced. Error assertions and ownership controls were
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
