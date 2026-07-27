---
id: REL-migration-guide-parity
area: REL
title: Keep both migration guides and the legacy disposition ledger identical
persona: Ada
journey: J-approve-compozy-beta-candidate
expected: The root and site migration guides normalize to identical canonical content across all eight required sections, describe runtime selection, input defaults, and run deep links as shipped, mark the live config migrator and first-boot legacy probe deferred to Task 14, and account for every audited legacy CLI, Web, extension, and SDK surface with a successor or an explicit removed/deferred disposition and workaround.
entry_points: MIGRATION_GUIDE.md; packages/site/content/runtime/migration/**; make migration-guide-check; scripts/verify-migration-guide-parity.sh; legacy-surface disposition ledger
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/beta-candidate/migration-guide-check.txt; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/beta-candidate/deferred-migrator.txt
last_report: docs/qa/reports/2026-07-27-devtool-oss-launch.md
overlaps: LP-loop-input-defaults; LP-loop-run-deep-link
---

story: As an autonomous migration operator, I can choose either official guide surface and receive
the same executable upgrade contract, including an honest boundary for work that has not shipped.

Task 12 QA plan: the normalized parity command and a manual disposition-ledger walk are both
required evidence. This row does not execute the deferred migrator.
