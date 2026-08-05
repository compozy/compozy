---
id: ET-managed-session-skill-loading
area: ET
title: Load a skill from a managed session
persona: Ada
journey: J-load-skill-in-managed-session
expected: A managed Codex session loads the same verified skill body through the native tool or CLI fallback while foreign workspace, mismatched agent, non-skill route, and mutation requests are denied.
entry_points: `compozy__skill_view`; `compozy skill view`; `GET /api/skills/:name/content`
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: session sess-fde197161ff83533 events 32,59,72,80,88,98,106,109; /Users/pedronauck/dev/qa-labs/compozy-issue-314-devtool-oss-launch-20260805-194336-181847-lab/qa-artifacts/qa/teardown.json; session sess-518717aea033acef events 50,58,64; /Users/pedronauck/dev/qa-labs/compozy-issue-314-read-only-skill-transport-20260805-211857-719811-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-08-05-issue-314-managed-skill-loading.md
overlaps: ET-003
---

The operator read, native managed read, CLI managed read, forged environment identity, scope denials, route allowlist, read-only boundary, recovery read, socket removal, and lab teardown were exercised in isolated runtime homes.
