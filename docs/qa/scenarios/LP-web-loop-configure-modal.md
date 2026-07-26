---
id: LP-web-loop-configure-modal
area: LP
title: Loop Configure runs as a modal on the shared entity shell
persona: Dora
journey:
expected: Configure opens as a centered `md` modal over the scrim, not a right-side sheet. It carries the shared chrome — accent icon well, "Loops · Configure" eyebrow, `Configure <loop>` title, body as sole scroll owner, 52px footer. The structural note keeps its interactive builder link and renders as a neutral Alert at the top of the body, with the link reading "Edit" for a workspace loop and "Fork & edit" otherwise. The four groups (Review gate, Human approval gate, Re-attempt strategy, Stop limits) render as hairline `FormSection` blocks; Review gate and Stop limits keep their right-aligned provenance eyebrows ("declared in the loop", "per-loop defaults"). The footer carries a ghost "Reset to defaults" on the leading edge plus Cancel and one "Save configuration" primary. While a save is in flight the strategy cards, Reset, Cancel, the editor link, and the primary all disable, and the header close is withdrawn — Cancel and Escape remain the exits. Saving closes on success and keeps the modal open with the entered values intact on failure. No cost-cap field exists (ADR-017), and structural fields stay non-editable here (ADR-009).
entry_points: web loop detail -> Configure
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-web-entity-modal-shell; LP-017; LP-018; LP-019; LP-020
---

story: As someone tuning how a loop runs I expect Configure to behave like every other entity editor in the product, because it is the same kind of bounded edit that ends in Save or Cancel.

`LoopConfigureSheet` was the last writable form hosted in a `Sheet`, and it hand-rolled the entity chrome — its own icon well, eyebrow, title, and footer — so it drifted from the shell it was imitating. `MODAL-STANDARD.md` reserves sheets for browse-heavy, long-lived, multi-entity work; a bounded single-entity edit belongs in a modal. The file is now `loop-configure-dialog.tsx` exporting `LoopConfigureDialog`; the old name is a hard cut with no alias.

The footer's `leading` slot is new shell API and holds ghost-tier commands only — Reset is not a second primary.

Read-only loop inspectors (`LoopRunInspectSheet`) deliberately stay sheets.

src: web/src/systems/loops/components/configure/loop-configure-dialog.tsx; web/src/systems/os/apps/loops/loop-configure-location.tsx; packages/ui/src/components/custom/entity-dialog-footer.tsx

inventory: Needs QA
