---
id: RT-cursor-agent-mode
area: RT
title: Start Cursor in a mode that can perform agent work
persona: Bruno
journey: J-17
expected: New session exposes and persists a supported Cursor operating mode so a normal agent task can create the requested artifact without an impossible switch instruction.
entry_points: web Start a new session runtime selector; web session runtime controls
qa_status: blocked-verify
bug_ids: BUG-20260713-cursor-agent-mode-unavailable
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38
evidence: /Users/pedronauck/dev/qa-labs/compozy-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-new-session-grok-transcript.dom.txt;/Users/pedronauck/dev/qa-labs/compozy-qa-rt-current-source-20260730-20260730-061631-252740-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: RT-new-session-fast-feedback
---

The final assertion is a real filesystem artifact created by Cursor in the isolated workspace; an analysis-only response is not sufficient.
