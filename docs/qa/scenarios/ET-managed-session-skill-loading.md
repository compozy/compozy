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
evidence: /Users/pedronauck/dev/qa-labs/compozy-managed-skill-cli-guard-20260806-021127-858332-lab/qa-artifacts/qa/verification-report.md;/Users/pedronauck/dev/qa-labs/compozy-managed-skill-cli-guard-20260806-021127-858332-lab/qa-artifacts/qa/cli-guard-evidence.json;/Users/pedronauck/dev/qa-labs/compozy-managed-skill-cli-guard-20260806-021127-858332-lab/qa-artifacts/qa/qa-audit-report.json;/Users/pedronauck/dev/qa-labs/compozy-managed-skill-cli-guard-20260806-021127-858332-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-08-05-review-findings-managed-skills.md
overlaps: ET-003
---

The current targeted run re-walked all twelve managed CLI verbs and the adjacent operator read/list
canary against the rebuilt binary. The unchanged delayed provider and native-tool legs retain their
full-walk evidence in the prior report.
