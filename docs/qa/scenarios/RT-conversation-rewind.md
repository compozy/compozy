---
id: RT-conversation-rewind
area: RT
title: Rewind an idle conversation and continue from its retained prefix
persona: Théo
journey: J-rewind-conversation
expected: The same session keeps the retained prefix, restores the selected prompt as a draft, continues with fresh provider context, and exposes the discarded suffix only through archived reads
entry_points: Web session thread; compozy session rewind; POST /api/workspaces/:workspace_id/sessions/:session_id/rewind
qa_status: pass
bug_ids: BUG-20260805-rewind-reader-unavailable
fix_status: verified
retest_status: pass
fix_commits: pending-branch-commit
evidence: docs/qa/evidence/session-rewind/rewind-confirmation.png; /Users/pedronauck/dev/qa-labs/compozy-session-rewind-20260805-024938-234761-lab/qa-artifacts/qa/verification-report.md; /Users/pedronauck/dev/qa-labs/compozy-session-rewind-20260805-024938-234761-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-08-04-session-rewind.md
overlaps:
---

Conversation rewind does not restore files, tool effects, network calls, or memory. The confirmation and structured output must preserve that boundary.
