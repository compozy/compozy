---
id: MS-web-entity-modal-shell
area: MS
title: Entity editor modals share one header, host token, and footer
persona: Dora
journey:
expected: Every migrated entity editor renders the shared ruled chrome — a 36px accent icon well (the only accent-tinted surface in the shell) beside an accent-strong eyebrow, the dialog title, and an optional description; the body is the sole scroll owner; the footer is 52px with an optional consequence hint on the leading edge, Cancel, and exactly one verb+object primary action that shows a spinner and blocks duplicate submit while saving. Host width comes from `--width-modal-{sm,md,lg,xl}` via `dialogShellClass`, never an ad-hoc `max-w-*`. Simple/Advanced is one disclosure tier that never hides a required field, and leaving Advanced snaps unsupported advanced-only selections back to a Simple-valid default. Secret controls are write-only: create shows a single password input, edit shows presence plus an explicit Replace, and cancelling a rotation preserves the existing binding without exposing plaintext. Fields an update contract cannot mutate render as readable summary rows, never as disabled inputs. The body grammar is shared too: one 20px gutter across header, mode toolbar, body, feedback strip, and footer (`modal-system.css:170,194,218,395`); one monotonic type ladder (dialog title 15/510, section title 12.5/600, field label 12/510, hint 11.5/400) so a label never outranks the value it names; sections are hairline-ruled `FormSection` blocks flush with the body gutter, with no card surface and no competing row rules; explanatory prose sits behind a `HelpTip` `(?)` beside its label, reachable by pointer, keyboard, and touch, while runtime truth, errors, and warnings stay visible; and the footer may carry one ghost `leading` command (reset, view toggle) without gaining a second primary.
entry_points: web task editor modal; web automation job/trigger editor; web vault create and sandbox profile create/edit via SettingsEditorDialog; web marketplace MCP install secret fields; web agent create; web provider detail; web loop configure modal
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/modals-redesign/evidence/visual/task_01/VC-01; .compozy/tasks/modals-redesign/evidence/visual/task_01/VC-02; .compozy/tasks/modals-redesign/evidence/visual/task_01/VC-03
last_report:
overlaps: NB-participation-controls-serialize; MS-030; ET-web-vault-opendesign-listing; TA-task-template-preserves-draft; MS-provider-detail-modal; MS-web-session-simple-advanced-launch; MS-web-workspace-add-directory-browser; MS-web-knowledge-edit-immutable-identity; NB-web-channel-fanout-policy; ET-web-vault-overwrite-confirmation; MS-web-task-editor-window-modal
---

story: As an operator I configure runtime entities through modals that look and behave the same everywhere, so I can predict where the title, the disclosure toggle, the scope control, and the one primary action will be.

Introduced by the modal redesign (`.compozy/tasks/modals-redesign/`, `_techspec.md` §2 F1-F7), task_01, implemented 2026-07-25. The shared primitives are `EntityDialogHeader`, `EntityDialogFooter`, `EntityDialogBody` (including the `split` variant), `EntityModeToolbar`, `SecretField`, `ImmutableIdentity`, and the `dialogShellClass` host helper, all exported from `@agh/ui`.

Coverage in task_01 is the foundation plus three surfaces: the task editor (R1 header restored, in-body description paragraph removed), the automation job/trigger editor (local `EditorHeader` deleted in favour of the shared primitive), and `SettingsEditorDialog` (vault create + sandbox profile create/edit chrome). The marketplace MCP install dialog now consumes the shared `SecretField` after its local copy was deleted.

task_02 (implemented 2026-07-25) extended the same shell to start session, add workspace (the `split` body host), knowledge create/edit, network channel create/edit, and the vault create body. Behaviour specific to those surfaces lives in its own scenario — see `overlaps` — while this scenario stays the shared-chrome contract.

The remaining surfaces migrate in tasks 03-04; this scenario should be re-scoped, not duplicated, as they land.

src: packages/ui/src/components/custom/entity-dialog-header.tsx; packages/ui/src/components/custom/entity-dialog-footer.tsx; packages/ui/src/components/custom/entity-dialog-body.tsx; packages/ui/src/components/custom/entity-mode-toolbar.tsx; packages/ui/src/components/custom/secret-field.tsx; packages/ui/src/components/custom/immutable-identity.tsx; packages/ui/src/lib/dialog-shell.ts

inventory: Needs QA
