---
id: RT-cursor-logical-runtime-options
area: RT
title: Apply Cursor logical model options through private launch bindings
persona: Théo
journey: J-17
expected: Grok 4.5 and 4.6 Reasoning/Fast combinations and Opus 5 Thinking variants resolve to private Cursor aliases before launch; changing a launch-bound value atomically replaces the process, verifies the reported transport model, and keeps only the logical model and typed options in public state.
entry_points: web session composer; CLI session prompt|runtime set; HTTP/UDS prompt runtime; session status and events
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-cursor-account-model-discovery; RT-session-prompt-runtime-transitions; RT-session-runtime-selection-continuity
---

Added for the ACP runtime catalog rebuild. Cover Grok 4.5 low/medium/high with Fast on/off, Grok 4.6
through xhigh with Fast, Opus 5 Thinking variants, and rejection of an invalid combination.
