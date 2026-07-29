---
id: ET-extension-legacy-key-rejection
area: ET
title: Reject removed extension config and manifest keys
persona: Ada
journey: J-extension-policy-admin
expected: Loading `extensions.marketplace.*` or a manifest with `[actions]`/`[security]` fails before mutation and the structured validation error names the exact `extensions.trust`/`extensions.sources` or `[permissions]` replacement.
entry_points: `compozy config set`; daemon config load; `compozy extension install`
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-016; ET-017; ET-018; ET-cli-extension-sideload-policy-block
---

Added by ext-improvs Task 02. Exercise every removed marketplace leaf separately so its diagnostic names the precise successor, then attempt a legacy manifest and prove no install row or artifact is written.
