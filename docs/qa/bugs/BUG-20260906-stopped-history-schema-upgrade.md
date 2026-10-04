# BUG-20260906-stopped-history-schema-upgrade: Upgraded daemon cannot read retained stopped histories

- **Status:** fixed — verified 2026-10-04
- **Impact (user-side):** Data-Loss
- **Severity:** High · **Priority:** P1
- **Persona:** Théo · **Journey:** J-14 read a finished transcript
- **Scenarios:** RT-session-context-rebuild; ET-web-session-transcript-calm-grammar; MS-web-session-deeplink-global-confirm; MS-web-menubar-global-scope-toggle
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

After upgrading the isolated lab from session migration 7 to 8 and restarting the daemon, both Release notes and Operator handbook returned HTTP 500 for retained transcript reads. The daemon log reports ErrSchemaBehind: read-only session databases remained at version 7. Completed sessions bypassed the mutable crash-repair path, leaving their valid forward migrations unapplied.

The repair asks the Manager to prepare retained session databases during the existing boot inventory phase, before read-only interaction recovery and public routes. A current database opens read-only; only ErrSchemaBehind invokes the existing owned writable opener/Goose stream. Ahead, corrupt, unreadable and foreign databases retain their refusal. SQL transformation belongs exclusively to the appended migration. Events, IDs, archives and sequence/generation fences remain unchanged.

Canonical checks: TestBootSessionRepair and TestManagerOpenQueryRecorderValidationAndCleanup. The separate SessionDB projection suite owns migration data preservation and repeated reopen; no duplicate transformation assertions in boot tests.

Real restart verified: the same stopped handbook opens through public HTTP and Web after daemon PID97015 starts. The public transcript retains the same authored guide ID/text and contains 32 tool parts with corrected final labels. Evidence: integrated lab evidence/handbook-transcript-migrated.json, evidence/handbook-reopened.png, and daemon.log. Original schema-behind failures remain at 08:03–08:04 America/Sao_Paulo.


## Regressed (2026-10-04)

Bruno cannot open a retained home-owned session after upgrading a genuine released
v0.3.0-beta.19 home to the current build. The old CLI creates and the old HTTP interface
reads sess-1910f4bd9212e8db normally. After a graceful old-daemon stop and a current-daemon
start against the same isolated home, the startup command exceeds 100 seconds and current
HTTP status, session-detail and owner routes return 500. The daemon itself remains running.
No database, metadata, or browser storage was fabricated or edited to prepare this state.

Charter: CH-global-scope-regression, Feature Tour; Bruno, desktop 1512x862, wifi-fast, en-US.
The first document encountered incomplete onboarding; after the public onboarding-complete
command and registration of Delivery Studio, a fresh walk still shows disabled Global scope,
an empty project menu except Add project, and no session content or scope confirmation after
following the retained permalink for 15 seconds. Both final screenshots were inspected and
the ten-frame recording was stopped. The earlier immediate blank checkpoint is a premature
driver capture, not independent evidence of an application stall.

Public UDS narrows the failure: status cannot read the migrated Global catalog row because
SessionInfo validation requires a workspace; owner lookup rejects creation profile version 3.
The boot log independently reports rejection of the migrated workspace-free health row.
Migration 00085 intentionally moves synthetic-home rows to Global, while current Go readers
still require project identities. Further ownership/metadata upgrade boundaries remain under
investigation. The prior session-schema repair remains present; this reopens the same
retained-history upgrade symptom at additional persisted boundaries.

Evidence under docs/qa/evidence/2026-10-02-untested/:
- scope-legacy-{release-checksum,home-session-create,old-session-readback,old-daemon-stop}.json
- scope-legacy-current-{daemon-start-timeout,status-readback,session-readback,owner-readback,uds-status,uds-owner}.json
- scope-legacy-bruno-{project-entry,project-observed,link-ended,failure-captured}.json
- scope-legacy-bruno-project-menu.png and scope-legacy-bruno-link-unresolved.png
- scope-legacy-health-regression-{red,green}.json and scope-legacy-catalog-regression-red.json

The repair accepts the migrated Global scope in catalog/health persistence and preserves
version 3–5 creation witnesses, including the retired fields in their original hash inputs.
Global catalog scope no longer substitutes for the immutable events.db owner: the retained
creation witness proves the original owner, and mismatched metadata/database substitutions
remain refused. No metadata, owner row or historical digest is rewritten.

The same released home now starts successfully and public status, owner, session, transcript,
search and outline reads return 200. The transcript retains the original three creation/stop
events and sequence watermark 3. New read-only Global transcript routes keep profile checks;
project sessions cannot be read through those routes. CLI transcript navigation must choose
the Global route when the authorized by-id record has no workspace. Native tools retain their
existing project/caller workspace boundary.

Web confirms Global scope before reading the linked history, preserves the remembered project,
and renders it without a composer or live runtime. Focused store/manager/API regressions and
146 Web tests pass; a broader Go run, fresh persona replay and the current delivery gate remain
pending. The bug is still open until that evidence is complete. Additional receipts use
scope-legacy-{creation-witness,owner-api-manager,persistence-owner-regression,global-transcript,
history-preflight,history-web,history-cli} prefixes in the same evidence directory.

The first repaired Bruno replay confirms cancel/confirm, Global reload and remembered-project
restoration. It also exposes a generic Session title and interactive empty-state guidance in
the new read-only view. The replay ended before engineering corrected those presentation
boundaries; a fresh full link replay is still required. Evidence: scope-legacy-replay-bruno-*
and the inspected scope-legacy-replay-global-{confirm,history}.png screenshots.

The second fresh document confirms that decline preserves project scope, but confirming the
permalink then redirects to an empty Sessions window while the retained row remains listed.
The eleven-frame replay ends with Global still on. Engineering found that the window controller
treats a catalog/workspace view mismatch as remote deletion and retires the shared window;
the new explicit Global owner exposes this path in a project-scoped document. Scope mismatch
must keep the history hidden locally without removing another document's window. The owning
session-window suite is extended to reproduce that lifecycle distinction before repair.
Evidence: scope-legacy-final-bruno-{agent-declined,history-observed,session-ended}.json and the
inspected scope-legacy-final-history-observed.png; the attempted final replay remains a failure.

The final 27-frame Bruno replay passes with a second project-scoped document kept open.
Both the agent link and permalink support decline and confirmation. Global opens the saved
history title without a composer, survives reload, and restores Delivery Studio when switched
off. The project document retains the shared history window without accessing its runtime;
its different scope no longer causes remote retirement. The project-session canary retains
its normal composer and an unsent draft when the Global shortcut is pressed in the editor.
Independent UDS reads still return the original three lifecycle events, watermark 3 and Global
owner. This fixture contains no authored provider narrative; it proves retained event/history
access and scope behavior, not a provider-content rendering claim.

Evidence: scope-legacy-crossdoc-bruno-{entry,confirm,observer-state,project-restore,project-draft}.json,
scope-legacy-crossdoc-independent-{owner,transcript,session,project}.json and the inspected
scope-legacy-crossdoc-{final-global,project-restored,project-draft}.png. The observer-reload driver
timeout expected a notice while the catalog tab was active; the following observer-state
receipt proves retained windows and correct local scope. The earlier failed recordings remain.

The bounded full GlobalDB race suite passes in 1390.477 seconds; complete HTTP/UDS suites and
the affected Web suites pass. React Doctor reports the same three baseline complexity warnings,
and the changed Go tests add no convention findings. The delivery gate found six lint issues;
they are repaired and the gate is running again. Status stays open until delivery validation
and the lab evidence audit complete.

## Verified closure (2026-10-04)

The final affected make gate passes all selected lanes, including zero-issue Go lint and 6,919 Web
tests. The exact final binary smoke retains Global history, and the strict evidence audit passes
with no blockers. Both Global labs are torn down by their exact manifests with clean=true and no
survivors. The two affected Global Web scenarios are verified; earlier failed receipts remain.
Evidence: scope-legacy-history-delivery-gate-retry.json, scope-legacy-history-evidence-audit-final.json,
scope-legacy-history-teardown.json and scope-completion-final-teardown.json in this cycle directory.
