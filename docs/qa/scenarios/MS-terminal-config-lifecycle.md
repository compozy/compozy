---
id: MS-terminal-config-lifecycle
area: MS
title: Configure terminal runtime policy safely
persona: Dora
journey: J-administer-runtime-settings
expected: Every `[terminal]` value defaults, validates, layers, and applies to the next terminal operation as documented, while a profile cannot raise the daemon-wide terminal cap.
entry_points: global, workspace, and profile config.toml [terminal]; structured configuration surfaces after public activation
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Added for the integrated-terminal config lifecycle. Task 01 establishes the internal contract; the
public configuration projections activate with task 06. The task loop defers this persona walk to
its dedicated QA phase.
