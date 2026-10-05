---
id: ET-terminal-windows-parity
area: ET
title: Run a full interactive terminal on Windows
persona: Dora
journey: J-operate-terminal-windows
expected: A local Windows workspace exposes the same interactive terminal controls and lifecycle as macOS and Linux.
entry_points: Terminal app; terminal CLI; local Windows workspace
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-09-10-qa-execution-unblock/platform-prerequisites.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-terminal-limits-capabilities
---

Flagged by integrated-terminal task 08. Task 10 owns the real-user Windows walk, evidence, and verdict.

Walk:

1. Open Terminal in a local Windows workspace and verify input, output, resize, attach, and recording are available.
2. Start a command that creates a child process, close the terminal, and verify the whole process tree exits.
3. Run `compozy terminal exec` and verify bounded output and the exit code.

## 2026-09-10 prerequisite reassessment

Current assigned lab is Darwin arm64/macOS26.6.2. A real Windows runtime remains required for ConPTY, attach/resize/record, process-tree teardown and bounded exec. Windows cross-compilation from the terminal fix is not runtime proof. Historical CH-terminal-platform-ladder is superseded; do not reintroduce control transfer. No runtime pass is claimed; retain blocked-verify with this exact external prerequisite.


2026-09-27 scope update: current coverage follows the surviving product surfaces; a fresh walk is required.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
