---
id: APP-desktop-settings-restart-in-place
area: APP
title: Restarting CompozyOS from Settings keeps the desktop window open
persona: Dora
journey: J-desktop-attach-daily
expected: With the desktop app open, saving a restart-required setting and choosing Restart CompozyOS keeps the product window open on the same route. The settings notice stays on "Restarting…" while the daemon is down, the window reconnects to the replacement daemon (new pid and start time) and shows "Restarted". This holds whether the app owns the runtime or attached to one started from the CLI, and for restarts that take longer than the shell's ~6 s health threshold. A runtime that never returns hands off to the boot window, which resolves the runtime again and offers Retry on failure; the app never exits silently.
entry_points: Settings → General → Privacy → "Hide passwords and keys in logs" → Save → Restart CompozyOS; `compozy daemon stop` against the attached home for the lost-runtime branch
qa_status: pass
bug_ids: GH-1336
fix_status: fixed
retest_status: pass
fix_commits:
evidence: PR for #1336 (window screenshots, timelines, and the baseline quit log)
last_report:
overlaps: APP-attach-running-daemon; APP-start-installed-daemon; APP-runtime-update-app-owned; APP-quit-contract
---

QA impact 2026-11-07 (#1336): the shell's runtime health monitor closed the product window after
three failed identity probes (~6 s) while the boot window had already been destroyed, so a restart
that outlasted the threshold left zero windows and `window-all-closed` quit the app cleanly. The
monitor now keeps the product window through the outage, reloads its route when the runtime
answers again, and only after a 45 s reconnect window reopens the boot window before closing the
product and running bootstrap again.

Walk, in an isolated home (never the operator's default home or port):

1. Launch the desktop app (dev build with `COMPOZY_DESKTOP_BUNDLE_ROOT`, or packaged), complete
   onboarding, open Settings → General, toggle "Hide passwords and keys in logs" and save.
2. Choose Restart CompozyOS. To exercise the slow branch, hold the lab's `compozy daemon relaunch`
   helper with `SIGSTOP` for 15 s, then `SIGCONT`. Never signal a process outside the lab home.
3. Expect one product window on `/settings/general` throughout, "Restarting…" while
   `/api/status/identity` is unreachable, `runtime unreachable; awaiting reconnect` and
   `runtime reconnected; reloading product window` in `logs/desktop.log`, then "Restarted" with a
   new daemon pid.
4. Repeat with the daemon started from the CLI before the app opens (`app.json` shows
   `owned: false`).
5. Lost runtime: `compozy daemon stop` with the app open. After ~50 s the boot window appears,
   bootstrap starts the runtime again, and the product window returns.

A session started from a daemon that a restart helper launched inherits
`COMPOZY_INTERNAL_RESTART_OPERATION_ID`; unset it before starting a lab daemon from such a shell,
or the lab daemon exits with `restart operation not found`.

## Issue 1614 requirement 7: liveness-based wait and soft reconnect

QA impact 2026-11-12 (#1614): this section replaces the 45 s cutoff and the reload described
above. Validation for this change is CI-only; the walk has not been run.

- The 45 s reconnect window no longer abandons a runtime by itself. After it, the shell keeps
  the product window for as long as a runtime process is alive: the pid it was connected to, the
  pid recorded in `daemon.lock`, or the owner recorded in `update.lock` (the restart or update in
  progress). It hands off to the boot window only when none is alive, or after 10 minutes.
- On reconnect the shell keeps the loaded page when the answering runtime has the same version,
  the page is loaded, and the runtime reports `daemon.readiness`; the page then refreshes its own
  data. `logs/desktop.log` reads `runtime reconnected; product window refreshes its data`.
  Otherwise the window reloads and the log reads `runtime reconnected; reloading product window`:
  a runtime update that changed the version, a page that failed to load during the outage, or a
  runtime without lifecycle phases.

Walk:

1. Step 2 of the walk above with the slow branch held for 90 s (past the 45 s window). Expect
   one product window throughout, no boot window, and the soft-reconnect log line. Text typed in
   an open composer before the restart is still there after it.
2. While the replacement daemon is in its serving phase, expect the desktop notice "Resuming
   interrupted sessions" (or the current step); it clears when the daemon is ready.
3. Lost runtime: `compozy daemon stop` with the app open. With no runtime process left, the boot
   window appears after about 50 s, as before.
4. Runtime update that changes the version: expect the reload log line and the new web bundle.

