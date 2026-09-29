# BUG-20260928-message-actions-enabled-during-remote-turn: Fork from here and Rewind stay enabled while a turn started elsewhere runs

- **Status:** fixed
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** J-14 derive branch: Fork from here while the thread runs
- **Scenarios:** ET-web-session-fork-from-here; RT-conversation-rewind
- **Found:** 2026-09-28 · **Report:** docs/qa/reports/2026-09-28-session-continue-fork-exec-b2.md

## Summary

A turn started from the CLI/API, from another window, or before a page reload is running ("Thinking…", badge running). Every "Fork from here" and "Rewind to here" on settled user messages stayed enabled (VC-05 requires both disabled).

## Root cause

`useSessionMessageActionGate` read only assistant-ui's local `thread.isRunning`, which reflects runs dispatched by this page.

## Fix

`useSessionRuntimeExtensions` exposes `sessionRunning` (`isSessionRunning` over the daemon session detail) through `SessionRuntimeRenderContext`, and the gate counts it as busy. Regression: `session-thread.test.tsx` "Should disable Fork from here together with Rewind while the daemon reports a running turn". Two tests that meant "idle" relied on the fixture's `badge: running` being ignored, so their harness detail now reads idle explicitly (`session-thread.test.tsx` fetch mock; `session-chat-runtime-provider.test.tsx` "…releases rewind"). Re-walked on `sess-545efc7fcf32d50c` (VC-05).

## Evidence

docs/qa/evidence/2026-09-28-session-continue-fork-b2/before-fork-enabled-while-running.png
