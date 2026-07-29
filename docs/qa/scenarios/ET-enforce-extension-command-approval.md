---
id: ET-enforce-extension-command-approval
area: ET
title: Enforce approval and availability for extension commands
persona: Ada
journey: J-run-extension-commands
expected: Extension exec and tool invoke return the same approval or unavailable reason, and a valid single-use approval token allows each path to execute exactly once with trusted scope unchanged.
entry_points: compozy extension exec <extension> --cmd approved; compozy tool invoke <tool-id>; compozy tool approve <tool-id>
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Added by ext-improvs Task 06 as a planning flag; no QA session ran. Compare deterministic stderr and
reason codes across both presentation paths, then prove unavailable backend diagnostics take
precedence over approval metadata and no denied request reaches the extension runtime.
