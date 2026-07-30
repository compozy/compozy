---
id: ET-extension-legacy-key-rejection
area: ET
title: Reject removed extension config and manifest keys
persona: Ada
journey: J-extension-policy-admin
expected: Loading `extensions.marketplace.*` or a manifest with `[actions]`/`[security]` fails before mutation and the structured validation error names the exact `extensions.trust`/`extensions.sources` or `[permissions]` replacement.
entry_points: `compozy config set`; daemon config load; `compozy extension install`
qa_status: pass
bug_ids: BUG-20260729-removed-extension-config-generic-error
fix_status: fixed
retest_status: pass
fix_commits:
evidence: docs/qa/bugs/BUG-20260729-removed-extension-config-generic-error.md
last_report: docs/qa/reports/2026-07-29-ext-improvs.md
overlaps: ET-016; ET-017; ET-018; ET-cli-extension-sideload-policy-block
---

Added by ext-improvs Task 02. Exercise every removed marketplace leaf separately so its diagnostic names the precise successor, then attempt a legacy manifest and prove no install row or artifact is written.

Task 11 reproduced a generic CLI error for an unrecognized leaf under the removed marketplace
subtree. After the contained fix, the release binary names `extensions.trust or
extensions.sources`; the existing manifest/config loader suites continue to prove fail-before-write
behavior and leaf-specific replacements.
