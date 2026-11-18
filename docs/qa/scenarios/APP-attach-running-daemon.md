---
id: APP-attach-running-daemon
area: APP
title: Attach to my already-running runtime and touch nothing
persona: Dora
journey: J-desktop-attach-daily
expected: With a healthy runtime and an active session, the app shows the identical workspace/session state the browser tab shows (same origin, same local UI state), no second daemon appears, the existing process is never terminated, and side-by-side actions reflect live in both surfaces.
entry_points: dock/launcher icon with a running daemon; browser tab open side by side
qa_status: pass
bug_ids: BUG-20260810-desktop-runtime-stalls
fix_status: fixed
retest_status: pass
fix_commits: b415f24b; b3aa3d27; bd610cfa; 02b55a46
evidence: docs/qa/reports/2026-08-17-electron-shell.md
last_report: docs/qa/reports/2026-08-17-electron-shell.md
overlaps:
---

PRD stories: US-003 (attach, zero writes; AC-3 isolated home; EC-1 stale record; EC-2 foreign
squatter; EC-3 unhealthy), US-020 (browser coexistence AC-1/AC-2, EC-1 different homes never
mixed). Test IDs: E2E-003, E2E-018; IT-001, IT-004, IT-026, IT-027, IT-028; UT-013–UT-023,
UT-089–UT-091.

Per-OS evidence: macOS and Linux capture process-table before/after app open (no spawn),
same-origin proof (session + local UI state parity with the tab), and the live two-way sync walk
(E2E-011). Both use Playwright `_electron` plus the recorded manual sync check.
Isolated-home labs only: the lab manifest's `COMPOZY_HOME` must be the one resolved — never the
operator default home.

QA impact 2026-08-11: the startup resolver now waits for a live recorded daemon and restricts any
automatic repair to verified desktop-owned processes. The isolated macOS walk reopened the app,
kept daemon PID `89043`, and retained one listener. Browser/session parity and the shipping OS
matrix remain blocked for artifact verification.

Issue 1252 — relaunch the app while the daemon is still booting: start the runtime (or let an
update relaunch it) so it holds `daemon.lock` but has not yet published `daemon.json` or opened its
listener — a slow boot migration on a large database is the natural trigger. Open the app or press
"Retry operation" during that window. Expected: the bootstrap stream shows one `start` attempt,
then `attach started` naming the booting pid, then `Attached to the running CompozyOS runtime.`
with that same pid once it is ready. There is no "could not start" screen, no `retrying` burst, and
exactly one daemon process for the home. If the booting daemon exits, or stays unready past the
attach deadline, the failure names the running daemon pid, not "detached daemon exited before
readiness". A dead lock owner (stale lock) still recovers through normal start retries.
Evidence 2026-11-04 (isolated `COMPOZY_HOME`, macOS, working-tree build): three runs of a
foreground `daemon start` with `daemon bootstrap` launched while the daemon held the lock without
`daemon.json` all ended `ready completed attach <same pid>` with one daemon process. A lock holder
killed mid-wait ended `attach failed` naming its pid.

Issue 1619 — open the app while the global config cannot be loaded: with a healthy daemon running,
add an unknown key to `$COMPOZY_HOME/config.toml` (for example `[daemon]` with `bogus = true`),
then open or relaunch the app. Expected: the bootstrap stream ends on one `failed` event with
`classification: "config-invalid"` and a `config` object naming the file and `unknown_keys`; there
is no `start` attempt, no `retrying` event, and no "daemon is listening but unhealthy" text. The
boot window shows "CompozyOS could not start" with the detail "Your config file has a setting
CompozyOS does not recognize: daemon.bogus. Remove or rename it in <path>, then try again.", the
code line `config_invalid` with no attempt counter, and the actions **Try again**, **Open config
file**, **Open logs** and **Quit**. **Open config file** opens `$COMPOZY_HOME/config.toml`. The
running daemon is never signaled and keeps its pid. Repeat with invalid TOML (for example
`socket = = 1`): the detail reads "Your config file is not valid TOML at line N, column M. Fix
<path>, then try again." Repeat with no daemon running: one launch is tried, then the same
`config-invalid` failure appears without the five-attempt backoff. Fix the file and press **Try
again**: the app attaches or starts normally. An app older than this change shows the same failure
as `runtime_start_failed` with the config error as its detail and does not retry.

## Issue 1614 requirement 7: desktop handoff

QA impact 2026-11-12 (#1614): the bootstrap waits on a runtime mutation instead of retrying,
fails once on a runtime that cannot be launched, and (asked by the app with `--ready-phase
serving`) reports the runtime ready once it serves reads. Validation for this change is CI-only;
the walk has not been run.

Walk, in an isolated home (never the operator's default home or port):

1. Runtime mutation in progress: hold `$COMPOZY_HOME/update.lock` from another process (a runtime
   update, `compozy daemon relaunch`, or a second `compozy daemon start` that is still waiting),
   then open the app. Expected: the bootstrap stream shows one `start started` event for attempt
   1, then one `start started` event with `classification: "mutation-in-progress"` and
   `wait: { reason: "runtime_mutation", owner_pid }`; there is no `retrying` event. The boot window
   reads "Starting CompozyOS", status word "Waiting", detail "Another update or restart is in
   progress (process N). CompozyOS continues when it finishes." When the lock is released the
   same attempt continues and the product opens.
2. Keep the lock held past the attach deadline (10 minutes; shorten it in a lab build). Expected:
   one `failed` event with `classification: "mutation-in-progress"`, the boot window shows
   "CompozyOS could not start" with "Another CompozyOS update or restart is still running. Wait
   for it to finish, then try again.", the code line `update_lock_held` with no attempt counter,
   and **Try again**.
3. Runtime that cannot be launched: remove the execute permission from `$COMPOZY_HOME/bin/compozy`
   and open the app. Expected: one launch, then one `failed` event with `classification:
   "runtime-unlaunchable"` and no `retrying` event; the boot window shows `runtime_start_failed`
   with the reason.
4. Open at the serving phase: start the app against a home with sessions interrupted mid-turn so
   boot recovery takes several seconds. Expected: the bootstrap stream ends on `ready completed`
   with `readiness.phase: "serving"` while `compozy status -o json` still reports
   `daemon.status: "draining"`; the product window opens and shows the notice "Resuming
   interrupted sessions" (or the current step) with "You can read your sessions now. Sending
   opens again when this finishes."; a prompt sent now is refused by the daemon; the notice
   clears within about 2 seconds of `daemon.readiness.phase: "ready"` without a page reload.
5. Older runtime: attach the app to a daemon that reports no `daemon.readiness`. Expected: the
   product opens only once `daemon.status` is `running`, as before, and no notice appears.
6. Relaunch during boot (issue 1252 walk): with the app asking for the serving phase, the
   bootstrap attaches to the booting lock owner as soon as it serves, before `daemon.json` exists.

`TestDaemonBootstrapDesktopHandoff` in the CLI bootstrap suite, `bootstrap-state.test.ts`,
`bootstrap-event.test.ts`, `bootstrap-runner.test.ts` and `boot-journey.test.ts` in the desktop
suites, and the status system's `use-daemon-health.test.ts` own these rules.
