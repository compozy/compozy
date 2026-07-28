---
id: MS-web-modal-help-tips
area: MS
title: Modal field explanations live behind a help tip, runtime truth stays visible
persona: Dora
journey:
expected: Inside an entity editor modal, an explanatory sentence no longer occupies a permanent line under its label. A `(?)` trigger sits beside the label (and beside a section title where the section itself needs explaining), and the prose appears on hover, on keyboard focus, and on click. The click path matters — on a touch device there is no hover, so tapping the trigger is the only way in, and tapping elsewhere or pressing Escape dismisses it. Escape closes the tip before it closes the dialog. The trigger is a real button with an accessible name of the form "About <label>", reaches 24x24 CSS px on desktop and 44x44 at 760px and below, and shows a 2px focus ring on keyboard focus. It is a sibling of the `<label>`, never a child, so the field's accessible name stays exactly the label text. Text the runtime owns never moves into a tip and stays on screen — "Project runtime defaults will be used.", catalog load/stale/error lines, validation errors, write-only boundary warnings, and any sentence stating what will happen on save.
entry_points: web agent create; web vault create; web sandbox profile create/edit; web automation job/trigger editor; web task editor modal; web provider detail
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-web-entity-modal-shell; MS-web-agent-create-simple-advanced; MS-provider-detail-modal
---

story: As someone configuring an agent I want a calm form I can scan, with the explanation one hover away when I actually need it — not a paragraph under every field I already understand.

The product register moved from operator-dense to people-first ("calm by default, deep on demand", `DESIGN.md` §1). The modals were authored under the old register, where every field carried a permanent description line; at ~10 fields per modal that prose was most of the vertical space.

`HelpTip` (`packages/ui/src/components/custom/help-tip.tsx`) is deliberately a focusable button rather than `aria-describedby` on the control: `TooltipContent` mounts conditionally, so a static `aria-describedby` would point at an id that does not exist while the tip is closed; the control's describedby slot is already owned by its error; and a described string is not in the tab order, so removing the visible line would otherwise strand keyboard users.

The split that matters for QA is explanation vs. runtime truth. Explanation is safe to hide. Anything reporting what the daemon will do with the current value must stay visible — hiding it would let someone submit against a stale catalog or an inherited default without knowing.

src: packages/ui/src/components/custom/help-tip.tsx; packages/ui/src/components/custom/form-section.tsx; packages/ui/src/components/field.tsx; web/src/systems/settings/components/setting-row.tsx

inventory: Needs QA
