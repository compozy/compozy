---
id: ET-web-vault-overwrite-confirmation
area: ET
title: Vault create warns and requires confirmation before overwriting an existing ref
persona: Dora
journey:
expected: Add vault secret splits into a reference section (ref, kind) and a value section carrying a write-only secret field that is never prefilled from any read path. Typing a reference that already exists — compared after trimming, the same normalization the daemon applies — raises a warning notice stating that saving rotates the stored value and that the current value stays unreadable, plus an explicit "Confirm replacement" switch. Store secret stays disabled until that switch is on. Pointing the reference at a different value retracts the confirmation, so an affirmation for one secret cannot carry over to another. A brand-new reference shows no notice and saves directly. A failed write keeps the reference, kind, and typed value. Replacing an existing secret continues to happen in the vault inspect sheet; this dialog stays create-only.
entry_points: web vault window → New secret
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/modals-redesign/evidence/visual/task_02/VC-08
last_report:
overlaps: MS-web-entity-modal-shell; ET-web-vault-opendesign-listing
---

story: As an operator I store a secret without discovering afterwards that I silently replaced a different one that everything else was already bound to.

Introduced by the modal redesign (`.compozy/tasks/modals-redesign/`, `_techspec.md` §4.12), task_02, implemented 2026-07-25. Before this change the create form used a bare password input and no collision check, so writing an existing reference overwrote it with no warning — `PUT /api/vault/secrets` is an upsert.

Collision detection reads the unfiltered secret list rather than the filtered listing, so an active namespace or prefix filter cannot hide the reference being overwritten.

src: web/src/systems/vault/routes/vault-page.tsx; web/src/systems/vault/hooks/use-vault-page.ts

inventory: Needs QA
