# Issue #1128 - Session monitor improvements

Status: implemented on `feat/monitor-improvements` (PR #1129). Durable monitors, restart recovery,
idempotent registration, and lifecycle logging are implemented and proven in an isolated real-daemon lab
whose raw evidence is saved with this report.

## Evidence statement

An earlier revision of this report described a lab proof (3m19 external wait, one wake, coalesced
duplicate registration). Those artifacts were not retained (empty journey log, deleted fixture), so that
claim is **unretained and self-reported**; it is not relied on here. Every lab claim below cites **fresh
saved evidence** under `docs/qa/reports/artifacts/2026-11-01-issue-1128/` (manifest, strict audit,
teardown, journey log, per-step command output, HTTP monitor responses, durable rows, event reads, daemon
log lines with descriptions elided, and focused test outputs).

## Research

Reference runtimes were read from the vendored `.resources` tree (read-only):

| Reference | Finding | Decision |
| --- | --- | --- |
| OpenClaw `bash-tools.exec-notify.ts` | Background exec captures its owner, then wakes that owner | stable `monitor:<id>` enqueue to the owning session (existing) |
| OpenClaw cron schedules / background-process docs | Adopted work is never re-executed; unknown outcomes are explicit | recovery never reruns a command; `terminal_unavailable` is explicit |
| Hermes `process_registry` | Persist before notify, idempotent finish | fired outcome is durable **before** the wake is admitted; stable message id |
| GoClaw delegate ledger / gateway boot | Durable delivery states, reconcile interrupted work at boot | `session_monitors` table + `Service.Recover` at boot |
| Codex `async_watcher` / input queue | Drain output before exit; exits do not wake the model | terminal `Wait` result carries the screen (existing) |

The earlier decision to exclude persistence rested on a wrong premise ("persistence necessarily revives
operator-stopped sessions"). Evidence: `daemon/session_shutdown.go` stops sessions with `CauseShutdown`
on restart and `session/restart_continuation.go` resumes ordinary sessions that carry `StopShutdown`,
excluding operator stops (`CauseUserRequested`). Restart-resumed sessions therefore lost their monitors.
Persistence can keep operator-stop cancellation, so it is implemented.

## Implemented

1. **Idempotent terminal registration.** The same terminal condition (session, terminal, `until`,
   `pattern`, normalized timeout, description) while one is active or delivering returns that monitor
   (`Monitor.Reused`, `json:"-"`); timers are not deduplicated.
2. **Lifecycle logging.** `monitor.created|reused|recovered|fired|wake_deferred|wake_queued|wake_failed|
   canceled|interrupted|persist_failed` with id, session, kind, status, and a closed reason set; no
   description, pattern, output, or raw error.
3. **Durable monitors** (global DB migration `00162`, table `session_monitors`). Definition, status, owner
   binding, and a write version. Terminal output is never stored. Writes happen outside the service lock
   and the version fences stale writes; the first write holds the lock so a monitor that cannot be
   persisted is not admitted (`Create` fails).
4. **Restart recovery** (`Service.Recover`, boot step after interrupted-session resume):
   - timer: resumes on its original deadline; one that came due while the daemon was down fires at once;
   - fired-but-unadmitted (`delivering`): re-delivered once under the stable `monitor:<id>` message id,
     noting that terminal output was not retained;
   - terminal: a terminal does not survive a restart; the monitor fires `terminal_unavailable`; **no
     command is ever rerun**;
   - owner stopped on purpose or removed: monitor canceled, no wake, no revive;
   - owner interrupted by the restart: readmitted only through the session manager
     (`ResumeMonitorOwner`) using the restart-eligibility rules (`restartSkipReason`: auto-continue,
     archive, worktree, route, permissions, policy, workspace, failure, attempt limit) against the
     binding captured while the owner was live. A live owner is also checked against that binding.
     Otherwise the monitor is persisted `interrupted` with reason `owner_<rule>`, visible in `monitor_list`
     and HTTP, with no wake and no held cap slot;
   - an idle owner has no turn boundary, so no restart checkpoint is fabricated: the monitor row carries
     the binding captured at registration, and the stored restart-continuation rules are reused unchanged.
5. **Decision durability.** Cancel, fired, and undelivered outcomes are tracked decisions: each entry keeps
   the version of its latest decision and the highest version known durable, so confirmation is monotonic
   whatever order overlapping writes finish in. A decision whose write fails is queued, retried in the
   background every 5 s, and flushed at shutdown; `Cancel` reports an error until the row is durable (the
   monitor is already canceled in memory and cannot wake). The fired outcome is persisted (with the delivery
   backoff) before any wake, and a store that stays unavailable for the retry window leaves the monitor
   `undelivered` (queued the same way) rather than waking from an outcome a restart could not reproduce.
   A wake whose queue entry was already admitted (queued, dispatching, or sent) is not replayed after a
   restart: the daemon waker checks the queue ledger first; a failed or canceled entry may be admitted again.
   Transient owner-resume failures at boot are retried with backoff for the monitor's lifetime (at least the
   15-minute delivery window) while it stays listed, active, and cancelable, so an owner stopped in that window
   cancels the monitor; only then is it recorded `interrupted`. Lifecycle logs go through a bounded queue (1024; info entries drop with a
   `monitor.log_dropped` count, never inline under a lock).
6. **Remote-wait guidance.** Tool descriptor, official skill, and site monitors page state there is no
   polling kind: run the bounded watch command via `terminal_exec`, monitor that terminal, set
   `timeout_seconds` past the expected wait.
7. Test cleanup: two discarded `Shutdown` errors in the owning suite now fail the test.

## Not implemented in this PR

- **First-class remote-condition kind (CI/HTTP poller).** Not implemented here. A daemon-run poller would
  execute provider commands or network calls outside terminal command approval and the shell-wait guard
  and needs per-provider auth; a bounded terminal under existing policy is the documented path. Whether
  to add such a kind is a separate product decision, tracked outside this PR.
- **Making `fired` mean "turn executed"** or batching wakes: no evidence of need; monitors are one-shot.
- **Real `gh pr checks --watch` against GitHub and a real provider**: the lab uses `acpmock` agents that
  call the real hosted native tools; no live provider or remote was involved.

## Known limits (stated, not hidden)

- A hard crash while the store is unavailable can still leave the previous durable state (graceful
  shutdowns flush queued decisions). The recovery rules are conservative: a stale `active` row of an owner
  stopped on purpose is canceled at boot, a stale `delivering` row is re-delivered only if the queue ledger
  does not already hold its wake, and `Cancel` reports an unconfirmed write instead of claiming success. An
  owner-stop cancellation whose write failed, followed by a manual resume and a hard crash before the
  background retry landed, could still revive that monitor; failures are logged at error level.
- Recovery resumes each eligible owner at boot (same posture as restart continuation), even when its timer
  is far in the future.
- Terminal output of a monitor that fired before a restart is not retained; the wake says so.
- The binding is the one captured at registration (a stopped session no longer carries a complete route),
  so a legitimate route change between registration and restart yields `interrupted`, never a wake under a
  different policy.

- Deleting a session that is not running raises no stop notification, so its recovering monitors are canceled by
  the next owner check (at most 30 s later), not at the moment of deletion; this is also stated in the site docs
  and the official skill. The first transient owner-check failure is persisted (`owner_check_failed_at`,
  migration `00163`) so the retry window (`max(deadline, first failure + 15 min)`) is not extended by restarts.

## Cross-surface impact audit

- **Native tools:** `compozy__monitor_create` result text and `monitor_list` descriptor change; the
  `status` output enum gains `interrupted` (additive). Input schemas unchanged.
- **CLI / HTTP / UDS / OpenAPI / SDKs:** monitor payload `status` is an unconstrained string; the new value
  needs no contract change or codegen. `POST …/monitors/{id}/cancel` can now return 500 when the durable
  write fails.
- **Extensibility / hooks / config:** no new hook or config key. Restart eligibility reuses
  `session.restart.auto_continue` and the existing restart rules.
- **Workspace data isolation / persistence:** new global-DB table (migration `00162`) keyed by session with
  workspace and profile columns; added to the user-scope migration inventory. Owner resume goes through the
  session manager only. SD-013: additive migration, no public shape change.
- **Official skill / docs / Web:** `skills/compozy/references/native-tools.md`, site `sessions/monitors.mdx`,
  scenario `RT-session-monitor-wake` (steps 11–12). Web needs no change (`interrupted` is not a waiting
  status, so the row/popover treat it as finished).

## Defects found by the lab and fixed

The saved earlier walks (`evidence-defects/`) found two real defects that unit tests did not:

1. The stop observer read the session's persisted stop classification, which is empty when observers run,
   so a restart-interrupted owner's monitors were durably **canceled** at shutdown. Fixed: the daemon's
   own restart intent (`restartRequested`, the flag `stopSessions` uses) plus the stop cause decide.
2. The binding refreshed from the stopped session was degraded (118 bytes vs 457), so recovery reported
   `owner_binding_changed`. Fixed: keep the binding captured while the owner was live.

## Isolated lab evidence

Real daemon built from this branch (`bin/compozy`, `acpmock-driver`) in an isolated home with its own ports
and socket (never the default daemon), `acpmock` agents calling the real hosted native tools over MCP.
All paths below are relative to `docs/qa/reports/artifacts/2026-11-01-issue-1128/`. The lab binaries were
built from commit `e6e92dea8` (see `build-commit.txt`). Later commits change documentation and evidence and
review fixes outside the exercised paths (retry horizon and cancelability of a monitor whose owner check is
failing, the warning bound of the log queue, the shutdown flush deadline) and, after the lab, the
single-pass registration of all recovered records before the first owner check and the additive migration
`00163` (`owner_check_failed_at`). The lab ran against schema `00162` and a recovery pass that registered
records one by one. The exercised restart paths (resumed owner, typed ineligible owner, operator-stopped owner)
keep their behavior and are covered at the unit and migration level for the later changes; the lab was not
repeated for them. The `scripts/` directory holds the fail-closed replay scripts used for this run.

**A. Long external wait + duplicate registration** (`external-wait/`). A terminal polling an external marker
file; the agent called `compozy__monitor_create` twice with the identical condition and ended its turn.
Marker written after about 3m22s idle; exactly one `monitor` synthetic wake (`scheduler_reason: monitor_exit`);
monitor `fired` `exit_code 0`; one `monitor.created`, one `monitor.reused`, one wake.

**B. Restart recovery, eligible owner** (`restart-eligible/`). One session with a 150 s timer and a terminal
monitor, one operator-stopped session with a timer; `POST /api/settings/actions/restart`. Durable rows
before restart `active`; after boot the owner was resumed and the terminal monitor fired `terminal_unavailable`
right after the daemon returned; the timer fired on its original deadline (`timer_elapsed`); two wakes in total
(`monitor_terminal_unavailable`, `monitor_timer_elapsed`); the operator-stopped session's monitor was
`canceled` (`session_stopped`) and nothing woke or resumed it.

**C. Restart recovery, ineligible owner** (`restart-ineligible/`). `session.restart.auto_continue=false`:
after restart both monitors are `interrupted` with `owner_disabled`, listed over HTTP, and no wake or
resume happened after the timer deadline passed.

**Hygiene.** `qa/teardown.json` shows `"clean": true`; the strict audit result is in
`qa/qa-audit-report.md`.

## Validation (focused; CI owns the gate)

`tests/` holds saved outputs of the focused runs: `go test -race ./internal/monitor`,
`./internal/session -run ResumeMonitorOwner|ResumeInterruptedSession`, `./internal/daemon -run Monitor`,
`./internal/store/globaldb -run SessionMonitors|UserScopeMigration`, `./internal/tools/builtin`,
`./internal/toolmeta`. No `make gate`, full suites, or local E2E were run.
