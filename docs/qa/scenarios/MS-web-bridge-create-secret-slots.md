---
id: MS-web-bridge-create-secret-slots
area: MS
title: Create bridge collects provider secret slots and binds them after the bridge exists
persona: Dora
journey:
expected: Opening Create bridge shows one surface, never a stepper. Simple carries the provider choice as a radiogroup (permanent identity), the display name, an "Enable after creation" toggle that defaults off, and one write-only SecretField per slot the provider manifest declares. Advanced adds DM policy, routing, delivery defaults, notification suppression, and the non-secret provider-config JSON. Required slots gate the primary. Submitting creates the bridge first (`POST /api/bridges`, no secret field on the contract) and then writes one secret binding per filled slot. If a binding fails, the dialog stays open with the created bridge shown as immutable identity, names exactly which slots failed, and reopens those fields empty — the earlier plaintext is never shown again and never re-sent implicitly. Switching provider clears every typed slot value. A manifest provider (Slack) does not gate the create on its slots, because the credentials only exist after the app is installed from the manifest.
entry_points: web desktop shell → Bridges → Create bridge (catalog toolbar and empty state)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/modals-redesign/evidence/visual/task_03/VC-04; .compozy/tasks/modals-redesign/evidence/visual/task_03/VC-05
last_report:
overlaps: MS-web-entity-modal-shell; NB-web-bridge-setup; NB-bridge-provider-setup
---

story: As an operator I connect a chat platform by picking the provider, naming it, and pasting the credentials it asks for — and if one credential fails to store, I am told which one instead of losing the whole bridge.

Introduced by the modal redesign (`.compozy/tasks/modals-redesign/`, `_techspec.md` §4.8), task_03, implemented 2026-07-25. Before this change the dialog was a three-step wizard that only *listed* the provider's secret slots read-only; binding them required a second trip to the detail panel.

The bridge is still created disabled by default: the provider-declared credentials are bound after the POST, so a bridge that started on acceptance would be running without them.

src: web/src/systems/bridges/components/bridge-create-dialog.tsx; web/src/systems/bridges/components/bridge-create-simple-section.tsx; web/src/systems/bridges/components/bridge-create-secret-slots.tsx; web/src/systems/bridges/components/bridge-create-advanced-section.tsx; web/src/systems/bridges/lib/bridge-secret-slot-submission.ts; web/src/hooks/routes/use-bridge-create-flow.ts
