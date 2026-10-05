# BUG-20261004-settings-idle-timeout-display: Saved idle timeout looks unsaved and reloads as Never

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-administer-runtime-settings, save and reread an idle-session cutoff
- **Scenarios:** MS-web-settings-takeover-redesign; MS-025
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Dora saves a four-hour idle cutoff. The daemon persists it, but Settings continues to report
Unsaved changes. Reload then displays Never, suggesting the saved setting was lost. The desired
config still contains four hours; this is not evidence that the daemon discarded it or that a
restart occurred. The truthful Restart needed notice remains visible.

## Reproduction

- **Charter:** CH-untested-041-administer-runtime-settings-dora · **Tour:** Back-Button Tour
- **Environment:** isolated real daemon and production Web; Dora, 1512 x 862, en-US,
  America/Los_Angeles, Wi-Fi 20 ms / 20 Mbps down / 5 Mbps up.

1. From Tasks, open Settings / General.
2. Focus End idle sessions after and choose 4 hours using the native selector.
3. Save once. Independently read GET /api/settings/general: session_timeout is 4h0m0s.
4. Observe that the save bar still says Unsaved changes, then reload.
5. Observe Never although the independent read still returns 4h0m0s.

**Expected:** Successful save clears the submitted draft, adopts the daemon's canonical duration
and displays 4 hours after reload while retaining the correct restart notice.
**Actual:** The saved draft remains dirty; reloaded canonical duration renders as Never.

## Evidence

All paths below are under docs/qa/evidence/2026-10-02-untested/.

- settings-idle-typeahead-dora-saved-observed.png: 4 hours, Restart needed and Unsaved changes.
- settings-idle-typeahead-dora-after-reload-observed.json/.png and reload-mismatch.png:
  Never after reload; all four screenshot checkpoints were opened and inspected.
- settings-idle-typeahead-dora-after-save.json and reload-general.json:
  independent successful reads retain 4h0m0s.
- The exact closed 13-frame recording is
  /Users/pedronauck/.config/browser-harness/agent-workspace/recordings/settings-idle-typeahead-dora.
- The first observer fails on a null innerText after the save; no save is retried. The later
  exact 4-hours assertion fails after reload. Failed driver receipts remain intact.
- Ada restores the entire original General config through public PATCH after the persona
  session ends. settings-idle-typeahead-cleanup-read.json confirms the complete baseline,
  zero active sessions and no daemon restart.

Registry searches found no existing General timeout owner. The older
BUG-20260729-skill-policy-normalized-dirty affects a separate Skills draft machine and policy
save channel, so its historical evidence is not reused as proof of this General failure.

## Fix

- **Root cause:** General's local parser only accepts a number plus one unit, rejecting the
  daemon's compound Go duration. Its save-success callback records a label but never releases
  the submitted draft override, leaving 14400s unequal to the refetched 4h0m0s.
- **Correction:** Reuse the exported duration parser. After successful mutation and
  canonical refetch, release only the exact submitted General draft; preserve newer edits and
  drafts owned by another workspace context. Do not change serialization, restart semantics,
  timeout enforcement or the server contract.
- **Fix commit:** pending.
- **Regression test:** existing General route suite owns canonical-duration display; existing
  use-settings-general-page suite owns save-baseline adoption and in-flight draft preservation.

## Verification

The red run fails exactly seven checks: six canonical-duration display cases and one
acknowledged-draft adoption case. Failure and newer-draft preservation controls already pass.
Evidence: settings-idle-timeout-focused-red.json. The production repair is applied; green run,
fresh original-persona save/reload/restore, required gate and fix commit remain pending.
The governor permits this bounded repair: known UI cause, existing parser and state owner,
no migration, no product decision, no new dependency.


### Original-persona replay — 2026-10-04

The fresh 20-frame settings-idle-final-dora recording is closed at
/Users/pedronauck/.config/browser-harness/agent-workspace/recordings/settings-idle-final-dora.
Dora chooses/discards four hours, then saves it once. The observed status changes from Saving
to Saved, the dirty bar disappears after the success flash, and an ordinary reload retains
4 hours beside the typed restart notice. Public GET confirms 4h0m0s as the only changed field.
Returning to Never and saving/reloading restores the complete original config. Public status
reports current config, restart_required=false and zero active sessions; PID 77401 is unchanged.
All five PNG checkpoints were opened and inspected.

The first driver assertion mistakenly expected Discard to disappear during the brief Saved
flash; the control was disabled and the status already said Saved. No duplicate write occurred.
The subsequent clean-state and reload receipts prove the intended result. An initial Settings
navigation click immediately after document reload was not observed to open the window; a single
later click did. This separate startup observation is retained for diagnosis, not used to obscure
the successful timeout save/reload assertions.

Evidence uses the settings-idle-final-dora prefix: save-completed.json (contains the early
driver assertion), canonical-reload.json, restored-ended.json, after-save.json,
baseline-restored.json and runtime-status.json. settings-idle-final-after-refusal.json also
confirms an invalid-duration 400 leaves the complete baseline unchanged.

The final root Turbo focused run passes all 48 tests; typecheck/build pass.
React Doctor remains 93/100 with four advisory complexity reports: three previously reported
files plus the now-scanned General hook. A separate untouched source extracted with git show
from 9108465d9 produces the same General 18/18 complexity and depth 2 as the final hook.
No rule is suppressed and no unrelated workflow refactor is introduced.
Fix commit and the required final-tree gate remain pending.
