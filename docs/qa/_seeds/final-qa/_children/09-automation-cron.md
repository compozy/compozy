---
name: 09-automation-cron
description: AGH pre-release QA — automation, cron, webhook triggers, and the durable scheduler cursor. Real-LLM scenarios required. Read-only research deliverable.
type: qa-child
module: automation-cron
owner: pre-release-qa
references:
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/openclaw-qa-patterns.md
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/hermes-qa-patterns.md
  - /Users/pedronauck/Dev/compozy/agh/CLAUDE.md
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md
  - /Users/pedronauck/Dev/compozy/agh/.resources/openclaw/qa/scenarios/scheduling/cron-natural-fire-no-duplicate.md
  - /Users/pedronauck/Dev/compozy/agh/.resources/openclaw/qa/scenarios/scheduling/cron-single-run-no-duplicate.md
  - /Users/pedronauck/Dev/compozy/agh/.resources/openclaw/qa/scenarios/scheduling/cron-one-minute-ping.md
---

# 09 — Automation, Cron, Webhooks & Scheduling Triggers QA

## 1. Module scope

This child stresses every documented invariant of the automation subsystem:
the durable cron/`every`/`at` scheduler, webhook ingress (signed payload +
replay window), event-driven trigger engine (session/hook/memory/extension
sources), the shared dispatcher (concurrency gate, fire-limits, retries,
hooks), and the lifecycle of automation sessions. Module 04 covers the
mechanical scheduler / idle registry; here we exercise the AUTOMATION
scheduler that owns `task_runs.lease`-equivalent cursor state for jobs.

Packages in scope (file:line citations are repo-absolute):

| Surface                | Path                                                                              | Authoritative API                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| ---------------------- | --------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Scheduler runtime      | `/Users/pedronauck/Dev/compozy/agh/internal/automation/schedule.go`               | `Scheduler.Start` (`:166`), `Stop` (`:197`), `Register` (`:255`), `Update` (`:278`), `Unregister` (`:345`), `executeScheduledJob` (`:593`), `buildSchedulePlan` (`:499`), `reconcileSchedulerState` (`:735`), `nextRunAfter` (`:854`), `nextRunAfterMissed` (`:885`), `predictNextRun` (`:824`), `scheduledFireID` (`:921`), `scheduleHash` (`:934`), `defaultSchedulerStopTimeout = 10s` (`:28`)                                                                                |
| Dispatcher             | `/Users/pedronauck/Dev/compozy/agh/internal/automation/dispatch.go`               | `Dispatcher.Dispatch` (`:366`), `dispatchAttempt` (`:412`), `reserveRun` (`:488`), `evaluateFireLimit` (`:554`), `dispatchTaskBackedAttempt` (`:624`), `transitionRun` (`:684`), `finishRunAfterSessionStop` (`:785`), `dispatchPreFireHook` (`:827`), `collectPromptError` (`:1259`), `defaultDispatcherSessionStopTimeout = 10s` (`:59`), `DefaultMaxConcurrentJobs = 5` (`internal/automation/model/types.go:13`)                                                            |
| Trigger engine         | `/Users/pedronauck/Dev/compozy/agh/internal/automation/trigger.go`                | `TriggerEngine.Register` (`:279`), `Update` (`:306`), `Unregister` (`:334`), `Fire` (`:356`), `FireSessionCreated`/`Stopped` (`:372,381`), `FireMemoryConsolidated` (`:390`), `FireHookCompletion` (`:402`), `HandleWebhook` (`:418`), `claimWebhookDelivery` (`:736`), `purgeDeliveriesLocked` (`:764`), `ParseWebhookEndpoint` (`:478`), `SignWebhookPayload` (`:521`), `ValidateWebhookSignature` (`:538`), `ValidateWebhookTimestamp` (`:559`), `DefaultWebhookFreshnessWindow = 5m` (`:52`) |
| Manager / composition  | `/Users/pedronauck/Dev/compozy/agh/internal/automation/manager.go`                | `Manager.Start` / `Shutdown` / `Status`, `buildSchedulerRuntime` (`:1380`), `buildTriggerRuntime` (`:1399`), `loadSchedulerRegistrations` (`:1413`), `loadTriggerRegistrations` (`:1422`), `syncConfigDefinitions` (`:1442`), `SyncManagedDefinitions` (`:1466`)                                                                                                                                                                                                                |
| Webhook ingress (HTTP) | `/Users/pedronauck/Dev/compozy/agh/internal/api/core/automation.go`               | `DeliverGlobalWebhook` (`:493`), `DeliverWorkspaceWebhook` (`:498`), `deliverWebhook` (`:502`), `webhookRequestFromHTTP` (`:695`), `WebhookTimestampHeader` / `WebhookSignatureHeader` / `WebhookDeliveryIDHeader` (`:22-30`), `maxWebhookPayloadSize = 1<<20` (`:32`), `http.MaxBytesReader` enforcement (`:727`)                                                                                                                                                               |
| HTTP routes            | `/Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/routes.go`                | Automation API (`:168-191`), webhook endpoints (`:345-349`: `POST /api/webhooks/global/:endpoint`, `POST /api/webhooks/workspaces/:workspace_id/:endpoint`)                                                                                                                                                                                                                                                                                                                    |
| CLI                    | `/Users/pedronauck/Dev/compozy/agh/internal/cli/automation.go`                    | `agh automation jobs|triggers|runs` verb tree (`:43-757`)                                                                                                                                                                                                                                                                                                                                                                                                                  |
| Config (TOML)          | `/Users/pedronauck/Dev/compozy/agh/internal/config/automation.go`                 | `AutomationConfig` (`:14`), `AutomationJob` (`:24`), `AutomationTrigger` (`:39`), `Validate` (`:93`), default timezone defaulted to `UTC` (`internal/automation/model/types.go:10`), `DefaultMaxConcurrentJobs = 5` (`internal/automation/model/types.go:13`)                                                                                                                                                                                                                  |
| Cron parser            | `gocron.NewDefaultCron(false)` (`internal/automation/schedule.go:507,832,863,894`) | `github.com/go-co-op/gocron/v2 v2.20.0` (`go.mod:13`), `robfig/cron/v3 v3.0.1` is the underlying parser (`go.mod:28`)                                                                                                                                                                                                                                                                                                                                                          |

Out of scope (covered by other children): mechanical scheduler / idle wake
(module 04), full coordinator bootstrap (module 04), AGH Network channel
transport (module 06), session manager state machine (module 03).

## 2. Authoritative invariants under test

These come straight from the implementation and `internal/CLAUDE.md`. Every
scenario below maps to one or more of these IDs. Coverage IDs follow the
openclaw lowercase dotted/dashed convention.

| Coverage ID                          | Invariant                                                                                                                                                            | Source                                                                                                                                                                                                                  |
| ------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `cron.next-fire-deterministic`       | Cron / `every` / `at` modes share one `buildSchedulePlan` deterministic next-fire computation; cursor is hashed with `scheduleHash` so config changes invalidate it. | `internal/automation/schedule.go:499-540,934-945`                                                                                                                                                                       |
| `cron.no-duplicate-per-window`       | Each scheduled fire is identified by `scheduledFireID(jobID, scheduledAt UTC)` and claimed atomically through `ClaimScheduledRun`; a duplicate claim is rejected.    | `internal/automation/schedule.go:921-932`, `:619-629`; `ErrScheduledFireAlreadyClaimed` in `internal/automation/persistence.go:14-15`                                                                                   |
| `cron.restart-resumes-pending`       | After daemon restart, the durable cursor in `SchedulerState.NextRunAt` is the source of truth; `predictNextRun` is only a fallback when no durable row exists.       | `internal/automation/schedule.go:464-476,735-800`                                                                                                                                                                       |
| `cron.restart-no-duplicate`          | If a daemon restart crosses a fire boundary, the rebuilt scheduler reads the existing `LastFireID` and won't re-claim a fire id already persisted.                   | `TestSchedulerRestartAfterClaimDoesNotDuplicateAlreadyClaimedFire` in `internal/automation/schedule_test.go:322-388`                                                                                                    |
| `cron.skip-policy`            | Missed cron fires reconcile via `SchedulerCatchUpPolicySkipMissed` — misfires are recorded (`MisfireCount++`, `LastMisfireAt`), never replayed.                      | `internal/automation/schedule.go:782-797`, `internal/automation/model/types.go:81-85`                                                                                                                                   |
| `cron.timezone-respected`            | Scheduler uses `WithSchedulerLocation(time.LoadLocation(config.Timezone))`; `config.Timezone` is required and validated.                                             | `internal/automation/manager.go:1380-1396`, `internal/config/automation.go:97-103`                                                                                                                                      |
| `cron.dst-no-double-fire`            | DST fall-back is handled by the underlying cron parser (`robfig/cron/v3`) — `cronImpl.Next` returns the next absolute time, never both occurrences of an ambiguous wall time. | `internal/automation/schedule.go:506-515,854-883`; gocron-v2 wraps robfig                                                                                                                                                |
| `at.past-rejected-as-skip`           | `ScheduleModeAt` with `time <= now` returns `schedulePlan{register: false}`; the job is skipped (logged) and not registered.                                         | `internal/automation/schedule.go:525-535`, `:417-427`                                                                                                                                                                   |
| `webhook.signature-required`         | HMAC-SHA256 signature with `sha256=<hex>` prefix is mandatory; `ValidateWebhookSignature` uses `hmac.Equal` constant-time compare.                                   | `internal/automation/trigger.go:521-555`                                                                                                                                                                                |
| `webhook.timestamp-window`           | `DefaultWebhookFreshnessWindow = 5m`; deltas outside the window return `ErrWebhookTimestampInvalid`.                                                                 | `internal/automation/trigger.go:51-52,559-578`                                                                                                                                                                          |
| `webhook.replay-protected`           | `claimWebhookDelivery(triggerID, deliveryID)` rejects duplicates inside the freshness window and self-purges entries past expiry.                                    | `internal/automation/trigger.go:736-770`                                                                                                                                                                                |
| `webhook.body-size-limit`            | HTTP webhook ingress wraps body in `http.MaxBytesReader(c.Writer, body, 1<<20)`; oversize returns a typed `webhook request body exceeds %d bytes` error.             | `internal/api/core/automation.go:32,727-737`                                                                                                                                                                            |
| `webhook.endpoint-format`            | Endpoint path segment must be `<slug>--<webhook_id>` with `webhook_id` prefixed `wbh_`; otherwise `ErrWebhookEndpointInvalid`.                                       | `internal/automation/trigger.go:478-508`                                                                                                                                                                                |
| `webhook.secret-required`            | Webhook triggers require a `WebhookSecretRef` in the `automation.*` vault namespace; `validateWebhookRegistration` rejects missing/wrong-namespaced refs.            | `internal/automation/trigger.go:689-704`, `:664-687`                                                                                                                                                                    |
| `dispatch.concurrency-gate`          | Shared semaphore at `DefaultMaxConcurrentJobs = 5` (configurable via `automation.max_concurrent_jobs`); over-limit returns `ErrConcurrencyLimitReached`.             | `internal/automation/dispatch.go:295,412-420`, `internal/automation/model/types.go:13`                                                                                                                                  |
| `dispatch.fire-limit`                | `evaluateFireLimit` counts non-cancelled runs in a rolling window; over-limit returns `FireLimitError{RetryAt}`; scheduler reschedules at `RetryAt`.                 | `internal/automation/dispatch.go:554-622`, `internal/automation/schedule.go:682-716`                                                                                                                                    |
| `dispatch.lifecycle-events`          | Run state machine: `scheduled → running|delegated → completed|failed|cancelled`; lifecycle hooks fire on every terminal transition.                                  | `internal/automation/dispatch.go:441-486,727-807`, `:880-977`                                                                                                                                                           |
| `dispatch.failure-cron-continues`    | A failed cron-fired prompt does not unregister the cron; the cursor advances to the next fire regardless of run outcome.                                             | `internal/automation/schedule.go:644-668`; cursor advanced before dispatch attempt at `:619-642`                                                                                                                        |
| `dispatch.session-stop-budget`       | Automation-spawned sessions are stopped within `defaultDispatcherSessionStopTimeout = 10s` after the run completes/fails/cancels; `context.WithoutCancel`-based stop. | `internal/automation/dispatch.go:59,809-825`                                                                                                                                                                            |
| `trigger.lineage-correlation`        | Cron- and trigger-fired sessions are `SessionTypeSystem`; manager records `RecordAutomationSessionTaskActor` so any task they spawn carries trusted automation provenance. | `internal/automation/dispatch.go:984-1006`, `internal/task` actor derivation                                                                                                                                              |
| `automation.cli-http-parity`         | `agh automation list -o json` and `GET /api/automation/jobs|triggers|runs` return the same shape; both consume `BaseHandlers` per `internal/api/core/automation.go`. | `internal/cli/automation.go:43-757`, `internal/api/httpapi/routes.go:168-191`, `internal/api/core/automation.go`                                                                                                        |
| `enable-disable.live`                | Disable mid-window: in-flight session completes; future fires suppressed via `Update(job, Enabled=false)` → `unregisterLocked` + `deleteSchedulerState`.             | `internal/automation/schedule.go:298-303,344-364`                                                                                                                                                                       |
| `webhook.unsigned-rejected`          | Missing `X-AGH-Webhook-Signature` header returns 400 before reaching trigger engine; missing `X-AGH-Webhook-Delivery-ID` likewise.                                   | `internal/api/core/automation.go:714-725`                                                                                                                                                                               |
| `extensibility.agent-manageable`     | All automation surfaces are exposed via CLI verbs + HTTP endpoints (no UI-only path); webhook delivery has CLI helper or the HTTP endpoint is documented for agents. | `internal/cli/automation.go`, `internal/api/httpapi/routes.go:168-191,345-349`; CLAUDE.md "Agent-manageable by default."                                                                                                |

## 3. Operating model

QA mode is **real-scenario** (per the standing directive on real-scenario
QA), not pytest-style assertions. Every scenario:

- Runs against an isolated AGH_HOME with unique daemon ports + tmux-bridge
  socket (per the `agh-worktree-isolation` skill).
- Resolves provider auth from the bootstrap manifest according to each
  provider contract: bound-secret, brokered, and explicitly isolated-home
  lanes use `PROVIDER_HOME` / `PROVIDER_CODEX_HOME`, while `native_cli`
  lanes with `home_policy=operator` preserve the operator `HOME` unless the
  scenario explicitly validates isolated provider-home behavior.
- Uses real Claude Code (`claude-opus-4-8` or `claude-sonnet-5` per
  scenario) as the subprocess agent driver. Cron-driven prompts hit the real
  driver, and the transcript proves the prompt fired on the cadence the
  scheduler claims it did.
- Emits four artifacts under `.artifacts/qa/<run-id>/crn-XX/`:
  - `crn-XX-report.md` (Worked / Failed / Blocked / Follow-up)
  - `crn-XX-summary.json` (machine-readable)
  - `crn-XX-events.json` (`automation_runs` rows + `automation.scheduler.*`
    log lines + emitted lifecycle hook payloads)
  - `crn-XX-output.log` (combined stdout/stderr)
- Asserts against the `automation_runs` table state, the
  `automation_scheduler_state` durable cursor, the daemon's structured log
  output, and (where applicable) the spawned session's transcript — never
  just process exit codes.

Scenarios are numbered `CRN-01..CRN-NN`; each is a fenced `qa-scenario`
block. Reproduce by running them sequentially or in parallel under unique
worktree isolation.

## 4. Provider matrix

| Mode                | When                                                                                                                | Driver                                                                                                                          |
| ------------------- | ------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| `real-claude-code`  | Default for every scenario where the cron-fired prompt must reach a real LLM and the transcript is the proof.      | `claude-opus-4-8` (parent automation prompts); `claude-sonnet-5` for cheap cron-loop scenarios (CRN-01).                  |
| `mock-acp` (gate)   | Determinism gate for race-sensitive scenarios where real models add nondeterminism (CRN-12, CRN-15).               | `internal/e2elane` mock ACP server used only to stabilize a race; the surrounding daemon, scheduler, and dispatcher are real.   |

`mock-acp` is the AGH equivalent of openclaw `mock-openai`; `real-claude-code`
is the AGH equivalent of openclaw `live-frontier`. Per openclaw's tri-state,
we do not include an `aimock` lane (additive only).

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
- `automation.enabled = true`, `automation.timezone = "UTC"` (unless the
  scenario explicitly overrides), `automation.max_concurrent_jobs >= 5`.
- Host clock synchronized to within ±1s of UTC (NTP in sync). Several
  scenarios assert minute-aligned cron firings; large clock skew is
  shippability-relevant and should be flagged in the report.

Provider-specific config:

```text
AGH_HOME=$HOME/.qa/crn-09/<scenario>/agh-home
AGH_DAEMON_HTTP=127.0.0.1:<unique-port>
AGH_DAEMON_UDS=$AGH_HOME/sock/uds.sock
PROVIDER_HOME=$AGH_HOME/provider-home
PROVIDER_CODEX_HOME=$AGH_HOME/provider-codex-home
AGH_WEB_API_PROXY_TARGET=http://127.0.0.1:<unique-port>
```

## 6. Cleanup (applies to every scenario)

- Disable any cron jobs created by the scenario:
  `agh automation jobs update <id> --enabled=false`.
- `agh daemon stop` (or kill PID from manifest).
- Inspect `automation_runs` for `status=running`/`scheduled` runs that the
  scenario didn't complete; if found, attach to the scenario report and DO
  NOT clean — it's evidence.
- Archive `events.db`, `agh.db`, and any audit-log fragments before tearing
  down the AGH_HOME.
- Tear down the worktree only after evidence artifacts are written.

## 7. Mandatory scenarios

### CRN-01 — Cron one-minute ping (5 fires over a 5-minute window)

```yaml qa-scenario
id: crn-01-cron-one-minute-ping
title: Schedule a `*/1 * * * *` cron firing a real Claude Code prompt; observe exactly one fire per minute over a 5-minute window
theme: automation.cron
coverage:
  primary:
    - cron.next-fire-deterministic
    - cron.no-duplicate-per-window
  secondary:
    - dispatch.lifecycle-events
    - trigger.lineage-correlation
risk: high
live: true
provider: real-claude-code
preconditions:
  - Fresh AGH_HOME with `automation.enabled = true`, timezone UTC.
  - One agent definition that points at Claude Code (`claude-sonnet-5` is fine; we just need a transcript).
  - Host wall clock NTP-synchronized within ±1s.
docs_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md
  - /Users/pedronauck/Dev/compozy/agh/.resources/openclaw/qa/scenarios/scheduling/cron-one-minute-ping.md
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/schedule.go:499-540
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/schedule.go:593-668
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/dispatch.go:412-486
  - /Users/pedronauck/Dev/compozy/agh/internal/cli/automation.go:121-191
steps:
  - Create the cron via CLI:
    `agh automation jobs create --name qa-crn01 --scope global --agent <agent> --schedule-mode cron --schedule-expr '*/1 * * * *' --prompt 'Reply with the literal marker {{.JobID}}-fire-<MINUTE>' --enabled --output json`.
  - Capture `job_id`, scheduler `next_run` from the response.
  - Wait `5m30s` from the moment the second after the next minute boundary
    elapses. Tail SSE / `automation.scheduler.job_fired` log lines.
  - Disable the cron at minute 5:
    `agh automation jobs update <job_id> --enabled=false`.
  - Snapshot `automation_runs` for the job:
    `agh automation runs --job-id <job_id> -o json`.
  - For each run, fetch the spawned session id and dump its transcript via
    `agh sessions transcript <session_id>`.
expected:
  - Exactly 5 rows in `automation_runs` for the 5-minute window, all in
    `status=completed`. Allow ±1 row only if the bottom of the window
    landed within the 1-second clock-sync grace.
  - `started_at` of consecutive rows is exactly 60s apart (±2s tolerance
    for scheduler dispatch latency).
  - 5 distinct `fire_id` values, each matching the
    `fire_<sha256[:24]>` shape of `scheduledFireID` (proof of cursor
    determinism, not wall-clock collisions).
  - Daemon log shows 5 `automation.scheduler.job_fired` lines with the
    same `job_id` and increasing `scheduled_at` minute boundaries.
  - 5 transcripts, each containing the literal marker that the prompt
    interpolated against the scheduled-at minute.
evidence:
  - `crn-01-runs.json` (`automation_runs` filtered to job_id).
  - `crn-01-events.json` (5 `automation.scheduler.job_fired` log lines and
    5 `automation.dispatch.completed` lines).
  - 5 transcripts attached as `crn-01-transcript-{1..5}.txt`.
failure_signatures:
  - More than 6 runs: `cron.no-duplicate-per-window` violated; double-fire.
  - Fewer than 4 runs: scheduler skipped a fire — investigate
    `automation.scheduler.fire_limit_deferred` / dispatch errors.
  - Non-deterministic `fire_id` values (e.g., per-attempt random ids):
    `cron.next-fire-deterministic` violated.
cleanup:
  - Delete the cron: `agh automation jobs delete <job_id>`.
  - Stop daemon. Archive evidence.
```

### CRN-02 — Cron natural-fire no-duplicate across daemon restart

```yaml qa-scenario
id: crn-02-cron-natural-fire-no-duplicate
title: Daemon restart that crosses a minute boundary does not duplicate the cron fire
theme: automation.cron
coverage:
  primary:
    - cron.no-duplicate-per-window
    - cron.restart-no-duplicate
    - cron.restart-resumes-pending
  secondary:
    - cron.skip-policy
risk: high
live: true
provider: real-claude-code
preconditions:
  - Fresh AGH_HOME.
  - Cron `*/1 * * * *` enabled with marker prompt.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/schedule.go:464-476
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/schedule.go:735-800
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/schedule.go:619-642
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/schedule_test.go:322-388
steps:
  - At T=0 (just after a minute boundary) create the cron and wait until
    `T+30s` (mid-window).
  - Confirm one row exists for the boundary that already fired (the very
    first fire is part of the warm-up; ignore it for restart proof).
  - Stop daemon: `agh daemon stop`. Confirm process exit.
  - Sleep until `T+90s` (next minute boundary +30s) — i.e., the daemon was
    DOWN across a fire boundary.
  - Start daemon: `agh daemon start`.
  - Watch for the next two natural fires (`T+120s` and `T+180s`).
  - Snapshot `automation_runs` and `automation_scheduler_state`.
expected:
  - The minute boundary that elapsed during the downtime appears in
    `automation_scheduler_state.misfire_count` (incremented), with
    `last_misfire_at` set, but does NOT show up as a completed run in
    `automation_runs`.
  - Subsequent natural fires after restart each produce exactly one
    `automation_runs` row per scheduled minute. Two new rows in this 2-min
    window.
  - `automation_scheduler_state.last_fire_id` for the post-restart fire is
    a new `fire_<sha256[:24]>` value distinct from the one persisted before
    the stop (proves the cursor advanced, not replayed).
  - Daemon log carries `automation.scheduler.dispatch_failed` zero times
    for the restart window. The `skip` policy must record misfires
    silently, not fail dispatch.
evidence:
  - `crn-02-runs.json`, `crn-02-scheduler-state.json` (pre/post restart).
  - Daemon log fragment over the restart boundary.
failure_signatures:
  - A duplicate run for the missed-during-downtime minute appears in
    `automation_runs`: `cron.no-duplicate-per-window` and
    `cron.skip-policy` both violated.
  - `last_fire_id` after restart equals the pre-restart value:
    `cron.restart-resumes-pending` violated; cursor is rolling back.
  - Two runs for the same post-restart fire id: critical race;
    `ClaimScheduledRun` not atomic.
cleanup:
  - Disable cron, stop daemon, archive evidence.
```

### CRN-03 — `at` mode single-run no-duplicate across restart

```yaml qa-scenario
id: crn-03-at-single-run-no-duplicate
title: A `mode = at` (single-run) job fires exactly once; restarting the daemon never produces a second fire
theme: automation.scheduling.at
coverage:
  primary:
    - cron.no-duplicate-per-window
    - cron.restart-resumes-pending
  secondary:
    - dispatch.lifecycle-events
risk: high
live: true
provider: real-claude-code
preconditions:
  - Fresh AGH_HOME.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/schedule.go:525-535
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/schedule.go:854-918
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/dispatch.go:412-486
steps:
  - Create an `at` job scheduled for `T+90s` (`agh automation jobs create
    --schedule-mode at --schedule-time <RFC3339> ...`). Capture `job_id`.
  - At `T+30s` stop daemon.
  - At `T+45s` start daemon.
  - At `T+120s` snapshot runs.
  - Restart daemon AGAIN at `T+180s` (post-fire) and immediately snapshot
    again.
expected:
  - Exactly one `automation_runs` row in `status=completed` with
    `started_at >= T+90s`.
  - After both restarts, `automation_scheduler_state` shows
    `next_run_at = NULL` (cron is removed from the registration map after
    the single fire — see `updateRegistrationState` in
    `internal/automation/schedule.go:718-733`).
  - Second restart does NOT create a second run.
  - The fire ID is stable: SHA-256 of `<job_id>|<scheduledAtUTC>`.
evidence:
  - `crn-03-runs.json` (must contain exactly one row).
  - `crn-03-scheduler-state-after-fire.json`.
failure_signatures:
  - Two completed runs: classic single-run dedup bug; `ClaimScheduledRun`
    not atomic across restarts.
  - The job remains in the scheduler registration map after firing:
    cleanup logic broken.
cleanup:
  - Delete the job; stop daemon; archive evidence.
```

### CRN-04 — Restart preservation of pending fires

```yaml qa-scenario
id: crn-04-restart-preservation
title: Cron scheduled for T+5min, daemon restart at T+1min — cron still fires at T+5min
theme: automation.cron
coverage:
  primary:
    - cron.restart-resumes-pending
    - cron.next-fire-deterministic
  secondary:
    - dispatch.lifecycle-events
risk: high
live: true
provider: real-claude-code
preconditions:
  - Fresh AGH_HOME.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/schedule.go:464-476
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/schedule.go:735-800
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/manager.go:1413-1440
steps:
  - Create an `at` job for `T+300s` (T+5min absolute).
  - Snapshot scheduler state — `next_run_at = T+300s`,
    `schedule_hash = <hex>`.
  - At `T+60s` stop daemon.
  - At `T+90s` start daemon.
  - Snapshot scheduler state — `next_run_at` MUST be unchanged at `T+300s`
    (cursor preserved, not rebuilt with `T+300s` from the new "now").
  - Wait until `T+300s` and verify the fire happens.
expected:
  - `next_run_at` is byte-identical to the pre-restart timestamp at the
    second snapshot.
  - One `automation_runs` row at T+300s ±2s.
  - Daemon log: no `automation.scheduler.skipped_past_one_time_job` entry
    (the job is still future-scheduled).
evidence:
  - `crn-04-scheduler-state-before.json`, `crn-04-scheduler-state-after.json`.
  - `crn-04-runs.json`.
failure_signatures:
  - `next_run_at` shifts after restart: cursor incorrectly recomputed —
    proves `predictNextRun` is overriding the durable row.
  - No fire at T+300s: scheduler did not pick up the persisted state on
    restart.
cleanup:
  - Stop daemon, archive evidence.
```

### CRN-05 — Crash recovery (kill -9 mid-fire)

```yaml qa-scenario
id: crn-05-crash-recovery
title: kill -9 the daemon while a cron is about to fire; on restart, ledger reflects no missed work
theme: automation.cron
coverage:
  primary:
    - cron.restart-no-duplicate
    - cron.skip-policy
  secondary:
    - dispatch.lifecycle-events
    - dispatch.session-stop-budget
risk: critical
live: true
provider: real-claude-code
preconditions:
  - Fresh AGH_HOME.
  - One `*/1 * * * *` cron creating a long-running prompt (60s of agent
    activity).
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/schedule.go:619-668
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/schedule.go:735-800
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/dispatch.go:412-486
steps:
  - Wait until the next minute boundary minus 5s.
  - `kill -9 $AGH_DAEMON_PID`.
  - Wait 30s.
  - Restart daemon. Tail the log + `automation_runs`.
expected:
  - One of these two outcomes (per `cron.skip-policy`):
    1. The fire that was about to happen is recorded as a misfire — no
       `automation_runs` row for it; `automation_scheduler_state.misfire_count`
       incremented; or
    2. The fire is recorded as `status=failed` with an error indicating
       the dispatch was canceled; the cursor advances cleanly past it.
  - In NEITHER case does the next-tick fire happen at the wrong cadence.
    The minute after restart fires on the original schedule.
  - No raw `claim_token` in any log line during the kill/restart window
    (out-of-scope for this child but a critical regression to flag).
evidence:
  - `crn-05-runs.json`, daemon log over the kill/restart boundary.
failure_signatures:
  - Two `automation_runs` rows for the killed fire (one orphan
    `running` from before the kill + one `completed` after restart):
    crash recovery left an orphan; `kill -9` is a supported termination
    mode and must be cleaned up.
  - Cursor regression: the post-restart `next_run_at` is earlier than
    the pre-kill scheduled fire — implies the daemon is replaying.
cleanup:
  - Stop daemon, archive evidence.
```

### CRN-06 — Webhook trigger fires session prompt (signed payload happy path)

```yaml qa-scenario
id: crn-06-webhook-happy-path
title: HTTP POST to `/api/webhooks/global/<slug>--<webhook_id>` with valid HMAC fires a session prompt
theme: automation.webhook
coverage:
  primary:
    - webhook.signature-required
    - webhook.timestamp-window
    - webhook.endpoint-format
    - dispatch.lifecycle-events
  secondary:
    - trigger.lineage-correlation
    - extensibility.agent-manageable
risk: high
live: true
provider: real-claude-code
preconditions:
  - Fresh AGH_HOME.
  - Vault contains an `automation.qa-crn06.secret` entry holding a
    32+ char shared secret.
  - Webhook trigger registered:
    `agh automation triggers create --event webhook --endpoint-slug qa-crn06 --webhook-secret-ref automation.qa-crn06.secret --enabled --prompt 'Reply with QA-CRN06-{{.Data.delivery_id}}' ...`.
  - Capture `webhook_id` (`wbh_<...>`) from the response.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/api/core/automation.go:493-520
  - /Users/pedronauck/Dev/compozy/agh/internal/api/core/automation.go:695-753
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/trigger.go:418-460
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/trigger.go:521-555
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/trigger.go:559-578
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/trigger.go:478-508
steps:
  - Build the canonical signing payload:
    `<unix_seconds>.<raw_request_body>`.
  - HMAC-SHA256 with the vault secret. Format header
    `X-AGH-Webhook-Signature: sha256=<hex>`.
  - POST `application/json` body `{"hello":"world"}` to
    `/api/webhooks/global/qa-crn06--<webhook_id>` with headers:
    - `X-AGH-Webhook-Timestamp: <RFC3339Nano>`
    - `X-AGH-Webhook-Delivery-ID: dlv-crn06-001`
    - `X-AGH-Webhook-Signature: sha256=<hmac>`
  - Capture HTTP response, then poll `automation_runs` for the trigger.
  - Fetch the spawned session and read its transcript.
expected:
  - HTTP 200 with `WebhookDeliveryResponse` shape; `result.matched == 1`,
    `result.runs.length == 1`.
  - One `automation_runs` row for the trigger in `status=completed`
    (or `running` if the agent is still streaming when polled).
  - Transcript contains `QA-CRN06-dlv-crn06-001` (or whatever the
    template renders for the delivery id).
  - `automation_scheduler_state` is unaffected (triggers don't write to
    scheduler state).
evidence:
  - `crn-06-http-response.json`, `crn-06-runs.json`,
    `crn-06-transcript.txt`.
failure_signatures:
  - HTTP 401/403 with valid signature: HMAC verification broken.
  - HTTP 400 "endpoint format invalid" with correct slug+webhookID:
    `webhook.endpoint-format` parser regression.
  - `result.matched == 0`: trigger not registered or filter mismatch.
cleanup:
  - Disable trigger, delete vault secret, stop daemon.
```

### CRN-07 — Webhook with invalid/missing signature (rejected + audited)

```yaml qa-scenario
id: crn-07-webhook-unsigned-rejected
title: Unsigned, mis-signed, and missing-header webhooks are rejected; daemon log records the rejection
theme: automation.webhook.security
coverage:
  primary:
    - webhook.unsigned-rejected
    - webhook.signature-required
  secondary:
    - extensibility.agent-manageable
risk: critical
live: true
provider: real-claude-code
preconditions:
  - Same setup as CRN-06.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/api/core/automation.go:714-725
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/trigger.go:538-555
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/trigger.go:1082-1092
steps:
  - Variant A: POST without any signature header. Expect 400.
  - Variant B: POST with `X-AGH-Webhook-Signature: sha256=deadbeef...` (a
    syntactically valid but wrong HMAC). Expect 4xx, body says signature
    invalid.
  - Variant C: POST with `X-AGH-Webhook-Signature: notsha256=abc` (wrong
    prefix). Expect 4xx invalid signature.
  - Variant D: POST without `X-AGH-Webhook-Delivery-ID`. Expect 400.
  - Variant E: POST with valid signature for delivery id `dlv-crn07-rep`
    twice within 30s (replay). Expect first 200, second 4xx
    `automation: webhook delivery already processed`.
  - Snapshot `automation_runs` after each variant.
expected:
  - Variants A/B/C/D return 4xx; no `automation_runs` rows created;
    daemon log carries one rejection line per attempt with the canonical
    error class (e.g.,
    `automation: webhook signature invalid`).
  - Variant E first call creates one run; second returns
    `ErrWebhookReplayDetected`; no second run created.
  - Rate of error responses matches request rate (no DoS amplification).
evidence:
  - Five `crn-07-variant-{A..E}-response.json` files.
  - Daemon log fragment showing rejections per variant.
  - `crn-07-runs.json` (proves only 1 row total — from variant E first
    call).
failure_signatures:
  - Any variant returns 200: critical security regression.
  - Replay variant succeeds twice: `webhook.replay-protected` violated;
    `claimWebhookDelivery` is not honoring the freshness window.
cleanup:
  - Stop daemon, archive evidence.
```

### CRN-08 — Webhook payload size limit (oversize rejected)

```yaml qa-scenario
id: crn-08-webhook-payload-size-limit
title: Oversize webhook body (>1 MiB) is rejected with a stable error and no buffer-overflow / OOM
theme: automation.webhook.security
coverage:
  primary:
    - webhook.body-size-limit
  secondary:
    - extensibility.agent-manageable
risk: high
live: false
provider: real-claude-code
preconditions:
  - Same setup as CRN-06.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/api/core/automation.go:32
  - /Users/pedronauck/Dev/compozy/agh/internal/api/core/automation.go:727-737
steps:
  - Generate a payload of exactly `1 << 20 = 1048576` bytes plus 1 (1 MiB
    + 1).
  - HMAC-sign as in CRN-06.
  - POST to the webhook endpoint.
  - Observe response. Sample daemon RSS before/after with `ps -o rss`
    every 100ms for 10s after the request.
  - Repeat 5 times back-to-back to look for any leak.
expected:
  - HTTP 4xx; body contains
    `webhook request body exceeds 1048576 bytes`.
  - Daemon RSS does NOT grow more than `~5 MiB` across the 5 requests
    (i.e., the daemon never buffered the full body before rejecting).
  - No `automation_runs` row created.
  - No panic or stack trace in the daemon log.
evidence:
  - `crn-08-response.json` (one of the 4xx responses).
  - `crn-08-rss-samples.csv`.
  - Daemon log fragment.
failure_signatures:
  - Daemon RSS grows monotonically across the 5 requests: leak; the
    body might be fully buffered before rejection.
  - HTTP 200: oversize accepted; `webhook.body-size-limit` violated.
  - Daemon panic: critical bug.
cleanup:
  - Stop daemon, archive evidence.
```

### CRN-09 — Scheduled `at` fire-once-now / past-rejected discipline

```yaml qa-scenario
id: crn-09-at-past-rejected
title: An `at` job whose `time` is in the past is logged-and-skipped, never auto-fired
theme: automation.scheduling.at
coverage:
  primary:
    - at.past-rejected-as-skip
  secondary:
    - cron.next-fire-deterministic
risk: medium
live: false
provider: real-claude-code
preconditions:
  - Fresh AGH_HOME.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/schedule.go:525-535
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/schedule.go:418-427
steps:
  - Create an `at` job with `time = T-300s` (5 minutes in the past).
  - Snapshot the response and the `automation_scheduler_state` row.
  - Wait 30s and snapshot `automation_runs`.
expected:
  - The CLI/HTTP create call returns success but `state.registered = false`
    (scheduler logs
    `automation.scheduler.skipped_past_one_time_job`).
  - `automation_scheduler_state.next_run_at` is `NULL` and
    `misfire_count` is `1`.
  - `automation_runs` is empty for this job.
  - The job IS persisted in the store as a definition (so the user can
    update or re-enable it), but it never auto-fired.
evidence:
  - `crn-09-create-response.json`, `crn-09-scheduler-state.json`,
    `crn-09-runs.json` (empty), daemon log fragment.
failure_signatures:
  - The job auto-fired despite `time < now`: `at.past-rejected-as-skip`
    violated.
  - `next_run_at` is set: scheduler is going to fire it later — the
    plan was supposed to come back as `register: false`.
cleanup:
  - Delete the job, stop daemon.
```

### CRN-10 — Timezone discipline (cron expression in non-UTC zone)

```yaml qa-scenario
id: crn-10-timezone-discipline
title: Cron expression with explicit `automation.timezone = America/New_York` fires on the configured zone, not host TZ or UTC
theme: automation.cron.timezone
coverage:
  primary:
    - cron.timezone-respected
    - cron.next-fire-deterministic
  secondary:
    - dispatch.lifecycle-events
risk: high
live: true
provider: real-claude-code
preconditions:
  - Fresh AGH_HOME.
  - `config.toml` overlay sets
    `automation.timezone = "America/New_York"`.
  - Host TZ (`/etc/timezone` or shell `TZ`) is `Europe/Amsterdam` (or any
    zone distinct from both UTC and NY) — so we can prove the daemon is
    NOT defaulting to host TZ.
  - Test conducted at a time when NY and UTC differ by exactly 4 or 5h
    (record DST status).
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/manager.go:1380-1396
  - /Users/pedronauck/Dev/compozy/agh/internal/config/automation.go:97-103
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/schedule.go:506-515
steps:
  - Pick an NY local hour that is NOT the same as the current UTC or host
    hour. Schedule a cron expression `0 <NY_HOUR> * * *` for tomorrow.
  - Verify the response's `next_run` is exactly tomorrow at NY_HOUR
    converted to UTC (i.e., `<NY_HOUR + offset>:00:00Z`).
  - Sleep until the fire happens (or use a fast-forward fake clock if
    test infra supports it; otherwise this is a slow scenario).
  - Snapshot `automation_runs.started_at` and confirm UTC timestamp
    matches the expected NY-local hour.
expected:
  - `next_run` matches the NY zone reading, not host TZ or UTC.
  - `started_at` for the actual fire is within ±5s of the computed UTC
    boundary corresponding to NY-local `NY_HOUR`.
  - Daemon log line `automation.scheduler.job_fired` carries
    `scheduled_at` in UTC (RFC3339Nano), but the underlying `cronImpl`
    used the NY location (proven by the timestamp arithmetic).
evidence:
  - `crn-10-create-response.json`, `crn-10-runs.json`, daemon log
    fragment, host `date` output for triangulation.
failure_signatures:
  - Fire at host-TZ hour: scheduler ignored `automation.timezone` config.
  - Fire at UTC hour matching NY_HOUR (i.e., 4-5h off): cron expression
    parsed in UTC; `WithSchedulerLocation` not honored.
cleanup:
  - Stop daemon, archive evidence.
```

### CRN-11 — DST fall-back (no double-fire across ambiguous hour)

```yaml qa-scenario
id: crn-11-dst-fall-back-no-double
title: A cron firing at 01:30 in the US/Eastern fall-back hour fires once, never twice
theme: automation.cron.dst
coverage:
  primary:
    - cron.dst-no-double-fire
    - cron.no-duplicate-per-window
  secondary:
    - cron.timezone-respected
risk: critical
live: false
provider: real-claude-code
preconditions:
  - Fresh AGH_HOME with `automation.timezone = America/New_York`.
  - Host clock fast-forwarded (or daemon launched with `WithSchedulerClock`
    fake) to the next fall-back date (typically first Sunday of November).
    Use a clockwork fake clock or run during the actual transition for
    real proof.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/schedule.go:506-515
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/schedule.go:854-883
  - /Users/pedronauck/Dev/compozy/agh/go.mod:13,28 (gocron-v2 + robfig/cron underneath)
steps:
  - Schedule cron `30 1 * * *` (daily at 01:30 NY).
  - Advance the clock to 00:00 NY of fall-back day; let the scheduler
    register and persist `next_run_at`.
  - Advance through the 01:00→02:00→01:00→02:00 ambiguous window.
  - Snapshot `automation_runs` after the window.
expected:
  - Exactly one `automation_runs` row for the day.
  - The row's `started_at` is the FIRST 01:30 NY occurrence (i.e., before
    the rewind), unambiguous in UTC.
  - `next_run_at` after the fire is the next day's 01:30 NY.
evidence:
  - `crn-11-runs.json`, daemon log fragment, advance-clock manifest.
failure_signatures:
  - Two rows for the same nominal local time: DST double-fire bug.
  - No row at all: scheduler skipped the fire across the rewind.
cleanup:
  - Stop daemon, archive evidence.
```

### CRN-12 — DST spring-forward (no missed fire in skipped hour)

```yaml qa-scenario
id: crn-12-dst-spring-forward
title: A cron at 02:30 in US/Eastern spring-forward window — fires once on the next valid 02:30, never re-fires the skipped instance
theme: automation.cron.dst
coverage:
  primary:
    - cron.dst-no-double-fire
    - cron.next-fire-deterministic
  secondary:
    - cron.skip-policy
risk: critical
live: false
provider: mock-acp
preconditions:
  - Fresh AGH_HOME with `automation.timezone = America/New_York`.
  - Spring-forward window staged via fake clock (typically second Sunday
    of March; 02:00→03:00 jump).
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/schedule.go:506-515
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/schedule.go:854-918
steps:
  - Schedule cron `30 2 * * *`. Advance clock to 01:55 NY.
  - Watch the daemon log for `automation.scheduler.registered` showing
    `next_run_at` for the spring day.
  - Advance through 02:00→03:00 jump.
  - Verify behavior of robfig/cron-via-gocron-v2 — historically robfig
    skips non-existent local times, so the scheduled `next_run_at`
    becomes the NEXT valid 02:30 (i.e., the day after).
  - Snapshot `automation_runs` for the spring day and the day after.
expected:
  - Spring-forward day has ZERO runs at the would-be 02:30 (the wall
    time never existed).
  - The day AFTER spring-forward has exactly one run at 02:30 NY.
  - Daemon log shows no `dispatch_failed`; the scheduler silently
    skipped the missing wall time.
evidence:
  - `crn-12-runs.json` (spring day = 0 rows; day-after = 1 row),
    daemon log fragment.
failure_signatures:
  - The scheduler attempts to fire at 02:30 on spring-forward day:
    invalid wall-time treated as valid; cron parser confused.
  - The scheduler fires twice on the day after: cursor double-stepped.
cleanup:
  - Stop daemon, archive evidence.
```

### CRN-13 — Concurrent cron triggers within one minute

```yaml qa-scenario
id: crn-13-concurrent-cron-fires
title: 50 cron jobs scheduled in the same minute — every one fires; backpressure handled; no missed fires
theme: automation.dispatch.concurrency
coverage:
  primary:
    - dispatch.concurrency-gate
    - dispatch.fire-limit
  secondary:
    - cron.next-fire-deterministic
    - dispatch.lifecycle-events
risk: high
live: true
provider: real-claude-code
preconditions:
  - Fresh AGH_HOME with `automation.max_concurrent_jobs = 5` (or whatever
    the SUT default is — be explicit in the report).
  - Pool of agent definitions (5+) that can run in parallel.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/dispatch.go:295,412-420
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/dispatch.go:554-622
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/schedule.go:682-716
steps:
  - Loop: create 50 cron jobs each with `*/1 * * * *` and a 3s prompt.
  - Wait for the next minute boundary plus 90s.
  - Snapshot `automation_runs`.
  - Inspect `automation.scheduler.fire_limit_deferred` and
    `automation: global concurrency limit reached` log lines for the
    window.
expected:
  - Exactly 50 successful fires recorded across the minute window. The
    dispatcher gate must SERIALIZE excess attempts; the scheduler must
    re-queue dispatches that hit the gate or fire-limit (it does NOT
    drop fires silently — the durable cursor advances regardless).
  - The first ~5 runs have `started_at` clustered at the boundary; the
    remaining 45 are spread across the next ~30-60s as the gate releases.
  - `automation_runs.status = completed` for at least 49/50 (allow one
    failure if the underlying agent provider hiccupped — note the
    failure but don't fail the scenario unless >2 failures).
evidence:
  - `crn-13-runs.json`, daemon log fragment with concurrency-limit and
    fire-limit-deferred lines.
failure_signatures:
  - Fewer than 48 runs: missed fires; backpressure dropped work.
  - More than 50 runs: duplicate fires; scheduler claimed a single
    fire id twice.
  - Gate never engaged (no concurrency-limit log lines despite 50
    parallel attempts): `dispatch.concurrency-gate` not enforced.
cleanup:
  - Bulk-delete the 50 jobs, stop daemon.
```

### CRN-14 — Cron-fired session lineage (system session + automation actor)

```yaml qa-scenario
id: crn-14-cron-session-lineage
title: A cron-fired session is `SessionType=system` and any task it spawns carries automation provenance
theme: automation.lineage
coverage:
  primary:
    - trigger.lineage-correlation
    - dispatch.lifecycle-events
  secondary:
    - extensibility.agent-manageable
risk: high
live: true
provider: real-claude-code
preconditions:
  - Fresh AGH_HOME.
  - One cron job whose prompt instructs the agent to call `agh task create`
    (or the equivalent in-session tool) so a downstream task is enqueued
    by the cron-fired session.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/dispatch.go:979-1006
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/dispatch.go:984-994
  - /Users/pedronauck/Dev/compozy/agh/internal/task (DeriveAutomationLinkedAgentSessionActorContext)
steps:
  - Create the cron at next minute boundary.
  - After fire completes, fetch the spawned session via
    `agh sessions get <session_id> -o json`.
  - Inspect `type` field — must be `system` (`SessionTypeSystem`).
  - Inspect any tasks the session created (`agh tasks list -o json`).
  - For each task, fetch `task_runs` and inspect `actor_kind` /
    `actor_ref`.
expected:
  - Cron-fired session has `type = system`.
  - Tasks created by the cron session have `actor` derived via
    `DeriveAutomationLinkedAgentSessionActorContext` —
    `actor_kind = automation`, `actor_ref` referencing the
    automation run id.
  - Task runs carry the same automation provenance through to the
    `task_runs` row's `actor_*` columns.
  - Sessions/tasks created OUTSIDE the cron-fired session (different
    parent context) do NOT carry the automation actor — provenance is
    not leaking.
evidence:
  - `crn-14-session.json`, `crn-14-tasks.json`, `crn-14-task-runs.json`.
failure_signatures:
  - Session type is not `system`: classification regression.
  - Task actor is `unknown` or `user`: provenance lost.
  - Provenance leaks to unrelated downstream sessions: actor recorder
    cleanup broken (`DeleteAutomationSessionTaskActor` not invoked).
cleanup:
  - Stop daemon, archive evidence.
```

### CRN-15 — `agh automation list -o json` parity with HTTP `/api/automation`

```yaml qa-scenario
id: crn-15-cli-http-parity
title: CLI list command and HTTP API return the same automation jobs/triggers/runs payload shape
theme: automation.transport-parity
coverage:
  primary:
    - automation.cli-http-parity
    - extensibility.agent-manageable
risk: high
live: false
provider: mock-acp
preconditions:
  - Fresh AGH_HOME with 3 jobs (one each for cron / every / at) + 3
    triggers (webhook / session.created / hook.completion) + several
    runs across them.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/cli/automation.go:43-757
  - /Users/pedronauck/Dev/compozy/agh/internal/api/core/automation.go
  - /Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/routes.go:168-191
steps:
  - Run:
    - `agh automation jobs -o json > cli-jobs.json`
    - `agh automation triggers -o json > cli-triggers.json`
    - `agh automation runs -o json > cli-runs.json`
  - HTTP equivalent (use `curl --unix-socket "$AGH_DAEMON_UDS"` or the
    HTTP port from the manifest):
    - `GET /api/automation/jobs > http-jobs.json`
    - `GET /api/automation/triggers > http-triggers.json`
    - `GET /api/automation/runs > http-runs.json`
  - Diff the JSON shapes after stripping volatile fields
    (`updated_at`, request id headers).
expected:
  - Per-resource diff is empty after stripping known-volatile fields.
  - The CLI's `-o json` produces an OBJECT with metadata + items array;
    the HTTP response uses the same shape per the contract types in
    `internal/api/contract/automation.go`. Field names match exactly.
  - Both transports respect the same query filters
    (`scope`, `workspace_id`, `source`, `limit`, `last`).
evidence:
  - 6 captured payloads + a diff transcript.
failure_signatures:
  - Field-name divergence (e.g., CLI uses `agent_name`, HTTP uses
    `agentName`): contract drift; one transport bypassed BaseHandlers
    or contract types.
  - Different filtering semantics: parser drift in
    `ParseAutomationJobListQuery` vs CLI flag handling.
cleanup:
  - Stop daemon, archive evidence.
```

### CRN-16 — Disable / enable cron mid-window (graceful drain)

```yaml qa-scenario
id: crn-16-disable-enable-cron-live
title: Disable cron mid-window — currently-running session completes; future fires suppressed; re-enable resumes naturally
theme: automation.lifecycle
coverage:
  primary:
    - enable-disable.live
    - cron.next-fire-deterministic
  secondary:
    - dispatch.session-stop-budget
    - dispatch.lifecycle-events
risk: high
live: true
provider: real-claude-code
preconditions:
  - Fresh AGH_HOME with one `*/1 * * * *` cron whose prompt takes ~30s.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/schedule.go:298-303,344-364
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/dispatch.go:785-825
steps:
  - Wait for the first fire. Once the spawned session is in
    `running`, immediately disable the cron:
    `agh automation jobs update <id> --enabled=false`.
  - Verify the in-flight session continues to completion (NOT killed by
    the disable).
  - Wait `90s` past the next minute boundary; verify NO new run fires.
  - Re-enable: `agh automation jobs update <id> --enabled=true`.
  - Wait for the next minute boundary; verify the cron fires again.
expected:
  - First in-flight run completes with `status=completed`.
  - During the disabled window, `automation_scheduler_state.next_run_at`
    is `NULL` and the registration is gone from the in-memory map.
  - Re-enabling re-registers the cron with `next_run_at` at the next
    minute boundary; the next fire is one normal `automation_runs` row.
  - No `dispatch_failed` events during the disable transition.
evidence:
  - `crn-16-runs.json` (3 rows: pre-disable, post-disable=0, post-enable),
    `crn-16-scheduler-state-{disabled,enabled}.json`.
failure_signatures:
  - Disable kills the in-flight session: `enable-disable.live` violated.
  - A new fire happens during the disabled window: cron wasn't
    unregistered from the runtime registry.
  - Re-enable doesn't resume firing: cursor not restored.
cleanup:
  - Disable cron, stop daemon, archive evidence.
```

### CRN-17 — Cron failure path (prompt errors; cron continues)

```yaml qa-scenario
id: crn-17-cron-failure-continues
title: A cron-fired prompt that errors records the error in `automation_runs`; cron still fires on the next schedule
theme: automation.dispatch.failure
coverage:
  primary:
    - dispatch.failure-cron-continues
    - dispatch.lifecycle-events
  secondary:
    - cron.next-fire-deterministic
risk: high
live: true
provider: real-claude-code
preconditions:
  - Fresh AGH_HOME with one `*/1 * * * *` cron whose prompt is malformed
    (e.g., calls a tool that doesn't exist or asks the agent to throw an
    error in a way the ACP transport propagates as an event-level
    error).
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/dispatch.go:441-486
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/dispatch.go:727-783
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/schedule.go:619-668
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/dispatch.go:1259-1287
steps:
  - Wait for two consecutive fires (T+60s and T+120s relative to the
    boundary).
  - Inspect `automation_runs`.
expected:
  - Both rows present.
  - Either both rows in `status=failed` with `error` capturing the
    underlying ACP-level error, OR (if the prompt is intentionally
    malformed in a recoverable way and the agent self-recovers) one
    row in `failed` and one in `completed`. The key invariant: the
    second fire happens regardless of the first row's outcome.
  - Daemon log shows `automation.dispatch.failed` for the failed run(s)
    AND `automation.scheduler.job_fired` for each minute boundary.
  - The cron is NOT auto-disabled (no policy auto-disables cron on
    first failure).
  - `automation_run_failed` lifecycle hook (if a hook is registered)
    fires once per failed run with `WillRetry=false` — the dispatcher's
    retry strategy is `none` by default.
evidence:
  - `crn-17-runs.json` (>=2 rows), daemon log fragment.
failure_signatures:
  - First failure auto-disables the cron: spec violation;
    `dispatch.failure-cron-continues` violated.
  - No second fire at all: the cron stopped firing despite the cursor
    having advanced — implies a coupling between dispatch outcome and
    cursor state.
  - `automation_runs.error` is empty for a failed run: error
    propagation broken.
cleanup:
  - Disable cron, stop daemon, archive evidence.
```

### CRN-18 — Real-LLM end-to-end (cron → coordinator-style spawn → child)

```yaml qa-scenario
id: crn-18-real-llm-end-to-end
title: Real Claude Code receives the cron prompt; spawns a child via the standard task-claim path; full lineage in transcripts
theme: automation.end-to-end
coverage:
  primary:
    - dispatch.lifecycle-events
    - trigger.lineage-correlation
    - cron.next-fire-deterministic
  secondary:
    - dispatch.session-stop-budget
    - extensibility.agent-manageable
risk: high
live: true
provider: real-claude-code
preconditions:
  - Fresh AGH_HOME with cron `*/1 * * * *`; prompt is "Spawn a worker via
    `agh sessions spawn` to summarize the last hour of activity".
  - Coordinator config enabled per module 04 conventions.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/dispatch.go:412-486
  - /Users/pedronauck/Dev/compozy/agh/internal/coordinator (module 04 covers depth)
  - /Users/pedronauck/Dev/compozy/agh/internal/session/spawn.go
steps:
  - Wait for two consecutive cron fires.
  - For each fire: capture the parent session id, the child session id,
    and both transcripts.
  - Verify `parent_session_id` / `root_session_id` lineage chain.
expected:
  - Both fires use the SAME shape:
    `automation_runs → session(create, type=system) → child(spawn) →
    child task_runs → child completed`.
  - Lineage rows exist for the parent→child relationship; depth is 1.
  - The transcript contains the literal string output the prompt
    requested ("Reply with QA-CRN18-marker-<minute>").
  - No raw `claim_token` (`agh_claim_*`) in any transcript or log line.
evidence:
  - 2 parent transcripts, 2 child transcripts, lineage dump.
failure_signatures:
  - Second fire takes a different code path: shortcut bug.
  - Lineage broken on second run: state-machine regression.
  - Token leak: critical security regression (cross-cutting with
    module 04's redaction audit).
cleanup:
  - Disable cron, reap children, stop daemon.
```

### CRN-19 — Webhook freshness window (clock-skew rejection)

```yaml qa-scenario
id: crn-19-webhook-stale-timestamp
title: Webhook with timestamp older than `DefaultWebhookFreshnessWindow = 5m` is rejected
theme: automation.webhook.security
coverage:
  primary:
    - webhook.timestamp-window
  secondary:
    - webhook.signature-required
risk: high
live: false
provider: real-claude-code
preconditions:
  - Same setup as CRN-06.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/trigger.go:51-52
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/trigger.go:559-578
steps:
  - Variant A: POST with `X-AGH-Webhook-Timestamp` set to `now - 6m`
    and a signature computed against the stale timestamp. Expect 4xx
    `webhook timestamp outside freshness window`.
  - Variant B: POST with timestamp `now + 6m` (future skew). Expect 4xx.
  - Variant C: POST with timestamp `now - 4m` (within window). Expect
    200; one run created.
expected:
  - Variants A/B are rejected; no `automation_runs` rows.
  - Variant C succeeds; one row.
  - Daemon log shows the freshness rejections.
evidence:
  - 3 response payloads, runs snapshot, daemon log fragment.
failure_signatures:
  - Variants A/B accepted: time-window not enforced.
  - Variant C rejected with valid timestamp: window math wrong.
cleanup:
  - Stop daemon, archive evidence.
```

### CRN-20 — Trigger from session.created lifecycle observer

```yaml qa-scenario
id: crn-20-session-created-trigger
title: A registered trigger with `event = session.created` fires when any session is created; envelope carries session metadata
theme: automation.trigger.session
coverage:
  primary:
    - dispatch.lifecycle-events
    - trigger.lineage-correlation
  secondary:
    - extensibility.agent-manageable
risk: medium
live: true
provider: real-claude-code
preconditions:
  - Fresh AGH_HOME.
  - One trigger registered with `event = session.created`,
    `prompt = "A session named {{.Data.session_name}} was just created"`.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/trigger.go:372-378
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/trigger.go:985-1027
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/dispatch.go:1100-1118 (renderTriggerPrompt)
steps:
  - Create a new session via `agh sessions create --workspace ...`.
  - Wait up to 30s for the trigger to fire.
  - Snapshot `automation_runs` filtered to the trigger.
  - Read the spawned session's transcript.
expected:
  - One `automation_runs` row, status `completed`.
  - The dispatched session's transcript contains the new session's name
    (template rendered against the envelope's `session_name`).
  - Filter precedence is honored if a `filter.session_type=user` is set
    — only user sessions trigger; system sessions don't.
evidence:
  - `crn-20-runs.json`, transcript file.
failure_signatures:
  - No fire: observer wiring broken.
  - Multiple fires for one session creation: observer fires twice.
  - Template not rendered: `renderTriggerPrompt` regression.
cleanup:
  - Disable trigger, stop daemon.
```

## 8. Optional / nice-to-have scenarios (run if time)

### CRN-21 — Webhook-secret rotation (vault update reflected on next request)

```yaml qa-scenario
id: crn-21-webhook-secret-rotation
title: Updating the vault entry for a webhook trigger's secret_ref makes the next request reject the old secret
theme: automation.webhook.security
coverage:
  primary:
    - webhook.secret-required
    - webhook.signature-required
  secondary:
    - extensibility.agent-manageable
risk: medium
live: false
provider: real-claude-code
preconditions:
  - Same setup as CRN-06.
steps:
  - Send a valid webhook with the old secret. Expect 200.
  - Update the vault secret value via `agh vault put ...`.
  - Send a webhook signed with the OLD secret. Expect 4xx invalid
    signature.
  - Send a webhook signed with the NEW secret. Expect 200.
expected:
  - Old secret rejected immediately after rotation; new secret accepted.
  - Daemon log shows rejection then acceptance.
evidence:
  - 3 responses, daemon log fragment.
failure_signatures:
  - Old secret accepted after rotation: caching bug; secret resolver
    not flushed.
cleanup:
  - Stop daemon, archive evidence.
```

### CRN-22 — `automation.scheduler.fire_limit_deferred` semantics

```yaml qa-scenario
id: crn-22-fire-limit-deferred
title: A cron firing past its `fire_limit` is deferred to `RetryAt`; cursor reflects the deferral; eventual fire succeeds
theme: automation.dispatch.fire-limit
coverage:
  primary:
    - dispatch.fire-limit
  secondary:
    - cron.next-fire-deterministic
risk: medium
live: true
provider: real-claude-code
preconditions:
  - Fresh AGH_HOME.
  - Cron `*/1 * * * *` with `fire_limit = { max: 1, window: "5m" }`.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/dispatch.go:554-622
  - /Users/pedronauck/Dev/compozy/agh/internal/automation/schedule.go:682-716
steps:
  - Let the first fire happen.
  - The next minute's fire should hit the limit and be deferred.
  - Snapshot `automation_scheduler_state.next_run_at` — should equal
    `RetryAt` from the fire-limit error.
  - Wait until the window expires; verify the next fire happens normally.
expected:
  - Daemon log:
    `automation.scheduler.fire_limit_deferred retry_at=<...> fires=1
    limit=1 window=5m0s`.
  - Exactly 2 successful runs in the first 6 minutes (the t=0 fire and
    the post-window fire).
  - The deferred minutes have no `automation_runs` rows in `failed` /
    `completed`. They may have `cancelled` rows if a reservation was
    written; consult `fireLimitRunStatus` (`internal/automation/dispatch.go:547-552`).
evidence:
  - `crn-22-runs.json`, scheduler state snapshots, daemon log.
failure_signatures:
  - Fires happen during the window despite the limit:
    `dispatch.fire-limit` violated.
  - No fire after the window expires: deferral didn't release.
cleanup:
  - Stop daemon, archive evidence.
```

## 9. Coverage matrix (this child)

| Coverage ID                          | Scenarios                                               |
| ------------------------------------ | ------------------------------------------------------- |
| `cron.next-fire-deterministic`       | CRN-01, CRN-04, CRN-09, CRN-10, CRN-12, CRN-13, CRN-15, CRN-16, CRN-17, CRN-18, CRN-22 |
| `cron.no-duplicate-per-window`       | CRN-01, CRN-02, CRN-03, CRN-11                          |
| `cron.restart-resumes-pending`       | CRN-02, CRN-03, CRN-04                                  |
| `cron.restart-no-duplicate`          | CRN-02, CRN-05                                          |
| `cron.skip-policy`            | CRN-02, CRN-05, CRN-12                                  |
| `cron.timezone-respected`            | CRN-10, CRN-11                                          |
| `cron.dst-no-double-fire`            | CRN-11, CRN-12                                          |
| `at.past-rejected-as-skip`           | CRN-09                                                  |
| `webhook.signature-required`         | CRN-06, CRN-07, CRN-19, CRN-21                          |
| `webhook.timestamp-window`           | CRN-06, CRN-19                                          |
| `webhook.replay-protected`           | CRN-07                                                  |
| `webhook.body-size-limit`            | CRN-08                                                  |
| `webhook.endpoint-format`            | CRN-06                                                  |
| `webhook.secret-required`            | CRN-06, CRN-21                                          |
| `webhook.unsigned-rejected`          | CRN-07                                                  |
| `dispatch.concurrency-gate`          | CRN-13                                                  |
| `dispatch.fire-limit`                | CRN-13, CRN-22                                          |
| `dispatch.lifecycle-events`          | CRN-01, CRN-03, CRN-05, CRN-06, CRN-13, CRN-14, CRN-16, CRN-17, CRN-18, CRN-20 |
| `dispatch.failure-cron-continues`    | CRN-17                                                  |
| `dispatch.session-stop-budget`       | CRN-05, CRN-16, CRN-18                                  |
| `trigger.lineage-correlation`        | CRN-01, CRN-06, CRN-14, CRN-18, CRN-20                  |
| `automation.cli-http-parity`         | CRN-15                                                  |
| `enable-disable.live`                | CRN-16                                                  |
| `extensibility.agent-manageable`     | CRN-06, CRN-07, CRN-08, CRN-14, CRN-15, CRN-18, CRN-20, CRN-21 |

Total: 20 mandatory + 2 optional = 22 scenarios. Every coverage ID is
exercised by at least one scenario; the high-risk IDs
(`cron.no-duplicate-per-window`, `webhook.signature-required`,
`webhook.body-size-limit`, `dispatch.failure-cron-continues`) are
exercised by at least two.

## 10. Forbidden-needle list (transcript and event payloads)

Per the openclaw `forbiddenNeedles` pattern. None of the following may
appear in any outbound message, transcript, SSE event, automation run
error, or audit log across any CRN scenario:

- Any literal raw `agh_claim_<>=12 random char>` (regex
  `agh_claim_[A-Za-z0-9_-]{12,}`). Cron-spawned sessions and webhook
  triggers MUST NOT leak claim tokens.
- Any provider API key shape: `sk-`, `xoxb-`, `AKIA`, `ya29.`.
- Any raw webhook secret value (the one stored under
  `automation.<...>` in the vault). Vault values must never appear in
  daemon logs, even in debug mode. If this scenario suite injects a
  high-entropy fixed string into the vault, it MUST NOT appear in any
  log output.
- Any reference to the deleted legacy `recipe`/`workflow`/`procedure`
  vocabulary in trigger prompts or run errors (per
  `docs/_memory/glossary.md` — canonical term is `capability`).
- Any literal `cron.run` / `cron.runs` openclaw vocabulary in AGH
  prompts: AGH uses `agh automation jobs trigger <id>`, not `cron.run`.
  Templates copied from the openclaw fixtures must be rewritten before
  use.

A single scenario test failure on this list is shippability-critical and
must be triaged immediately.

## 11. Reporting contract

Each scenario writes the four-artifact set required by the openclaw
operator-flow pattern (markdown report + JSON summary + observed events
+ combined log). The aggregate `crn-summary.json` for this child carries
the coverage matrix from §9 alongside per-scenario `outcome ∈ {worked,
failed, blocked, follow-up}` and machine-readable timing.

The scenario operator runs in-character (per the `real-scenario-qa`
skill); every run ends with a Worked / Failed / Blocked / Follow-up
section covering all 20 mandatory scenarios. A child run is shippable
only when:

- Every mandatory scenario is `worked` or has an explicit accepted
  follow-up.
- CRN-02, CRN-03, CRN-05, CRN-08, CRN-11, CRN-12, CRN-13, and CRN-17
  are clean (the dedup, crash-safety, DST, concurrency, and failure
  invariants are non-negotiable for a v0 release).
- CRN-07 is clean (security): every signature-rejection variant returns
  4xx and the replay variant rejects the duplicate.
- No forbidden-needle hit anywhere.
- `make verify` passed on the SUT branch before this child ran (cite
  commit SHA in `crn-summary.json`).
