# BUG-20261004-settings-offline-save-stuck: Settings stays on Saving while offline

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Dora
- **Journey Step:** J-administer-runtime-settings, save an Extensions policy draft without a connection
- **Scenarios:** MS-web-settings-takeover-redesign; MS-attention-settings-roundtrip
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Dora loses the connection before saving a draft. Settings stays on Saving with Save and Discard
disabled, without an error or an explanation. When the connection returns, the write runs
automatically. The configuration eventually saves, but Dora cannot recover or discard the
pending change while offline.

## Reproduction

- **Charter:** CH-untested-041-administer-runtime-settings-dora · **Tour:** Back-Button Tour
- **Environment:** isolated real daemon and production Web, 1512 x 862, DPR 2, en-US,
  America/Los_Angeles; Wi-Fi 20 ms / 20 Mbps down / 5 Mbps up, then browser offline.

1. From Tasks, open Settings and Extensions. Disable GitHub acquisition in the draft.
2. Take the browser offline before clicking Save changes once.
3. Observe Saving with both action buttons disabled. It remains unchanged throughout the
   six-second observation and a subsequent read; no failure or offline explanation appears.
4. Restore the original connection without clicking Save again.
5. Observe the automatic save, then reload and confirm the disabled GitHub source persists.
6. Restore the original enabled source through the visible Save changes action.

**Expected:** A failed explicit save settles as an error, preserves the editable draft and
allows Discard or an explicit retry. Reconnection alone does not submit a failed draft.
**Actual:** The operation pauses before transport while the UI reports active saving; it resumes
automatically and the user cannot discard it in the meantime.

## Evidence

Paths are under docs/qa/evidence/2026-10-02-untested/ unless absolute.

- settings-pages-dora-extensions-offline-save.json and extensions-offline.png show the stall.
- settings-pages-extensions-after-offline-attempt.json proves the saved config is unchanged.
- settings-pages-dora-extensions-reconnect-observed.json and
  settings-pages-extensions-after-reconnect.json prove submission on reconnection alone.
- extensions-reconnected.png and extensions-reloaded.png show the resulting saved setting.
- settings-pages-final-extensions.json proves complete baseline restoration. The final runtime
  status reports apply_state=current, restart_required=false and zero active sessions, with the
  same PID 77401. No restart or package/trust mutation occurs.
- The exact 41-frame recording is closed at
  /Users/pedronauck/.config/browser-harness/agent-workspace/recordings/settings-pages-dora.
  All 15 session PNGs were opened and inspected. PNG basenames above have the
  settings-pages-dora- prefix.

The registry search found no existing owner for this Settings offline-save symptom.

## Fix

- **Root cause:** Settings section mutations inherit TanStack's online network mode, which
  pauses before calling the adapter when offline. The save-bar owner maps isPending to Saving
  and disables both actions. Application mutation retries are already disabled, but that does
  not prevent the initial pause or automatic continuation on reconnect.
- **Correction:** Give explicit Settings form writes their own immediate transport policy, so
  adapter failure reaches the existing error/draft recovery path. Convert fetch rejection to
  SettingsApiError with actionable guidance and the original cause, preserving cancellation
  and server validation errors. Keep global query and other
  domain mutation policies intact; preserve the direct Attention gesture-owned request.
- **Fix commit:** pending.
- **Regression test:** web/src/systems/settings/hooks/__tests__/use-settings-mutations.test.tsx.
  Invariant: an offline explicit form save reaches a settled error, remains unsubmitted after
  reconnection and succeeds only on an explicit retry. The Settings mutation hook layer owns
  admission/settlement; the existing save-bar behavior and fresh browser replay own recovery UI.

The session is closed before engineering changes. This is a bounded Web-only repair with no
schema, transport contract, dependency or persistence change. The current draft/error contract
already owns retry and discard; no new offline queue or product choice is introduced.

## Verification

Pending owning-suite red/green proof and a fresh original-persona offline/discard/retry/reload
walk, with adjacent Settings saves and full baseline restoration.


### First repaired replay — 2026-10-04

The nine owning mutation cases fail before repair; six focused suites pass 59 tests afterward.
Final Web typecheck/build pass after correcting only the new table callback's union inference.
React Doctor stays 93/100 with its four established complexity advisories.

Dora's 26-frame settings-offline-final-dora recording is closed and all eleven PNGs are inspected.
Extensions offline error/discard, no automatic reconnect write, explicit retry, reload and full
restoration pass. Memory's adjacent draft/discard and Notifications' immediate Sound failure,
reconnect, explicit write and restoration pass. Public reads independently confirm every state;
runtime is current with no restart and zero active sessions. The incorrect Notifications heading
wait is retained as a driver error and corrected by the semantic page read.

The error now settles but displays only Failed to fetch. Complete the promised recovery feedback
at the Settings write adapter, retaining the native transport error as cause and passing
cancellation through unchanged. Existing HTTP validation diagnostics retain precedence.
The Settings API suite owns transport classification, separate from mutation admission; exact
prose is verified in the real browser rather than frozen by another assertion.


### Final original-persona replay — 2026-10-04

The final seven focused suites pass 101 tests, including both distinct red-before/green-after
invariants. Web typecheck/build pass. The served final index SHA256 is
016bcb1387411bf2eb36732c4e42880c6b23d0081a32aa46d4836d080701b813.

Dora repeats the journey from Tasks. The offline failure now names the unsuccessful save and
asks her to check the connection and try again. Both Save and Discard remain usable. Discard
restores the saved source while offline. A repeated failed draft remains unsent through
reconnection; independent GET proves the whole baseline unchanged. Explicit retry persists only
the intended source field, shows Saved with the typed restart notice and survives reload.
Restore the original policy through Web.

Adjacent Memory and Notifications show the same guidance with no locked action or false saved
value. Memory's discard restores five recalls. Sound stays enabled after failure/reconnection;
only a new toggle sends PATCH 200. Restore it and reload. Independent complete configs all match
their baselines; runtime is current, no restart required, zero active sessions, unchanged PID
77401, and browser notification permission remains default.

The exact closed 26-frame recording is
/Users/pedronauck/.config/browser-harness/agent-workspace/recordings/settings-offline-guidance-dora.
All eight final PNGs were opened and inspected. Receipts use settings-offline-guidance-;
settings-offline-save-final-focused.json and final-build.json own engineering checks.
The required gate and fix commit remain before delivery bookkeeping promotes this defect.
