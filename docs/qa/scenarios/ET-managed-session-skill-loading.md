---
id: ET-managed-session-skill-loading
area: ET
title: Load an omitted skill in a managed session
persona: Ada
journey: J-load-skill-in-managed-session
expected: A real managed Codex session whose startup is delayed beyond the hosted-MCP bind TTL and whose truncated catalog omits an installed skill still discovers and loads its exact marker-bearing body through compozy__skill_view, without invoking the operator CLI or reading the skill file directly; managed environment markers make every compozy skill command fail before resolution or UDS access, while an operator can still read the same body with compozy skill view.
entry_points: delayed managed session prompt; `compozy__skill_view`; managed-env `compozy skill`; operator `compozy skill view`
qa_status: untested
bug_ids: BUG-20260805-hosted-mcp-cold-start-nonce-expiry
fix_status: fixed
retest_status: pass
fix_commits: PR #323 remediation commit
evidence: /Users/pedronauck/dev/qa-labs/compozy-pr372-extension-agent-session-skills-native-cli-20260813-181110-157690-lab/qa-artifacts/qa/hosted-skill-summary.json;/Users/pedronauck/dev/qa-labs/compozy-pr372-extension-agent-session-skills-native-cli-20260813-181110-157690-lab/qa-artifacts/qa/provider-hosted-skill-walk.json;/Users/pedronauck/dev/qa-labs/compozy-pr372-extension-agent-session-skills-native-cli-20260813-181110-157690-lab/qa-artifacts/qa/qa-audit-report.md
last_report: docs/qa/reports/2026-08-13-pr372-extension-agent-session-skills-native-cli.md
overlaps: ET-003
---

The current targeted run re-walked all twelve managed CLI verbs and the adjacent operator read/list
canary against the rebuilt binary. The unchanged delayed provider and native-tool legs retain their
full-walk evidence in the prior report.

QA impact 2026-08-13: reset because extension-published agents now fall back to their concrete session definition after authored lookup reports `agent not found`. Re-walk prompt catalog injection and `skill_list/search/view` through hosted MCP.

QA evidence correction 2026-08-13: the prior pass is not valid evidence for PR #372 because its build predates this PR head. It is historical only and does not set this scenario status.

QA verdict 2026-08-13 (fresh native-CLI lab): passed. A real operator-home Codex `reviewer` session exposed the exact ten-name catalog (`compozy` plus the nine `spec-cycle` skills) in prompt, command, hosted `skill_list`, empty `skill_search`, and all ten `skill_view` calls; the provenance-filtered extension subset was exactly the nine `spec-cycle` names. This is a substantive persona-walk verdict only: the QA report remains blocked on C14 until a successful final gate exists.

QA impact 2026-08-25 (skill sources): reset because the skill-sources cycle changed the exact seam this scenario depends on. Provider-aware injection suppression now omits skills from the prompt *by design* when the session's provider already reads their winning root natively, so "the catalog omitted it" is no longer only a truncation symptom — and `compozy__skill_view` gained `origin`, `owner_scope`, and `exposures[]` in its header, with fresh schema digests. This is the cycle's adjacent canary: the journey's whole promise is that an agent still reaches a skill the prompt left out, which is precisely where a suppression bug becomes invisible. Re-walk the delayed managed session, the native-tool load, the managed-environment CLI refusal, and the operator read. Charter: `CH-skill-sources-managed-session-canary`.
