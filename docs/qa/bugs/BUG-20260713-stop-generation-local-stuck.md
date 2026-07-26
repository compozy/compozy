# BUG-20260713-stop-generation-local-stuck: Stop generation cannot recover a pre-daemon optimistic run

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-17, recover a first prompt that has not reached the daemon
- **Scenarios:** RT-new-session-fast-feedback
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** First-prompt live Cursor/Grok diagnostic continuation

## Summary

When a prompt is optimistic and submitted in assistant-ui but has not reached the daemon, clicking `Stop generation` calls the daemon cancellation path but leaves the local run submitted forever. `Working…` remains visible, the composer does not recover, and the session-level stop control can remain in `Loading` even though the daemon has no active prompt to cancel.

## Reproduction

1. Open fresh Cursor/Grok session `sess-8eb726b62df96bb3` after creating it through the Agents modal.
2. Send the first `/goal` command and observe the optimistic `Working…` state while daemon inspection remains idle with `active_prompt=false`.
3. Click `Stop generation`.
4. Observe the local composer and the authoritative session state.

**Expected:** Stop cancels the local assistant-ui run first, removes the optimistic working state, and restores the composer even when no daemon prompt was admitted. The existing daemon cancellation control still runs when applicable.
**Actual:** The backend callback has nothing to cancel, while assistant-ui remains submitted and the visible controls never recover.

## Evidence

- Live session `sess-8eb726b62df96bb3`, workspace `ws_30f28bfa2ef7ac98`.
- Browser DOM retained `Working…` after `Stop generation`; the session banner rendered `Stop session` as `Loading`.
- Durable inspection remained healthy/idle with `active_prompt=false`, Goal `null`, and no prompt POST.
- The canonical StrictMode integration reproduced the local failure with a pending prompt response: the external cancel callback fired once, but the Stop button remained and Send stayed disabled before the fix.

## Fix

- **Root cause:** `SessionThread` forwarded cancellation only to the daemon-facing `onCancelPrompt` callback. It never called assistant-ui's `aui.thread().cancelRun()`, so a request stuck before daemon admission had no owner capable of unwinding the local submitted state.
- **Fix commit:** uncommitted QA remediation batch
- **Regression test:** The canonical session runtime/provider integration mounts the exact hook-only epoch-0/generation-0 transcript under React StrictMode, reproduces EventSource open/cleanup/open, keeps the prompt response pending and abortable, clicks the public Stop control, and requires one external cancellation plus disappearance of Stop and a re-enabled Send control.

## Verification

- The exact StrictMode regression passed after the minimum component correction.
- The full canonical session runtime/provider suite passed 30/30.
- Forced repo-root Turbo Web typecheck, focused zero-warning formatting/lint, and `git diff --check` passed.
- Same-persona Chrome retest passed in fresh Cursor/Grok session `sess-2a768148b6106dc3`: the ordinary prompt POST completed once, `Stop generation` issued exactly one `/prompt/cancel` request with HTTP 200, daemon inspection converged to `idle` with `active_prompt=false`, the local Stop control disappeared, and the composer accepted a new unsent draft.
- Evidence: `/Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/network/catalog-global-goal-acceptance.json`, `qa/screenshots/stop-generation-before.png`, and `qa/screenshots/stop-generation-composer-ready.png` in the same lab.
