# BUG-20261004-settings-startup-false-offline: Settings reports an offline runtime during startup

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Dora
- **Journey Step:** J-administer-runtime-settings, enter Settings after refreshing Tasks
- **Scenarios:** MS-web-settings-takeover-redesign
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Dora clicks the enabled Settings button after refreshing Tasks. Settings does not open and a
toast says CompozyOS is not reachable, although the server is responding. After initialization,
a later click opens General. This adds an unnecessary retry at the start of a settings visit.

## Reproduction

- **Charter:** CH-untested-041-administer-runtime-settings-dora · **Tour:** Back-Button Tour
- **Environment:** real isolated daemon, production Web, desktop 1512 x 862, en-US,
  America/Los_Angeles, Wi-Fi 20 ms / 20 Mbps down / 5 Mbps up.

1. Start at Tasks and refresh the document.
2. Click Settings as soon as its enabled rail button appears.
3. Observe the toast: Settings → General — CompozyOS isn't reachable right now, with Retry.
4. After initialization, click Settings again. General opens.

**Expected:** The command launcher waits for its owning command to become available, then opens
Settings on the first enabled click.
**Actual:** Settings is enabled before its command, dispatches an unavailable action and reports
a misleading offline condition. Neighboring app launchers already wait for their own readiness.

## Evidence

Paths are under docs/qa/evidence/2026-10-02-untested/ unless absolute.

- settings-idle-final-dora-save.json retains the original-persona first-entry failure; the later
  settled timeout workflow succeeds.
- settings-startup-availability-diagnostic.json shows Settings enabled at 1.919 seconds while
  Vault is disabled. The click produces the exact toast above, with no Settings window.
  Network responses for status and the command catalog are 200.
- The closed four-frame engineering recording is
  /Users/pedronauck/.config/browser-harness/agent-workspace/recordings/settings-startup-availability-diagnostic.
  Frame 0002.jpg was opened and inspected and shows the refusal during initial loading.
- settings-startup-after-recovery-attempt.png was opened and inspected and shows General.
  The recovery click succeeded; the first observer incorrectly waited for a General heading
  (the real host heading is Settings). Its failure is retained and corrected by
  settings-startup-availability-diagnostic-closed.json. No config is changed.

Registry search found no existing owner. The historical settings-nav-stale-open-history bug
concerns superseded section navigation, not this startup command availability mismatch.

## Fix

- **Root cause:** OsRailFoot exposes the Settings command as an always-enabled button. The
  dispatch seam correctly refuses an unconfirmed palette catalog, but the launcher does not
  consume that same command's availability. Window-manager readiness alone is insufficient:
  palette catalog confirmation is a separate input.
- **Correction:** Read the existing settings.general projection through usePaletteCommand and
  disable only that command button until it is available. Keep local theme controls usable.
  Reuse the existing command resolver; do not bypass dispatch or add timed retries.
- **Fix commit:** pending.
- **Regression test:** existing OS dock component suite, os-dock.test.tsx, owns the rail foot
  and compact presentation. Cover missing, unavailable, available and lost availability, while
  local theme changes remain reachable.

## Verification

Pending red/green proof and a fresh original-persona reload/entry/close walk in both dock
presentations. The small repair has a known component owner, no state migration, no API change,
no dependency and no product trade-off.

### Repair replay — 2026-10-04

The initial owning-suite run reproduces both missing-command guard failures. It also exposes
an existing UT-086 timing issue: menu text can exist before focus transfers. A focused trace
confirms the menu then first-item focus transition and one tab activation. The existing test
now asserts those focus transitions before sending the next key, preserving the original
one-activation assertion. Its compact roving-focus action also runs inside act. No menu
production code or assertion is weakened. The final full dock suite passes all 34 tests
without warnings; root Web typecheck/build pass. React Doctor remains 93/100 with the same four
existing advisory complexity reports.

The production index SHA256 is
30465e3c536483fd902a61e7ae897ffa2240f8b907fba205cd91a49c00bd72b7, verified against the served
document. Fresh Dora entry observes Settings disabled on initial appearance at both 1512px and
720px. Clicking in that state produces no offline toast. Once enabled, one click opens General.
The local theme control switches both ways and returns to dark. Public General GET equals the
complete baseline, with zero active sessions and unchanged PID 77401.

The exact 14-frame closed recording is
/Users/pedronauck/.config/browser-harness/agent-workspace/recordings/settings-startup-final-dora.
Desktop and compact open-state PNGs, compact closed state and the restored desktop PNG were
opened and inspected. The compact host exposes Close window rather than Close Settings; it
closes the containing frame and returns to Home. Dora reopens Tasks through the visible dock.
The first locale reapplication was rejected by CDP because the matching override was already
active; the next read confirms en-US / America/Los_Angeles. Those driver receipts remain intact,
as do the incorrect desktop-label and immediate-Tasks waits; neither is an app-entry failure.

Evidence: settings-startup-availability-focused-red.json, focused-green.json (the separate
UT-086 failure), settings-startup-dock-keyboard-focus-diagnostic.json,
settings-startup-availability-final-focused.json, web-build.json, react-doctor.json and
build-identity.json; settings-startup-final-dora-completed.json (successful startup entries),
complete-closed.json and settings-startup-final-general-unchanged.json.
Required gate and fix commit are the remaining delivery steps for this defect.
