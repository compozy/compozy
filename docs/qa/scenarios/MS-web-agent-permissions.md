---
id: MS-web-agent-permissions
area: MS
title: Agent Permissions page and Settings share one five-level vocabulary
persona: Dora
journey: J-31
expected: An agent's home lists Permissions under Setup with the current level as its summary. The Permissions page shows the five levels in one column with one-line descriptions — Ask before every action, Ask before changes, Accept edits, Auto-review, Full access. An always-on agent with nothing set selects Accept edits with a Default pill; an on-demand agent adds Follow Settings naming the current Settings level. Choosing Auto-review on a provider that declares no reviewer shows the Not available pill and the "has no AI reviewer" warning; a running session that fell back shows the degraded warning. Full access on Codex explains Codex keeps its own reviewer. Always-on agents show "Whose requests can it answer?" with Only its own sessions and Any session it can reach. Exceptions start closed; Add an exception adds an action, path and Allow/Never row, a path outside the project blocks Save with the reason, and listed exceptions show the coverage note. Save writes `permissions`, `session_approval` and `permission_rules` in one digest-guarded update and the values survive a reread and `agent show -o json`. An extension-owned agent shows the page read-only. Settings → General → Permissions shows the same five labels in one column, and New agent's Advanced section uses the same labels.
entry_points: web desktop shell → Agents → agent home → Permissions; Settings → General → Permissions; New agent → Advanced
qa_status: partial
bug_ids: BUG-20261103-reviewer-mode-omitted
fix_status: fixed
retest_status: verified
fix_commits: 2f5647d5b, 4056dbcc2
evidence: web/src/systems/agent/components/__tests__/agent-editors.test.tsx (Permissions); web/src/routes/_app/settings/__tests__/-general.test.tsx; Storybook systems/agent/components/AgentEditors Permissions* stories; docs/qa/reports/artifacts/2026-11-03-stays-on-hardening-qa/ (2026-11-03 always-on QA walk)
last_report: docs/qa/reports/2026-11-03-vocabulary-alignment-qa.md
overlaps: MS-web-agent-create-simple-advanced, ET-web-agent-detail-tab-parity
---

Always-on hardening S5, S6, S14, S15. In an isolated lab, open an always-on agent with no `permissions`,
confirm Accept edits is selected with Default, switch to Ask before changes, choose Any session it
can reach, add an Edit files exception for `docs/**` set to Allow and a Delete files exception set to
Never, and save. Reread the agent (`compozy agent show <name> -o json`) and confirm `permissions`,
`session_approval: workspace` and both `permission_rules`. Remove both exceptions, save, and confirm
`permission_rules` is gone. Try `../secrets` as a path: Save stays blocked with the reason. Open an
on-demand agent and confirm Follow Settings names the level from Settings → General. Open an agent
installed by an extension and confirm every control is disabled.

Provider reviewer availability reads `reviewer_mode` from `GET /api/providers`; when the daemon omits
the field the page shows no reviewer warning. Verify against a provider configured with an empty
`reviewer_mode` once the field ships.

QA verification 2026-11-03 (always-on hardening S5/S6/S15, soh/qa 583415ef5, isolated acpmock lab): Partial. An always-on agent with no `permissions` showed Accept edits with Default, and the five levels sat in one column with descriptions. Saving Ask before changes, Any session it can reach, `docs/**` Allow and a Delete files exception set to Never on `qa-courier` read back as `approve-reads`, `session_approval: workspace` and both `permission_rules`. Settings → General shows the same labels (`05-s5-permissions-levels.png`, `06-s6-approval-reach.png`, `15-s15-settings-levels.png`). Fail: Auto unavailable can never show, because `GET /api/providers` omits `reviewer_mode` for every provider without a reviewer and the Web only warns on an empty string (BUG-20261103-reviewer-mode-omitted). Not walked: removing exceptions, the `../secrets` block, Follow Settings on an on-demand agent, and an extension-installed agent.

Re-walk 2026-11-03 on 4056dbcc2 (S5/S15): BUG-20261103-reviewer-mode-omitted is verified fixed. `GET /api/providers` sends `reviewer_mode: ""` for providers without a reviewer, and an always-on Gemini agent's Permissions page marks Auto-review as Not available (`https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/stays-on-hardening/4056dbcc2/05-s5-permissions-levels.png`). Settings → General now shows the same level icons as the agent page (`https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/stays-on-hardening/4056dbcc2/15-s15-settings-levels.png`). Difference from the draft: a pill replaces the draft's alert naming the provider. Still not walked: the `../secrets` block, Follow Settings and an extension-installed agent.

QA impact 2026-11-03 (#1199 vocabulary, v0.5.1): the levels read Ask before every action, Ask before changes, Accept edits, Auto-review and Full access in the agent Permissions page, Settings → General and New agent's Advanced section; the saved values (`deny-all` … `approve-all`) are unchanged. The agent home lists Permissions under Setup. Not walked; S4 walks it on the integration head.
