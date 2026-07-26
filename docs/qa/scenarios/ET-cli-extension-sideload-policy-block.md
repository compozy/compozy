---
id: ET-cli-extension-sideload-policy-block
area: ET
title: Block unverified extension side-loads through structured CLI output
persona: Ada
journey: J-extension-policy-admin
expected: With `extensions.marketplace.allow_unverified=false`, `agh extension install <non-curated-slug> --allow-unverified --yes -o json` fails before download with the stable policy diagnostic and points to Settings › Extensions; enabling policy live still requires the request flag.
entry_points: agh extension install; agh config set extensions.marketplace.allow_unverified; Settings › Extensions
qa_status: pass
bug_ids: BUG-20260715-extension-cli-slow-boot-offline
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/extension-cli-slow-boot-reachability.json; docs/qa/reports/2026-07-15-marketplace.md
last_report: docs/qa/reports/2026-07-15-marketplace.md
overlaps: ET-017; ET-018; ET-023; ET-049
---

Added by marketplace Task 05. QA should prove both consent levels, live apply without a daemon restart, deterministic JSON diagnostics over the daemon transport, and absence of install files or registry rows after a policy block.
