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
evidence: /home/franciscpd/dev/qa-labs/compozy-extension-agent-session-skills-20260813-122950-240954-lab/qa-artifacts/qa/provider-hosted-mcp-summary.json;/home/franciscpd/dev/qa-labs/compozy-extension-agent-session-skills-20260813-122950-240954-lab/qa-artifacts/qa/native-skill-list.json;/home/franciscpd/dev/qa-labs/compozy-extension-agent-session-skills-20260813-122950-240954-lab/qa-artifacts/qa/native-skill-search.json;/home/franciscpd/dev/qa-labs/compozy-extension-agent-session-skills-20260813-122950-240954-lab/qa-artifacts/qa/native-skill-view.json;/home/franciscpd/dev/qa-labs/compozy-extension-agent-session-skills-20260813-122950-240954-lab/qa-artifacts/qa/managed-cli-skill-view.stderr
last_report: docs/qa/reports/2026-08-13-extension-agent-session-skills.md
overlaps: ET-003
---

The current targeted run re-walked all twelve managed CLI verbs and the adjacent operator read/list
canary against the rebuilt binary. The unchanged delayed provider and native-tool legs retain their
full-walk evidence in the prior report.

QA impact 2026-08-13: reset because extension-published agents now fall back to their concrete session definition after authored lookup reports `agent not found`. Re-walk prompt catalog injection and `skill_list/search/view` through hosted MCP.

QA verdict 2026-08-13: passed. A live Codex session using the extension-published `reviewer` agent received all nine `dev-cycle` skills in its prompt and invoked `compozy__skill_list`, `compozy__skill_search`, and source-qualified `compozy__skill_view` through hosted MCP with no `agent not found`. The managed CLI guard rejected `compozy skill view` before access, while the operator and native reads both returned the `cy-review-round` body.
