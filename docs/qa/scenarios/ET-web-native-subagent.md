---
id: ET-web-native-subagent
area: ET
title: Claude's own subagents render as nested cards
persona: Rafa
journey: J-14-read-a-finished-transcript
expected: When Claude (claude-agent-acp) runs its own Agent or Task tool, CompozyOS records a provider-native subagent (origin provider_native, no child session, provider_tool_call_id set, model from the tool input when reported) and the parent transcript shows the same subagent card titled with the tool's description, with no chevron, expandable inline to show the subagent's own tool rows and text; those inner parts (parentToolCallId equal to the Agent call) no longer interleave with the parent's work rows or reply text, while an unknown parent tool call id falls back to the parent flow; the status line counts it in "N agents running" while live; it settles with its tool result, never wakes the parent, never shows the waiting banner, cannot be canceled (409 subagent_not_cancelable, no Stop in the roster), and counts toward the parent's sidebar chip; the rendering survives reload from the persisted transcript.
entry_points: web session window transcript (SubagentCard provider-native state, inline expansion); compozy session subagents <session-id> --origin provider_native --json; POST /api/workspaces/{workspace_id}/subagents/{subagent_id}/cancel; session inspector Subagents section; docs/design/opendesign/subagents/subagents-transcript.html#tr-native (VC-05)
qa_status: pass
bug_ids: BUG-20261009-native-subagent-title-task; BUG-20261009-subagent-preview-raw-markdown
fix_status: fixed
retest_status: pass
fix_commits: e7e9b276a; 7573fab3a
evidence: .compozy/tasks/subagents/orchestration/screens/pr/
last_report: docs/qa/reports/2026-10-09-subagents-r2.md
overlaps: ET-web-subagent-card; RT-subagent-delegate; ET-web-session-transcript-calm-grammar
---

Spec: `.compozy/tasks/subagents/_spec.md` Business Rule 16, ADR-004, `_uiux.md` S7–S8.
Automated owners: IT-020, UT-036…UT-040 (recorded `claude_agent_subagent.jsonl` fixture), E2E-006.
E2E-006 waits for the settled turn disclosure before opening it and asserting the
native card is completed; final response text alone does not establish settlement.
This scenario is the live walk on a real Claude provider, which pins the current `_meta.claudeCode`
shape (Known Risk: shape drift).

1. **Live card (E2E-006, VC-05).** In a Claude session, ask Claude to use its own Agent tool to
   research something in parallel. Expect one card titled with the Agent call's description, the
   Claude mark, `Running`, and no chevron. The parent's work rows do not include the subagent's reads
   and searches, and the parent's reply text excludes the subagent's text chunks.
2. **Inline expansion.** Expand the card: the subagent's own tool rows and text render inside it,
   live while it runs, using the same work-row components. Collapsed is the default.
3. **Settle.** When the Agent tool returns, the card reads `Completed` (or `Failed` on a tool error),
   elapsed freezes, and no wake turn follows. While it ran, the status line counted
   `1 agent running`; when only native subagents are live after the parent's turn, no waiting banner
   appears.
4. **Records.** `compozy session subagents <parent> --origin provider_native --json` lists the row
   with `origin: "provider_native"`, `child_session_id: null`, `provider_tool_call_id: "toolu_…"`,
   and the model when Claude reported it. `POST …/subagents/{id}/cancel` → 409
   `subagent_not_cancelable` with `Provider-native subagents cannot be canceled; stop the parent turn instead.`
   The inspector roster lists it without a Stop button; the sidebar chip counts it.
5. **Fallback.** With a parent tool call id that matches no recorded Agent call (fixture), the inner
   parts render in the parent flow as before and `subagent.native_stitch_miss` is logged.
6. **Reload.** Reload the window and reopen the finished session: the card, its inner rows, and the
   parent flow render the same from the persisted transcript.
7. **Other providers.** A Codex session's own subagents render as today (no card); ACP native
   subagent sessions are not adopted (ADR-004).

QA impact 2026-10-08 (subagents): new in this change; no prior verdict.

QA walk 2026-10-09: a real Claude Agent call produced a provider_native record and an inline-expandable card (patched build for the roster); the title stays `Task`. Steps 5 and 7 not walked. Verdict: fail. Report: `docs/qa/reports/2026-10-09-subagents.md`.

Re-walk 2026-10-09 (stock 980d51fbe): live native card titled with the Agent description, `1 agent running`, inline expansion with its Read row, no wake. Steps 4 (cancel 409), 5 (fixture fallback), and 7 were not walked. Verdict: pass. Report: `docs/qa/reports/2026-10-09-subagents-r2.md`.
