---
id: ET-web-native-subagent
area: ET
title: Claude's own subagents render as nested cards
persona: Rafa
journey: J-14-read-a-finished-transcript
expected: When Claude (claude-agent-acp) runs its own Agent or Task tool, CompozyOS records a provider-native subagent (origin provider_native, no child session, provider_tool_call_id set, model from the tool input when reported) and the parent transcript shows the same subagent card titled with the tool's description, with no chevron, expandable inline to show the subagent's own tool rows and text; those inner parts (parentToolCallId equal to the Agent call) no longer interleave with the parent's work rows or reply text, while an unknown parent tool call id falls back to the parent flow; the status line counts it in "N agents running" while live; it settles with its tool result — or, when Claude runs it in the background (the Agent tool returns a launch receipt), stays Running until its own task reports a terminal state (Completed, Failed, or Canceled for a stopped/killed task) as soon as that task finishes, while the parent and sibling tasks keep running, with the task's summary as its result once Claude reports it; only a background row with no task evidence falls back to Completed with no result when the parent's turn ends (Interrupted when the turn is canceled or the session stops) — never wakes the parent, never shows the waiting banner, cannot be canceled (409 subagent_not_cancelable, no Stop in the roster), and counts toward the parent's sidebar subagent indicator and appears in its subagent branch as a clickable row that opens the parent session, reveals its card, expands available nested activity, and briefly highlights it; the rendering survives reload from the persisted transcript.
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
Automated owners: IT-020, UT-036…UT-040 (recorded `claude_agent_subagent.jsonl` fixture, plus
`claude_agent_background_subagent.jsonl` for the background launch and
`claude_agent_acp_0.98.0_task_lifecycle.jsonl` for raw SDK task lifecycle frames), E2E-006.
Task lifecycle owners: `TestClaudeAgentSubagentContract` (decoder), `TestSubagentNativeLifecycle`,
`TestSubagentNativeRecoveryEdges`, `TestNativeSubagentPersistenceOrder`, and the SQL enrichment case
in `TestGlobalDBSubagentNativeRecovery`.
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
4. **Background launch (issue #766, per-task lifecycle).** Ask Claude to start two Agent subagents
   in the background, one short and one longer, and keep working. The Agent calls return at once
   with a launch receipt, but both cards and the composer control stay `Running` (status line
   `2 agents running`) while their inner steps keep arriving; the receipt is never shown as a result.
   When the short task finishes, its card alone reads `Completed` right away, while the long task and
   the parent are still running; `compozy session subagents <parent> --origin provider_native --json`
   shows that row `completed` with a `settled_at` at the task's end (before the turn's `done`) and,
   once Claude reports it, the task's summary as `result` (it may be `null` for a moment between the
   terminal patch and the summary). The long task settles the same way when it finishes. A genuinely
   failed task reads `Failed` with its error; a stopped/killed task reads `Canceled`. Only a background
   row without any task evidence falls back to `Completed` with `result: null` at the turn's end.
   Stop the parent's turn while one runs: a task the provider reports stopped reads `Canceled`; one
   with no terminal evidence reads `Interrupted`. Record the resolved claude-agent-acp version.
5. **Records.** `compozy session subagents <parent> --origin provider_native --json` lists the row
   with `origin: "provider_native"`, `child_session_id: null`, `provider_tool_call_id: "toolu_…"`,
   and the model when Claude reported it. `POST …/subagents/{id}/cancel` → 409
   `subagent_not_cancelable` with `Provider-native subagents cannot be canceled; stop the parent turn instead.`
   The inspector roster lists it without a Stop button; the sidebar subagent indicator counts it.
6. **Fallback.** With a parent tool call id that matches no recorded Agent call (fixture), the inner
   parts render in the parent flow as before and `subagent.native_stitch_miss` is logged.
7. **Reload.** Reload the window and reopen the finished session: the card, its inner rows, and the
   parent flow render the same from the persisted transcript.
8. **Other providers.** A Codex session's own subagents render as today (no card); ACP native
   subagent sessions are not adopted (ADR-004).

QA impact 2026-10-08 (subagents): new in this change; no prior verdict.

QA walk 2026-10-09: a real Claude Agent call produced a provider_native record and an inline-expandable card (patched build for the roster); the title stays `Task`. Steps 5 and 7 not walked. Verdict: fail. Report: `docs/qa/reports/2026-10-09-subagents.md`.

Re-walk 2026-10-09 (stock 980d51fbe): live native card titled with the Agent description, `1 agent running`, inline expansion with its Read row, no wake. Steps 4 (cancel 409), 5 (fixture fallback), and 7 were not walked. Verdict: pass. Report: `docs/qa/reports/2026-10-09-subagents-r2.md`.

QA impact 2026-10-16 (#766): background Agent launches no longer settle on their launch receipt
(step 4). Codex's own collab subagents are still not recorded as provider-native rows (step 8).
Real-provider walk pending; previous verdict kept for the foreground path.

QA walk 2026-10-19 (#835, scoped): an isolated source-built daemon and deterministic ACP fixture streamed an edit and four native reviewers. The changed-files summary stayed absent throughout child updates, appeared once after settlement, expanded to its file row, and persisted on reload. Live cards remained visible. Focused pass only; other steps and real-provider paths retain their previous status. Report: `docs/qa/reports/2026-10-19-issue-835-transcript-flicker.md`.

Review re-walk 2026-10-19 (#835): fresh-browser stopped history also retains the saved file summary after a one-time authoritative roster read, without interval polling. The live streaming scenario was repeated and passed. See the same scoped report.


QA impact 2026-10-19 (issues #851–#852): extend the parallel-parent walk to the
intervals before delegation and after every child settles, including completion
steer segments. Parent work must not fold/unfold or toggle its changed-file
roll-up between tool calls. Observe four real Claude subagents, confirm delegation
rows settle and React reports no duplicate keys or cross-component render updates,
then stop a later parent turn and reload. Compare the compositor and an independent
window. The broader foreground/background/native cancellation matrix above
retains its prior status; this follow-up is recorded in the
[issues 851–854 report](../reports/2026-10-19-issues-851-854.md).

QA impact 2026-10-24 (#947): from the sidebar branch, All projects session list, composer
roster, and inspector, click a provider-native row. The parent opens (or stays open), older
transcript pages load when needed, and enclosing turn/group/card disclosures open to reveal
that subagent with a brief focus highlight. Close it manually and click the row again: it
reopens. A card without projected nested activity still receives focus; no child session is
invented. Verify keyboard activation and a narrow session window. Delegated rows continue
to open their actual child sessions. E2E-006 owns the daemon-served native click journey;
`session-thread-navigation.test.tsx` owns paging and repeat-reveal behavior.

QA re-walk 2026-10-24 (merge train, E2E-006): the isolated daemon-served scenario
passed in 12.3s (48.3s total). Sidebar reveal, manual close/reopen, and All projects
modal reveal of the settled fold all passed. Native reveal now locates the fold's
card by subagent ID: retained streaming rows may lack projected part indices.
The modal gesture uses the existing branch-toggle test ID and preserves an
already-expanded branch. All content, fold and completion assertions remain.
This is deterministic ACP-fixture evidence; the real-provider matrix above is unchanged.

QA impact 2026-10-27 (native task lifecycle): CompozyOS now asks claude-agent-acp (session
`_meta.claudeCode.emitRawSDKMessages`, four `system` task subtypes only) for raw SDK task messages
and settles each background native row on its own `task_updated`/`task_notification` evidence,
filling a missing result from a later summary without a second settle. Step 4 changes from
turn-end-only completion to per-task completion with the turn-end fallback; foreground cards still
settle with their tool result. A real-provider walk of steps 3–4 (two background tasks, a failure,
a parent cancel) and a daemon restart mid-run is pending; deterministic fixture evidence only
(`claude_agent_acp_0.98.0_task_lifecycle.jsonl`, adapter 0.98.0).
