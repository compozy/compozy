# BUG-20261004-settings-restart-stale-success: Settings claims later changes are active before restart

- **Status:** verified
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
finding prevents a passing scenario verdict.

## Root cause and repair

The Web restart hook derived the requirement only from mutations recorded in this document, while
the presentation let an older successful operation outrank that requirement. External writes did
not update the stored mutation. The hook now reads the daemon's canonical status query; apply
history identifies only the candidate whose notice was dismissed. A later candidate can re-open
the notice, and current restart requirements outrank an older success receipt. Local writes and
terminal restart observations invalidate those same canonical queries. The persisted interaction
state migrates losslessly to add the dismissed candidate identity.

The existing restart hook and presentation suites reproduce both defects before the repair and
pass all 20 focused cases afterward, including old-envelope hydration. Root Turborepo typecheck,
lint and production build pass; build output retains the existing CSS/bundle advisory warnings.
React Doctor reports 93/100 with three complexity warnings outside the settings repair.
Evidence: `settings-restart-owning-{red,green}.json`,
`settings-restart-web-validation-formatted.json`, and `settings-restart-react-doctor.json`.
The first Chrome replay below exposed the remaining profile scope defect.

## First repair replay — profile scope still fails

A fresh Dora session in Chrome saved the editorial profile Skills polling from 3s to 4s. HTTP and
UDS independently confirmed the value, but the repaired Web banner offered no restart action.
The latest public apply record (`cfgapp-543f53a9781fb751`) names `profile-config`, `blocked` and
`restart-daemon`, while public daemon status says `restart_required=false` and `current`.
The global status flag alone therefore cannot replace the existing scoped restart requirement.
The session ended as Fail before engineering inspection; its profile value was restored through
UDS and read back over HTTP. No broad scenario pass is claimed.

Evidence: `settings-restart-replay-{begin,ended}.json`,
`settings-restart-replay-profile-save-{no-notice,status,apply-records}.json`, and
`settings-restart-replay-profile-save-no-notice.png`. Recording:
`/Users/pedronauck/.config/browser-harness/agent-workspace/recordings/settings-restart-replay`.
The missed initial textbox lookup used the wrong label; it was corrected from the current AX tree
before editing. That driver error is separate from the absent restart control after the saved write.


## Scoped runtime repair

HasPendingConfigRestart compared only the global desired/active hash. A successful restart-required
profile write has no global hash drift, so it returned false even though the write's own result and
apply record required restart. The settings apply owner now retains that scoped requirement in its
current runtime state. Unrelated live writes cannot clear it; a new daemon initializes it cleared
because boot consumes the saved overlay. History remains intact and no persisted schema changes.

The canonical config-apply service suite reproduces the false flag before repair, then passes the
profile-save/live-write/new-runtime sequence with real SQLite. The first fixture incorrectly chose
Defaults as a live write; its existing restart-required contract was retained and the fixture now
uses the established live Shell section. Receipt: settings-profile-restart-owning-green-live-shell.json.

The first enclosing gate passed codegen, Go lint and Go race tests but failed 16 Web hook assertions.
Six existing unit suites mocked the settings transport without the newly consumed apply-history and
status reads. Adding those I/O boundary responses preserves every assertion; all 175 settings hook
and restart presentation cases now pass (settings-restart-expanded-hooks-green.json). Fresh Chrome
replay and a new enclosing gate remain required.

## Verification — 2026-10-04

The full fresh Dora replay passes after commit 9f1457296. A profile-only Skills save now displays
the required restart in Web and CLI; its saved value survives HTTP/UDS reads and browser reload.
Automation writes through all three surfaces agree, and four invalid candidates leave valid
settings intact. After the first Web restart, later API writes replace the old success with the
required restart. Deferral survives reload but does not clear server state; another external
candidate restores the notice after 28.61 seconds. A second Web restart activates the restored
3s/5 baseline, preserves attention settings, and leaves zero active sessions.

The exact recording is
/Users/pedronauck/.config/browser-harness/agent-workspace/recordings/settings-restart-scoped-replay
(25 frames). Evidence: settings-scoped-replay-{build-witness,begin,ended,final-status}.json,
the transport/write/refusal receipts under the same prefix, and the inspected checkpoint PNGs.
native-cancel-settings-restart-delivery-gate-scoped.json passes all affected local lanes.
