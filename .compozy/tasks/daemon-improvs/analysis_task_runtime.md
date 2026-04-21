# Task Runtime Analysis — Looper vs AGH

Focus: per-run execution pipeline, concurrency, subprocess lifecycle, persistence/snapshotting, reentry, observability. This scopes looper's `internal/daemon/run_manager.go` + `internal/core/run/*` against AGH's `internal/daemon/task_runtime.go`, `internal/task/`, `internal/subprocess/`, `internal/hooks/`, and the AGH harness bridges (`harness_detached_work.go`, `harness_reentry_bridge.go`, `harness_observability.go`).

---

## 1. Quick assessment

Looper and AGH occupy different shapes of the same problem space:

- **Looper is a batch-workflow runner.** A run is a finite list of prepared jobs that each shell out to an ACP agent subprocess. Lifecycle is linear: `startRun -> prepare -> executeJobsWithGracefulShutdown -> finalize`. Pause/resume and detachment are not first-class — the only "resume" surface is exec-mode session reload via `resumeExistingExecRun`. See `internal/daemon/run_manager.go:921-1296`.
- **AGH is a session-and-task platform.** Tasks are durable objects with approval policy, scope (global/workspace), retries, owners, and events. Runs attach to an ACP session through a `SessionExecutor` bridge (`internal/daemon/task_runtime.go:78-209`). Task manager + session manager + hooks + reentry bridge are composable; the daemon assembles them at boot (`bootTasks` at `task_runtime.go:211-289`).

Strengths **looper already has** that are comparable or better than AGH on a per-run basis:

- **Append-before-publish journal with backpressure** — `internal/core/run/journal/journal.go` (5 s submit timeout, drop counter, `ErrSubmitTimeout`, atomic sequence, bus forwarding). Stronger than AGH's `store.EventSummaryStore` which is best-effort logging only.
- **Startup crash reconciliation with synthetic `run.crashed` events** — `internal/daemon/reconcile.go:93-207`. AGH reconciliation is coarser: it picks "requeue / mark_running / fail" decisions but does not emit synthetic terminal events into a per-run journal.
- **Dense snapshot replay** — `internal/daemon/run_snapshot.go` rebuilds job state, transcript, and token usage from events. Comparable to what AGH builds for sessions, and looper already applies it to runs.
- **Graceful shutdown state machine with forced escalation** — `internal/core/run/executor/shutdown.go:43-226` (initializing→running→draining→forcing→shutdown→terminated, plus `forceActiveClients` to SIGKILL ACP subprocesses on escalate). AGH's subprocess layer has an equivalent at the process level (`internal/subprocess/process.go:343-410`), but looper has it at the *executor* level.

Strengths **AGH has** that looper does not:

1. **Subprocess primitive as a first-class package** (`internal/subprocess/`) — process launches are managed, health-probed, and provide a shared JSON-RPC transport. Each launch hits a typed shutdown ladder: cooperative RPC → `closeInput` → `SIGTERM` via `terminateManagedProcess` → `SIGKILL` via `killManagedProcess`, with group-wait. Looper's `internal/core/agent/client.go:456-479` conflates ACP client shutdown with subprocess shutdown and has no health monitor.
2. **Health monitor with consecutive-failure threshold** — `internal/subprocess/health.go:77-187`. Looper has no stall/crash detection while a job is "running".
3. **Per-run detached work pattern** — `harness_detached_work.go:115-403` lets a running session submit a task that runs out-of-band, returning immediately and later notifying the owner via synthetic reentry (`harness_reentry_bridge.go`). Looper has no detached/background work concept at all.
4. **Synthetic reentry after terminal events** — `harness_reentry_bridge.go:340-487` decides (emitted / silent / dropped) based on the wake target state, records the decision in run metadata (`detachedHarnessReentry`), and queues `PromptSynthetic` wakes per-session ordered by `completedAt` + sequence. Looper has nothing equivalent: a completed run is just a row update.
5. **Typed hook pipeline with patch semantics** — `internal/hooks/dispatch.go` dispatches `DispatchSessionPreCreate`, `DispatchSessionPostCreate`, `DispatchSessionPreResume`, etc., with `denied` predicates and patches. Looper has hooks but they are ad-hoc payloads via `model.DispatchMutableHook("job.pre_execute", …)` / `"run.pre_start"` (see `internal/core/run/executor/hooks.go`) without a structured pipeline, without a deny predicate, and without a strict mutation contract beyond hand-rolled allowlists (`validateWorkflowPreparedStateMutation` at `execution.go:333-368`).
6. **Observability recorder with deferred writes** — `harness_observability.go:30-98` queues summaries for sessions that do not yet exist and flushes on `OnSessionCreated`. Looper writes lifecycle events to `rundb`, but has no equivalent of startup/resolver/augmenter observations separate from raw events.
7. **Boot recovery planning as a pure function** — `planTaskRunRecovery` at `task_runtime.go:352-408` maps `(runStatus, sessionLive)` to one of `requeue/mark_running/fail`. This is testable in isolation. Looper's reconcile only has one action — mark crashed — and no notion of "resume a live session if it still exists".
8. **Actor/Origin identity on every write** — `taskpkg.DeriveDaemonActorContext(...)` seals write authority into the run metadata. Looper uses a looser `RequestIDFromContext` (informational only; `run_manager.go:1017`) and has no "origin kind" concept beyond `Mode` (`task`/`review`/`exec`).

Strengths **neither has, but looper would benefit from**:

- **Pause/resume mid-run.** Neither product supports true pause/resume of an in-flight subprocess. AGH has "detached" (new background run) but not "pause this run". For looper this is future.

---

## 2. Gaps — AGH reference, why for looper, action, priority

### G1. Subprocess lifecycle primitive — promote to shared package

- **AGH reference:** `internal/subprocess/process.go:117-410` — `Launch()` returns a `Process` with `Shutdown(ctx)` implementing cooperative-RPC → stdin-close → SIGTERM → SIGKILL ladder plus process-group wait.
- **Why for looper:** `internal/core/agent/client.go:456-479` exposes `Close()` and `Kill()` but the underlying process management is buried in `internal/core/subprocess` (imported at `agent/client.go:19`) and is not reused by the exec path or future daemon subprocesses. Forced kill on shutdown escalation lives in the executor (`execution.go:618-631 forceActiveClients`), not in the agent client. Any future daemon-owned subprocess (git hook runner, extension host, MCP server) will re-implement this ladder.
- **Action:** Audit `internal/core/subprocess` and align it with the AGH `Launch/Shutdown/HealthState` interface. Extract group-wait helpers (AGH has `forceManagedProcessGroupExit` with `defaultProcessGroupWait`). Make the `Kill()` path on `agent.Client` delegate to a uniform primitive.
- **Priority:** Medium. Looper only launches ACP agents today, but extensions and the MCP bridge are on the roadmap. Sharing one primitive prevents drift.

### G2. Subprocess health monitoring — detect stalled ACP agents

- **AGH reference:** `internal/subprocess/health.go:77-187` — interval-based `health_check` probe, `HealthFailureThreshold`, `HealthState.Healthy`, last-error retention.
- **Why for looper:** Looper launches Claude Code / Codex / Droid and streams ACP updates. If an agent stops streaming but does not exit (common in Claude hangs), the job runs to its `cfg.Timeout` with no visibility. There is no intermediate "unhealthy" signal. `jobRunner.executeAttempt` (`runner.go:139-153`) only observes exit codes and the overall context deadline.
- **Action:** Add an idle/heartbeat detector on `agent.Client` that tracks time since last `SessionUpdate`. When it exceeds a threshold, emit an `agent.unhealthy` event on the journal bus and optionally escalate to kill-and-retry. The probe does not have to be an RPC — session-update inactivity is already observable.
- **Priority:** High. This is the #1 cause of "why is my run stuck?" support friction, and looper's linear workflow makes stalls more visible than AGH's multi-session model.

### G3. Per-job re-entry / reuse after daemon restart — session resume

- **AGH reference:** `harness_reentry_bridge.go:340-487`; `planTaskRunRecovery` in `task_runtime.go:352-408` maps a still-live ACP session to `RunBootRecoveryMarkRunning`.
- **Why for looper:** Looper's exec mode already persists `ACPSessionID` and exposes `resumeExistingExecRun` (`run_manager.go:1026-1103`), but only for **exec**, only on the client side, and only when the operator re-invokes. If a **task** or **review** run was mid-flight when the daemon restarted, `ReconcileStartup` marks it `crashed` even if the underlying ACP agent subprocess survived and the session is still usable. There is no "sessionLive → mark_running" branch and no automatic continuation.
- **Action (two-phase):**
  1. **Phase 1 (detection):** During `ReconcileStartup` (`reconcile.go`), check whether the per-job subprocess/PID survived. If so, synthesize `run.resumed` instead of `run.crashed`. Looper has no PID tracking yet — add a `daemon_runs.session_pid` column or sidecar file.
  2. **Phase 2 (continuation):** Wire a bridge analogous to AGH's `harnessReentryBridge` so surviving ACP sessions can be re-attached by the daemon and finish their job. This is a larger refactor that requires stable session IDs at the job level.
- **Priority:** High for Phase 1 (observability honesty), Medium for Phase 2 (true resumption).

### G4. Detached / background sub-runs from within a run

- **AGH reference:** `harness_detached_work.go:70-204` — `submit` creates a task+run keyed by idempotent `SubmissionKey`, returns immediately, and records the `wake_target` for synthetic reentry.
- **Why for looper:** Today, if an extension inside a job wants to kick off a secondary run ("also do code review on these files"), it has no API. Everything must complete synchronously inside `executeJobsWithGracefulShutdown`. The `run.pre_shutdown` / `run.post_shutdown` hooks fire, but they cannot *spawn* new runs that survive their parent.
- **Action:** Add a `DaemonHost.SubmitDetachedRun(parentRunID, spec)` method on the extension bridge (`internal/core/extension`). Persist parent/child linkage in globaldb. This is essentially what AGH's `detachedHarnessSubmission` does but without the complex reentry — looper can defer reentry (G6) and still ship the detach primitive.
- **Priority:** Medium. Unlocks review + task workflows running in parallel from a single trigger without spawning a new CLI invocation.

### G5. Typed hook pipeline with explicit mutation contract

- **AGH reference:** `internal/hooks/dispatch.go:10-94` — `dispatchConfig[P, R]` with `match`, `apply`, `denied`, `denyErr`, `guard` predicates. Each hook kind has its own typed dispatch entry.
- **Why for looper:** Looper's hook dispatch is through generic `model.DispatchMutableHook(ctx, manager, "run.pre_start", payload)` + ad-hoc `validateWorkflowPreparedStateMutation` guard (`executor/execution.go:288-368`). The guard has to enumerate every field a hook is forbidden to mutate — adding a field requires editing the guard. There is no "deny this whole dispatch" primitive; hooks can only mutate.
- **Action:** Introduce a `HookDispatcher[P, R]` generic wrapper in `internal/core/run/executor/hooks.go` or `internal/core/model` that makes allowed mutations explicit via a `Patch` type (the return-value type of the hook) rather than returning the whole payload. Keep the existing `run.pre_start` / `job.pre_execute` / `job.pre_retry` hook names but reshape their contracts.
- **Priority:** Medium. Quality-of-life that pays off when extensions multiply. Not urgent.

### G6. Synthetic reentry / completion notifications to callers

- **AGH reference:** `harness_reentry_bridge.go:199-487` — event observer on task events, per-session ordered wake queues, `PromptSynthetic` dispatch with metadata correlating `TaskID` + `TaskRunID`, idempotent via `syntheticEventExists`.
- **Why for looper:** Partially implicit in looper's `OpenStream` (live subscription + replay, `run_manager.go:597-622`). But there is no mechanism for "notify the original caller's *other session* (e.g. IDE) that this run finished" beyond the stream. If the CLI that started the run has exited, the run's completion is only discoverable by polling `Get` or re-opening the stream. There's no hook mechanism like "call this webhook when this run terminates".
- **Action:** Either (a) add a `RunCompletionNotifier` interface the daemon can dispatch to (SSE, webhook, extension) or (b) defer — CLI-only operators may not need it. The real value appears once looper grows a web UI or Slack-style surface.
- **Priority:** Low today. Revisit when a non-CLI surface lands.

### G7. Stronger boot recovery with session-live detection

- **AGH reference:** `planTaskRunRecovery` (`task_runtime.go:352-408`) inspects `session.Info.State` via `taskSessionRuntimeState` and produces one of three actions per run.
- **Why for looper:** `reconcile.go:93-207` only has one path — mark every interrupted run as crashed. Looper loses the ability to distinguish "daemon was killed but the ACP agent is a grandchild that survived" from "daemon+agent both dead". Even if looper does not restart and resume today (G3), this is free observability.
- **Action:** In `ReconcileStartup`, before appending `run.crashed`, probe for the session PID (once tracked per G3). If alive, append `run.orphaned` instead and leave `status=running` until a subsequent boot confirms death. Adds a third terminal status like looper's existing `crashed`.
- **Priority:** Medium. Depends on G3 Phase 1 (PID tracking).

### G8. Observability event summaries (non-runtime events)

- **AGH reference:** `harness_observability.go:30-280` — `harnessLifecycleRecorder` with `queue` (pending until session exists), `record` (write-through), and typed summary writers (`RecordStartupContextResolved`, `RecordAugmenterApplied`, etc.).
- **Why for looper:** Looper writes strong *workflow* events (`job.queued`, `session.update`, `run.crashed`) but has no separate timeline for *daemon* events that correlate to a run — e.g. "workspace sync completed," "preflight aborted," "extension registered." Today these are logged via slog and disappear. AGH surfaces them alongside the session timeline with `event_summaries`.
- **Action:** Add a lightweight summary writer that writes to globaldb (not per-run rundb) with `(run_id, timestamp, type, summary)`. Useful targets: preflight decisions (`preflight.go` already has `Decision` and `logPreflightDecision`), `syncWorkflowBeforeRun`, watcher events, extension lifecycle.
- **Priority:** Low-to-medium. Pays off once users debug "why did my run start slowly?" kind of questions.

### G9. Typed actor/origin identity on writes

- **AGH reference:** `taskpkg.DeriveDaemonActorContext(ref, origin)` stamps every task write; origin prefix examples in `harness_detached_work.go:22` and `task_runtime.go:268`.
- **Why for looper:** Run rows store only `RequestID` (informational) and `Mode` ("task"/"review"/"exec"). There is no record of who started the run (CLI? extension? daemon reconcile?). Audit, multi-actor environments, and bulk cancellation policies will eventually need it.
- **Action:** Add an `ActorContext` on `startRunSpec` carrying `(kind, ref, origin)`. Persist on `globaldb.Run`. Start simple (`cli`, `daemon`, `extension`, `remote`).
- **Priority:** Low today. Schedule once a second write ingress (HTTP API, web UI) is planned.

### G10. Backpressure contract for live subscribers

- **AGH reference:** AGH's `PromptSynthetic` channel path closes on terminal event and saturation triggers a rescan (`OnTaskEvent` in `harness_reentry_bridge.go:177-197` with `b.requestRescan()` fallback).
- **Why for looper:** Looper's `Events()` bus supports drop detection (`subscription.bus.DroppedFor(subID)`) and emits a `RunStreamOverflow` item (`run_manager.go:1611-1617`). This is already good. The gap is the **producer side**: the journal returns `ErrSubmitTimeout` after 5 s (`journal.go:275-292`), but the executor only logs a warning via `submitEventOrWarn` (`execution.go:685-689`). A dropped `job.completed` event can corrupt the snapshot rebuild.
- **Action:** Escalate `ErrSubmitTimeout` on terminal event kinds (`run.completed`, `run.failed`, `job.completed`, `job.failed`) to a forced retry or a run-level `crashed` transition. Consider adding a per-run "events_dropped" counter on the run row so `Snapshot` can indicate incomplete data.
- **Priority:** Medium. Rarely fires today but silent corruption is the worst failure mode.

---

## 3. Explicitly skipped

- **Approval/policy state machine** (AGH `ApprovalPolicy`, `ApprovalState`): looper runs are autonomous batch workflows; no human-in-the-loop gate is planned.
- **Task priority + dependency graph** (AGH `Priority`, `DependencyKind`): looper's `plan.Prepare` already orders work; a DAG across runs is out of scope.
- **Multi-actor authority scopes** (AGH `CreateGlobal` / `CreateWorkspace`): single-user CLI; not needed until multi-tenant landing.
- **Session types / scopes** (AGH `SessionTypeSystem`, `ScopeGlobal`): looper's "run" is effectively always workspace-scoped; no need for a global/workspace dichotomy.
- **Network channels** (`NetworkChannel`, `validateChannel`): looper is local-first per `CLAUDE.md` constraints.
- **Full hooks `matchSessionPreCreate` / `pre_resume` etc.**: looper has no "session" concept separate from a run; adopting the full AGH hook taxonomy is over-engineering.
- **Typed `EventObserver` interface + task event records**: looper's journal-as-single-source-of-truth is simpler and sufficient for the linear workflow.
- **Event summaries with `EventSummaryStore.WriteEventSummary`**: partially addressed by G8; the full AGH API (pending-for-not-yet-created-session) is unnecessary.

---

## Key file references

Looper:

- `/Users/pedronauck/Dev/compozy/looper/internal/daemon/run_manager.go:921-1296` — `startRun`, `runAsync`, `executeWorkflowRun`, `executeExecRun`, `finishRun`
- `/Users/pedronauck/Dev/compozy/looper/internal/daemon/run_manager.go:1026-1103` — `resumeExistingExecRun` (exec-mode-only persisted resume)
- `/Users/pedronauck/Dev/compozy/looper/internal/daemon/run_manager.go:1760-1896` — RunDB lease cache
- `/Users/pedronauck/Dev/compozy/looper/internal/daemon/run_snapshot.go` — snapshot rebuild from events
- `/Users/pedronauck/Dev/compozy/looper/internal/daemon/reconcile.go:93-207` — `ReconcileStartup` + `appendSyntheticCrashEvent`
- `/Users/pedronauck/Dev/compozy/looper/internal/core/run/executor/execution.go:28-108, 409-631` — `Execute`, `jobExecutionContext`, `forceActiveClients`
- `/Users/pedronauck/Dev/compozy/looper/internal/core/run/executor/shutdown.go:43-226` — graceful shutdown state machine
- `/Users/pedronauck/Dev/compozy/looper/internal/core/run/executor/runner.go:29-81` — per-job `jobRunner.run` retry loop
- `/Users/pedronauck/Dev/compozy/looper/internal/core/run/executor/hooks.go` — untyped hook payload shapes
- `/Users/pedronauck/Dev/compozy/looper/internal/core/run/journal/journal.go:275-341` — submit backpressure, drops counter, `ErrSubmitTimeout`
- `/Users/pedronauck/Dev/compozy/looper/internal/core/run/exec/exec.go:230-300` — `ExecuteExec` single-prompt pipeline
- `/Users/pedronauck/Dev/compozy/looper/internal/core/run/preflight/preflight.go` — preflight decisions (observability target)
- `/Users/pedronauck/Dev/compozy/looper/internal/core/agent/client.go:456-479` — `Close` and `Kill`

AGH (reference):

- `/Users/pedronauck/dev/compozy/agh/internal/daemon/task_runtime.go:78-289` — session bridge + boot recovery wiring
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/task_runtime.go:352-408` — `planTaskRunRecovery`
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/harness_detached_work.go:115-204` — detached harness work submission
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/harness_reentry_bridge.go:177-487` — event-driven synthetic reentry with ordered per-session wake queues
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/harness_observability.go:30-280` — lifecycle recorder with pending-until-session-created queue
- `/Users/pedronauck/dev/compozy/agh/internal/subprocess/process.go:117-410` — typed process lifecycle with shutdown ladder
- `/Users/pedronauck/dev/compozy/agh/internal/subprocess/health.go:77-187` — health monitor with threshold
- `/Users/pedronauck/dev/compozy/agh/internal/hooks/dispatch.go:10-120` — typed hook pipeline with patch/deny semantics
- `/Users/pedronauck/dev/compozy/agh/internal/task/manager.go:1621-1683` — `RecoverRunOnBoot` three-way action dispatch
