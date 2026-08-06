---
id: ET-managed-session-skill-loading
area: ET
title: Load an omitted skill in a managed session
persona: Ada
journey: J-load-skill-in-managed-session
expected: A real managed Codex session whose startup is delayed beyond the hosted-MCP bind TTL and whose truncated catalog omits an installed skill still discovers and loads its exact marker-bearing body through compozy__skill_view, without invoking the operator CLI or reading the skill file directly; managed environment markers make every compozy skill command fail before resolution or UDS access, while an operator can still read the same body with compozy skill view.
entry_points: delayed managed session prompt; `compozy__skill_view`; managed-env `compozy skill`; operator `compozy skill view`
qa_status: pass
bug_ids: BUG-20260805-hosted-mcp-cold-start-nonce-expiry
fix_status: fixed
retest_status: pass
fix_commits: PR #323 remediation commit
evidence: /Users/pedronauck/dev/qa-labs/compozy-issue-314-delayed-native-skill-acceptance-20260806-001626-699034-lab/qa-artifacts/qa/verification-report.md
last_report: docs/qa/reports/2026-08-05-issue-314-managed-skill-loading.md
overlaps: ET-003
---

The delayed first launch, exact native/operator body parity, full managed CLI guard, persisted events,
and clean teardown passed in the linked report and isolated QA evidence.
