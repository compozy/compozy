---
id: ET-web-vault-overwrite-confirmation
area: ET
title: Vault create warns and requires confirmation before overwriting an existing ref
persona: Dora
journey: J-keep-secrets-contained
expected: New secret shows a Name field, a write-only Secret value field that is never prefilled from any read path, and an optional Label behind "More options". A bare name gets the `vault:` prefix added client-side and a "Saved as vault:…" preview; a name typed with the prefix is kept as-is. Typing a name that already exists — compared after trimming and prefixing, the same normalization the daemon applies — raises a warning notice ("A secret with this name already exists. Saving replaces its value everywhere it's used.") plus an explicit "Replace it" switch. Save secret stays disabled until that switch is on. Pointing the name at a different secret retracts the confirmation, so an affirmation for one secret cannot carry over to another. A brand-new name shows no notice and saves directly. A failed write keeps the name, label, and typed value. Replacing an existing secret continues to happen in the vault inspect sheet; this dialog stays create-only.
entry_points: web vault window → New secret
qa_status: pass
bug_ids: BUG-20261004-vault-name-recovery-missing; BUG-20261004-vault-warning-covered
fix_status: fixed
retest_status: pass
fix_commits: baec8d019
evidence: docs/qa/evidence/2026-10-02-untested/settings-vault-final-commit-identity.json; docs/qa/evidence/2026-10-02-untested/settings-vault-final-delivery-gate.json; docs/qa/reports/2026-10-02-untested.md
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: MS-web-entity-modal-shell; ET-web-vault-opendesign-listing
---

story: As a person running agent work I store a secret without discovering afterwards that I silently replaced a different one that everything else was already bound to.

Introduced by the modal redesign (`.compozy/tasks/modals-redesign/`, `_techspec.md` §4.12), task_02, implemented 2026-07-25. Before this change the create form used a bare password input and no collision check, so writing an existing reference overwrote it with no warning — `PUT /api/vault/secrets` is an upsert.

Collision detection reads the unfiltered secret list rather than the filtered listing, so an active namespace or prefix filter cannot hide the reference being overwritten.

src: web/src/systems/vault/routes/vault-page.tsx; web/src/systems/vault/hooks/use-vault-page.ts

inventory: Needs QA

2026-10-04: Dora's shared-editor walk found that an unsupported namespace is rejected without
correction guidance. The draft is retained and nothing is written. The repair keeps existing
namespace validation and adds field help plus an actionable redacted diagnostic. Re-walk name
correction, then the existing overwrite/cancel contract. Evidence and bug history are in
docs/qa/reports/2026-10-02-untested.md; the complete scenario remains untested.

2026-10-04 repair re-walk: supported-name guidance, retained failed-write input, trimmed/prefixed
collision detection even while the listing hides that secret, consent retraction, explicit overwrite,
and write-only inspect replacement pass through Web and metadata CLI reads. The additional warning
overlap is repaired with AlertActions and passes desktop/narrow browser evidence. Both recordings
are closed and the owned secrets deleted. Fix SHA and gate checkpoint remain pending.

2026-10-04 closure: the recorded original-persona walks and affected gate pass at baec8d019.
Name correction, failed-write draft retention, hidden-list collision, consent retraction,
overwrite, rotation cancellation and cleanup pass with independent metadata reads and reload.
