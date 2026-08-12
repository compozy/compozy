---
id: RT-session-rename-durable
area: RT
title: Rename a user session without changing its identity
persona: Dora
journey: J-rename-session
expected: A valid workspace-scoped rename persists across refresh and daemon restart in Web, CLI, HTTP, UDS, and `compozy__session_rename`, while invalid, managed, and foreign-session requests change nothing.
entry_points: web session row/topbar; compozy session rename; PATCH /api/workspaces/{workspace_id}/sessions/{session_id}; compozy__session_rename
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-open-issues-20260812-002435-338441-lab/qa-artifacts/web-session-rename-pass.png;/Users/pedronauck/dev/qa-labs/compozy-open-issues-20260812-002435-338441-lab/qa-artifacts/cli-session-rename.json;/Users/pedronauck/dev/qa-labs/compozy-open-issues-20260812-002435-338441-lab/qa-artifacts/api-session-rename.json;/Users/pedronauck/dev/qa-labs/compozy-open-issues-20260812-002435-338441-lab/qa-artifacts/cli-sessions-after-restart.json
last_report: docs/qa/reports/2026-08-11-open-issues.md
overlaps:
---

Introduced for issue #344. The display name is the only mutable datum; session id, transcript, runtime, archive state, lineage, and workspace ownership remain authoritative.

2026-08-11 retest: passed. Web, CLI, and HTTP renamed the same stopped user session; a daemon restart preserved the final name, session id, stopped state, and workspace id.
