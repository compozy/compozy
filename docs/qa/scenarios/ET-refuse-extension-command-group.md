---
id: ET-refuse-extension-command-group
area: ET
title: Refuse execution of an extension command group
persona: Bruno
journey: J-run-extension-commands
expected: Selecting a presentation group or unknown command path returns its available leaves and a useful suggestion without any invocation reaching the extension runtime.
entry_points: compozy extension exec <extension> --cmd <group-or-unknown-path>
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/extension-charters.json
last_report: docs/qa/reports/2026-07-29-ext-improvs.md
overlaps: ET-discover-extension-command-tree
---

Added by ext-improvs Task 06 as a planning flag; no QA session ran. Exercise `--cmd review` against
the fixture's declared group and a close typo of `review/fetch`, then verify the fixture invocation
sequence is unchanged.
