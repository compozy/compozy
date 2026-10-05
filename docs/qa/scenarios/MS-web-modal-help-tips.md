---
id: MS-web-modal-help-tips
area: MS
title: Modal field explanations live behind a help tip, runtime truth stays visible
persona: Dora
journey: J-administer-runtime-settings
expected: Inside an entity editor modal, an explanatory sentence no longer occupies a permanent line under its label. A `(?)` trigger sits beside the label (and beside a section title where the section itself needs explaining), and the prose appears on hover, on keyboard focus, and on click. The click path matters — on a touch device there is no hover, so tapping the trigger is the only way in, and tapping elsewhere or pressing Escape dismisses it. Escape closes the tip before it closes the dialog. The trigger is a real button with an accessible name of the form "About <label>", reaches 24x24 CSS px on desktop and 44x44 at 760px and below, and shows a 2px focus ring on keyboard focus. It is a sibling of the `<label>`, never a child, so the field's accessible name stays exactly the label text. Text the runtime owns never moves into a tip and stays on screen — "Project runtime defaults will be used.", catalog load/stale/error lines, validation errors, write-only boundary warnings, and any sentence stating what will happen on save.
entry_points: web agent create; web vault create; web automation job/trigger editor; web task editor modal; web provider detail
qa_status: untested
bug_ids: BUG-20261004-help-tip-discards-draft; BUG-20261004-help-tip-vanishes-on-tap
fix_status: fixed
retest_status: pass
fix_commits: 6aec02734
evidence: docs/qa/evidence/2026-10-02-untested/help-tip-touch-dora-task-paths.json; docs/qa/evidence/2026-10-02-untested/help-tip-touch-dora-job-ended.json; docs/qa/evidence/2026-10-02-untested/help-tip-preserve-guidance-commit-proof.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: MS-web-entity-modal-shell; MS-web-agent-create-simple-advanced; MS-provider-detail-modal
---

story: As someone configuring an agent I want a calm form I can scan, with the explanation one hover away when I actually need it — not a paragraph under every field I already understand.

The product register moved from operator-dense to people-first ("calm by default, deep on demand", `DESIGN.md` §1). The modals were authored under the old register, where every field carried a permanent description line; at ~10 fields per modal that prose was most of the vertical space.

`HelpTip` (`packages/ui/src/components/custom/help-tip.tsx`) is deliberately a focusable button rather than `aria-describedby` on the control: `TooltipContent` mounts conditionally, so a static `aria-describedby` would point at an id that does not exist while the tip is closed; the control's describedby slot is already owned by its error; and a described string is not in the tab order, so removing the visible line would otherwise strand keyboard users.

The split that matters for QA is explanation vs. runtime truth. Explanation is safe to hide. Anything reporting what the daemon will do with the current value must stay visible — hiding it would let someone submit against a stale catalog or an inherited default without knowing.

src: packages/ui/src/components/custom/help-tip.tsx; packages/ui/src/components/custom/form-section.tsx; packages/ui/src/components/field.tsx; web/src/systems/settings/components/setting-row.tsx

inventory: Needs QA

2026-08-20: Job and trigger create/edit dialogs moved helper paragraphs onto HelpTip (prompt, schedule UTC, cron frequency, catch-up, grace, enabled). Reset to untested.

2026-08-20 targeted review walk: passed the changed HelpTip behavior in live extension and task dialogs. The trigger remained a separately named button, hover exposed the guidance, initial dialog focus stayed inside the dialog, and no browser errors were emitted. Keyboard/click behavior remains owned by the canonical HelpTip and dialog suites.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

2026-10-04: Task guidance opens on hover and keyboard focus with clean field names. Escape from
the hover path closes the enclosing dialog and loses its title; the keyboard-focus path preserves
the dialog. BUG-20261004-help-tip-discards-draft owns the failure. Other editor/touch legs remain pending.

2026-10-04 repair replay: the first Escape retains task/job drafts. Click and narrow touch reveal
another failure: guidance disappears immediately after activation, owned by
BUG-20261004-help-tip-vanishes-on-tap. Both fixes and the remaining entry points are still in progress.

2026-10-04 final repair replay: task and job guidance pass on the rebuilt pointerdown-ownership
bundle. Narrow touch stays readable until outside touch or Escape; desktop hover/focus/click and
draft retention pass. The 20-frame help-tip-touch-dora recording is closed, five screenshots are
inspected, and CLI reads confirm no unintended task/job save. Broader entry-point coverage remains
pending. Evidence: docs/qa/evidence/2026-10-02-untested/help-tip-touch-dora-task-paths.json and
docs/qa/evidence/2026-10-02-untested/help-tip-touch-dora-job-ended.json.

Both linked bugs are verified at 6aec02734. The scenario returns to untested for the remaining
entry points after this behavior change; it is not yet a full-scenario pass.
