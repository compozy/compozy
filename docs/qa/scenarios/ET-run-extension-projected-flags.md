---
id: ET-run-extension-projected-flags
area: ET
title: Run an extension command with projected flags
persona: Bruno
journey: J-run-extension-commands
expected: Projected string, boolean, integer, number, nullable, enum, and repeated flags produce the declared typed input fields, omit absent optionals, and invoke the selected tool exactly once.
entry_points: compozy extension exec <extension> --cmd <path> <projected-flags>
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Added by ext-improvs Task 06 as a planning flag; no QA session ran. Confirm conversion failures name
both the CLI flag and schema field, and verify the result uses the standard human, JSON, JSONL, and
TOON output bundle.
