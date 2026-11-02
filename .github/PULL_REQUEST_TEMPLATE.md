<!--
Title format: <type>: <description> — type is one of feat | fix | refactor | perf | docs | test | build | ci.
  Good: fix: drop stale session sockets on daemon restart
  Bad:  Fixed a bug · update stuff · fix #123
-->

## What & why

<!-- What changes for someone using CompozyOS, and why.
     Link the issue if there is one: Fixes #123 -->

## How you verified it

<!-- Commands you ran and what they showed. Screenshots or a short clip for UI changes.
     If you added no tests, say why. -->

## Impact

<!-- Does this change a CLI command, an HTTP/UDS route, a config.toml key, or a web surface?
     Name which, and whether the docs (packages/site) need an update.
     "No impact" is a fine answer — just say it. -->

---

- [ ] `make gate` passes locally; this PR is delivered only after its required CI checks are green
- [ ] New or changed behavior is covered by tests, or I explained above why not
- [ ] If an agent wrote or co-wrote this, I named it above and verified the result myself

<!-- First time here? See .github/CONTRIBUTING.md. For anything bigger than a bug fix,
     open an issue first — it protects your time as much as ours. -->

<!--
Historical reference: desktop update repair, PR #886 (2026-10-21).
This retained report applies only to PR #886. It is not verification, authorship,
or scope evidence for a new pull request. New PRs must fill the prompts above.

## What & why

Desktop updates could remain staged indefinitely or close the application before the native installer was ready. Users then had to cancel the staged operation in Settings and download a new app manually. This change repairs the installation lifecycle, stalled-operation recovery, app/runtime coordination and retry action.

The intended desktop-owned interaction is one Update action: download and verify the app bundle, finish native staging while the window remains available, let the native installer restart the app, then observe the installed app and ready runtime. A failed download or native staging attempt exposes **Retry update** directly.

Authored and verified by Codex using an isolated worktree. The implementation baseline was `b1008af26`; the initial implementation commit is `c3e787382`.

## Investigation and confirmed causes

| Finding                                                          | Evidence                                                                                          | Change                                                                                                        |
| ---------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| Ordinary shutdown could beat native staging                      | Real Electron BrowserWindow reproduction showed the last window closing before Squirrel was ready | Prepare the native installer before destructive cleanup and give it ownership of quit                         |
| A persisted watchdog did not interrupt a live blocked waiter     | Blocked-I/O and late-completion regressions in the existing consumer suite                        | Execute phase deadlines, propagate cancellation and reject late results                                       |
| A failed acquisition could consume the watcher revision          | Filesystem watcher reproduction                                                                   | Acknowledge only accepted revisions so the existing polling loop retries transient failures                   |
| The app plan could disagree with MacUpdater under Rosetta        | Pinned updater implementation and Go architecture-selection regressions                           | Select the app asset using Apple Silicon hardware identity and disable incompatible differential download     |
| A joint update replaced a desktop-owned runtime before its owner | Verified planner regression                                                                       | Normalize eligible app-plus-runtime intent to one app bundle and let existing bootstrap provision its runtime |
| App completion could precede daemon readiness                    | Real local daemon HTTP/discovery fixtures                                                         | Wait for published daemon identity, compatible version and a matching runtime snapshot                        |
| A settled failure had diagnostics but no direct retry action     | Existing Web presentation and menubar regressions                                                 | Retain the failure and expose Retry update when the release is eligible and the operation channel is free     |

The operator's desktop log also contained 22 instances of `The runtime update operation is invalid.` on 2026-10-20. The released beta.35 parser rejects runtime journal phases that beta.36/37 already support. This is corroborating historical evidence, not a new parser fix in this PR: the canceled journal and executing app version were unavailable, so the incident cannot be reconstructed uniquely. The current shutdown and liveness defects were reproduced independently.

## Reverse engineering and competitor comparison

- **Grok Bot 0.75.0:** REA analyzed the installed Electron/ASAR artifact without launching or modifying it. Artifact hashes, exact packaged JavaScript and machine-readable evidence are recorded. Parsing covered 759 JavaScript files with no parse failures; semantic graph coverage is partial, and nine unpacked native integrity discrepancies remain explicitly untrusted. Its update flow separates download, bounded native staging, installer-owned quit and recovery.
- **Synara:** inspected revision `299fd573b1d6520c323a87c78d30be74e87e53e1`. Background preparation, bounded phases, durable update identity and native quit ownership informed the lifecycle comparison.
- **T3 Code:** inspected revision `5b2ee393c3d6576bcf44c5d2b74565770943e189`. It has both a local confirmation flow and a combined countdown flow, with explicit native installation lifecycle handling.
- **MonoCode:** inspected revision `115f0e97c4f869188b5868cc4e9969d6f1a3f972`. The Tauri sidebar performs download/install/relaunch from one action; Settings can add confirmation. Its error visibility also has limitations.

These source inspections establish implementation patterns, not measured competitor reliability. Detailed citations are recorded in:

- [Research summary](../docs/research/desktop-update-flow/README.md)
- [Grok Bot analysis](../docs/research/desktop-update-flow/grok-bot.md) and [REA evidence JSON](../docs/research/desktop-update-flow/grok-rea-evidence.json)
- [Competitor comparison](../docs/research/desktop-update-flow/competitors.md)
- [Compozy causal reproductions](../docs/research/desktop-update-flow/compozy-causes.md)
- [App-owned planner and CLI evidence](../docs/research/desktop-update-flow/compozy-app-owned-plan.md)

## Implementation

### Desktop download, staging and shutdown

- Verify the planned version and artifact before installation, retain digest verification, and propagate AbortSignal to the downloader cancellation token.
- Add explicit native preparation that waits for readiness, error or cancellation before cleanup. Installation requires successful preparation.
- Keep ordinary app/window shutdown from winning once the installer owns quit; restore ordinary ownership when preparation fails.
- Bound checking/downloading with a 60-second inactivity deadline and a 10-minute absolute deadline, native staging with five minutes, and final cleanup/handoff with 30 seconds. Only increasing download progress refreshes inactivity.
- Renew the operation lease every 30 seconds during preparation, drain renewal before cleanup, and fence late asynchronous results after cancellation.
- Preserve persisted recovery deadlines for restart recovery. Terminal failures remain acknowledged; transient pre-acquisition failures remain retryable.
- Use Electron's Rosetta detection for differential-download policy and remove the superseded internal cross-architecture helper and its redundant test.

### Planner, runtime ownership and CLI completion

- Validate the original requested targets before normalizing a verified, unmanaged desktop-owned joint update to the app target. Caller inputs are not mutated; explicit runtime-only and independent/managed behavior is preserved.
- Probe Apple Silicon hardware with the existing command boundary and a bounded timeout for Darwin/amd64 app selection. Runtime archive selection still follows the runtime architecture; cancellation remains observable.
- After app installation, wait for the actual local daemon's published identity, readiness and compatible version. Confirm the runtime snapshot matches status, retain a compatible newer runtime, and report restart truth from observation.
- Preserve verified app success if runtime readiness fails. Closed-app staging retains its existing behavior.

### Web, documentation and native test fixtures

- Add Retry update to the menubar and General Settings for an eligible settled app failure; preserve error visibility and refresh release eligibility on retry.
- Update the official desktop skill, getting-started and operations documentation, two existing QA journeys and six affected scenario contracts.
- Build distinct baseline/candidate runtimes in the existing native fixture and assert actual daemon readiness after installation.
- Isolate native Squirrel identity/cache using a worktree-specific E2E bundle ID; derive the existing ad-hoc signature requirement from that ID.
- Preserve process state and desktop logs on restarted-shell teardown failure. No timeout or behavior assertion was weakened.
- After review, add a native filesystem-failure regression and finish existing cleanup before quitting on a failed installer handoff. Compare pre-update and post-update daemon PID/start time instead of copying an incomplete restart flag, and assert specific CLI observation errors.
- Treat the previous daemon identity as optional restart-reporting evidence. A corrupt or unreadable discovery file cannot prevent acquiring the update; final runtime health and version checks still determine completion.

## How you verified it

Regressions were added to existing owning suites after reproducing the defects. No standalone duplicate test suite or production dependency was introduced.

| Verification                                         | Result                                                                                                                            |
| ---------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| Root `make gate` and current-tree `make gate-status` | All affected local lanes passed: Go lint/race tests, code generation, desktop and Web lint/typecheck/tests                        |
| Desktop suite                                        | 191 tests in 22 files passed                                                                                                      |
| Web suite                                            | 7,262 tests in 687 files passed                                                                                                   |
| Existing native macOS rehearsal                      | 8 tests passed in 4.4 minutes, including real replacement/relaunch from app/runtime 0.3.0 to 0.3.1                                |
| Native recovery                                      | Expired handoff retains old-version truth, displays Retry update, then a fresh staged attempt installs and starts the new runtime |
| Adjacent native boundaries                           | Shell shutdown/relaunch, updating during bootstrap failure, and rejection of an unverified bundled runtime passed                 |
| Go focused regressions                               | Update package and owning CLI suites passed with the race detector; default/integration lint reported zero issues                 |
| Windows compilation boundary                         | `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./internal/update` passed                                                       |
| React Doctor locally                                 | 100/100, no issues                                                                                                                |
| Root Turborepo Web build/typecheck                   | Passed                                                                                                                            |
| Pre-commit tasks                                     | Passed using pinned Bun 1.4.0 and Go 1.27.1 tooling, with lint-staged stash/revert behavior disabled                              |

Native command, invoked from the repository root under the existing verification machine lock:

```sh
bun run --cwd desktop test:e2e --grep 'E2E-016|E2E-017|E2E-007|A staged app update'
```

Lint reports zero warnings/errors. Existing Web `act(...)` notices, expected failure-path console errors, experimental SQLite warnings and build diagnostics are documented; the complete test/build logs are not claimed to be warning-free.

The [verification report](../docs/qa/reports/2026-10-21-desktop-update-flow.md) records intermediate failures, corrections, environment isolation, local evidence paths and teardown. Screenshots and large logs/binaries remain local ignored artifacts. The full six-scenario contracts retain `untested` status because the targeted rehearsal is narrower than the complete release journeys.

## Impact

- **User-facing:** one app-bundle update and native restart for desktop-owned joint intent; direct retry after a settled failure; truthful CLI runtime completion.
- **CLI/HTTP/UDS/native tools:** existing update entry points and operation schema remain in use. No new verb, flag, route, DTO, `compozy__*` ID or journal format.
- **State/config/extensibility:** no database migration, config key, hook or extension SDK change. Existing ownership/eligibility boundaries remain enforced.
- **Web/Docs/official skill:** retry behavior and desktop update instructions are updated with the implementation.
- **Isolation:** research is read-only against the installed Grok app; native validation uses fixture apps/homes/ports and a distinct native updater identity.
- **Dependencies:** none added; no second updater or update service introduced.

The owning [cross-surface impact audit](../docs/_memory/change-impact.md) records these surfaces and the affected QA contracts.

## Remaining release acceptance and limitations

- Native fixtures use local ad-hoc signatures and a seeded verified operation. Both app versions contain the repaired code. They prove native replacement and runtime bootstrap, not the production GitHub/Sigstore/notarization chain or an already shipped old updater consuming a signed candidate.
- A real Rosetta installation, Linux AppImage update and complete browser/CLI apply journey are not claimed as executed. Broader charter branches, including active-agent shutdown and install-directory permission failures, remain outside this targeted rehearsal.
- An already installed broken updater cannot receive its own fix before it installs new code. Affected historical versions may need one recovery install or a separately verified bridge.
- Download/native-staging failures remain recoverable before cleanup. An exceptional failure during final destructive cleanup is journaled; the drained shell exits and can be relaunched.
- This PR does not publish a release. Local evidence does not substitute for current-head CI, CodeRabbit, Greptile or React Doctor review results.

---

- [x] `make gate` passed for the implementation; this description is editorial.
- [x] Changed behavior is covered by existing owning regression suites and a real native run.
- [x] Agent authorship, verification evidence and limitations are disclosed above.
- [ ] All applicable required CI checks are green on the current PR head.
- [ ] CodeRabbit and Greptile findings have evidence-backed dispositions and no unresolved actionable threads.
- [ ] React Doctor is green on the current PR head.
- [ ] Squash merge is performed only after the current head passes the delivery checks above.

-->

<!--
Historical reference: concurrent-session responsiveness audit (2026-11-01).
This retained measurement report applies only to branch perf/responsiveness-audit-20261101
and baseline de5ab2f4d. It is not verification, authorship or scope evidence for a new PR.
New PRs must fill the prompts above. Full reproductions, review dispositions and final CI
are recorded in docs/qa/reports/2026-11-01-performance/README.md.
Authored by Codex with ten scoped audit workers and independent cross-reviews.

## Scope and evidence

The audit covers the 92 `internal/` package roots and all 28 frontend systems,
with distinct owners and individual findings in [the coverage inventory](../docs/qa/reports/2026-11-01-performance/coverage.md).
Nested package coverage belongs to each owner's report. The work includes database
queries, navigation/data loading, event reconciliation, state subscriptions, retained
memory, and whether concurrency would actually remove a measured bottleneck.

Previous performance changes in PRs #723 and #983 are already part of the baseline.
Their historical measurements are not reused as results for this branch.

Each retained change must have a before/after comparison using the same fixture and
budget, behavioral coverage at its owning layer, and an explicit correctness check.
Operation counts, allocations, synthetic CPU timings, browser observations, and
production traffic are different evidence categories and are reported separately.
No change may claim user-facing latency improvements based only on a unit benchmark.

## Retained improvements

The tables report comparable fixture medians or exact operation counts, with the unit
shown for each row. They are not additive: several optimizations affect the same request.
The linked reports include population sizes, controls, sample counts, ranges, allocation
measurements, test invariants and rejected alternatives. Darwin/arm64 measurements use
an Apple M4 Max. Go measurements run with two CPUs; frontend runtime versions are recorded
per experiment. Benchmark timings include unrelated shared-host noise.

| Area and workload | Before | After | Evidence |
| --- | ---: | ---: | --- |
| Session Info, 128 model options | 34,512 B/op; 12 allocations | 12,400 B/op; 6 allocations | [Session/OS](../docs/qa/reports/2026-11-01-performance/session-os.md) |
| Scoped catalog, 1,024 active sessions in 32 workspaces | 7.309 ms; 37.02 MB/op | 0.223 ms; 0.592 MB/op | [Session/OS](../docs/qa/reports/2026-11-01-performance/session-os.md) |
| Process-demand read, 256 idle active sessions | 0.902 ms; 3,179,008 B/op | 0.00499 ms; 2,304 B/op | [Session/OS](../docs/qa/reports/2026-11-01-performance/session-os.md); baseline already includes the Info fix |
| Transcript fold retained heap per measured cache | 6,303,472 B | 4,199,638 B | [Store/session](../docs/qa/reports/2026-11-01-performance/store-session.md) |
| SQLite catalog page without repository filter, 4,096 rows | 8.969 ms | 7.353 ms | [Store/session](../docs/qa/reports/2026-11-01-performance/store-session.md); redundant PR joins removed |
| Session invalidation CPU, surfaces / usage fixtures | 9.392 / 6.707 ms | 7.652 / 4.153 ms | [Store/session](../docs/qa/reports/2026-11-01-performance/store-session.md); custom query hashes preserved |
| Scheduler capability sweep, 128 sessions | 9.672 ms; 256 unrelated SQL reads | 0.0387 ms; 0 unrelated reads | [Daemon/runtime](../docs/qa/reports/2026-11-01-performance/daemon-runtime.md) |
| Scheduler projection, 256 runs / 64 tasks | 27.700 ms; 768 SQL reads | 4.787 ms; 128 SQL reads | [Daemon/runtime](../docs/qa/reports/2026-11-01-performance/daemon-runtime.md) |
| Scoped observe snapshot, one of 64 tasks with 16 events each | 4.367 ms; 1,024 decoded events | 0.304 ms; 16 decoded events | [Daemon/runtime](../docs/qa/reports/2026-11-01-performance/daemon-runtime.md); identical event digest |
| Loop complete roster, 10,000 outputs | 141.979 ms; 142.72 MB/op | 8.795 ms; 6.72 MB/op | [Loop/scheduler](../docs/qa/reports/2026-11-01-performance/loop-loops.md) |
| Scheduler selection, 500 runs and sessions in 50 workspaces | 12.072 ms; 42.89 MB/op | 1.198 ms; 2.27 MB/op | [Loop/scheduler](../docs/qa/reports/2026-11-01-performance/loop-loops.md); same 500 targets |
| Scheduler selection, single-workspace control | 30.767 ms; 42.89 MB/op | 26.155 ms; 2.20 MB/op | [Loop/scheduler](../docs/qa/reports/2026-11-01-performance/loop-loops.md) |
| Worktree aggregate, 64 rows / two workspaces | 5.620 ms; 64 workspace + 64 PR reads | 2.682 ms; 2 workspace + 1 PR reads | [Worktree/workspace](../docs/qa/reports/2026-11-01-performance/worktree-workspace.md) |
| Clean worktree status with real Git | 45.699 ms; three Git subprocesses | 16.169 ms; one Git subprocess | [Worktree/workspace](../docs/qa/reports/2026-11-01-performance/worktree-workspace.md) |
| Extension tool List + Resolve, 128 extensions | 11.434 ms; 29,549 allocations | 6.206 ms; 15,531 allocations | [Extension/marketplace](../docs/qa/reports/2026-11-01-performance/extension-marketplace.md) |
| Extension inventory, 128 extensions | 1.990 ms; 128 attachment reads | 0.452 ms; one attachment read | [Extension/marketplace](../docs/qa/reports/2026-11-01-performance/extension-marketplace.md) |
| Warm package-availability check, 8 MiB archive | 4.725 ms; 36,296 B/op | 0.180 ms; 3,183 B/op | [Extension/marketplace](../docs/qa/reports/2026-11-01-performance/extension-marketplace.md); installation still hashes |
| Skill frontmatter parse | 18.566 µs; 20,768 B/op | 9.479 µs; 11,136 B/op | [Extension/marketplace](../docs/qa/reports/2026-11-01-performance/extension-marketplace.md) |
| Terminal ring, 100 writes then full replay | 44.866 ms; 317.29 MB/op | 0.299 ms; 2.23 MB/op | [Terminal/attachments](../docs/qa/reports/2026-11-01-performance/terminal-terminal.md) |
| Attachment Put, 512 retained objects | 123.970 ms; 6.10 MB/op | 77.965 ms; 3.07 MB/op | [Terminal/attachments](../docs/qa/reports/2026-11-01-performance/terminal-terminal.md); filesystem fixture, no SQL-pin claim |
| Attachment Open, 4 MiB content | 3.385 ms; two content validations | 1.803 ms; one held-descriptor validation | [Terminal/attachments](../docs/qa/reports/2026-11-01-performance/terminal-terminal.md) |
| Repeated secret registration, 1,024 secrets | 209.715 µs; 36,912 B/op | 0.0507 µs; 0 B/op | [Session/OS](../docs/qa/reports/2026-11-01-performance/session-os.md) |
| Slash parser, 128 skills / approximately 0.85 MB prompt | 217.029 ms; 527.54 MB/op | 1.699 ms; 0.133 MB/op | [Events/settings](../docs/qa/reports/2026-11-01-performance/events-settings.md); UTF-16 positions preserved |
| Toolset discovery, 32 toolsets / 128 descriptors | 8.853 ms; 128,534 allocations | 2.162 ms; 4,640 allocations | [Events/settings](../docs/qa/reports/2026-11-01-performance/events-settings.md) |
| Unrelated profile read during controlled 20 ms credential I/O | 21.679 ms | 1.458 µs | [Usage/dashboard](../docs/qa/reports/2026-11-01-performance/usage-dashboard.md); isolation metric, not provider latency |

| Frontend workload | Before | After | Evidence |
| --- | ---: | ---: | --- |
| Dashboard, 100 lifecycle events / three observers | 300 reads; 297 cancellations | 6 reads; 0 cancellations | [Usage/dashboard](../docs/qa/reports/2026-11-01-performance/usage-dashboard.md); all caches reach revision 100 |
| Loop, 100 events / 100 ms reads | 100 reads; 99 cancellations | 3 reads; 0 cancellations | [Loop/scheduler](../docs/qa/reports/2026-11-01-performance/loop-loops.md); final revision 100 |
| Task event, 50 cached run-detail/inspect pairs | 100 rereads | 2 rereads | [API/tasks](../docs/qa/reports/2026-11-01-performance/api-tasks.md); unknown owners remain conservative |
| Worktree catalog, 100 lifecycle frames | 400 rereads | 3 rereads | [Worktree/workspace](../docs/qa/reports/2026-11-01-performance/worktree-workspace.md) |
| Worktree detail scope | 8 rereads; one sibling reread | 4 rereads; zero sibling rereads | [Worktree/workspace](../docs/qa/reports/2026-11-01-performance/worktree-workspace.md) |
| Scheduler mutation settlement, each of six cases | 2 status reads; 1 cancellation | 1 status read; 0 cancellations | [Loop/scheduler](../docs/qa/reports/2026-11-01-performance/loop-loops.md) |
| Runtime selector closed, 40 SSR mounts / 2,500 models | 143.728 ms; 100,000 popup rows | 11.440 ms; zero popup rows | [Daemon/runtime](../docs/qa/reports/2026-11-01-performance/daemon-runtime.md); SSR, not browser latency |
| Closed About over 30 seconds | 4 full-status reads | 0 full-status reads | [Session/OS](../docs/qa/reports/2026-11-01-performance/session-os.md); open About retains diagnostics |
| Settings restart, 100 unrelated status updates | 100 React commits | 0 React commits | [Events/settings](../docs/qa/reports/2026-11-01-performance/events-settings.md); relevant update still commits once |
| Terminal pane catalog, 1,000 rotations | 1,000 retained records; 192,817 serialized bytes | 1 record; 230 serialized bytes | [Terminal/attachments](../docs/qa/reports/2026-11-01-performance/terminal-terminal.md); serialized size is not heap |
| Terminal store, 1,000 subsequent presence updates | 20.548 ms | 0.192 ms | [Terminal/attachments](../docs/qa/reports/2026-11-01-performance/terminal-terminal.md) |
| OS fallback selection, 256 windows | 20.660 µs | 5.376 µs | [Session/OS](../docs/qa/reports/2026-11-01-performance/session-os.md); same two owners |

Connection-only status/home consumers also share the existing bounded identity endpoint
instead of enabling full diagnostics. No route or DTO was added. The real-query suite
verifies one shared initial identity read, zero full-status reads and unchanged refresh/error
behavior; diagnostic panels keep the complete status operation.

## Tradeoffs, limits and rejected ideas

- The terminal ring trades immutable read copies for much cheaper writes. Full replay alone
  measured 222.503 → 343.244 µs and 1,048,640 → 2,097,216 B/op. Snapshot allocations are
  unchanged; noisy snapshot timing is not claimed as a win.
- Credential isolation adds one transient allocation in the whole fixture (60 → 61);
  the credential owner still waits for its I/O. Pointer, epoch, identity and publication
  sequence protect newer ownership, including an A-to-B-to-A rotation.
- The package verification cache is capped at 1,024 entries and requires a reliable held-file
  identity/change token plus expected digest. Unsupported token platforms rehash. Installation
  never skips hashing. No authorization, provider credential or configuration TTL was added.
- Attachment retention still sweeps authoritative state and rereads pins between reservation
  passes. Worktree status/activity hydration still has per-row work. These costs are not hidden
  by the filesystem and PR-batching figures.
- Single-toolset timings overlapped host noise and did not improve; all-workspace session
  timings also varied too much for a latency claim. Allocation measures remain separately reported.
- Rejected: query-hash indexing that could violate custom TanStack hashes; a facet CTE and
  additional session catalog reconciliation with no stable benefit; native process timestamp
  replacement without equivalent persisted identity; speculative pools and blanket goroutines.
- No new production goroutine was added. Owners audited existing bounded workers, admission,
  cancellation and join behavior. Batching, scoped projection and releasing mutexes before I/O
  removed measured bottlenecks without introducing new lifecycle owners.

## Independent review

[Scope/SQL review](../docs/qa/reports/2026-11-01-performance/review-scopes.md) compared extension precedence against the previous
resolver in 4,608 independent SQLite combinations. [Cache/Loop review](../docs/qa/reports/2026-11-01-performance/review-cache-loops.md),
[credential/worktree review](../docs/qa/reports/2026-11-01-performance/review-usage-worktree.md), [ring/dashboard review](../docs/qa/reports/2026-11-01-performance/review-ring-dashboard.md)
and the scheduler/session review distinguish source inspection from executed reproductions.
Confirmed findings retain their identifiers and fix dispositions. The dashboard teardown and
credential ABA findings have failing-before tests; final receipts are recorded in their reports.


## Verification status

Real SQLite/Git/filesystem checks, focused race and frontend suites passed. Canonical
make codegen and make codegen-check passed. The initial real browser comparison used
32 active sessions, eight controlled ACP streams and 640 retained sessions, with zero
page errors and successful teardown. Repeated browser comparison and current-head CI
remain in progress at draft creation; use the owning report for final receipts.

Per user authorization, broad gates run in CI rather than occupying the shared host.
This report does not claim local make gate, production telemetry, INP or live-LLM latency.
-->
