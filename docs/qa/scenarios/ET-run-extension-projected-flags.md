---
id: ET-run-extension-projected-flags
area: ET
title: Run an extension command with projected flags
persona: Bruno
journey: J-run-extension-commands
expected: Projected string, boolean, integer, number, nullable, enum, and repeated flags produce the declared typed input fields, omit absent optionals, and invoke the selected tool exactly once.
entry_points: compozy extension exec <extension> --cmd <path> <projected-flags>
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/extension-charters.json
last_report: docs/qa/reports/2026-07-29-ext-improvs.md
overlaps:
---

Added by ext-improvs Task 06 as a planning flag; no QA session ran. Confirm conversion failures name
both the CLI flag and schema field, and verify the result uses the standard human, JSON, JSONL, and
TOON output bundle.
