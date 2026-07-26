---
name: 04-autonomy-kernel
description: AGH pre-release QA — autonomy kernel module (task_runs + scheduler + hooks + coordinator). Real-LLM scenarios required. Read-only research deliverable.
type: qa-child
module: autonomy-kernel
owner: pre-release-qa
references:
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/openclaw-qa-patterns.md
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/hermes-qa-patterns.md
  - /Users/pedronauck/Dev/compozy/agh/CLAUDE.md
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/autonomous/
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/_archived/20260410-021708-hooks/
---

# 04 — Autonomy Kernel QA

## 1. Module scope

The autonomy kernel is the load-bearing inner ring of AGH. It owns durable
work intake, atomic ownership transfer, lease lifecycle, hook dispatch at
authoritative call sites, mechanical wake/observe/sweep, and coordinator
agent bootstrap. This QA child stresses every documented invariant from
`internal/CLAUDE.md` against the real implementation, with real Claude Code
subagents wherever the scenario calls for live behavior.

Packages in scope (file:line citations are repo-absolute):

| Surface                | Path                                                                       | Authoritative API                                                                                                                                                                                                                                                                                                                              |
| ---------------------- | -------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Task domain            | `/Users/pedronauck/Dev/compozy/agh/internal/task/`                         | `Service.ClaimNextRun` (`internal/task/lease_manager.go:14`), `HeartbeatRunLease` (`:58`), `ReleaseRunLease` (`:94`), `ReleaseSessionRunLeases` (`:136`), `CompleteRunLease` (`:188`), `FailRunLease` (`:223`), `RecoverExpiredRunLeases` (`:259`), `LookupActiveRunForSession` (`:347`), claim-token primitives (`internal/task/lease.go:151`) |
| Mechanical scheduler   | `/Users/pedronauck/Dev/compozy/agh/internal/scheduler/`                    | `Scheduler.RunOnce` (`internal/scheduler/scheduler.go:238`), `sweepExpiredLeases` (`:262`), `selectWakeTargets` (`:421`), `dispatchWakeTargets` (`:312`), `Rebuild` (`:194`)                                                                                                                                                                    |
| Hook taxonomy/dispatch | `/Users/pedronauck/Dev/compozy/agh/internal/hooks/`                        | `Hooks.DispatchTaskRunPreClaim` family (`internal/hooks/dispatch.go`), event registry (`internal/hooks/events.go:54-130`), pipeline (`internal/hooks/pipeline.go:37-172`), subprocess executor timeout (`internal/hooks/executor_subprocess.go:23,224`)                                                                                         |
| Coordinator            | `/Users/pedronauck/Dev/compozy/agh/internal/coordinator/`                  | `DecideBootstrap` (`internal/coordinator/coordinator.go:104`), `IsExecutableRunStatus` (`:150`), `PermissionPolicy` (`:173`), `Lineage` (`:193`), `PromptOverlay` (`:238`), `ToolAllowlist` (`:64`)                                                                                                                                             |
| Retry primitives       | `/Users/pedronauck/Dev/compozy/agh/internal/retry/`                        | `internal/retry/retry.go`                                                                                                                                                                                                                                                                                                                      |
| Automation overlap     | `/Users/pedronauck/Dev/compozy/agh/internal/automation/`                   | `internal/automation/dispatch.go`, `internal/automation/trigger.go` (only the trigger→task-run-enqueue boundary is in-scope here; full automation is module 09).                                                                                                                                                                                |

Out of scope (covered by other children): full automation cron/webhook
execution (module 09), AGH Network channel transport (module 06), session
manager state machine (module 03), web UI surfaces (module 08).

## 2. Authoritative invariants under test

These come straight from `internal/CLAUDE.md` and the implementation. Every
scenario below maps back to one or more of these IDs. Coverage IDs follow
the openclaw lowercase dotted/dashed convention.

| Coverage ID                       | Invariant                                                                                                                                                  | Source                                                                                                                                  |
| --------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| `task_runs.single-queue`          | `task_runs` is the single durable queue. No peer package replicates `claim/own`; only `internal/task/` writes claim columns.                               | `internal/CLAUDE.md` "Authoritative primitives are exclusive."                                                                          |
| `task_runs.claim-authoritative`   | `Service.ClaimNextRun` is the only authoritative claim primitive (token-fenced).                                                                           | `internal/task/lease_manager.go:14`                                                                                                     |
| `task_runs.claim-token-redaction` | Raw `agh_claim_*` tokens NEVER appear in logs, SSE, web, db, or status APIs. Only `claim_token_hash` over the wire.                                        | `internal/CLAUDE.md` "claim_token redaction is non-negotiable."; `internal/task/lease.go:151,160,168`                                   |
| `task_runs.lease-token-fence`     | Heartbeat/Release/Complete/Fail require raw token verification via `VerifyClaimToken` (constant-time).                                                     | `internal/task/lease.go:178`                                                                                                            |
| `task_runs.lease-expiry-sweep`    | Expired leases (`lease_until < now`) are reclaimed by `RecoverExpiredRunLeases`; previous holders' late writes hit `ErrInvalidClaimToken`.                 | `internal/task/lease_manager.go:259`                                                                                                    |
| `scheduler.no-claim`              | The mechanical scheduler does NOT call `ClaimNextRun`. It performs sweep, observe, wake only.                                                              | `internal/CLAUDE.md` "Wake/observe/sweep are allowed; claim/own is not. The mechanical scheduler does not call ClaimNextRun."           |
| `scheduler.sweep-recovery`        | After daemon restart, scheduler sweep reclaims expired leases without orphaning task_run rows.                                                             | `internal/scheduler/scheduler.go:262`                                                                                                   |
| `scheduler.idle-wake`             | Idle eligible sessions are woken when a queued run matches their workspace/scope/channel/capabilities, with cooldown debounce.                             | `internal/scheduler/scheduler.go:421-466`                                                                                               |
| `hooks.typed-dispatch`            | Hooks fire at the call site that owns the state transition, not by tailing event/log tables.                                                               | `internal/CLAUDE.md` "Hooks are typed dispatch, not an event bus."                                                                      |
| `hooks.deny`                      | A hook returning `deny` short-circuits the operation with a typed error; audit trail is recorded (`HookRunOutcomeDenied`).                                 | `internal/hooks/pipeline.go:163-171`; `internal/task/lease_manager.go:557-563`                                                          |
| `hooks.narrow`                    | A hook MAY narrow capabilities/permissions; downstream callers honor the narrowed value.                                                                   | `internal/CLAUDE.md` "Hooks may deny/narrow/annotate but cannot bypass safety primitives"                                               |
| `hooks.annotate`                  | Hook-supplied metadata flows into transcript/SSE events.                                                                                                   | Same.                                                                                                                                   |
| `hooks.no-bypass`                 | Hooks cannot bypass safety primitives (claim tokens, leases, TTL, lineage, spawn caps, permission narrowing).                                              | Same.                                                                                                                                   |
| `hooks.timeout-fail-open`         | Subprocess hook executor uses 5s default timeout; non-required hook timeouts fail-open (dispatch proceeds, error logged).                                  | `internal/hooks/executor_subprocess.go:23,224`; `internal/hooks/pipeline.go:68-79`                                                      |
| `hooks.required-fail-closed`      | A `Required` hook failure halts the dispatch chain with a wrapped error.                                                                                   | `internal/hooks/pipeline.go:71-77`                                                                                                      |
| `hooks.ordering`                  | Multiple hooks at the same event fire in `orderedResolvedHooksIfNeeded` precedence then declaration order.                                                 | `internal/hooks/pipeline.go:59`; `internal/hooks/ordering.go`                                                                           |
| `coordinator.bootstrap`           | First eligible workspace task_run on a fresh AGH_HOME causes a workspace coordinator session to be bootstrapped exactly once.                              | `internal/coordinator/coordinator.go:104,143`                                                                                           |
| `coordinator.permissions`         | Coordinator sessions are constrained to `ToolAllowlist` and never spawn another coordinator (`SpawnRoleAllowed` rejects coordinator role).                 | `internal/coordinator/coordinator.go:64-78,188`                                                                                         |
| `coordinator.lineage`             | Every spawned worker carries `parent_session_id` and `root_session_id`; `SpawnDepth` cannot exceed `DefaultSpawnMaxDepth`.                                 | `internal/store/session_lineage.go:13-83`; `internal/session/spawn.go:215`                                                              |
| `manual-equals-peer`              | A manual prompt is just a peer claim path: it goes through the same `ClaimNextRun` primitive with `actor.Kind == AgentSession`. There is no shortcut path. | `internal/CLAUDE.md` "manual=peer mode"                                                                                                 |
| `lineage.coverage`                | Every task_run event emits `parent_session_id`, `root_session_id`, and `claim_token_hash` correlation keys.                                                | `internal/CLAUDE.md` Observability bullet                                                                                               |
| `spawn.cap`                       | `SpawnDepth` and `MaxChildren` enforce hard ceilings; recursive spawn beyond cap is rejected with a typed error.                                           | `internal/store/session_lineage.go:127-140`; `internal/session/spawn.go:215-237`                                                        |
| `event.lineage-correlation`       | Every canonical task_run event carries the correlation keys named in `internal/CLAUDE.md` Observability section.                                           | `internal/CLAUDE.md` Observability bullet                                                                                               |

## 3. Operating model

QA mode is **real-scenario** (per the standing directive on real-scenario
QA), not pytest-style assertions. Every scenario:

- Runs against an isolated AGH_HOME with unique daemon ports + tmux-bridge
  socket (per `agh-worktree-isolation` skill).
- Resolves provider auth from the bootstrap manifest according to each
  provider contract: bound-secret, brokered, and explicitly isolated-home
  lanes use `PROVIDER_HOME` / `PROVIDER_CODEX_HOME`, while `native_cli`
  lanes with `home_policy=operator` preserve the operator `HOME` unless the
  scenario explicitly validates isolated provider-home behavior.
- Uses real Claude Code (`claude-opus-4-8` or `claude-sonnet-5` per
  scenario) as the subprocess agent driver, not mocks. OpenClaw and Hermes
  are referenced where their behavior differs.
- Emits four artifacts under `.artifacts/qa/<run-id>/aut-XX/`:
  - `aut-XX-report.md` (Worked / Failed / Blocked / Follow-up)
  - `aut-XX-summary.json` (machine-readable)
  - `aut-XX-events.json` (EventStore rows scoped to the scenario window)
  - `aut-XX-output.log` (combined stdout/stderr)
- Asserts against EventStore rows + `task_runs` table state + structured
  log output, never just process exit codes.

Scenarios are numbered `AUT-01..AUT-NN`; each is a fenced `qa-scenario`
block plus a flow narrative. Reproduce by running them sequentially or in
parallel under unique worktree isolation.

## 4. Provider matrix

| Mode                | When                                                                                              | Driver                                                                                                                                          |
| ------------------- | ------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| `real-claude-code`  | Default for all scenarios that exercise real subagent behavior, lineage, hook dispatch, spawn cap | `claude-opus-4-8` for the parent coordinator; `claude-sonnet-5` for the spawned children where indicated.                                 |
| `real-openclaw`     | Cross-driver sanity (AUT-09 only) to prove the autonomy kernel is driver-agnostic                 | OpenClaw bundled-plugin runtime via the AGH ACP client.                                                                                         |
| `real-hermes`       | Reference comparison only (AUT-09 only)                                                           | Hermes via ACP.                                                                                                                                 |
| `mock-acp` (gate)   | Determinism gate for race-sensitive scenarios where real models add nondeterminism (AUT-01 only). | `internal/e2elane` mock ACP server used only to make a 5-way race deterministic; the surrounding daemon runs real code paths.                    |

`mock-acp` is the deterministic dispatcher described in the openclaw
tri-state policy; AGH should expose it as `mock-acp`. `real-claude-code` is
the AGH equivalent of openclaw `live-frontier`. We do NOT include an
`aimock` lane here — per openclaw's own honest framing, it is additive and
not a replacement.

## 5. Preconditions (apply to every scenario)

- Fresh QA bootstrap via the `agh-qa-bootstrap` skill. Manifest path saved
  to `bootstrap-manifest.json`; `bootstrap.env` exported into the shell
  before any `agh` command.
- Unique `AGH_HOME` per worktree (per the worktree-isolation directive).
- Bound-secret, brokered, and explicitly isolated-home auth staged into
  `PROVIDER_HOME` / `PROVIDER_CODEX_HOME`; `native_cli` providers with
  `home_policy=operator` intentionally use the operator `HOME` / native login
  state unless the scenario explicitly validates isolated provider-home
  behavior.
- Daemon started in background. HTTP / UDS listeners reachable.
- `make verify` is green on the SUT branch before QA runs (per the
  Critical Rules).

Provider-specific config:

```text
AGH_HOME=$HOME/.qa/aut-04/<scenario>/agh-home
AGH_DAEMON_HTTP=127.0.0.1:<unique-port>
AGH_DAEMON_UDS=$AGH_HOME/sock/uds.sock
PROVIDER_HOME=$AGH_HOME/provider-home
PROVIDER_CODEX_HOME=$AGH_HOME/provider-codex-home
AGH_WEB_API_PROXY_TARGET=http://127.0.0.1:<unique-port>
```

## 6. Cleanup (applies to every scenario)

- `agh daemon stop` (or kill PID from manifest).
- Inspect `task_runs` for any stuck `claimed`/`running` rows; if found,
  attach to the scenario report and DO NOT clean — it's evidence.
- Archive `events.db` and `agh.db` snapshots before tearing down the
  AGH_HOME.
- Tear down the worktree only after evidence artifacts are written.

## 7. Mandatory scenarios

### AUT-01 — ClaimNextRun race (5 concurrent agents, exactly one winner)

```yaml qa-scenario
id: aut-01-claim-race
title: Five concurrent peers race ClaimNextRun; exactly one wins, raw token never logged
theme: autonomy-kernel.task_runs
coverage:
  primary:
    - task_runs.claim-authoritative
    - task_runs.single-queue
    - task_runs.claim-token-redaction
  secondary:
    - hooks.typed-dispatch
    - event.lineage-correlation
risk: high
live: true
provider: mock-acp
preconditions:
  - Fresh AGH_HOME with no pending task_runs.
  - Five sessions registered in the live SessionRegistry, each with
    capability `code` and shared workspace `wsp-aut01`.
  - One queued task_run inserted via `agh task create` (or HTTP) with
    `required_capabilities=[code]` and `scope=workspace`.
docs_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/autonomous/_techspec.md
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/task/lease_manager.go:14
  - /Users/pedronauck/Dev/compozy/agh/internal/task/lease.go:151
  - /Users/pedronauck/Dev/compozy/agh/internal/store/globaldb/global_db.go:331
steps:
  - Spawn 5 goroutines (one per peer session). Each calls `agh task next
    --workspace wsp-aut01 --capability code --output json` simultaneously
    behind a `sync.Barrier`-equivalent (curl pacing or in-process driver).
  - Capture every JSON response.
  - Capture daemon stderr / `events.db` rows for the scenario window.
expected:
  - Exactly one peer receives a non-empty `claim_token` and `run_id`. The
    other four receive `404`/`no_eligible_run` (typed error).
  - Winner's response contains `claim_token` (raw) and `claim_token_hash`.
  - `task_runs` row shows `status=claimed`, `claimed_by_kind=agent_session`,
    `claimed_by_ref=<winner-session-id>`, `claim_token_hash=<sha256>`,
    `lease_until` ≈ now + DefaultRunLeaseDuration.
  - EventStore has exactly one `task.run.claimed` event with the winner's
    `claim_token_hash`. Zero events with raw `agh_claim_*` strings in
    `payload` text.
  - `grep -E 'agh_claim_[A-Za-z0-9_-]{8,}' aut-01-output.log` returns
    nothing — the only allowed appearance is the deliberate `[REDACTED]`
    placeholder produced by `RedactClaimTokens`.
evidence:
  - `aut-01-events.json` filtered to `event_type LIKE 'task.run.%'`.
  - `task_runs` row dumped via `agh debug sql 'SELECT * FROM task_runs
    WHERE id = ?'` (mask raw token field on output).
  - Daemon log fragment showing one `task.run.claimed` log line.
failure_signatures:
  - Two or more winners: `task_runs.claim-authoritative` violated.
  - Raw `agh_claim_<RAW>` appears in any log/SSE/event row:
    `task_runs.claim-token-redaction` violated.
  - `claim_token_hash` missing from event payload: observability gap.
cleanup:
  - Release winner's lease via `agh task release --run-id <id>
    --token <raw>` so the row is requeued; verify
    `claim_token_hash` cleared and `status=queued`.
```

`mock-acp` is the lane here only because race determinism is the test
target; the daemon, scheduler, hooks, and SQLite are all real code paths.
Per the openclaw provider-mode tri-state, this is the only legitimate
mock-mode use case in this child.

### AUT-02 — Lease expiry, sweep reclaim, late-write rejection

```yaml qa-scenario
id: aut-02-lease-expiry-sweep
title: Expired lease is reclaimed by sweep; original holder's late-write hits ErrInvalidClaimToken
theme: autonomy-kernel.lease
coverage:
  primary:
    - task_runs.lease-token-fence
    - task_runs.lease-expiry-sweep
    - scheduler.sweep-recovery
  secondary:
    - hooks.typed-dispatch
    - event.lineage-correlation
risk: high
live: true
provider: real-claude-code
preconditions:
  - One queued task_run, one eligible session.
  - `lease_duration` overridden to 5s via the claim criteria (or daemon
    config) so we don't wait DefaultRunLeaseDuration.
  - Scheduler sweep interval set to 2s for this scenario.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/task/lease_manager.go:259
  - /Users/pedronauck/Dev/compozy/agh/internal/scheduler/scheduler.go:262
  - /Users/pedronauck/Dev/compozy/agh/internal/task/lease.go:178
steps:
  - Peer A claims the run; record raw token + lease_until.
  - Sleep `lease_duration + 3s` so the lease expires.
  - Verify scheduler ran a sweep cycle (`agh scheduler stats` or events
    `task.run.lease_expired` + `task.run.lease_recovered`).
  - Peer B claims the run (now requeued).
  - Peer A attempts `agh task complete --run-id <id> --token <stale-raw>`.
expected:
  - Sweep emits exactly one `task.run.lease_expired` event for the run,
    with `previous_claim_token_hash` redacted to hash form, never raw.
  - The run is now `status=queued`, `claim_token_hash=""`,
    `session_id=""`.
  - Peer B's claim succeeds, new `claim_token_hash`.
  - Peer A's late-write returns `ErrInvalidClaimToken` (HTTP 403 / typed
    error). `task_runs` is NOT mutated by the late call.
  - Two `task.run.claimed` events with distinct `claim_token_hash` values.
evidence:
  - EventStore window dump (`aut-02-events.json`).
  - `task_runs` snapshot at three points: T0 (claimed by A), T1 (sweep
    completed), T2 (claimed by B).
  - Peer A's late `agh task complete` HTTP/UDS response body.
failure_signatures:
  - Sweep does not fire: `scheduler.sweep-recovery` violated.
  - Peer A's late-write succeeds: `task_runs.lease-token-fence` violated;
    constant-time `VerifyClaimToken` in `internal/task/lease.go:179` is
    bypassed.
  - Late-write attempt returns 200 OK with stale token: production bug.
cleanup:
  - Have Peer B `release` the run, archive evidence, stop daemon.
```

### AUT-03 — pre_tool_use hook denies tool dispatch

```yaml qa-scenario
id: aut-03-hook-deny
title: A pre-tool hook returns deny; tool dispatch refused, audit event written, agent receives typed error
theme: autonomy-kernel.hooks
coverage:
  primary:
    - hooks.deny
    - hooks.typed-dispatch
  secondary:
    - hooks.no-bypass
    - event.lineage-correlation
risk: high
live: true
provider: real-claude-code
preconditions:
  - Workspace skill seeded with a hook declaration matching event
    `tool.pre_call` for tool name `fs.write` returning a deny patch.
  - Real Claude Code session active in the workspace.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/hooks/pipeline.go:163-171
  - /Users/pedronauck/Dev/compozy/agh/internal/hooks/events.go:99
  - /Users/pedronauck/Dev/compozy/agh/internal/hooks/matcher.go:249
steps:
  - Prompt Claude Code to write a file: "Create the file
    `qa-aut03.txt` with content `hello`".
  - Capture transcript + EventStore for the prompt window.
expected:
  - `tool.pre_call` event dispatched with `tool_name=fs.write` (or the
    actual canonical name for write-file in the AGH tool registry).
  - Hook outcome `denied`; `denyReason` propagates back to the agent.
  - `fs.write` is NOT executed; no file at `qa-aut03.txt`.
  - Agent receives typed permission-denied error and acknowledges in the
    transcript ("I cannot write the file because policy denies it").
  - Hook run record persisted with `outcome=denied` (per
    `HookRunRecord` in `internal/hooks/types.go:295`).
evidence:
  - Transcript line containing the denial reason.
  - EventStore row for the `tool.pre_call` hook run with `outcome=denied`.
  - Filesystem check: `test ! -e qa-aut03.txt`.
failure_signatures:
  - File is created: hook dispatch is non-authoritative or hook fired
    after the call site.
  - No `tool.pre_call` event recorded: typed-dispatch violated; the call
    site is not dispatching at the right boundary.
cleanup:
  - Remove the seeded workspace skill.
```

### AUT-04 — Hook narrows tool capability set

```yaml qa-scenario
id: aut-04-hook-narrow
title: Hook narrows agent tool set to read-only; subsequent write attempt is rejected at the boundary
theme: autonomy-kernel.hooks
coverage:
  primary:
    - hooks.narrow
    - hooks.no-bypass
  secondary:
    - hooks.typed-dispatch
risk: high
live: true
provider: real-claude-code
preconditions:
  - Workspace skill declaring a `session.pre_create` hook that narrows
    `permission_policy.tools` to a read-only set (excludes `fs.write`).
  - Real Claude Code session created in this workspace.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/hooks/dispatch.go:20
  - /Users/pedronauck/Dev/compozy/agh/internal/coordinator/coordinator.go:64-78
  - /Users/pedronauck/Dev/compozy/agh/internal/store/session_lineage.go
steps:
  - Create the session via `agh sessions create --workspace wsp-aut04`.
  - Verify the resulting `permission_policy.tools` is narrowed.
  - Prompt the agent to write a file.
expected:
  - Session record's effective tool set excludes `fs.write` (and any
    other tool the hook narrowed).
  - Agent's write attempt is denied at the boundary — i.e. the daemon
    rejects the tool call BEFORE invocation, regardless of the agent
    "wanting" to call it.
  - Audit trail records the narrowed permission set as the active
    policy at session creation time.
evidence:
  - Session row showing `permission_policy.tools` narrowed.
  - EventStore row for the rejected write attempt with reason
    `permission_denied`.
  - Transcript line where Claude Code explains the rejection.
failure_signatures:
  - Write succeeds: narrow not honored; downstream caller didn't read
    the patched policy.
  - Hook bypassed safety primitive: critical bug.
cleanup:
  - Stop the session, remove the seeded skill.
```

### AUT-05 — Hook annotates; metadata flows to transcript and SSE

```yaml qa-scenario
id: aut-05-hook-annotate
title: Hook adds metadata field; metadata survives into SSE events and transcript
theme: autonomy-kernel.hooks
coverage:
  primary:
    - hooks.annotate
    - hooks.typed-dispatch
  secondary:
    - event.lineage-correlation
risk: medium
live: true
provider: real-claude-code
preconditions:
  - Workspace skill declaring a `task.run.post_claim` hook that annotates
    the run metadata with `qa.aut05_marker=<random-uuid>`.
  - One queued task_run + eligible session.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/hooks/events.go:118
  - /Users/pedronauck/Dev/compozy/agh/internal/task/lease_manager.go:50
steps:
  - Trigger claim, capture EventStore + SSE stream + transcript.
expected:
  - `task.run.claimed` event payload includes `qa.aut05_marker=<uuid>`.
  - SSE consumer subscribed to the run sees the same marker on the
    `task.run.claimed` event.
  - Coordinator transcript / situation surface reflects the annotated
    metadata where appropriate.
evidence:
  - SSE replay log fragment.
  - EventStore row dump.
failure_signatures:
  - Marker missing from any of the three surfaces: annotation flow
    broken.
cleanup:
  - Remove seeded skill.
```

### AUT-06 — Hook bypass attempt is rejected (cannot release a lease)

```yaml qa-scenario
id: aut-06-hook-bypass-rejected
title: A hook attempts to release a foreign lease; pipeline guard rejects the patch
theme: autonomy-kernel.hooks
coverage:
  primary:
    - hooks.no-bypass
    - task_runs.lease-token-fence
  secondary:
    - hooks.typed-dispatch
risk: critical
live: true
provider: real-claude-code
preconditions:
  - Workspace skill declaring a malicious `task.run.pre_claim` hook that
    returns a patch attempting to release the previous lease for the
    same run, or attempting to forge a `claim_token_hash` field, or
    attempting to set `spawn_depth` to a forbidden value.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md
  - /Users/pedronauck/Dev/compozy/agh/internal/hooks/pipeline.go:148-160
  - /Users/pedronauck/Dev/compozy/agh/internal/task/lease_manager.go:521-571
steps:
  - Pre-seed an active lease on a run held by Peer A.
  - Trigger Peer B's claim attempt; the malicious hook fires at
    `task.run.pre_claim`.
  - Capture EventStore, daemon log, and final `task_runs` state.
expected:
  - Hook outcome `rejected` (the `patchGuard` in
    `internal/hooks/pipeline.go:148-160` returns `ErrHookPatchRejected`
    for fields a hook is not allowed to mutate).
  - Peer A's existing lease is unchanged.
  - Peer B's claim either succeeds normally (with the legitimate
    criteria, ignoring the rejected patch fields) or fails with
    `ErrActiveRunLease` if the lease is still active — but in NO case is
    a release performed by the hook.
  - Hook run record outcome=`rejected` with `error` mentioning the
    forbidden field.
evidence:
  - Hook run record with `outcome=rejected`.
  - `task_runs` snapshot showing Peer A's lease intact at T0 and T1.
failure_signatures:
  - Lease released by the hook: critical safety violation; the patch
    guard is missing or wrong.
  - Hook outcome=`applied` instead of `rejected`: critical security bug.
cleanup:
  - Have Peer A release legitimately. Remove malicious skill.
```

### AUT-07 — Mechanical scheduler wakes idle session WITHOUT calling ClaimNextRun

```yaml qa-scenario
id: aut-07-scheduler-wake-no-claim
title: Scheduler wakes an idle session for a queued run; the wake path itself never claims
theme: autonomy-kernel.scheduler
coverage:
  primary:
    - scheduler.no-claim
    - scheduler.idle-wake
  secondary:
    - task_runs.claim-authoritative
risk: critical
live: true
provider: real-claude-code
preconditions:
  - One idle eligible session (state=active, prompting=false,
    capability matches, workspace matches).
  - One queued task_run with no claimer.
  - Scheduler running with default 15s interval (or override to 3s for
    determinism).
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/scheduler/scheduler.go:238
  - /Users/pedronauck/Dev/compozy/agh/internal/scheduler/scheduler.go:312-403
  - /Users/pedronauck/Dev/compozy/agh/internal/scheduler/scheduler.go:421-466
  - /Users/pedronauck/Dev/compozy/agh/internal/task/lease_manager.go:14
steps:
  - Grep the scheduler binary path / source to verify
    `Scheduler.RunOnce` does not call `Service.ClaimNextRun`. Static
    proof: `grep -n 'ClaimNextRun' internal/scheduler/` returns zero.
  - Wait for one scheduler cycle.
  - Capture EventStore, scheduler stats, and `task_runs` row state.
  - The session, having been woken, calls `agh task next` of its own
    accord. Now `ClaimNextRun` is invoked — by the SESSION's path, not by
    the scheduler.
expected:
  - Scheduler emits `scheduler.wake` log with the session_id + run_id.
  - Between wake and session-driven claim, `task_runs` row remains
    `status=queued`, `session_id=""`, `claim_token_hash=""`. The
    scheduler did NOT mutate the queue.
  - The session's subsequent `agh task next` call is the FIRST and
    ONLY claim attempt against this run.
  - `task_runs` final state: `status=claimed` with the woken session as
    `claimed_by_ref`.
  - One `task.run.claimed` event correlated to the scheduler's
    `scheduler.wake` log line by `run_id`.
evidence:
  - Static grep proof (saved).
  - Scheduler stats `WakeAttempts >= 1`, `WakeSucceeded >= 1`.
  - Time-ordered log: `scheduler.wake` precedes `task.run.claimed`.
failure_signatures:
  - `task_runs` row mutated between wake and session call:
    `scheduler.no-claim` violated.
  - `internal/scheduler` source contains a call to `ClaimNextRun`:
    architectural invariant violated.
cleanup:
  - Release the run, stop daemon.
```

### AUT-08 — Daemon kill -9 mid-claim, restart, sweep recovers without orphan

```yaml qa-scenario
id: aut-08-restart-sweep-no-orphan
title: kill -9 the daemon while a task is leased; on restart, scheduler sweep reclaims, no orphan task_run
theme: autonomy-kernel.scheduler
coverage:
  primary:
    - scheduler.sweep-recovery
    - task_runs.lease-expiry-sweep
  secondary:
    - coordinator.bootstrap
    - event.lineage-correlation
risk: critical
live: true
provider: real-claude-code
preconditions:
  - One claimed run with `lease_duration=10s` and a real Claude Code
    session actively executing tool calls.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/scheduler/scheduler.go:262
  - /Users/pedronauck/Dev/compozy/agh/internal/coordinator/coordinator.go:104
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md (Forensic Bug Fixes "Inactive metadata repair")
steps:
  - Acquire claim, observe live tool calls.
  - `kill -9 $AGH_DAEMON_PID` immediately.
  - Wait `lease_duration + 3s`.
  - Restart daemon (`agh daemon start`).
  - Inspect `task_runs` and EventStore over the recovery window.
expected:
  - On restart, the scheduler's first sweep emits
    `task.run.lease_expired` for the killed run, then
    `task.run.lease_recovered`.
  - `task_runs` row transitions: `running` → (kill) → `running` (stale)
    → `queued` (after sweep). No orphan rows in `claimed`/`running`/
    `starting` after sweep completes.
  - Coordinator does NOT bootstrap a duplicate session for the
    recovered run; the existing coordinator (if alive) picks it up via
    a fresh peer claim.
  - No raw `claim_token` ever logged across the kill/restart window.
evidence:
  - `task_runs` snapshot at T0 (running), T1 (post-kill), T2 (post-sweep).
  - EventStore window dump.
  - Daemon logs across the restart boundary (combined log).
failure_signatures:
  - Stale `running` row left after sweep: orphan; `scheduler.sweep-recovery`
    violated.
  - Two coordinators bootstrap for the same workspace post-restart:
    `coordinator.bootstrap` "exactly once" violated.
  - Raw token in any log line: `task_runs.claim-token-redaction`
    violated.
cleanup:
  - Stop daemon. Archive `events.db`, `agh.db` snapshots.
```

### AUT-09 — Coordinator bootstrap on fresh AGH_HOME

```yaml qa-scenario
id: aut-09-coordinator-bootstrap
title: Fresh AGH_HOME boots coordinator; coordinator subscribes to task_runs and dispatches first work item to a real Claude Code subagent
theme: autonomy-kernel.coordinator
coverage:
  primary:
    - coordinator.bootstrap
    - coordinator.permissions
    - coordinator.lineage
  secondary:
    - manual-equals-peer
    - hooks.typed-dispatch
risk: high
live: true
provider: real-claude-code
preconditions:
  - Brand-new AGH_HOME (no agh.db, no events.db).
  - Coordinator config enabled (`config.coordinator.enabled=true`,
    `default_ttl=1h`, `max_children=3`,
    `max_active_per_workspace=1`).
  - Cross-driver sanity: re-run with OpenClaw and Hermes set as the child
    driver to prove driver-agnostic behavior.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/coordinator/coordinator.go:104,143,193
  - /Users/pedronauck/Dev/compozy/agh/internal/coordinator/coordinator.go:64-78
  - /Users/pedronauck/Dev/compozy/agh/internal/session/spawn.go:215
steps:
  - Initialize workspace `wsp-aut09` and create one workspace-scoped
    task with status=queued and a coordination_channel attached.
  - Wait for daemon to autonomously bootstrap a coordinator session.
  - Coordinator session prompts a real Claude Code child via
    `agh.spawn` (or equivalent).
  - Capture session lineage on both rows; capture coordinator's
    permission policy.
expected:
  - Exactly one coordinator session in `sessions` table for
    `wsp-aut09`. `Decision.Reason == bootstrap`.
  - Coordinator's `permission_policy.tools` matches `ToolAllowlist`
    EXACTLY (no extra tool, no missing tool).
  - Child session row has `parent_session_id=<coordinator-id>`,
    `root_session_id=<coordinator-id>`, `spawn_depth=1`.
  - `SpawnRoleAllowed("coordinator") == false` proven via attempted
    coordinator-of-coordinator spawn (must be rejected).
  - For OpenClaw and Hermes runs: identical `task_runs` lifecycle, only
    `agent_driver` field differs.
evidence:
  - `sessions` table dump showing exactly one coordinator.
  - `session_lineage` rows showing parent/root/depth correlation.
  - Coordinator's permission policy JSON.
  - Three matrix runs: `aut-09-claude-summary.json`,
    `aut-09-openclaw-summary.json`, `aut-09-hermes-summary.json`.
failure_signatures:
  - Two coordinators bootstrap: singleton violated.
  - Coordinator's tool set deviates from `ToolAllowlist`:
    `coordinator.permissions` violated.
  - Child lineage missing parent/root: `coordinator.lineage` violated.
cleanup:
  - Stop coordinator (which reaps children), tear down workspace.
```

### AUT-10 — manual=peer (manual prompt routes through ClaimNextRun)

```yaml qa-scenario
id: aut-10-manual-equals-peer
title: An agent submitting a manual prompt for another agent goes through ClaimNextRun on the same path; no shortcut exists
theme: autonomy-kernel.task_runs
coverage:
  primary:
    - manual-equals-peer
    - task_runs.claim-authoritative
  secondary:
    - hooks.typed-dispatch
    - event.lineage-correlation
risk: high
live: true
provider: real-claude-code
preconditions:
  - Two agent sessions A, B; both eligible for the workspace.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md (manual=peer)
  - /Users/pedronauck/Dev/compozy/agh/internal/task/lease_manager.go:14,501-519
steps:
  - Agent A submits a manual prompt addressed to agent B (via the
    documented agent-to-agent prompt API).
  - Capture the call chain via daemon log + EventStore.
expected:
  - A `task_runs` row is created for the manual prompt, queued for B's
    capability set, with `actor_kind=agent_session`,
    `actor_ref=<agent-A>`.
  - B's session calls `ClaimNextRun` (via `agh task next` or its driver
    equivalent). Same code path, same hook chain, same audit trail as
    any other peer claim.
  - There is NO direct prompt-injection shortcut: search the daemon's
    code path; the manual prompt does not call `Manager.Prompt` or
    similar bypass before going through `task_runs`.
  - Hooks `task.run.pre_claim` and `task.run.post_claim` fire as for any
    other claim.
evidence:
  - Static grep: no path from "manual prompt API" to the session manager
    that skips `task_runs`. Manual = `task_runs` row + `ClaimNextRun`.
  - EventStore window: `task.run.enqueued` → `task.run.claimed` →
    `task.run.completed` chain identical to non-manual claims.
failure_signatures:
  - Manual prompt arrives at agent B's stdin without a `task_runs` row:
    shortcut path exists; manual=peer violated.
  - Hook chain partially fired: typed-dispatch incomplete on manual
    path.
cleanup:
  - Cancel any leftover task_run.
```

### AUT-11 — Spawn cap enforcement (depth + max_children)

```yaml qa-scenario
id: aut-11-spawn-cap
title: SpawnDepth > MaxDepth and MaxChildren overflow are rejected with typed errors
theme: autonomy-kernel.coordinator
coverage:
  primary:
    - spawn.cap
    - coordinator.lineage
  secondary:
    - hooks.no-bypass
risk: high
live: true
provider: real-claude-code
preconditions:
  - Coordinator with `max_children=2`, `max_depth=1`.
  - One real Claude Code child already alive (spawn_depth=1).
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/store/session_lineage.go:127-140
  - /Users/pedronauck/Dev/compozy/agh/internal/session/spawn.go:215-237
  - /Users/pedronauck/Dev/compozy/agh/internal/coordinator/coordinator.go:202-208
steps:
  - Coordinator attempts to spawn child #2 (allowed, total=2).
  - Coordinator attempts child #3 (must be rejected — `MaxChildren`).
  - Child #1 attempts to spawn its own child (`spawn_depth=2`,
    must be rejected — `MaxDepth=1`).
  - Capture rejection error type + EventStore.
expected:
  - Both rejections come back with typed errors (depth-cap and
    children-cap), not generic 500s.
  - `sessions` table never contains a row with `spawn_depth > 1` or
    children-count > 2 for this lineage.
  - EventStore records `spawn.pre_create` events with a deny outcome
    where the cap was hit.
evidence:
  - Typed error responses captured.
  - `session_lineage` snapshot proving cap.
failure_signatures:
  - Spawn succeeds beyond cap: `spawn.cap` violated; safety primitive
    bypassed.
cleanup:
  - Reap children, stop coordinator.
```

### AUT-12 — Lineage emission coverage matrix

```yaml qa-scenario
id: aut-12-lineage-coverage
title: Every task_run lifecycle event emits parent_session_id, root_session_id, claim_token_hash
theme: autonomy-kernel.observability
coverage:
  primary:
    - lineage.coverage
    - event.lineage-correlation
  secondary:
    - task_runs.claim-token-redaction
risk: high
live: true
provider: real-claude-code
preconditions:
  - One full task lifecycle: enqueue → claim → heartbeat → complete (or
    fail), spawned by a coordinator with one child.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md (Observability bullet)
  - /Users/pedronauck/Dev/compozy/agh/internal/task/lease_manager.go:41-89,118-256
  - /Users/pedronauck/Dev/compozy/agh/internal/store/session_lineage.go:13-83
steps:
  - Drive the full lifecycle via real Claude Code + coordinator.
  - For each emitted event, parse the JSON payload and check that
    `parent_session_id`, `root_session_id`, and (where applicable)
    `claim_token_hash` are present and non-empty.
expected:
  - Coverage matrix passes for every event type:
    `task.run.enqueued`, `task.run.pre_claim`, `task.run.post_claim`,
    `task.run.claimed`, `task.run.lease_extended`,
    `task.run.released`, `task.run.completed`, `task.run.failed`,
    `task.run.lease_expired`, `task.run.lease_recovered`,
    `spawn.pre_create`, `spawn.created`, `spawn.parent_stopped`,
    `spawn.ttl_expired`, `spawn.reaped`.
  - Every event has `claim_token_hash` (where the run is in a leased
    state) or the field is omitted (queued/terminal); raw token is
    NEVER present.
evidence:
  - Coverage-matrix table in `aut-12-summary.json`:
    `{event_type, parent_session_id_present, root_session_id_present,
    claim_token_hash_present_or_n_a}`.
failure_signatures:
  - Any required correlation key missing on any event: observability gap;
    fail.
  - Raw `agh_claim_*` in any payload: redaction violated.
cleanup:
  - Stop coordinator.
```

### AUT-13 — Hook ordering and required-vs-non-required semantics

```yaml qa-scenario
id: aut-13-hook-ordering
title: Multiple hooks at one event fire in declared order; required failure halts; non-required failure continues
theme: autonomy-kernel.hooks
coverage:
  primary:
    - hooks.ordering
    - hooks.required-fail-closed
    - hooks.timeout-fail-open
  secondary:
    - hooks.typed-dispatch
risk: medium
live: true
provider: real-claude-code
preconditions:
  - Three hooks declared at `tool.pre_call` with distinct names and
    deterministic order (per `internal/hooks/ordering.go` precedence
    rules):
    - Hook A: `mode=sync`, `required=false`, returns `applied`.
    - Hook B: `mode=sync`, `required=false`, returns `failed` (panic or
      exit non-zero).
    - Hook C: `mode=sync`, `required=true`, returns `applied`.
  - Second variant: swap B's `required` to `true` and re-run.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/hooks/pipeline.go:59-92
  - /Users/pedronauck/Dev/compozy/agh/internal/hooks/ordering.go
  - /Users/pedronauck/Dev/compozy/agh/internal/hooks/executor_subprocess.go:23,224
steps:
  - Trigger one tool call; capture trace.
  - Re-run with B promoted to required; trigger same tool call.
  - Re-run with a hook D at the same event whose subprocess sleeps
    longer than `defaultSubprocessHookTimeout=5s`.
expected:
  - Variant 1: A applies, B fails (logged, dispatch continues), C
    applies; tool call proceeds. Hook run records show `applied,
    failed, applied` in order.
  - Variant 2: A applies, B fails (required), pipeline halts with
    wrapped error `hooks: required hook %q failed for event %q: %w`
    (per `internal/hooks/pipeline.go:71-77`); C never fires.
  - Variant 3 (timeout): D's run record shows `outcome=failed` with
    `error` wrapping `context.DeadlineExceeded`; non-required → dispatch
    proceeds.
evidence:
  - Hook trace ordering captured in event payload `report.Trace`.
  - Pipeline error type matches the documented wrapped form.
failure_signatures:
  - Order is non-deterministic across runs: ordering violated.
  - Required hook failure does not halt: critical safety bug.
  - Non-required timeout halts dispatch: fail-open violated.
cleanup:
  - Remove seeded hooks.
```

### AUT-14 — task_runs is the only authority (static + runtime)

```yaml qa-scenario
id: aut-14-only-authority
title: No peer package replicates task_runs claim logic; static + runtime checks
theme: autonomy-kernel.task_runs
coverage:
  primary:
    - task_runs.single-queue
    - task_runs.claim-authoritative
  secondary:
    - scheduler.no-claim
risk: critical
live: true
provider: real-claude-code
preconditions:
  - Repo at the SUT commit, a fresh checkout.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md (Authoritative primitives)
  - /Users/pedronauck/Dev/compozy/agh/internal/task/lease_manager.go:14
  - /Users/pedronauck/Dev/compozy/agh/internal/task/lease.go
steps:
  - Static heuristics:
    - `rg -n 'UPDATE\s+task_runs\s+SET\s+claimed_by' --glob '!internal/task/**'` returns zero hits.
    - `rg -n 'UPDATE\s+task_runs\s+SET\s+claim_token_hash' --glob '!internal/task/**'` returns zero hits.
    - `rg -n 'INSERT\s+INTO\s+task_runs.*claimed_by' --glob '!internal/task/**'` returns zero hits.
    - `rg -n 'ClaimNextRun' --glob 'internal/scheduler/**'` returns zero hits.
    - `rg -n 'agh_claim_[A-Za-z0-9_-]{8,}' --glob '!internal/task/lease.go' --glob '!**/*_test.go'`
      returns zero hits in production code (other than the redaction
      regex itself in `internal/task/lease.go:151-166`).
  - Runtime: under load, only `Service.ClaimNextRun` increments the
    `claim_token_hash` column for any row. Tail SQLite via WAL:
    `agh debug sql 'SELECT id, claim_token_hash, claimed_by_kind FROM
    task_runs ORDER BY id DESC LIMIT 50'` snapshots before/after
    several real prompts.
expected:
  - All five `rg` checks pass with zero hits.
  - Runtime: every `task_runs` row's `claim_token_hash` change is
    explained by a `task.run.claimed` / `task.run.released` event
    with `actor` traceable to `Service.ClaimNextRun` (or the lease
    primitive owning that transition).
evidence:
  - `aut-14-static-grep.txt` capturing the five rg outputs.
  - Diff of `task_runs` snapshots correlated to EventStore rows.
failure_signatures:
  - Any `rg` hit outside `internal/task/`: peer claim replication;
    architectural rule violated.
  - `task_runs` row mutated without a corresponding event: silent
    write; trust broken.
cleanup:
  - None (read-only static + runtime audit).
```

### AUT-15 — Real-LLM end-to-end with cron-driven re-fire (overlap with module 09)

```yaml qa-scenario
id: aut-15-real-llm-end-to-end
title: Real Claude Code spawns child via agh sessions spawn; cron fires same path next minute; full lineage in transcript
theme: autonomy-kernel.end-to-end
coverage:
  primary:
    - coordinator.bootstrap
    - coordinator.lineage
    - task_runs.claim-authoritative
    - lineage.coverage
  secondary:
    - hooks.typed-dispatch
    - scheduler.idle-wake
risk: high
live: true
provider: real-claude-code
preconditions:
  - Cron job seeded that fires every minute and enqueues a workspace
    task with payload "summarize the last hour of activity".
  - Coordinator running.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/dispatch.go
  - /Users/pedronauck/Dev/compozy/agh/internal/coordinator/coordinator.go
  - /Users/pedronauck/Dev/compozy/agh/internal/scheduler/scheduler.go:238
steps:
  - Wait for one cron fire; let coordinator dispatch to a real Claude
    Code child via `agh sessions spawn`.
  - Verify the child claims its `task_runs` row.
  - Verify the child completes; `task.run.completed` emitted.
  - Wait for next minute; cron fires again; verify the SAME path is
    used (no second coordinator, no shortcut).
evidence:
  - Two complete lifecycle traces (cron fire #1 and #2) with
    identical event sequences.
  - Transcript on the child session showing the prompt was received
    via the standard task-claim path (not a side channel).
expected:
  - Both lifecycle traces are identical in shape.
  - Lineage chain: cron-trigger → task_run → coordinator → child;
    every node has parent/root/depth metadata.
  - No raw token leaks across either lifecycle.
failure_signatures:
  - Second cron fire takes a different code path: shortcut bug.
  - Lineage broken on second run: state-machine regression.
cleanup:
  - Disable cron, reap children, stop coordinator.
```

### AUT-16 — Claim-token redaction grep audit across every surface

```yaml qa-scenario
id: aut-16-token-redaction-audit
title: Across logs, SSE, web responses, and SQLite rows produced during a real run, NO raw agh_claim_* appears
theme: autonomy-kernel.security
coverage:
  primary:
    - task_runs.claim-token-redaction
  secondary:
    - lineage.coverage
risk: critical
live: true
provider: real-claude-code
preconditions:
  - One full task lifecycle (claim, heartbeat, release/complete) over
    real Claude Code, with 3+ minutes of activity.
  - All four output sinks captured: combined daemon log, SSE replay
    log (`agh sse replay`), web API responses (every documented
    endpoint that returns task_run state), `agh.db` + `events.db`
    dumps.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md (Security Invariants)
  - /Users/pedronauck/Dev/compozy/agh/internal/task/lease.go:151-166
  - /Users/pedronauck/Dev/compozy/agh/internal/task/lease_manager.go:35
steps:
  - Drive the lifecycle.
  - Run the audit:
    `rg -n 'agh_claim_[A-Za-z0-9_-]{12,}' aut-16-daemon.log aut-16-sse-replay.jsonl aut-16-web-responses.jsonl <(sqlite3 agh.db .dump) <(sqlite3 events.db .dump)`
expected:
  - Audit output is empty. The only legitimate "agh_claim_" hit anywhere
    is the placeholder `agh_claim_[REDACTED]` produced by
    `RedactClaimTokens` (per `internal/task/lease.go:160-166`) — the
    audit grep MUST NOT match the redaction placeholder because of the
    `{12,}` random-payload length floor.
evidence:
  - Audit output saved as `aut-16-redaction-audit.txt` (must be empty).
  - All four sinks attached as evidence so a reviewer can re-run the
    grep.
failure_signatures:
  - Any hit (other than the deliberate placeholder, which the regex
    skips by length): critical security violation; the run cannot ship.
cleanup:
  - Archive sinks, stop daemon.
```

## 8. Optional / nice-to-have scenarios (run if time)

These extend coverage without being strictly required for ship.

### AUT-17 — Hook async-mode panic does not affect sync chain

```yaml qa-scenario
id: aut-17-async-hook-panic
title: An async-mode hook that panics is contained; the sync dispatch chain is unaffected
theme: autonomy-kernel.hooks
coverage:
  primary:
    - hooks.no-bypass
  secondary:
    - hooks.timeout-fail-open
risk: medium
live: false
provider: mock-acp
preconditions:
  - One hook at `event.post_record` with `mode=async` and a body that
    panics.
  - One hook at `tool.pre_call` with `mode=sync` and `required=true`
    that does meaningful work.
steps:
  - Trigger a tool call; capture EventStore + log.
expected:
  - Async panic is contained per `internal/hooks/dispatch_async.go`;
    sync chain completes normally.
  - Async hook run record has `outcome=failed` with the recovered panic
    message.
evidence:
  - Daemon log shows `recovered async hook panic` with hook name.
failure_signatures:
  - Daemon crash: panic not contained.
cleanup:
  - Remove seeded hooks.
```

### AUT-18 — Reconcile cascade after partial failure

```yaml qa-scenario
id: aut-18-reconcile-cascade
title: After a mid-flight failure between store mutation and event emission, reconcile recovers consistent state
theme: autonomy-kernel.task_runs
coverage:
  primary:
    - task_runs.claim-authoritative
  secondary:
    - lineage.coverage
risk: medium
live: true
provider: real-claude-code
preconditions:
  - Inject a fault between `store.ClaimNextRun` and `recordTaskEvent`
    using a fault-injection hook (test-only build tag).
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/task/lease_manager.go:31-54
steps:
  - Trigger the claim path with the fault enabled.
  - On daemon restart, observe whether the run is consistent: either
    fully claimed with event present, or rolled back to queued.
expected:
  - State after restart is consistent — no half-state where
    `claim_token_hash` is set but no `task.run.claimed` event exists,
    or vice versa.
evidence:
  - Snapshot diff showing the store + EventStore agree.
failure_signatures:
  - Half-state visible: cascade not idempotent.
cleanup:
  - Disable fault, restart cleanly.
```

## 9. Coverage matrix (this child)

| Coverage ID                       | Scenarios                                                                                                              |
| --------------------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| `task_runs.single-queue`          | AUT-01, AUT-10, AUT-14                                                                                                 |
| `task_runs.claim-authoritative`   | AUT-01, AUT-02 (indirect), AUT-10, AUT-14, AUT-15                                                                      |
| `task_runs.claim-token-redaction` | AUT-01, AUT-02, AUT-08, AUT-12, AUT-16                                                                                 |
| `task_runs.lease-token-fence`     | AUT-02, AUT-06                                                                                                         |
| `task_runs.lease-expiry-sweep`    | AUT-02, AUT-08                                                                                                         |
| `scheduler.no-claim`              | AUT-07, AUT-14                                                                                                         |
| `scheduler.sweep-recovery`        | AUT-02, AUT-08                                                                                                         |
| `scheduler.idle-wake`             | AUT-07, AUT-15                                                                                                         |
| `hooks.typed-dispatch`            | AUT-01, AUT-03, AUT-04, AUT-05, AUT-06, AUT-10, AUT-13                                                                 |
| `hooks.deny`                      | AUT-03                                                                                                                 |
| `hooks.narrow`                    | AUT-04                                                                                                                 |
| `hooks.annotate`                  | AUT-05                                                                                                                 |
| `hooks.no-bypass`                 | AUT-04, AUT-06, AUT-11, AUT-17                                                                                         |
| `hooks.timeout-fail-open`         | AUT-13, AUT-17                                                                                                         |
| `hooks.required-fail-closed`      | AUT-13                                                                                                                 |
| `hooks.ordering`                  | AUT-13                                                                                                                 |
| `coordinator.bootstrap`           | AUT-08, AUT-09, AUT-15                                                                                                 |
| `coordinator.permissions`         | AUT-09                                                                                                                 |
| `coordinator.lineage`             | AUT-09, AUT-11, AUT-15                                                                                                 |
| `manual-equals-peer`              | AUT-09, AUT-10                                                                                                         |
| `lineage.coverage`                | AUT-12, AUT-15, AUT-16                                                                                                 |
| `spawn.cap`                       | AUT-11                                                                                                                 |
| `event.lineage-correlation`       | AUT-01, AUT-02, AUT-05, AUT-08, AUT-10, AUT-12                                                                         |

Total: 16 mandatory + 2 optional = 18 scenarios. Every coverage ID is
exercised by at least two scenarios.

## 10. Forbidden-needle list (transcript and event payloads)

Per the openclaw `forbiddenNeedles` pattern. None of the following may
appear in any outbound message, transcript, SSE event, or audit log
across any AUT scenario:

- Any literal raw `agh_claim_<>=12 random char>` (regex
  `agh_claim_[A-Za-z0-9_-]{12,}`).
- Any provider API key shape: `sk-`, `xoxb-`, `AKIA`, `ya29.`.
- Any of the hard-banned phrases for hook bypass evidence:
  `bypass safety`, `release foreign lease`, `forge claim_token`.
- Any reference to the deleted legacy `recipe`/`workflow`/`procedure`
  vocabulary in coordinator/agent prompts (per `docs/_memory/glossary.md`
  — canonical term is `capability`).

A single scenario test failure on this list is shippability-critical and
must be triaged immediately.

## 11. Reporting contract

Each scenario writes the four-artifact set required by the openclaw
operator-flow pattern (markdown report + JSON summary + observed events
+ combined log). The aggregate `aut-summary.json` for this child carries
the coverage matrix from §9 alongside per-scenario `outcome ∈ {worked,
failed, blocked, follow-up}` and machine-readable timing.

The scenario operator runs in-character (per the `real-scenario-qa`
skill); every run ends with a Worked / Failed / Blocked / Follow-up
section covering all 16 mandatory scenarios. A child run is shippable
only when:

- Every mandatory scenario is `worked` or has an explicit accepted
  follow-up.
- AUT-14 + AUT-16 are both clean (the static + runtime claim-token
  redaction audits are non-negotiable).
- No forbidden-needle hit anywhere.
- `make verify` passed on the SUT branch before this child ran (cite
  commit SHA in `aut-summary.json`).
