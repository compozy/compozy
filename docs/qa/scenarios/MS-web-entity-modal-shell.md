---
id: MS-web-entity-modal-shell
area: MS
title: Entity editor modals share one header, host token, and footer
persona: Dora
journey: J-administer-runtime-settings
expected: Every migrated entity editor renders the shared ruled chrome — the 26px mint identity well (`KindIcon` well; a semantic warning/danger tint only for a dialog whose tone is warning or danger) beside an optional muted eyebrow (omitted when the title already identifies the entity), the dialog title, and an optional description; the body is the sole scroll owner; the footer has a 52px minimum with an optional consequence hint on the leading edge, Cancel, and exactly one verb+object primary action that shows a spinner and blocks duplicate submit while saving. Host width comes from `--width-modal-{sm,md,lg,xl}` via `dialogShellClass`, never an ad-hoc `max-w-*`. Simple/Advanced is one disclosure tier that never hides a required field, and leaving Advanced snaps unsupported advanced-only selections back to a Simple-valid default. Secret controls are write-only: create shows a single password input, edit shows presence plus an explicit Replace, and cancelling a rotation preserves the existing binding without exposing plaintext. Fields an update contract cannot mutate render as readable summary rows, never as disabled inputs. The body grammar is shared too: one 20px gutter across header, mode toolbar, body, feedback strip, and footer (`modal-system.css:170,194,218,395`); one monotonic type ladder (dialog title 14/500, section title 13/600, field label 13/500, hint 12/425) so a label never outranks the value it names; sections are hairline-ruled `FormSection` blocks flush with the body gutter, with no card surface and no competing row rules; explanatory prose sits behind a `HelpTip` `(?)` beside its label, reachable by pointer, keyboard, and touch, while runtime truth, errors, and warnings stay visible; and the footer may carry one ghost `leading` command (reset, view toggle) without gaining a second primary.
entry_points: web task editor modal; web automation job/trigger editor; web vault create via SettingsEditorDialog; web marketplace MCP install secret fields; web agent create; web provider detail; web loop configure modal
qa_status: pass
bug_ids: BUG-20261004-help-tip-discards-draft; BUG-20261004-help-tip-vanishes-on-tap; BUG-20261004-vault-name-recovery-missing; BUG-20261004-vault-warning-covered; BUG-20261004-vault-help-name-generic
fix_status: fixed
retest_status: pass
fix_commits: 6aec02734; baec8d019
evidence: docs/qa/evidence/2026-10-02-untested/settings-vault-final-commit-identity.json; docs/qa/evidence/2026-10-02-untested/settings-vault-final-delivery-gate.json; docs/qa/reports/2026-10-02-untested.md; docs/qa/evidence/2026-10-02-untested/qa/visual-contract/entity-modal-shell/contract.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-web-vault-opendesign-listing; TA-task-template-preserves-draft; MS-provider-detail-modal; MS-web-session-simple-advanced-launch; MS-web-workspace-add-directory-browser; MS-web-knowledge-edit-immutable-identity; ET-web-vault-overwrite-confirmation; MS-web-task-editor-window-modal
---

story: As a person running agent work I configure runtime entities through modals that look and behave the same everywhere, so I can predict where the title, the disclosure toggle, the derived destination statement, and the one primary action will be.

Introduced by the modal redesign (`.compozy/tasks/modals-redesign/`, `_techspec.md` §2 F1-F7), task_01, implemented 2026-07-25. The shared primitives are `EntityDialogHeader`, `EntityDialogFooter`, `EntityDialogBody` (including the `split` variant), `EntityModeToolbar`, `SecretField`, `ImmutableIdentity`, and the `dialogShellClass` host helper, all exported from `@compozy/ui`.

Coverage in task_01 is the foundation plus three surfaces: the task editor (R1 header restored, in-body description paragraph removed), the automation job/trigger editor (local `EditorHeader` deleted in favour of the shared primitive), and `SettingsEditorDialog` (vault create chrome). The marketplace MCP install dialog now consumes the shared `SecretField` after its local copy was deleted.

task_02 (implemented 2026-07-25) extended the same shell to start session, add workspace (the `split` body host), knowledge create/edit and the vault create body. Behaviour specific to those surfaces lives in its own scenario — see `overlaps` — while this scenario stays the shared-chrome contract.

The remaining surfaces migrate in tasks 03-04; this scenario should be re-scoped, not duplicated, as they land.

src: packages/ui/src/components/custom/entity-dialog-header.tsx; packages/ui/src/components/custom/entity-dialog-footer.tsx; packages/ui/src/components/custom/entity-dialog-body.tsx; packages/ui/src/components/custom/entity-mode-toolbar.tsx; packages/ui/src/components/custom/secret-field.tsx; packages/ui/src/components/custom/immutable-identity.tsx; packages/ui/src/lib/dialog-shell.ts

inventory: Needs QA

2026-08-12 qa-impact: create/install destination pills were deleted. The shared shell now carries a derived `workspace-scope-statement` (toolbar chip or footer note) from the menubar Global switch, not a scope picker. Reset to untested.

2026-08-12 walk: blocked-verify. This implementation cycle captured Storybook visual-contract evidence (`.compozy/tasks/global-workspace-menubar/evidence/visual/menubar-toggle/VC-01`–`VC-04`) and unit/typecheck coverage. An isolated QA lab with a live daemon (`COMPOZY_HOME`, production-parity web) was not started, so a persona walk through public entry points could not meet the qa-execution evidence standard.

2026-08-20 qa-impact: entity editor chrome is one `--color-canvas-soft` surface (header, body, ruled footer). Simple/Advanced sits on a full-width recessed `--color-canvas-tint` strip (`EntityModeToolbar`); dialogs without a mode tier do not paint an empty bar. Job, trigger, and task destination statements live in the footer hint. Reset to untested.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

qa-impact: 2026-09-30 shell rail v2. Type weights moved to 425/500/600 and the form label/hint tiers to 13px/12px (`--text-form-label`, `--text-form-hint`). Already untested; expectation updated.

qa-impact: 2026-09-30 shell rail v2 (dialog identity). The header well is the neutral identity well with a muted eyebrow by default, the close control is the quiet icon button, and the dialog opens focused on its popup (no ring on the first control). Already untested; expectation updated.

2026-10-04 preflight: DESIGN.md and packages/ui/src/tokens.css now own a 14px modal title
(`--text-modal-title: 0.875rem`), and a 13px small-body section title, superseding the historical 15px/12.5px prototype values. Apply
those current tokens when evaluating title hierarchy; the remaining chrome and interaction requirements
remain in force. The historical `.compozy/tasks/modals-redesign/` spec and visual bundles are
absent from both the isolated worktree and the owner checkout; they are provenance, not current
visual proof. Functional modal walks continue independently of a fresh named-reference comparison.

2026-10-04: The task editor's hovered HelpTip lets Escape close the enclosing form and lose the
unsaved title (BUG-20261004-help-tip-discards-draft). The keyboard-focus path retains the dialog.
Evidence: docs/qa/evidence/2026-10-02-untested/shared-modals-dora-draft-reopen-keyboard.json.
The broader shared-chrome, disclosure, secret/immutable and visual-reference legs remain pending.

2026-10-04 footer preflight: EntityDialogFooter explicitly defines the 52px token as a minimum,
composing the ruled DialogFooter padding and current button height. The measured 55px desktop
footer satisfies that floor; the scenario now states the minimum rather than a fixed height.
No production style changes or named-reference parity verdict follow from this clarification.

2026-10-04 final repair replay: task and job guidance pass on the rebuilt pointerdown-ownership
bundle. Narrow touch stays readable until outside touch or Escape; desktop hover/focus/click and
draft retention pass. The 20-frame help-tip-touch-dora recording is closed, five screenshots are
inspected, and CLI reads confirm no unintended task/job save. Broader entry-point coverage remains
pending. Evidence: docs/qa/evidence/2026-10-02-untested/help-tip-touch-dora-task-paths.json and
docs/qa/evidence/2026-10-02-untested/help-tip-touch-dora-job-ended.json.

Both linked bugs are verified at 6aec02734. The scenario returns to untested for the remaining
entry points after this behavior change; it is not yet a full-scenario pass.

2026-10-04 shared-editor continuation: Task, Trigger, Agent and Provider disclosure/help/immutable
legs pass with abandoned drafts confirmed by CLI where applicable. Vault's invalid-name recovery
and unobstructed overwrite warning are repaired and replayed; write-only presence, cancel, rotation
and deletion pass with independent metadata and reload evidence. The broad scenario remains pending
for other named entry points and visual-reference comparison. See the dated report's shared-editor
and Vault debriefs rather than inferring a full pass from the two repaired findings.

2026-10-04 continuation: Vault help needs an explicit field-specific accessible name for its decorated Name label. The remaining Loop configure and Marketplace secret-control abandonment legs pass; the dated report carries their receipts and precise scope.

2026-10-04 closure: the recorded original-persona walks and affected gate pass at baec8d019.
All shared entry points have current functional evidence, and six named-reference bundles
are inspected and validated. Existing TA automation and MS Agent/Provider contracts explicitly
own their changed layouts. EntityDialogHeader permits omitting a redundant eyebrow; its neutral
tone uses the mint identity well. This shared-shell verdict does not settle separate domain flows.
