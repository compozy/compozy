# Issue #1128 - Session monitor improvements

Status: implemented on `feat/monitor-improvements`; lab proof recorded below.

## Research

Reference runtimes were read from the vendored `.resources` tree (read-only). Findings and how they
map onto the current daemon monitor:

| Reference | Finding | Already covered by | Decision |
| --- | --- | --- | --- |
| OpenClaw `bash-tools.exec-notify.ts` | Background exec captures its owner, then wakes that owner | `monitor:<id>` stable enqueue to the owning session | none needed |
| OpenClaw cron schedules / background-process docs | Bounded condition gates; adopted work is never re-executed; unknown outcomes are explicit | `terminal_unavailable` outcome, bounded caps | no polling kind (below) |
| Hermes `process_registry` | Persist before notify, idempotent finish; wait policy avoids false wake promises | `markFired` once, stable message id, `settle` | idempotent registration added |
| GoClaw delegate ledger / gateway boot | Durable delivery states, reconcile interrupted work at boot | `delivering`/`undelivered`, 15 min retry window | persistence excluded (below) |
| Codex `async_watcher` / input queue | Drain output before exit; fence stale turns; exits do not wake the model | terminal `Wait` result carries the screen; durable queue | none needed |

## Compozy path inspected

`terminal_exec` -> terminal handle `Wait` -> `monitor.Service.run` -> daemon `monitorSessionWaker` ->
`EnqueueSyntheticWake`. `StatusFired` means "admitted to the durable input queue", not "turn executed";
the queue owns later dispatch; queue acceptance alone does not prove execution. It was left unchanged: redefining it would change a public status
meaning for no observed defect.

`OnSessionStopped` runs only from stop finalization and failed-start notification, not from an in-place
runtime-selection rebuild, so `CancelSession` there cancels monitors only when the session actually ended.

## Implemented

1. **Idempotent terminal registration.** Registering the same terminal condition (session, terminal,
   `until`, `pattern`, normalized timeout, description) while one is active or delivering returns that monitor (`monitor.Reused`, not on the
   wire) instead of adding another wake. Evidence: the per-terminal cap of 4 let a retried
   `monitor_create` (agents retry after ambiguous results) queue up to four identical wakes for one exit.
   The tool output tells the agent no duplicate was registered. Distinct conditions on one terminal remain
   separate monitors and still count against the caps.
2. **Remote-wait guidance.** The tool descriptor, `skills/compozy`, and the site monitors page state that
   there is no polling kind: run the bounded watch command via `terminal_exec`, monitor that terminal, set
   `timeout_seconds` past the expected wait. This keeps command approval and the shell-wait guard in the
   loop.
3. **Lifecycle logging**: structured `monitor.created|reused|fired|wake_deferred|
   wake_queued|wake_failed|canceled|interrupted` events with id, session, kind, status and a closed reason
   set; no description, pattern, output or raw error. This supplies the trace the local aggregate lacked.
4. Test cleanup: two discarded `Shutdown` errors in the owning suite now fail the test.

## Excluded proposals

- **Durable monitor definitions / restart reconciliation.** Not implemented. The current documented
  contract cancels monitors on session stop and loses them on daemon restart. Reliable restart recovery
  needs persisted ownership plus a distinction between operator stops and recoverable runtime stops.
  This remains a product/lifecycle decision; persistence does not inherently require reviving sessions
  stopped by the operator. Clean shutdown is logged, but crashes cannot emit a loss event.
- **First-class remote-condition kind (CI/HTTP poller).** It would make the daemon execute provider
  commands or network calls on a timer outside terminal command approval and the shell-wait guard, and
  needs per-provider auth. A bounded terminal under existing policy already provides it.
- **Making `fired` mean "turn executed"** or batching wakes: compatibility cost, no evidence; monitors are
  one-shot so recurring-flood controls do not apply.
- **Polling or model wakes on process exit (Codex)**: Compozy wakes deliberately.

## Evidence limits

The local aggregate scan (8 log files, 1,895 monitor mentions, zero create/cancel/deferred/undelivered
patterns) is not a usage count and not proof of absence; it only showed the lack of lifecycle trace that
item 3 fixes. Only aggregate counters from the local log scan were exposed; no raw private log lines were included.

## Cross-surface impact audit

- **Native tools:** `compozy__monitor_create` description and result preview change; input and output
  schemas are unchanged (additive behavior only). `monitor_list` and `monitor_cancel` unchanged.
- **CLI / HTTP / UDS / OpenAPI / SDKs:** no change; `Reused` is `json:"-"`. No codegen.
- **Extensibility / hooks / config:** no new hook, config key or extension surface.
- **Workspace data isolation / persistence:** idempotency keys include the session (and terminal key's
  workspace and profile); no schema or migration change; in-memory only. SD-013: no public shape change.
- **Official skill / docs / Web:** `skills/compozy/references/native-tools.md`, site `sessions/monitors.mdx`,
  and `docs/qa/scenarios/RT-session-monitor-wake.md` (step 11). Web needs no change.

## Validation

- `go test ./internal/monitor`, `./internal/tools/builtin`, `./internal/toolmeta`, `./internal/shellwait`,
  `./internal/mcp`, and `./internal/daemon -run Monitor` pass.

## Isolated lab evidence

Real daemon built from this branch in an isolated home with its own ports and socket (never the default
daemon), acpmock agent calling the real hosted native tools over MCP, no provider.

1. A terminal polling an external marker for about 3m20s was running (`still_running`).
2. The agent called `compozy__monitor_create` twice with the identical terminal condition and ended its
   turn. First call: monitor `active`. Second call: the result said the monitor already watches the
   condition and no duplicate was registered. HTTP list showed exactly one `active` monitor.
3. The session sat idle for 3m19s. When the marker appeared and the process exited, the daemon started a
   new turn without any operator input: a `monitor` synthetic re-entry (`scheduler_reason: monitor_exit`)
   carrying `exited with code 0` and the untrusted output tail, followed by the agent's report. Exactly one
   wake arrived.
4. `monitor_list` afterwards: one monitor, `fired`, `reason: exit`, `exit_code: 0`.
5. Daemon log showed the lifecycle trace in order: `monitor.created`, `monitor.reused`, `monitor.fired`
   (`delivering`), `monitor.wake_queued` (`fired`), with ids, kind, status and reason only.

Not exercised live: timer restore after restart (excluded by design), the `delivering` retry path, and
real-provider behavior; those stay owned by `internal/monitor` and daemon waker unit suites. This proves
the daemon wake path for a long external wait; it does not prove a real `gh pr checks --watch` against
GitHub, which needs a live provider and remote.
