---
id: MS-web-bridge-edit-delivery-fold
area: MS
title: Bridge edit locks identity, rotates credentials, and owns the only delivery test
persona: Dora
journey:
expected: Opening Edit bridge shows platform, extension, and scope as readable locked identity — never as disabled inputs — because the update contract omits them. Simple carries the display name and DM policy. Advanced adds credential rotation (presence plus the stored vault reference, never a value), routing, delivery defaults, the provider-config JSON, and the delivery test. Save stays inert until something actually changes: reformatting the provider-config JSON without changing the object does not enable it, while a typed rotation does. Saving commits the PATCH and any typed rotation through the same single primary. The delivery test is reachable only from Advanced — the standalone check/send dialogs are gone. From the bridge detail, one "Test delivery" button opens the editor already in Advanced. The dry run resolves a target with no provider side effect; the real send needs a message and an enabled bridge, and says so when the bridge is disabled. An indeterminate provider result is reported as such rather than as success.
entry_points: web desktop shell → Bridges → bridge detail → Edit bridge, or bridge detail → Test delivery
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/modals-redesign/evidence/visual/task_03/VC-06
last_report:
overlaps: MS-web-entity-modal-shell; NB-bridge-edit-reply; NB-indeterminate-bridge-delivery; NB-web-bridge-setup
---

story: As an operator I change what a bridge does and prove it can still deliver, without wondering which of two delivery dialogs I am in.

Introduced by the modal redesign (`.compozy/tasks/modals-redesign/`, `_techspec.md` §4.9, decision D2), task_03, implemented 2026-07-25. Before this change the delivery check and the real send were two separate dialogs launched from the detail panel, each with its own target draft that could silently drift from the other.

The artboard keeps a "Test delivery" button in the editor footer in both tiers; folding it into the Advanced body is an authorized difference so the wave keeps one entry point and one primary footer action.

src: web/src/systems/bridges/components/bridge-edit-dialog.tsx; web/src/systems/bridges/components/bridge-edit-advanced-section.tsx; web/src/systems/bridges/components/bridge-delivery-test-panel.tsx; web/src/systems/bridges/lib/bridge-update-dirty.ts; web/src/systems/os/apps/bridges/use-bridge-secret-rotations.ts; web/src/hooks/routes/use-bridge-delivery-tests.ts
