---
id: MS-web-knowledge-edit-immutable-identity
area: MS
title: Knowledge edit locks name and type and omits them from the PATCH
persona: Dora
journey:
expected: Editing a knowledge entry shows its name, type, and filename as a readable locked summary — not as disabled inputs — with a hint naming retrieval stability as the reason they cannot change. Only description and content are editable, and the save enables on a change to either one (a description-only edit is savable). The request sent to `PATCH /api/memory/{filename}` carries `content`, `description`, and the scope keys, and never `name` or `type`. Knowledge create keeps its four-card type picker with the runtime memory types, and both dialogs render on the compact modal host with the shared ruled header, a single close control, and one primary action.
entry_points: web knowledge window → entry detail → Edit; web knowledge window → Create entry
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/modals-redesign/evidence/visual/task_02/VC-04; .compozy/tasks/modals-redesign/evidence/visual/task_02/VC-05
last_report:
overlaps: MS-web-entity-modal-shell
---

story: As an operator I refine what a knowledge entry says without silently breaking the references agents already use to retrieve it.

Introduced by the modal redesign (`.compozy/tasks/modals-redesign/`, `_techspec.md` §4.4-4.5), task_02, implemented 2026-07-25. Before this change the edit dialog sent `name` and `type` back on every PATCH even though neither was editable, and the save was gated on content changes only, so a description-only edit could not be saved.

src: web/src/systems/knowledge/components/knowledge-edit-dialog.tsx; web/src/systems/knowledge/components/knowledge-create-dialog.tsx; web/src/systems/os/apps/knowledge/use-knowledge-page.ts

inventory: Needs QA
