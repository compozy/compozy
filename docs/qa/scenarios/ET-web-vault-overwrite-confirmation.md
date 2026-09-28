---
id: ET-web-vault-overwrite-confirmation
area: ET
title: Vault create warns and requires confirmation before overwriting an existing ref
persona: Dora
journey: J-keep-secrets-contained
expected: New secret shows a Name field, a write-only Secret value field that is never prefilled from any read path, and an optional Label behind "More options". A bare name gets the `vault:` prefix added client-side and a "Saved as vault:…" preview; a name typed with the prefix is kept as-is. Typing a name that already exists — compared after trimming and prefixing, the same normalization the daemon applies — raises a warning notice ("A secret with this name already exists. Saving replaces its value everywhere it's used.") plus an explicit "Replace it" switch. Save secret stays disabled until that switch is on. Pointing the name at a different secret retracts the confirmation, so an affirmation for one secret cannot carry over to another. A brand-new name shows no notice and saves directly. A failed write keeps the name, label, and typed value. Replacing an existing secret continues to happen in the vault inspect sheet; this dialog stays create-only.
entry_points: web vault window → New secret
qa_status: untested
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: .compozy/tasks/modals-redesign/evidence/visual/task_02/VC-08;/Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa;/Users/pedronauck/dev/qa-labs/compozy-replay-config-agent-vault-20260730-061728-520459-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: MS-web-entity-modal-shell; ET-web-vault-opendesign-listing
---

story: As a person running agent work I store a secret without discovering afterwards that I silently replaced a different one that everything else was already bound to.

Introduced by the modal redesign (`.compozy/tasks/modals-redesign/`, `_techspec.md` §4.12), task_02, implemented 2026-07-25. Before this change the create form used a bare password input and no collision check, so writing an existing reference overwrote it with no warning — `PUT /api/vault/secrets` is an upsert.

Collision detection reads the unfiltered secret list rather than the filtered listing, so an active namespace or prefix filter cannot hide the reference being overwritten.

src: web/src/systems/vault/routes/vault-page.tsx; web/src/systems/vault/hooks/use-vault-page.ts

inventory: Needs QA
