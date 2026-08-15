---
id: LP-web-node-dialog-modal-contract
area: LP
title: Node control dialogs and the quarantine sheet follow the modal contract
persona: Dora
journey: J-05
expected: All seven node/run control dialogs carry a 36px verb icon well (neutral pause/cancel-run, accent resume/requeue/wait-resume, danger kill) with the eyebrow above the title and muted body copy; the machine micro trail sits in the footer note slot. A verb that no longer applies renders as a calm tone-true answer panel (info/success), never a red failure — danger is reserved for transport errors. Cancel confirms use the tinted danger button; Kill uses solid danger. The pause dialog pairs 28px icon wells (hourglass drain / ban cancel) and its trail reflects the chosen mode; pause and requeue accept an optional reason that lands in provenance and on the sheet's episode boundary; wait-resume shows the wait's expect shape as the hint and disables confirm while the JSON does not match. Confirms and both sheets share the modal-sm width. The quarantine sheet renders its classified chain on the canonical timeline (spine, tone-ringed dots, glyphs, mono trails with attempt start→end spans), episode dividers aligned to the spine, the hint as a quiet note, collapsible Info and History sections with count gists, and a footer with gated Cancel plus primary Requeue while the node stays quarantined.
entry_points: web /loop-runs/:runId (node row actions, quarantine entry)
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits: 9a694ff2; 54ce025f
evidence:
last_report:
overlaps: LP-web-run-page-section-grammar
---

Added by the loops visual-contract parity pass (2026-08-14). Walk needs a quarantined node with multiple episodes and an approval/event wait; deferred to the next seeded QA cycle — dialog, sheet, and control-hook suites (including the nested requeue confirm and the after-requeue story) are green at 9a694ff2.
