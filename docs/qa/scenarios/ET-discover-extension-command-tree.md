---
id: ET-discover-extension-command-tree
area: ET
title: Discover an extension command tree
persona: Bruno
journey: J-run-extension-commands
expected: Human discovery renders active groups and leaves as a tree while JSON returns the same workspace-filtered flat descriptors with flags, risk, and approval metadata.
entry_points: compozy extension commands; GET /api/extensions/commands
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Added by ext-improvs Task 06 as a planning flag; no QA session ran. Include the flat `greet` leaf,
the declared `review` group, and the nested `review/fetch` leaf, then confirm disabled, unavailable,
global-foreign, and workspace-foreign extensions contribute nothing.
