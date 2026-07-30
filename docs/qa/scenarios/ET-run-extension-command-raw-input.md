---
id: ET-run-extension-command-raw-input
area: ET
title: Run an extension command with raw input
persona: Ada
journey: J-run-extension-commands
expected: A valid --input JSON document passes unchanged through schema validation to one canonical tool invocation, while invalid JSON or mixing projected flags fails before invocation.
entry_points: compozy extension exec <extension> --cmd <path> --input <json>
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/extension-charters.json
last_report: docs/qa/reports/2026-07-29-ext-improvs.md
overlaps: ET-run-extension-projected-flags
---

Added by ext-improvs Task 06 as a planning flag; no QA session ran. Use this escape path for schema
shapes outside the closed flag-projection subset and confirm structured diagnostics remain stable.
