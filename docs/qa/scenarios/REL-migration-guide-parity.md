---
id: REL-migration-guide-parity
area: REL
title: Keep both migration guides and the legacy disposition ledger identical
persona: Ada
journey: J-approve-compozy-beta-candidate
expected: The root and site migration guides normalize to identical canonical content across all eight required sections, describe runtime selection, input defaults, daemon startup, live-only session attachment, and run deep links as shipped, state that no v0.3 beta config migrator exists, and account for every audited legacy CLI, Web, extension, and SDK surface with a successor or an explicit removed/deferred disposition.
entry_points: MIGRATION_GUIDE.md; packages/site/content/docs/migration/**; make migration-guide-check; scripts/verify-migration-guide-parity.sh; legacy-surface disposition ledger
qa_status: pass
bug_ids:
fix_status: 
retest_status: 
fix_commits: 
evidence: docs/qa/reports/2026-08-17-electron-shell.md
last_report: docs/qa/reports/2026-08-17-electron-shell.md
overlaps: LP-loop-input-defaults; LP-loop-run-deep-link
---

story: As an autonomous migration operator, I can choose either official guide surface and receive
the same executable upgrade contract, including an honest boundary for work that has not shipped.

Task 12 QA plan: the normalized parity command and a manual disposition-ledger walk are both
required evidence. This row does not execute the deferred migrator.

QA impact 2026-07-29: both guide projections now pin beta.2, distinguish one-time bootstrap from
daemon startup, define `session resume` as live attachment only, and replace internal Task 14
language with the current unavailable contract. The next QA cycle owns parity and command
verification.
