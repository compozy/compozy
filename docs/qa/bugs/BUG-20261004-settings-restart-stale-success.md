# BUG-20261004-settings-restart-stale-success: Settings claims later changes are active before restart

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-administer-runtime-settings, save another section after a successful restart
- **Scenarios:** MS-037
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

In CH-settings-skills-automation-sections / Garbage Tour, save Skills polling at 4s and
Automation max_concurrent_jobs at 8 through the public Web and settings API. Click Restart
CompozyOS. The new daemon activates both values and Web displays Restarted.

Save a subsequent candidate through the public API: restore Skills polling to 3s and Automation
max_concurrent_jobs to 5. Both successful writes report restart-required. Reload the Automation
settings page: the saved field reads 5, but the banner still says Restarted / All saved changes
are now active. No Restart CompozyOS button is present. Independent `compozy status --json`
reports restart_required=true and apply_state=pending_restart. Dismiss removes the old success
banner without exposing the required restart.

The session ends before engineering inspection. Public CLI stop/start then activates the initial
values as cleanup; that recovery does not validate the Web behavior.

## Evidence

Under `docs/qa/evidence/2026-10-02-untested/`: settings-sections-restore-{skills,automation}.json,
settings-sections-final-status.json, settings-sections-restore-restart-ready.json,
settings-sections-stale-restart-receipt.json, settings-sections-stale-restart-receipt.png,
settings-sections-walk-ended.json, and settings-sections-cleanup-final-status.json.
Recording: `/Users/pedronauck/.config/browser-harness/agent-workspace/recordings/settings-skills-automation-sections`.

Earlier section writes, invalid-candidate rejection, HTTP/UDS parity, unrelated-section
preservation, reload persistence, and the first Web restart all succeeded. The stale success
finding prevents a passing scenario verdict. Root cause and repair remain pending.
