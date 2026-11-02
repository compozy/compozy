# Concurrent-session responsiveness audit

Status: implementation, focused validation and independent review complete; repeated
browser comparison and current-head CI are in progress. This is not a production-latency claim.

Branch: `perf/responsiveness-audit-20261101`. Baseline: `de5ab2f4d`.
The worktree is isolated from the primary checkout and production runtime state.

## Scope and evidence

The audit covers the 92 `internal/` package roots and all 28 frontend systems,
with distinct owners and individual findings in [the coverage inventory](coverage.md).
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
| Session Info, 128 model options | 34,512 B/op; 12 allocations | 12,400 B/op; 6 allocations | [Session/OS](session-os.md) |
| Scoped catalog, 1,024 active sessions in 32 workspaces | 7.309 ms; 37.02 MB/op | 0.223 ms; 0.592 MB/op | [Session/OS](session-os.md) |
| Process-demand read, 256 idle active sessions | 0.902 ms; 3,179,008 B/op | 0.00499 ms; 2,304 B/op | [Session/OS](session-os.md); baseline already includes the Info fix |
| Transcript fold retained heap per measured cache | 6,303,472 B | 4,199,638 B | [Store/session](store-session.md) |
| SQLite catalog page without repository filter, 4,096 rows | 8.969 ms | 7.353 ms | [Store/session](store-session.md); redundant PR joins removed |
| Session invalidation CPU, surfaces / usage fixtures | 9.392 / 6.707 ms | 7.652 / 4.153 ms | [Store/session](store-session.md); custom query hashes preserved |
| Scheduler capability sweep, 128 sessions | 9.672 ms; 256 unrelated SQL reads | 0.0387 ms; 0 unrelated reads | [Daemon/runtime](daemon-runtime.md) |
| Scheduler projection, 256 runs / 64 tasks | 27.700 ms; 768 SQL reads | 4.787 ms; 128 SQL reads | [Daemon/runtime](daemon-runtime.md) |
| Scoped observe snapshot, one of 64 tasks with 16 events each | 4.367 ms; 1,024 decoded events | 0.304 ms; 16 decoded events | [Daemon/runtime](daemon-runtime.md); identical event digest |
| Loop complete roster, 10,000 outputs | 141.979 ms; 142.72 MB/op | 8.795 ms; 6.72 MB/op | [Loop/scheduler](loop-loops.md) |
| Scheduler selection, 500 runs and sessions in 50 workspaces | 12.072 ms; 42.89 MB/op | 1.198 ms; 2.27 MB/op | [Loop/scheduler](loop-loops.md); same 500 targets |
| Scheduler selection, single-workspace control | 30.767 ms; 42.89 MB/op | 26.155 ms; 2.20 MB/op | [Loop/scheduler](loop-loops.md) |
| Worktree aggregate, 64 rows / two workspaces | 5.620 ms; 64 workspace + 64 PR reads | 2.682 ms; 2 workspace + 1 PR reads | [Worktree/workspace](worktree-workspace.md) |
| Clean worktree status with real Git | 45.699 ms; three Git subprocesses | 16.169 ms; one Git subprocess | [Worktree/workspace](worktree-workspace.md) |
| Extension tool List + Resolve, 128 extensions | 11.434 ms; 29,549 allocations | 6.206 ms; 15,531 allocations | [Extension/marketplace](extension-marketplace.md) |
| Extension inventory, 128 extensions | 1.990 ms; 128 attachment reads | 0.452 ms; one attachment read | [Extension/marketplace](extension-marketplace.md) |
| Warm package-availability check, 8 MiB archive | 4.725 ms; 36,296 B/op | 0.180 ms; 3,183 B/op | [Extension/marketplace](extension-marketplace.md); installation still hashes |
| Skill frontmatter parse | 18.566 µs; 20,768 B/op | 9.479 µs; 11,136 B/op | [Extension/marketplace](extension-marketplace.md) |
| Terminal ring, 100 writes then full replay | 44.866 ms; 317.29 MB/op | 0.299 ms; 2.23 MB/op | [Terminal/attachments](terminal-terminal.md) |
| Attachment Put, 512 retained objects | 123.970 ms; 6.10 MB/op | 77.965 ms; 3.07 MB/op | [Terminal/attachments](terminal-terminal.md); filesystem fixture, no SQL-pin claim |
| Attachment Open, 4 MiB content | 3.385 ms; two content validations | 1.803 ms; one held-descriptor validation | [Terminal/attachments](terminal-terminal.md) |
| Repeated secret registration, 1,024 secrets | 209.715 µs; 36,912 B/op | 0.0507 µs; 0 B/op | [Session/OS](session-os.md) |
| Slash parser, 128 skills / approximately 0.85 MB prompt | 217.029 ms; 527.54 MB/op | 1.699 ms; 0.133 MB/op | [Events/settings](events-settings.md); UTF-16 positions preserved |
| Toolset discovery, 32 toolsets / 128 descriptors | 8.853 ms; 128,534 allocations | 2.162 ms; 4,640 allocations | [Events/settings](events-settings.md) |
| Unrelated profile read during controlled 20 ms credential I/O | 21.679 ms | 1.458 µs | [Usage/dashboard](usage-dashboard.md); isolation metric, not provider latency |

| Frontend workload | Before | After | Evidence |
| --- | ---: | ---: | --- |
| Dashboard, 100 lifecycle events / three observers | 300 reads; 297 cancellations | 6 reads; 0 cancellations | [Usage/dashboard](usage-dashboard.md); all caches reach revision 100 |
| Loop, 100 events / 100 ms reads | 100 reads; 99 cancellations | 3 reads; 0 cancellations | [Loop/scheduler](loop-loops.md); final revision 100 |
| Task event, 50 cached run-detail/inspect pairs | 100 rereads | 2 rereads | [API/tasks](api-tasks.md); unknown owners remain conservative |
| Worktree catalog, 100 lifecycle frames | 400 rereads | 3 rereads | [Worktree/workspace](worktree-workspace.md) |
| Worktree detail scope | 8 rereads; one sibling reread | 4 rereads; zero sibling rereads | [Worktree/workspace](worktree-workspace.md) |
| Scheduler mutation settlement, each of six cases | 2 status reads; 1 cancellation | 1 status read; 0 cancellations | [Loop/scheduler](loop-loops.md) |
| Runtime selector closed, 40 SSR mounts / 2,500 models | 143.728 ms; 100,000 popup rows | 11.440 ms; zero popup rows | [Daemon/runtime](daemon-runtime.md); SSR, not browser latency |
| Closed About over 30 seconds | 4 full-status reads | 0 full-status reads | [Session/OS](session-os.md); open About retains diagnostics |
| Settings restart, 100 unrelated status updates | 100 React commits | 0 React commits | [Events/settings](events-settings.md); relevant update still commits once |
| Terminal pane catalog, 1,000 rotations | 1,000 retained records; 192,817 serialized bytes | 1 record; 230 serialized bytes | [Terminal/attachments](terminal-terminal.md); serialized size is not heap |
| Terminal store, 1,000 subsequent presence updates | 20.548 ms | 0.192 ms | [Terminal/attachments](terminal-terminal.md) |
| OS fallback selection, 256 windows | 20.660 µs | 5.376 µs | [Session/OS](session-os.md); same two owners |

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

[Scope/SQL review](review-scopes.md) compared extension precedence against the previous
resolver in 4,608 independent SQLite combinations. [Cache/Loop review](review-cache-loops.md),
[credential/worktree review](review-usage-worktree.md), [ring/dashboard review](review-ring-dashboard.md)
[scheduler/session review](review-scheduler-session.md) distinguish source inspection from executed reproductions.
Confirmed findings retain their identifiers and fix dispositions. The dashboard teardown and
credential ABA findings have failing-before tests; final receipts are recorded in their reports.

## Resource limits

Task-owned compilers, tests and benchmarks share one serialized validation slot.
Go uses `GOMAXPROCS=2` and `-p=2`; Turbo uses concurrency 1 and focused Vitest runs
use one worker. Existing unrelated processes and verification jobs are preserved.
Broad integration and platform verification will run in PR CI as authorized by the
user. A draft PR or incomplete CI is work in progress.

Local receipts and the frozen baseline source are under `.cache/performance/`.
The frozen archive is physically outside the repository and linked into that directory,
so generator ownership scans do not see a second source tree. Bun 1.4.0 is explicit for
the final direct benchmarks. Turbo task logs resolved the host Bun 1.4.4 launcher even
when invoked through the pinned launcher; Vitest/typecheck run under Node. Those focused
suite receipts are behavior checks, not pinned-Bun timing evidence. CI uses repository
toolchain pins. No installed global runtime or unrelated process was changed.
Worker reports retain the reproducible commands, fixture sizes, medians/ranges,
correctness invariants, attempted ideas and outcomes needed to assess each change.

## Delivery audit

| Requirement | Authoritative evidence | Current state |
| --- | --- | --- |
| New isolated worktree | Git branch/worktree inventory | Created from `de5ab2f4d` |
| Per-package/system delegated audit | Coverage inventory and ten worker reports | All 92 roots and 28 systems audited; nested/entrypoint coverage in reports |
| Database/query improvements | Real SQLite plans and owning benchmarks/tests | Measured and focused suites passed |
| Frontend loading/state/memory improvements | Real Query observers, component suites and browser runs | Measured and focused suites passed; integrated browser comparison pending |
| Safe concurrency review | Package reports, ownership/cancellation analysis and race tests | No new production goroutines; focused race and independent reviews passed |
| Before/after measures for every retained optimization | Worker reports plus consolidated result table | Recorded above with controls and tradeoffs |
| Real app/integration validation under concurrent sessions | Isolated daemon/browser evidence and current-head CI | Pending |
| Cross-surface impact and QA scenario updates | Change-impact entry and affected scenario files | Updated; integrated receipts pending |
| Detailed PR and requested `.github/PULL_REQUEST_TEMPLATE.md` | Published PR description and tracked template section | Pending |
| Final required CI and clean worktree | Current PR head, CI checks and Git status | Pending |

## Attempts and correctness

The audit favors removing repeated work over introducing concurrency or caches.
Request-local batching must preserve scope, ordering, authorization and freshness.
Stream coalescing must preserve a trailing authoritative read, errors, disconnects,
teardown and profile/workspace fences. Memory changes must preserve immutable
snapshots, replay offsets, UTF-8 and retained-history semantics.

In-progress cross-package edits can temporarily prevent a focused check from
compiling or reaching Vitest. Those attempts remain failed/unrun evidence. They
are rerun after the relevant owner and generated artifacts agree; no tests are
weakened to accommodate a performance change.
