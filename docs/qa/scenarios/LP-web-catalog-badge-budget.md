---
id: LP-web-catalog-badge-budget
area: LP
title: Loops catalog enforces the badge budget and states one facts line
persona: Dora
journey: J-05
expected: Catalog rows and cards carry no source, cap, category, or best pills — the only chip is the state `LoopStatusPill`, which survives every viewport width. Category leads a single shared facts line (identical in rows and cards) that ends with the last run's mono identity (`MonoId` + relative `Time`); `best` appears there as a plain metric. Group headers read server facet counts (falling back to the loaded length), card descriptions clamp to two lines, the lede is `LoopPageLede` with the closing hairline, a zero-loop workspace keeps the page heading with the single roster empty state, and the row Run button promotes to accent on hover.
entry_points: web /loops
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits: 65a689f8
evidence:
last_report:
overlaps: LP-delete-custom-loop
---

Added by the loops visual-contract parity pass (2026-08-14, commits 7f204c11..9a694ff2). Walk needs a workspace with built-in + custom loops and at least one loop with a finished run (recency + best segments); deferred to the next seeded QA cycle — component suites (`loop-catalog.test.tsx`, `loop-catalog.test.ts`) and the catalog stories are green at 9a694ff2.
