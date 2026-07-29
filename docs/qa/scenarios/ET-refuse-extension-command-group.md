---
id: ET-refuse-extension-command-group
area: ET
title: Refuse execution of an extension command group
persona: Bruno
journey: J-run-extension-commands
expected: Selecting a presentation group or unknown command path returns its available leaves and a useful suggestion without any invocation reaching the extension runtime.
entry_points: compozy extension exec <extension> --cmd <group-or-unknown-path>
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-discover-extension-command-tree
---

Added by ext-improvs Task 06 as a planning flag; no QA session ran. Exercise `--cmd review` against
the fixture's declared group and a close typo of `review/fetch`, then verify the fixture invocation
sequence is unchanged.
