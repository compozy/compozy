# Analysis: orchestration-domain

**Restart baseline:** Fresh analysis performed on 2026-08-01 after the Go skills changed. No conclusion or text from an earlier analysis was reused.

**Slice scope:** All Go production, test, integration-test, and benchmark files directly under `internal/automation/**`, `internal/task/**`, `internal/loop/**`, `internal/scheduler/**`, `internal/observe/**`, `internal/heartbeat/**`, `internal/reasoning/**`, `internal/situation/**`, `internal/soul/**`, `internal/speed/**`, and `internal/events/**`.

**Question:** Under the current `golang-master` doctrine, which modernization and Fowler-style refactorings are real improvements to orchestration/domain state machines while preserving ownership, leases, deterministic ordering, durable audit semantics, and business behavior?

**Corpus:** 587 Go files, 156,286 physical lines, 108 test files, and 14 benchmark functions across 17 direct package groups. Every file in the slice was read. No production file exceeds the 500-line hard cap; several are close enough that the next responsibility must be extracted rather than appended.

**Analysis boundary:** Store implementations, HTTP/UDS/native-tool adapters, session compatibility helpers, `fileutil`, and callers outside the named directories were not inspected. Recommendations that require those owners are explicitly marked as cross-slice work or open questions.

## Overview

The slice does not need a broad rewrite. It already uses many current Go facilities correctly: `slices`, `maps.Clone`/`maps.Copy`, `omitzero`, `strings.SplitSeq`, `os.OpenRoot`, stable comparators, `sync.WaitGroup.Go` in the generic scheduler, injected clocks, typed errors, and small domain packages. The most valuable work is not syntactic. It is moving authoritative decisions to the persistence boundary, making runtime ownership explicit, bounding deliberately detached cleanup contexts, and removing duplicated or legacy policy.

The restart found ten actionable clusters:

| ID | Priority | Finding | Severity | Confidence | Primary Fowler technique |
| --- | --- | --- | --- | --- | --- |
| OD-01 | P0 | Automation fire-limit evaluation and reservation are not atomic | critical | high | Move Function; Combine Functions into Command |
| OD-02 | P0 | Task lease mutations can commit before task reconciliation/audit succeeds | high | high | Split Phase; Move Function |
| OD-03 | P1 | Automation and retention shutdown lose ownership after a timed-out wait | high | high | Replace Primitive with Object; Extract Class |
| OD-04 | P1 | Goal terminal recovery detaches cancellation without any new bound | high | high | Extract Function; Introduce Parameter Object |
| OD-05 | P1 | HEARTBEAT/SOUL duplicate a security-sensitive sidecar filesystem subsystem | high | medium | Extract Module; Move Function |
| OD-06 | P1 | Observe and situation retain explicit legacy compatibility seams | high / medium | high / medium | Remove Dead Code; Inline Function |
| OD-07 | P2 | The coordinator FSM models a fixed straight-line call sequence | medium | high | Inline Class; Remove Middle Man |
| OD-08 | P2 | Event registry and task wake handling are near-cap multi-responsibility growth points | medium | high | Extract Class; Extract Module |
| OD-09 | P3 | Several local idioms can move to current standard-library forms | low | high | Substitute Algorithm |
| OD-10 | P3 | Scheduler sweep failures are logged at two ownership levels | low | high | Remove Dead Code; Consolidate Duplicate Fragments |

The following invariants are load-bearing and constrain every recommendation:

- **Ownership:** actor authority and workspace/session scope checks must occur before mutation. New store commands must carry the same actor and scope data rather than reconstructing it later.
- **Lease fencing:** a raw claim token is returned once, persisted only as a hash, redacted from metadata, and compared in constant time. No modernization may intern, log, copy into an event, or widen the lifetime of a raw token (`internal/task/lease.go:92-214`, `internal/task/lease_manager_claim.go:26-45`).
- **Atomic durable truth:** task/run state, canonical audit events, sequence assignment, and parent-task reconciliation must describe one committed transition. Hooks and wakes are post-commit publication, not part of the transaction (`internal/task/interfaces.go:170-218`, `internal/task/completion_settlement.go:3-16`).
- **Deterministic ordering:** scheduler selection remains priority descending, queue time ascending, and ID ascending; automation list cursors remain source/name/ID ordered. Iterators must not expose map order or replace snapshot materialization where pagination depends on a stable collection (`internal/scheduler/selection.go:156-180`, `internal/automation/model/list_semantics.go:9-35`, `internal/automation/model/list.go:332-360`).
- **Reservation identity:** scheduled runs retain their durable ID, retry attempt, participation snapshot, fire-limit retry time, and schedule-specific cancellation behavior (`internal/automation/dispatch_reservation.go:128-165`).
- **Cancellation semantics:** terminal ambiguity recovery must outlive caller cancellation long enough to write durable truth, but it must not become unbounded. The desired change is `WithoutCancel` plus a new bound, not restoration of the canceled caller context.
- **Observable FSM behavior:** `internal/loop/watch` emits transitions as part of its result contract and has real waiting/ready/stalled branches; it is intentionally retained. Only the private, linear coordinator phase wrapper is a removal candidate (`internal/loop/watch/types.go:36-65`, `internal/loop/watch/fsm.go:19-56`).

Recommended execution order is OD-01 and OD-02 first, then OD-03 and OD-04, then OD-05 and OD-06, and finally the structure/idiom work. This sequencing keeps mechanical churn out of the highest-risk transactional diffs.

## Mechanisms / Patterns

### OD-01 — Make fire-limit reservation one authoritative store command

`Dispatcher.evaluateFireLimit` builds a rolling-window query, takes `fireLimitMu`, performs `ListRuns`, computes the count/retry time, and releases the mutex on return (`internal/automation/dispatch_reservation.go:168-231`). A new run is inserted only afterward in `reserveRun` (`internal/automation/dispatch_reservation.go:97-124`). Therefore two concurrent dispatches can both observe `count < max` and both insert. The mutex also protects only one `Dispatcher` instance and is held across store I/O, so it simultaneously violates the no-lock-across-I/O rule and fails to protect cross-instance execution.

The scheduled/reserved branch has the analogous check-to-use gap: it excludes its own durable ID, evaluates the limit, and returns the reservation for a later transition (`internal/automation/dispatch_reservation.go:128-159`). Existing tests prove sequential persistence across dispatcher recreation and special handling for scheduled reservations, but do not prove a concurrent limit (`internal/automation/dispatch_test.go:819-950`).

**Refactoring:** apply **Move Function** and **Combine Functions into Command**. Replace `evaluateFireLimit` plus later insert/activation with a store-owned operation such as:

```go
type DispatchReservation struct {
    Candidate    Run
    ExistingID   string
    WindowStart  time.Time
    WindowEnd    time.Time
    Limit        int
    Kind         DispatchKind
}

reserved, err := d.runs.ReserveDispatch(ctx, reservation)
```

The implementation must count qualifying rows and either insert a new run or compare-and-set the existing reservation in one SQLite transaction. It must return the authoritative count and earliest retry time on rejection. `countsTowardFireLimit` semantics (`status != cancelled`), `ExcludeID`, job-versus-trigger identity, and schedule/webhook failure status remain explicit inputs. Delete `Dispatcher.fireLimitMu` after all stores implement the command.

**Required test placement:** the invariant is “at most `Max` counted fires commit inside a rolling window, including concurrent callers and independent dispatcher objects.” The owning layer is the durable automation run store; its canonical real-database integration suite must prove the transaction. Extend `internal/automation/dispatch_integration_test.go` for service-level concurrent dispatch and retain `internal/automation/dispatch_test.go` for error shape, retry time, reserved-run ID, and status semantics. Do not duplicate the transaction invariant in both suites.

### OD-02 — Convert lease state changes into settlement commands

`ClaimNextRun` commits the raw run claim first, strips raw-token metadata, then reconciles the task, records `task.run_claimed`, dispatches post-claim hooks, and binds network participation (`internal/task/lease_manager_claim.go:26-52`). Failure during reconciliation or event recording returns an error after the lease is already held, without returning the raw token and without releasing the claim. Network binding has a bounded release cleanup, but it runs only when the later bind step fails (`internal/task/lease_network_binding.go:16-60`).

Heartbeat and release have the same split-durable-truth shape. Heartbeat extends the lease before loading the task and recording the extension event (`internal/task/lease_manager.go:11-46`). Release clears ownership before reconciliation and event recording (`internal/task/lease_manager.go:50-94`). Existing failure tests intentionally prove network restoration after the lease mutation, which confirms that post-mutation failures are part of the supported path rather than an unreachable edge (`internal/task/lease_test.go:613-723`). By contrast, completion already delegates the authoritative run/task settlement to `CompleteRunLeaseSettlement` and only publishes after success (`internal/task/lease_manager.go:172-195`).

**Refactoring:** use the existing completion pattern as the local template. Apply **Split Phase** and **Move Function**:

```go
settlement, err := m.store.ClaimNextRunSettlement(ctx, command)
if err != nil {
    return nil, err
}

// Post-commit only: hooks, wakes, and external network binding.
m.publishClaimSettlement(context.WithoutCancel(ctx), settlement, actor)
```

The transaction should claim/heartbeat/release the run, reconcile affected task status, append canonical event rows with sequence values, and return a settlement object. External hooks remain best-effort post-commit. Network binding remains outside SQLite; if it fails, invoke an equally atomic release settlement. The returned claim retains its one-time raw token, while the settlement/event payload exposes only `ClaimTokenHash`.

This is preferable to adding ad hoc compensating releases at every service return: compensation can fail, obscures whether the first transition committed, and still creates a time window where observers see a lease without its audit event.

**Required test placement:** the invariant is “a committed lease transition and its canonical task/event projection are visible together, or none is visible.” The persistence integration suite owns rollback/commit and event-sequence assertions. Extend `internal/task/lease_test.go` only for service behavior—raw-token redaction, network bind/release cleanup, actor fencing, and post-commit hook behavior—and reuse `internal/task/hooks_integration_test.go` for “hooks observe committed audit.” Do not add a standalone test that re-proves the same transaction.

### OD-03 — Represent owned runtimes with cancel-and-done handles

Automation `Stop` marks the scheduler stopped before waiting, creates an untracked waiter goroutine around `wg.Wait`, and clears registrations/runtime handles even when the caller deadline wins (`internal/automation/schedule_lifecycle.go:42-91`). A second `Stop` then returns success at lines 48-50 although the original worker may still be alive. The worker itself still uses manual `Add`/`Done` (`internal/automation/schedule_execution.go:67-75`). One-way `Start`-after-stop behavior is already tested and should remain unchanged (`internal/automation/schedule_test.go:979-1000`).

Observe retention clears `retentionCancel` before waiting and creates the same untracked waiter goroutine (`internal/observe/retention.go:100-130`). A timeout makes later shutdown calls return success without waiting. A nil shutdown context is silently converted to `context.Background`, permitting an unbounded shutdown. Natural termination of `runRetentionLoop` also does not clear the running marker (`internal/observe/retention.go:182-197`).

The generic scheduler provides the correct in-slice pattern: it stores `{cancel, done}`, starts via `wg.Go`, leaves the handle intact on a shutdown deadline, and finalizes state only after `done` closes (`internal/scheduler/scheduler.go:115-209`). Its canonical lifecycle suite explicitly proves retry after the first shutdown deadline (`internal/scheduler/scheduler_lifecycle_test.go:15-124`).

**Refactoring:** apply **Replace Primitive with Object** and **Extract Class** around a small runtime handle:

```go
type runtimeHandle struct {
    cancel context.CancelFunc
    done   <-chan struct{}
}
```

The owned worker closes `done` in its own deferred completion path; shutdown never spawns a waiter. A timed-out call returns the context error and retains the handle so a later call can wait again. Use `WaitGroup.Go` where the wait group remains useful, reject nil public contexts, and keep automation's explicit-stop ownership distinct from scheduler's start-context ownership.

**Required test placement:** automation lifecycle belongs in `internal/automation/schedule_test.go`; add a subtest matching the generic scheduler's retryable-timeout invariant. Observe retention lifecycle belongs in `internal/observe/observer_test.go`; add deterministic start/tick/cancel/retry coverage with `testing/synctest`, because retention uses a real `time.Ticker`. Existing sweep-content tests remain the canonical place for retention data semantics.

### OD-04 — Bound detached goal recovery once and thread the context

`recoverAwaitFailure` calls `context.WithoutCancel(ctx)` independently for checkpoint load, terminal reconciliation, result application, ambiguity marking, reload, and recovered control (`internal/loop/goal/recovery.go:179-233`). Caller cancellation is intentionally ignored so ambiguous terminal state can be persisted, but the detached contexts have neither a deadline nor a shared cancellation cause. Any blocked store/recovery call can hold goal termination indefinitely.

There is already a correct local pattern in compaction cleanup: detach once, immediately add `WithTimeout`, defer cancel, then perform the cancel/drain sequence through that bounded context (`internal/loop/goal/compaction.go:177-188`).

**Refactoring:** apply **Extract Function** and **Introduce Parameter Object**. Construct one recovery context at the start of `recoverAwaitFailure` using `context.WithTimeoutCause(context.WithoutCancel(ctx), recoveryTimeout, errRecoveryTimeout)` and thread it through all recovery operations. Keep joining the original await error with recovery failures. A dependency/config option should own the timeout; do not scatter a literal across calls.

**Required test placement:** the invariant is “caller cancellation does not prevent ambiguity persistence, but a blocked recovery dependency cannot outlive the configured recovery bound.” The owning layer is the goal executor; extend `internal/loop/goal/executor_test.go`, using its existing fake store/recovery support. `testing/synctest` is appropriate if the test exercises the timeout; no wall-clock sleep should be introduced.

### OD-05 — Extract one rooted managed-sidecar subsystem

HEARTBEAT and SOUL independently implement nearly the same path normalization, workspace containment, symlink rejection, managed-component traversal, agent-name checks, authoring target resolution, atomic write/remove, revision persistence, rollback, and history purge. The duplicated path algorithms are visible in `internal/heartbeat/source_path.go:18-125` versus `internal/soul/soul_path.go:15-117`, and in `internal/heartbeat/authoring_path.go:13-146` versus `internal/soul/authoring_path.go:13-133`. Their purge helpers differ primarily by sidecar name (`internal/heartbeat/history_purge.go:13-75`, `internal/soul/history_purge.go:13-79`).

This is security-sensitive shotgun surgery: a containment or macOS-path fix must be made twice and kept byte-for-byte semantically aligned. In addition, validation uses absolute paths, `EvalSymlinks`, and `Lstat`, but the later atomic write/remove receives an ordinary absolute path (`internal/heartbeat/authoring.go:248-309`, `internal/soul/authoring.go:249-318`). The validation-to-use window is an inferred TOCTOU risk; the `fileutil` implementation was outside this slice and must be inspected before classifying exploitability.

**Refactoring:** apply **Extract Module** and **Move Function** to a narrow managed-sidecar package. It should own:

- a root-relative managed path value, never an unchecked absolute string;
- workspace containment and the current strict “no symlink component” diagnostic contract;
- root-relative atomic write/remove primitives;
- generic authored-sidecar revision path derivation and rollback scaffolding.

Use `os.OpenRoot` as the execution authority, following the existing loop source-store precedent (`internal/loop/source_store.go:164-183`). Retain the explicit no-symlink validation even if `os.Root` prevents escape: rejecting all managed-path symlinks is a stricter product rule than merely preventing traversal outside the root. Domain-specific parsing, diagnostics codes/copy, schedule validation, and provenance remain in `heartbeat` and `soul`; do not create a generic “agent document” god package.

**Required test placement:** path/security invariants remain in the existing authoring suites: `internal/heartbeat/authoring_status_test.go`, `internal/soul/authoring_test.go`, and their internal-package tests. Convert both suites to a shared contract fixture only if each still asserts its domain-specific diagnostic identity. Add no file-existence-only test; test traversal, symlink swap resistance, digest fencing, atomic rollback, and revision ownership behavior.

### OD-06 — Delete explicit legacy compatibility

Observe reconciliation calls `session.RepairLegacyProvider`, mutates legacy session metadata, and silently skips unrecoverable legacy providers (`internal/observe/reconcile.go:129-145`). A large test block exists solely for repair/skip behavior (`internal/observe/reconcile_test.go:396-590`). This conflicts directly with the repository's greenfield, zero-legacy policy.

Situation exposes `PromptSection` with an explicit comment that it implements a “legacy workspace-scoped prompt provider seam,” and its only in-slice test calls it directly (`internal/situation/service_context.go:137-153`, `internal/situation/service_test.go:1260-1273`). Consumers outside the slice were not inspected, so deletion of this method needs a repository-wide reference audit first.

**Refactoring:** apply **Remove Dead Code**. Delete observe repair/skip code and legacy-only fixtures; ordinary invalid-state handling should remain explicit and non-mutating. Delete `PromptSection` and its test after callers move to the current situation/context API. Do not add aliases, fallback branches, or adapter shims. The session helper itself is an outside-slice delete target.

### OD-07 — Inline the private linear coordinator FSM; retain watch FSM

The coordinator machine defines exactly four one-way transitions—hydrated → derived → evaluated → assembled → yielded (`internal/loop/coordinator_fsm.go:12-60`). `Coordinator.Tick` invokes them in that fixed order between already-ordered phase functions (`internal/loop/coordinator.go:149-167`). There is no branch, persisted state, retry transition, or external transition result; the FSM adds runtime strings and failure paths without expressing a business state machine.

Apply **Inline Class** and **Remove Middle Man**: remove `coordinatorMachine`, call the phase functions directly, and keep structured phase logging if it is operationally useful. The looplab FSM dependency cannot necessarily be removed because `internal/loop/watch` uses it for real branching and returns transition history (`internal/loop/watch/adapter.go:52-112`).

The canonical verification is existing coordinator behavior in `internal/loop/coordinator_test.go` and integration suites. Do not create tests for private phase strings. `internal/loop/watch/adapter_test.go` remains unchanged and protects the genuinely observable FSM.

### OD-08 — Split growth points before they cross the production cap

No production file exceeds 500 lines, but two concrete files already combine unrelated change reasons:

- `internal/events/registry.go` is 449 lines and contains domain types, outcome/component constants, the base catalog, public validation, and registry construction (`internal/events/registry.go:11-449`). Component-specific files already exist, but composition is a nested append chain (`internal/events/registry_mcp_auth.go:3-15`). Apply **Extract Module**: keep contract types/validation separate from per-component entry catalogs and compose them once through a clear builder. Preserve duplicate-name, outcome, and component validation.
- `internal/task/wake.go` is 473 lines and combines public wake contracts, validation, delivery, in-memory dedupe/cache eviction, durable audit lookup/recording, and user-facing summary construction (`internal/task/wake.go:13-473`). Apply **Extract Class** and **Split Phase** into contract, dispatch, dedupe reservation, audit, and summary files. Keep the cache lock boundary and durable event dedupe ordering intact.

`internal/task/lease.go` at 407 lines also mixes public lease commands/results, cryptographic token operations, redaction, normalization, and validation; split it opportunistically when OD-02 changes the contract. Other near-cap files (`task/manager_task_crud.go`, `task/coordinator.go`, `loop/linter.go`, `loop/goal/route.go`, `automation/model/validate.go`, `scheduler/scheduler.go`, and `soul/authoring.go`) must not grow, but this analysis found no justified standalone reorganization for them.

These are structural refactors. Existing behavioral suites and compile/boundary gates own correctness; do not add tests that assert filenames, line counts, or internal organization.

### OD-09 — Apply small modernizations only in touched code

The safe mechanical batch is:

- replace five production `errors.As` target-variable patterns with `errors.AsType` (`internal/loop/input_defaults.go:63`, `internal/loop/runtime_types.go:146`, `internal/loop/goal/executor.go:215`, `internal/loop/goal/route.go:68`, `internal/task/autonomy.go:51`);
- replace the remaining benchmark `for range b.N` with `for b.Loop()` (`internal/loop/coordinator_watch_test.go:806-840`);
- use `WaitGroup.Go` while implementing OD-03 (`internal/automation/schedule_execution.go:67-75`, `internal/observe/retention.go:91-96`);
- replace `sync.Once` plus a single close closure with `sync.OnceFunc` in the live task stream (`internal/task/live.go:31-57`);
- replace hand-built sorted key slices with `slices.Sorted(maps.Keys(m))` where sorted materialization is already required (`internal/heartbeat/source_path.go:218-224`, `internal/loop/resource_spec_precedence.go:71-77`, `internal/loop/resource_spec_projection.go:112-118`);
- replace `int(right.Attempt-left.Attempt)` with `cmp.Compare(right.Attempt, left.Attempt)` to avoid subtraction-based comparator overflow (`internal/situation/task_context_redaction.go:35-38`);
- optionally use range-over-integer syntax in the three remaining straightforward counter loops only when it improves readability (`internal/heartbeat/source_path.go:101-104`, `internal/soul/soul_path.go:93-96`, `internal/loop/control_plan.go:304`).

Do not turn this into a repository-wide style diff. Apply each change when its owning file is already touched, except the benchmark and comparator, which are isolated enough for a small mechanical batch.

### OD-10 — Log scheduler sweep failures once

Both scheduler sweep helpers record metrics, log a warning, and return a wrapped error (`internal/scheduler/scheduler.go:350-375`). The owning loop then logs the joined cycle error again (`internal/scheduler/scheduler.go:406-419`). Preserve the metric updates but remove the lower-level warning, letting `RunOnce` callers decide how to report returned failures. The background loop remains the single logging owner. This avoids duplicate messages without losing context because the helper already wraps the operation name.

Also replace `slog.String("error", err.Error())` in retention with an error-valued attribute when OD-03 touches the file (`internal/observe/retention.go:200-219`), preserving the error object for handlers.

## Relevant Sources

### Complete package coverage

Counts are direct-package counts, so nested packages appear separately and the total does not double-count files. LOC includes tests. “Benchmarks” counts benchmark functions, which are a subset of test files.

| Package group | Go files | LOC | Test files | Benchmarks | Fresh-review disposition | Representative evidence |
| --- | ---: | ---: | ---: | ---: | --- | --- |
| `internal/automation` | 86 | 22,755 | 19 | 4 | OD-01 fire-limit transaction; OD-03 scheduler lifecycle; otherwise preserve reservation/retry semantics | `dispatch_reservation.go:97-235`; `schedule_lifecycle.go:12-96` |
| `internal/automation/model` | 15 | 3,139 | 4 | 1 | Stable list ordering/cursors are intentional materialization; no iterator API rewrite | `list_semantics.go:9-35`; `list.go:213-360` |
| `internal/task` | 145 | 45,896 | 18 | 4 | OD-02 lease settlements; OD-08 wake/lease decomposition; retain actor/token fencing | `lease_manager_claim.go:5-52`; `wake.go:13-473` |
| `internal/loop` | 142 | 32,885 | 25 | 1 | OD-07 inline only private coordinator FSM; OD-09 benchmark/API cleanup | `coordinator_fsm.go:12-95`; `coordinator_watch_test.go:806-840` |
| `internal/loop/dsl` | 12 | 1,472 | 2 | 0 | Current value-object layout and `omitzero` use are appropriate | `runtime.go:56-57`; `gate_start.go:10` |
| `internal/loop/dsl/refs` | 4 | 1,384 | 1 | 0 | Buffer is template rendering, not framing; no `Buffer.Peek` opportunity | `template.go:30-40` |
| `internal/loop/gate` | 11 | 3,366 | 2 | 0 | Buffer use is rendering/compaction; clone helpers already use `maps`/`slices` | `judge.go:116-124,282-289`; `clone.go:52-60` |
| `internal/loop/goal` | 27 | 7,485 | 2 | 0 | OD-04 bounded recovery; existing compaction cleanup is the template | `recovery.go:179-269`; `compaction.go:177-188` |
| `internal/loop/watch` | 5 | 693 | 1 | 0 | Retain: observable branching FSM with explicit transition contract | `fsm.go:19-56`; `adapter.go:52-112` |
| `internal/scheduler` | 12 | 4,940 | 5 | 0 | Lifecycle is the positive runtime-handle exemplar; OD-10 single error owner | `scheduler.go:115-209,350-419`; `selection.go:156-180` |
| `internal/observe` | 44 | 13,010 | 13 | 4 | OD-03 retention lifecycle/synctest; OD-06 remove legacy repair | `retention.go:82-131,182-219`; `reconcile.go:129-145` |
| `internal/heartbeat` | 28 | 8,572 | 4 | 0 | OD-05 shared rooted sidecar subsystem | `source_path.go:18-125`; `authoring_path.go:13-146` |
| `internal/reasoning` | 2 | 122 | 1 | 0 | Small vocabulary package; no justified modernization | `reasoning.go:1-74`; `reasoning_test.go:1-48` |
| `internal/situation` | 22 | 4,912 | 4 | 0 | OD-06 legacy prompt seam; OD-09 safe comparator | `service_context.go:137-153`; `task_context_redaction.go:31-38` |
| `internal/soul` | 19 | 4,511 | 5 | 0 | OD-05 shared rooted sidecar subsystem | `soul_path.go:15-117`; `authoring_path.go:13-133` |
| `internal/speed` | 2 | 189 | 1 | 0 | Small bounded policy package; no justified change | `speed.go:1-119`; `speed_test.go:1-70` |
| `internal/events` | 11 | 955 | 1 | 0 | OD-08 split catalog growth point; do not intern static names | `registry.go:11-449`; `registry_mcp_auth.go:3-15` |
| **Total** | **587** | **156,286** | **108** | **14** | Full slice covered | — |

### Current-Go feature decision matrix

The table has exactly the 20 features requested by the updated modernization review. Status values are deliberately limited to `adopt`, `already`, `reject`, `defer`, and `not applicable`.

| # | Feature | Status | Evidence and scoped decision |
| ---: | --- | --- | --- |
| 1 | `errors.AsType[T]` | adopt | Four production sites already use it (`loop/linter.go:424`, `automation/schedule_execution.go:141`, `task/lease_manager.go:188`, `task/coordinator_failure.go:95`); convert the five target-variable sites listed in OD-09. Keep interface-target extraction where it is valid. |
| 2 | `b.Loop()` | adopt | Thirteen benchmarks already use it (`automation/perf_bench_test.go:63-138`, `task/perf_bench_test.go:15-56`, `observe/perf_bench_test.go:36-65`, `automation/model/template_bench_test.go:24`); convert the one remaining `b.N` benchmark at `loop/coordinator_watch_test.go:840`. |
| 3 | JSON `omitzero` | already | Value-time fields already use `omitzero` (`task/types_identity.go:26-28`, `task/scheduler_controls.go:73-74`, `loop/dsl/runtime.go:56-57`). Do not blanket-replace `omitempty` on pointers, maps, strings, or fields whose wire absence semantics differ. |
| 4 | `os.OpenRoot` / `os.Root` | adopt | Loop source copying already uses rooted operations (`loop/source_store.go:164-183`). Extend the pattern to HEARTBEAT/SOUL execution after extracting a root-aware atomic writer; retain their stricter no-symlink diagnostics (`heartbeat/authoring_path.go:49-131`, `soul/authoring_path.go:43-121`). |
| 5 | `strings.SplitSeq` / string sequences | already | `SplitSeq` is used where parts stream naturally (`loop/action_channel_result_content_rule.go:59,99`). Remaining `Split` sites either need indexing/joining or exact diagnostic line numbers (`heartbeat/source_path.go:101-108,228-239`, `soul/soul_fields.go:89-100`); changing them without profile data adds counters/complexity for small documents. |
| 6 | `sync.WaitGroup.Go` | adopt | Generic scheduler already demonstrates it (`scheduler/scheduler.go:137-144`). Use it in automation and retention while fixing lifecycle ownership (`automation/schedule_execution.go:70-75`, `observe/retention.go:93-96`). |
| 7 | range over integers | adopt | Existing tests already use the syntax (`automation/model/list_test.go:51`, `task/lease_test.go:272`). Convert only clear production counting loops such as the duplicated sidecar path scan and control-plan branch allocation; indexes are still needed, so this is readability-only. |
| 8 | `slices` / `maps` / `min` / `max` | adopt | The slice is already broadly modern (`scheduler/selection.go:157-179`, `observe/bridges.go:276-285`, `loop/runtime_config.go:31-40`). Finish sorted-map-key collectors with `slices.Sorted(maps.Keys(m))`; do not change ordering. |
| 9 | `testing/synctest` | adopt | Retention owns a real ticker (`observe/retention.go:182-197`) but lacks lifecycle timing coverage; add it to `observe/observer_test.go`. Consider it for goal recovery timeout. Do not force DB/external-goroutine integration tests into a synctest bubble that cannot quiesce. |
| 10 | `iter.Seq` and iterator pipelines | adopt | Use indirectly and locally for map-key pipelines (`maps.Keys` → `slices.Sorted`). Reject exported iterator rewrites of automation/task store lists because snapshots, errors, pagination, and deterministic order are part of those APIs (`automation/model/list.go:213-360`). |
| 11 | `os.Process.WithHandle` | not applicable | No process creation, signaling, killing, or handle ownership occurs in the 587-file slice. Process lifecycle belongs to runtime/session packages outside this scope. |
| 12 | `sync.OnceFunc` / `OnceValue` | adopt | `task/live.go:31-57` has exactly the `sync.Once` + close closure shape for `OnceFunc`. The remaining `sync.Once` occurrences are test coordination (`scheduler/scheduler_lifecycle_test.go:141-142`); no `OnceValue` candidate exists. |
| 13 | `math/rand/v2` | reject | The scoped random operations are security/identity generation and correctly use `crypto/rand` (`task/lease.go:178-184`, `heartbeat/wake_helpers.go:114`). Replacing either with `math/rand/v2` would weaken the contract. |
| 14 | `cmp.Or` | defer | No clear fallback chain has “first non-zero” semantics independent of domain validation. Zero enum/time/ID values often mean invalid, absent, or global; introducing `cmp.Or` could collapse those meanings. Separately adopt `cmp.Compare` for the unsafe attempt comparator. |
| 15 | `T.ArtifactDir`, `T.Attr`, `T.Output` | not applicable | These package tests do not own durable QA artifact production. Diagnostics use ordinary assertions/log buffers; repository QA tooling outside the slice owns evidence paths and metadata. |
| 16 | `http.CrossOriginProtection` | not applicable | No HTTP handler or browser-origin boundary exists in this domain/orchestration slice. Adoption belongs to HTTP adapter packages after their own route audit. |
| 17 | execution tracer `FlightRecorder` | not applicable | No daemon diagnostics bootstrap or trace lifecycle is owned here. Adding a recorder to a domain service would invert ownership and create a new long-lived runtime. |
| 18 | typed `net.Dialer.Dial` APIs | not applicable | No socket dialing is performed in the scoped packages. Network participation here is a domain contract delegated through interfaces. |
| 19 | `bytes.Buffer.Peek` | not applicable | Buffers only render templates/prompts or collect test logs (`loop/dsl/refs/template.go:30`, `loop/gate/judge.go:116,282`); there is no framed protocol parser that needs non-consuming lookahead. |
| 20 | `unique` string interning | reject | Dynamic workspace/session/task/run IDs are high-cardinality and may be sensitive; interning would retain them for process lifetime without profile evidence. Event names are a small static registry (`events/registry.go:31-203`) and gain nothing material from interning. Never intern raw claim tokens. |

## Transferable Patterns

1. **Put cross-observer invariants in the authoritative transaction.** A process mutex can coordinate local work, but it cannot make “read policy, then write state” atomic. OD-01 and OD-02 should return settlement values from the store, with state and audit already committed. The service layer then publishes hooks, wakes, and external network effects.

2. **Use settlement objects instead of compensating service choreography.** `CompletedRunSettlement` is the in-slice proof that a store can return the run, reconciled task, status transitions, and rollups from one transaction (`internal/task/completion_settlement.go:3-16`). Extend that concept to claims, heartbeats, and releases rather than proliferating cleanup branches.

3. **An owned goroutine needs a durable completion signal.** The scheduler's `{runtimeCancel, runtimeDone, stopping, stopped}` protocol survives a timed-out shutdown and makes natural exit converge on the same finalizer (`internal/scheduler/scheduler.go:115-209`). Reuse the protocol, not necessarily the exact fields, for automation and retention.

4. **Detached cleanup must immediately acquire a new bound.** `context.WithoutCancel` is appropriate for durable reconciliation only when followed by a timeout/cause owned by the cleanup operation. `cancelAndDrainCompaction` is the canonical local example (`internal/loop/goal/compaction.go:177-188`).

5. **Separate deterministic materialization from pull iteration.** Internal set-to-sorted-slice conversions benefit from `maps.Keys` and `slices.Sorted`; cursor pages, scheduler snapshots, and externally returned lists benefit from explicit materialization. Iterator adoption is an implementation detail, never permission to weaken ordering or error lifetimes.

6. **Treat filesystem roots as capabilities, not strings.** Path validation produces good domain diagnostics; rooted operations enforce the authority at use time. The shared sidecar module should accept a root-relative managed path and keep the `os.Root` alive for the operation. It should not hand an absolute path back to domain code for later use.

7. **Share infrastructure, not domain vocabulary.** HEARTBEAT and SOUL can share containment, atomic mutation, rollback, and revision mechanics while retaining their own parsers, diagnostics, allowed fields, schedules, and product copy. A shared module that accepts arbitrary callbacks for every difference would merely hide duplication.

8. **Keep genuine state machines and remove ceremonial ones.** Watch's branching transitions are observable behavior. Coordinator's private straight line is ordinary control flow. The decision criterion is not dependency preference; it is whether the machine constrains legal state transitions that callers or recovery logic can observe.

9. **One error, one reporting owner.** Lower layers wrap and return; the background owner logs. Metrics can be updated where the failure is classified. This preserves both structured cause chains and signal-to-noise.

10. **Modernize after semantic seams are stable.** `AsType`, `WaitGroup.Go`, `OnceFunc`, `cmp.Compare`, range integers, and iterator pipelines are useful, but mixing them into the initial transactional changes would make review harder. Apply them per owning file after the higher-risk command boundaries are established.

Suggested implementation waves:

| Wave | Scope | Exit evidence |
| --- | --- | --- |
| 1 | OD-01 automation reservation command; OD-02 task lease settlements | Concurrent real-store tests; failure injection proves all-or-nothing state/event visibility; existing reservation/token/hook behavior passes |
| 2 | OD-03 runtime handles; OD-04 bounded recovery | Retryable shutdown tests, no orphan waiter goroutines, deterministic ticker/timeout tests |
| 3 | OD-05 rooted sidecar module; OD-06 hard legacy deletion | Existing traversal/symlink/digest/rollback suites; repository-wide consumer/delete-target audit |
| 4 | OD-07/08 structural extraction; OD-09/10 idioms and logging | Existing behavior suites and repository gates; no new structure-only tests |

## Risks / Mismatches

| Risk | Why a plausible refactor can be wrong | Required guard |
| --- | --- | --- |
| Fire-limit off-by-one or wrong population | Scheduled reservations exclude their own ID; canceled runs do not count; retry time derives from the earliest counted `StartedAt` | Transaction command must take `ExcludeID`, definition identity, window bounds, counted statuses, and dispatch kind explicitly; preserve `FireLimitError` fields |
| SQLite transaction contention | Moving the count into the write transaction may increase lock duration | Query through an indexed definition/time/status path; measure the real store; do not restore a process mutex as a workaround |
| Lease token exposure | Settlement objects can accidentally become generic JSON/hook payloads | Return the raw token only on the direct claim result; settlements/events carry hashes; preserve constant-time verification and recursive redaction |
| Hook timing regression | Moving service steps can fire hooks before commit or make hook failure roll back committed state | Publish only from committed settlement results; reuse commit-observer behavior and existing hook integration suite |
| Network participation mismatch | SQLite cannot atomically bind an external session network | Commit claim settlement, bind post-commit, and on bind failure use atomic release settlement with bounded cleanup; preserve participation snapshot identity |
| Shutdown state regression | Clearing state early looks convenient but makes a timed-out stop non-retryable | Keep runtime handle until worker closes `done`; one-way automation restart prohibition remains independent of drain completion |
| Detached recovery loses durable ambiguity | Replacing `WithoutCancel` with caller context would abandon recovery exactly when needed | Detach once, add a separate deadline/cause, and prove ambiguity persists after caller cancel |
| Root adoption weakens symlink policy | `os.Root` escape protection is not necessarily the same as “no symlinks anywhere” | Retain explicit component diagnostics; use rooted I/O as an additional use-time authority, not a replacement for product validation |
| Shared sidecar module becomes a god package | HEARTBEAT and SOUL have distinct parsing, schedules, diagnostic codes, and copy | Extract only path/mutation/revision mechanics; keep domain policies in their packages |
| Iterator changes reorder data | Map iteration is nondeterministic; lazy errors can move outside the current call boundary | Collect and sort before any ordered consumer; leave pagination/store APIs materialized |
| Legacy deletion breaks unseen callers | `PromptSection` and `RepairLegacyProvider` implementations/callers extend beyond this slice | Perform repository-wide reference audit and list hard delete targets; do not retain aliases after callers migrate |
| Structural tests freeze implementation | File splits are architecture, not product contracts | Reuse behavior suites and build/boundary gates; add no filename/line-count/config snapshot tests |
| Overusing `unique` retains secrets/IDs | Interned values live for the process and high-cardinality identities are unbounded | Reject without allocation/profile evidence; categorically exclude bearer tokens and user-controlled content |

Implementation planning also needs the repository's cross-surface impact audit; this slice alone cannot truthfully declare all surfaces unaffected:

- **Native tools:** OD-01 and OD-02 intend no tool ID/schema change, but automation dispatch and task lease tool/CLI/API adapters must be checked after store command design. Any changed error/retry result is a public-contract change.
- **Extensibility and hooks:** task post-claim/lease hooks, automation pre/post-fire hooks, wakes, event commit observers, network participation binding, and config lifecycle for a recovery timeout are directly affected. Preserve post-commit visibility and existing capability gates.
- **Workspace data isolation:** new commands must propagate current global/workspace scope, workspace ID, task/run/session identity, and actor through store predicates and returned settlements. Concurrency tests need two workspaces to prove no cross-workspace count/claim/event leakage where the underlying store keys permit it.
- **Official Compozy skill:** no immediate update is justified for behavior-preserving internals. If public failure semantics, retry timing, CLI/native-tool output, config keys, or lifecycle behavior change, the bundled skill and docs become mandatory co-ship surfaces.

Several attractive rewrites are explicitly rejected:

- Do not replace `crypto/rand` with `math/rand/v2`.
- Do not intern IDs, event payload strings, or claim tokens.
- Do not convert ordered list/store APIs to lazy iterators.
- Do not remove the watch FSM merely because the coordinator FSM is inlined.
- Do not blanket-convert `omitempty` to `omitzero`.
- Do not use `strings.Lines` where retained newline/final-empty semantics could alter diagnostic line calculations.
- Do not introduce generic helpers whose type parameters hide a single concrete domain operation.
- Do not add compatibility aliases while deleting legacy seams.

## Open Questions

1. **Automation store ownership:** Which concrete run-store implementations must implement `ReserveDispatch`, and can every implementation perform the rolling count plus insert/compare-and-set in one transaction? Their files were outside the slice.
2. **Task settlement scope:** Can the task store atomically assign event sequence numbers and notify its post-commit observer for claim, heartbeat, and release as it does for other command settlements, or is an interface/store-schema change required?
3. **Recovery bound ownership:** Should goal terminal-recovery timeout reuse an existing internal drain bound, gain a dependency option, or become a documented `config.toml` key? The choice determines config, docs, and agent-manageability impact.
4. **Root-aware atomic I/O:** Does `fileutil.AtomicWriteFile`/`AtomicRemoveFile` already close directory-swap races internally, and can it accept an `*os.Root` plus relative path without weakening atomic rename or permission behavior?
5. **Legacy prompt consumer:** Are there any callers of `situation.Service.PromptSection` outside the scoped packages? If so, which current situation/context API should replace them before the hard deletion?
6. **Legacy session state policy:** After deleting `session.RepairLegacyProvider`, should invalid pre-Goose session metadata fail reconciliation, be ignored as ordinary corrupt state, or be deleted by a separate explicit operator action? It must not be silently repaired.
7. **FSM dependency:** Is looplab/fsm used outside `internal/loop/watch` and the removable coordinator wrapper? The module dependency can be deleted only after a repository-wide usage audit.
8. **Multi-workspace fire limits:** Are job and trigger IDs globally unique in every store, or must the atomic fire-limit predicate include explicit workspace scope/ID? Current `RunQuery` evidence in this slice is insufficient to prove the store key invariant.

## Evidence

All citations below are real paths relative to the repository root unless absolute. Each source is listed once with the line regions used by this analysis.

### Governing Go doctrine

- `.agents/skills/golang-master/references/modernize.md:3-42` — modernization is contextual and gradual; versioned facilities include `slices`/`maps`, `OnceFunc`, range integers, `rand/v2`, `cmp.Or`, iterators, `b.Loop`, `synctest`, and `errors.AsType`.
- `.agents/skills/golang-master/references/concurrency.md:7-40,81` — goroutine ownership/waitability, `WaitGroup.Go`, and pull iterators versus genuinely concurrent channels.
- `.agents/skills/golang-master/references/testing.md:56,85-91` — deterministic concurrent-time tests with `testing/synctest` and benchmark preference for `b.Loop`.
- `.agents/skills/golang-master/references/errors.md:39` — wrapped error matching and Go 1.26 `errors.AsType` guidance.
- `.agents/skills/golang-master/references/interfaces-generics.md:122` — check standard `slices`/`maps` helpers before creating generic collection utilities.

### Automation

- `internal/automation/dispatch.go:24-56,189-190,242-257` — fire-limit error contract, run-store query surface, and dispatcher-local mutex.
- `internal/automation/dispatch_context.go:51-65` — the separate process-local concurrency gate and release.
- `internal/automation/dispatch_reservation.go:14-25,97-124,128-165,168-235` — gate acquisition, non-atomic limit/create, reserved-run semantics, mutex-held list query, counted statuses, and retry calculation.
- `internal/automation/dispatch_test.go:819-950,1037-1070` — sequential cross-dispatcher persistence, reserved-run fire-limit status/timing, and canceled-run exclusion.
- `internal/automation/dispatch_integration_test.go:17-252` — existing real-store dispatcher integration owner for concurrent and lifecycle persistence invariants.
- `internal/automation/schedule_lifecycle.go:12-96` — explicit-stop context detachment, premature stopped state, untracked wait goroutine, timeout, and unconditional handle clearing.
- `internal/automation/schedule_execution.go:67-86,141` — manual wait-group spawn and an existing `errors.AsType` exemplar.
- `internal/automation/schedule_test.go:947-1000` — stop option and one-way start/shutdown lifecycle contract.
- `internal/automation/model/list_semantics.go:9-35,173-204` — deterministic source/name/ID ordering.
- `internal/automation/model/list.go:213-360` — materialized pages and cursor search over stable order.
- `internal/automation/model/template_bench_test.go:5-27` — benchmark already using `b.Loop`.
- `internal/automation/perf_bench_test.go:15-142` — four benchmarks already using current loop form.

### Task

- `internal/task/interfaces.go:170-218` — run mutation/store commands, existing completion settlement, event store, and post-commit observer surface.
- `internal/task/lease_manager_claim.go:5-52` — claim-first choreography, metadata redaction, task reconcile, audit record, hook, and network bind ordering.
- `internal/task/lease_network_binding.go:14-60` — bounded network-bind failure cleanup through lease release.
- `internal/task/lease_manager.go:11-95,128-195` — heartbeat/release partial durable ordering and completion's atomic settlement pattern.
- `internal/task/completion_settlement.go:3-16` — run/task/status/rollup settlement value.
- `internal/task/lease_test.go:613-723,813-840,1517` — post-mutation network restoration failure paths, bind-failure release behavior, and remaining manual wait-group test spawn.
- `internal/task/hooks_integration_test.go:16-55` — post-claim hook observes committed audit.
- `internal/task/lease.go:53-214` — workspace/session claim identity, raw token contract, `crypto/rand`, hashing, redaction, and constant-time verification.
- `internal/task/live.go:11-57` — buffered stream ownership and single close guarded by `sync.Once`.
- `internal/task/wake.go:13-473` — contracts, validation, dispatch, dedupe, audit, and summary responsibilities in one near-cap file.
- `internal/task/perf_bench_test.go:10-59` — four benchmarks already using `b.Loop`.
- `internal/task/autonomy.go:51-75` — one production `errors.As` target and a suitably narrow store interface.

### Loop, goal, gate, and watch

- `internal/loop/coordinator_fsm.go:12-95` — private fixed linear state/event definitions and wrapper logging.
- `internal/loop/coordinator.go:149-167` — fixed transition calls interleaved with the already-linear coordinator phases.
- `internal/loop/coordinator_watch_test.go:806-840` — sole remaining benchmark using `b.N`.
- `internal/loop/watch/fsm.go:19-56` — real ready/wait/stall branches and transition enforcement.
- `internal/loop/watch/adapter.go:52-112` — transition history returned for ready, waiting, and stalled outcomes.
- `internal/loop/watch/types.go:36-65` — observable outcome and transition result contract.
- `internal/loop/goal/recovery.go:179-269` — repeated unbounded `context.WithoutCancel` calls across terminal/ambiguity recovery.
- `internal/loop/goal/compaction.go:177-188` — local bounded detached cancel/drain pattern.
- `internal/loop/goal/executor.go:20,215,375-386` — segment timeout ownership and one `errors.As` modernization site.
- `internal/loop/goal/route.go:68,319-383` — one `errors.As` modernization site and explicit terminal judge outcome handling.
- `internal/loop/input_defaults.go:63,187-205` — one `errors.As` site and deterministic map-key collection.
- `internal/loop/runtime_types.go:146` — one `errors.As` modernization site.
- `internal/loop/linter.go:424` — existing production `errors.AsType` exemplar.
- `internal/loop/action_channel_result_content_rule.go:59-105` — appropriate existing `strings.SplitSeq` use.
- `internal/loop/resource_spec_precedence.go:71-77` — hand-built sorted map-key collector.
- `internal/loop/resource_spec_projection.go:95-118` — sorted value/key materialization suitable for `maps.Keys` collection.
- `internal/loop/source_store.go:164-183` — existing `os.OpenRoot` execution precedent.
- `internal/loop/dsl/runtime.go:56-57` — current `omitzero` use on value structs.
- `internal/loop/dsl/refs/template.go:30-40` — buffer used for rendering, not protocol framing.
- `internal/loop/gate/judge.go:116-124,282-289` — buffers used for prompt rendering/compaction, not peeking.

### Scheduler and observe

- `internal/scheduler/scheduler.go:115-209,350-419` — owned runtime handle, retryable shutdown state, `WaitGroup.Go`, duplicate lower/upper error logs, and background loop ownership.
- `internal/scheduler/scheduler_lifecycle_test.go:15-124,141-142` — explicit shutdown-retry invariant and test-only `sync.Once` values.
- `internal/scheduler/selection.go:118-184` — active-session ownership set and stable run/session ordering.
- `internal/observe/retention.go:82-131,182-219` — manual goroutine start, premature handle clearing, untracked waiter, real ticker, natural exit, and stringified logging error.
- `internal/observe/observer_test.go:367-465` — canonical retention sweep behavior suite; lifecycle coverage is absent.
- `internal/observe/reconcile.go:129-145` — legacy provider repair and unrecoverable-state skip.
- `internal/observe/reconcile_test.go:396-590` — legacy repair/skip fixtures that become delete targets.
- `internal/observe/bridges_test.go:544` — only literal `time.Sleep` in the scoped tests; it should be assessed under its synchronization invariant if that suite is touched.
- `internal/observe/perf_bench_test.go:33-68` — four benchmarks already using `b.Loop`.

### Managed sidecars and small domain packages

- `internal/heartbeat/source_path.go:18-125,218-248` — containment/path projection, hand-built sorted keys, and diagnostic line indexing.
- `internal/heartbeat/authoring_path.go:13-146` — managed-root resolution, `EvalSymlinks`, component `Lstat`, and strict symlink rejection.
- `internal/heartbeat/authoring.go:248-309` — validation followed later by absolute-path atomic write/remove.
- `internal/heartbeat/history_purge.go:13-75` — heartbeat-specific revision-source derivation and purge.
- `internal/heartbeat/validation.go:142-180` — line-indexed body diagnostics that justify materialization unless deliberately rewritten.
- `internal/heartbeat/wake_helpers.go:114` — correct `crypto/rand` use.
- `internal/soul/soul_path.go:15-117` — duplicated containment/path projection.
- `internal/soul/authoring_path.go:13-133` — duplicated managed-root/symlink validation.
- `internal/soul/authoring.go:249-318` — validation followed later by absolute-path atomic write/remove.
- `internal/soul/history_purge.go:13-79` — generic-looking sidecar revision derivation implemented only in SOUL.
- `internal/soul/soul_fields.go:88-109` — line-indexed metadata diagnostics.
- `internal/soul/soul_parse.go:185-204` — line-indexed reserved-section diagnostics.
- `internal/situation/task_context_redaction.go:31-38` — subtraction-based attempt comparator.
- `internal/situation/service_context.go:137-153` — explicitly documented legacy prompt-provider seam.
- `internal/situation/service_test.go:1260-1273` — only in-slice direct test of that legacy seam.
- `internal/reasoning/reasoning.go:1-74` and `internal/reasoning/reasoning_test.go:1-48` — complete small reasoning package; no actionable issue.
- `internal/speed/speed.go:1-119` and `internal/speed/speed_test.go:1-70` — complete small speed policy package; no actionable issue.

### Event registry

- `internal/events/registry.go:11-449` — outcome/type definitions, component and event constants, base entries, public validation, and registry construction in one near-cap file.
- `internal/events/registry_mcp_auth.go:3-15` — component catalog plus nested append-based final composition.
- `internal/events/registry_queries.go:9-82` — sorted public registry queries that must retain deterministic output.
