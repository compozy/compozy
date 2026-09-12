---
id: ET-terminal-windows-parity
area: ET
title: Run a full interactive terminal on Windows
persona: Dora
journey: J-operate-terminal-windows
expected: A local Windows workspace exposes the same interactive terminal controls and lifecycle as macOS and Linux, while a sandbox workspace remains execute-only.
entry_points: Terminal app; terminal CLI; local Windows workspace; sandbox workspace
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status: blocked-verify
fix_commits:
evidence: docs/qa/evidence/2026-09-10-qa-execution-unblock/platform-prerequisites.json
last_report: docs/qa/reports/2026-09-10-qa-execution-unblock.md
overlaps: ET-terminal-limits-capabilities
---

Flagged by integrated-terminal task 08. Task 10 owns the real-user Windows walk, evidence, and verdict.

Walk:

1. Open Terminal in a local Windows workspace and verify input, output, resize, attach, and recording are available.
2. Start a command that creates a child process, close the terminal, and verify the whole process tree exits.
3. Run `compozy terminal exec` and verify bounded output and the exit code.
4. Open Terminal in a sandbox workspace and verify command execution remains available while interactive controls are absent.

## 2026-09-10 prerequisite reassessment

Current assigned lab is Darwin arm64/macOS26.6.2. A real Windows local and sandbox runtime remains required for ConPTY, attach/resize/record, process-tree teardown and bounded exec. Windows cross-compilation from the terminal fix is not runtime proof. Historical CH-terminal-platform-ladder is superseded; do not reintroduce control transfer. No runtime pass is claimed; retain blocked-verify with this exact external prerequisite.
