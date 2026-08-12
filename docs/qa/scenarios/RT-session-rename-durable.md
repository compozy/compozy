---
id: RT-session-rename-durable
area: RT
title: Rename a user session without changing its identity
persona: Dora
journey: J-rename-session
expected: A valid workspace-scoped rename persists across refresh and daemon restart in Web, CLI, HTTP, UDS, and `compozy__session_rename`, while invalid, managed, and foreign-session requests change nothing.
entry_points: web session row/topbar; compozy session rename; PATCH /api/workspaces/{workspace_id}/sessions/{session_id}; compozy__session_rename
qa_status: pass
bug_ids: BUG-20260812-rename-dialog-double-escape
fix_status: fixed
retest_status: pass
fix_commits: review-round-1 commit
evidence: docs/qa/evidence/2026-08-12-pr-351-review-round-1/CH-rename-session-parity-error.png;docs/qa/evidence/2026-08-12-pr-351-review-round-1/CH-rename-session-parity-refresh.png;docs/qa/evidence/2026-08-12-pr-351-review-round-1/CH-rename-session-parity-single-escape.png;/Users/pedronauck/dev/qa-labs/compozy-pr-351-review-round-1-20260812-030348-308615-lab/qa-artifacts/cli-session-after-recovery.json;/Users/pedronauck/dev/qa-labs/compozy-pr-351-review-round-1-20260812-030348-308615-lab/qa-artifacts/api-session-after-recovery.json
last_report: docs/qa/reports/2026-08-12-pr-351-review-round-1.md
overlaps:
---

Introduced for issue #344. The display name is the only mutable datum; session id, transcript, runtime, archive state, lineage, and workspace ownership remain authoritative.

2026-08-11 retest: passed. Web, CLI, and HTTP renamed the same stopped user session; a daemon restart preserved the final name, session id, stopped state, and workspace id.

2026-08-12 review round 1: qa_status reset to untested because a rejected Web rename now remains open and exposes a retryable inline error.

2026-08-12 review round 1 retest: passed. A daemon interruption left the dialog and entered name intact with an inline error; recovery allowed the same dialog to retry successfully, refresh preserved the new name, CLI and HTTP agreed on identity, and a foreign-workspace PATCH remained denied. The same walk found and fixed BUG-20260812-rename-dialog-double-escape.
