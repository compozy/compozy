---
id: ET-web-subagent-card
area: ET
title: Watch, open, and stop subagents from the session view
persona: Bruno
journey: J-13-follow-a-live-run
expected: When a session's agent delegates, the parent transcript shows a Checked subagent capabilities row and, at the delegation point, a subagent card (provider mark with status dot, title clipped at 72 characters, status word Queued/Running/Waiting for you/Completed/Failed/Canceled/Interrupted, live progress line coalesced to once per second, ticking elapsed frozen at settle, chevron only when a child session exists) that replaces the delegate tool row and is exempt from turn folding; adjacent same-turn cards collapse into one group ("3 subagents", "2 working · 1 needs you · 1 done · 1 failed", up to 3 avatars plus +N); hover or keyboard focus opens a hover card with model (Not reported when absent), effort, status, elapsed, and a 280-character preview, and Escape closes it; click opens the child in the same window under a "Subagent of <parent>" divider with Open parent, and ⌘/Ctrl-click opens a new window; when the parent's turn ends with live delegated subagents the composer shows "Waiting on subagent <title>" or "Waiting on N subagents" with name buttons, "and N more", and Stop (Stopping…, toast "Could not stop subagents." on failure); the status line counts live subagents as "N agents running"; the card flips to Completed and the wake turn renders before the parent's final reply; every surface matches its board VC in docs/design/opendesign/subagents/.
entry_points: web session window transcript (SubagentCard, SubagentGroup, SubagentHoverContent); composer SubagentWaitingBanner and working status line; child session transcript divider; docs/design/opendesign/subagents/subagents-transcript.html (VC-01…07) and subagents-composer.html (VC-01…03)
qa_status: fail
bug_ids: BUG-20261009-subagent-routes-unavailable; BUG-20261009-subagent-card-hosted-tool-name; BUG-20261009-subagent-card-live-missing; BUG-20261009-subagent-running-count-settled; BUG-20261009-subagent-preview-raw-markdown
fix_status: pending
retest_status:
fix_commits:
evidence: .compozy/tasks/subagents/orchestration/screens/pr/
last_report: docs/qa/reports/2026-10-09-subagents-r2.md
overlaps: RT-subagent-delegate; ET-web-native-subagent; ET-web-session-sidebar-threads; ET-web-session-transcript-calm-grammar
---

Spec: `.compozy/tasks/subagents/_uiux.md` S1–S7 and ADR-005; copy in `COPY.md` §6 "Subagent Terms".
Automated owners: E2E-001…E2E-005 (acpmock "claude" + "codex"), UT-W01…UT-W16, and the subagent
component stories. This scenario is the real-provider walk with visual parity against the boards.

1. **Card (E2E-001, transcript VC-01/VC-02).** In a Claude session, ask for a Codex second opinion.
   Expect `Checked subagent capabilities` (a single call shows no count), then a card with the Codex mark, the title,
   `Running`, and a ticking elapsed; no "Used session spawn" or delegate tool row. While Codex works,
   line 2 shows its current step. On settle the card reads `Completed` with the result's first line,
   elapsed freezes, a wake turn renders, and the parent's final reply follows. A failed delegate call
   (for example an unauthenticated provider) stays a failed tool row with no card.
2. **States.** Drive or seed queued, running with and without progress, waiting for you (child
   approval), failed (destructive line 2), canceled, interrupted, unknown provider mark (bot glyph), and
   a 72-character title; compare each against `subagents-transcript.html#tr-card`. After a reconnect,
   a stale card does not tick.
3. **Group (E2E-002, VC-03).** Delegate three in one turn: one group card `3 subagents`, stacked marks,
   `3 working`; expanding lists three cards; collapse persists while the window stays open; as they
   settle the summary reads `2 done · 1 failed` (destructive tone). With a waiting child it adds
   `1 needs you`. Four or more show 3 avatars and `+N`. Collapsed and all settled → dimmed.
4. **Hover (E2E-003, VC-04).** Hover a card (opens after ~200 ms) and Tab to it (opens on focus):
   title, provider mark + model (`Not reported` when absent) · effort, fast icon only for fast, status +
   elapsed, preview (progress or result, ≤ 280 chars + `…`). Escape closes it.
5. **Drill-in (E2E-004, VC-07).** Click the card: the same window shows the child with the divider
   `Subagent of <parent title>` and a composer; `Open parent` returns. ⌘-click (Ctrl-click) opens a
   new window. With the parent deleted the divider reads `Subagent of a deleted session` with no action.
   A plain `session_spawn` child shows no divider.
6. **Waiting banner (E2E-005, composer VC-01/VC-02).** End the parent turn with two delegated subagents
   still running: banner `Waiting on 2 subagents` with both names as buttons (each opens its subagent);
   with more than three, `and N more` opens the inspector roster. `Stop` reads `Stopping…`, cancels
   both, and the banner disappears. Force a stop failure: toast `Could not stop subagents.` Only
   provider-native subagents live → no banner.
7. **Tool phrases and status line (VC-06, composer VC-03).** Status and cancel calls render as
   `Read subagent status` / `Canceled a subagent`, grouped as `… 2 times`; a capabilities call with no
   success reads `Tried to check subagent capabilities`. During a turn the status line reads
   `Working for … · N agents running`, counting live subagents of both origins once.
8. **Accessibility.** Cards are buttons with `aria-label="Open <title>"` and the status as
   `aria-description`; group disclosure exposes `aria-expanded`; elapsed is text, not color only.
9. **Visual parity.** Capture each state with `eng-ui-screenshot` and compare with the cited board
   VC ids; record differences in the QA report.

QA impact 2026-10-08 (subagents): new in this change; no prior verdict.

Fixture follow-up 2026-10-09: `internal/testutil/acpmock/testdata/native_tool_delegate_fixture.json`
provides `subagent-delegator` and `subagent-worker`. Register both and prompt the parent with
`delegate child work`. Its `native_tool_call` steps execute hosted MCP from the agent process,
so the delegate result and subagent card marker come from the persisted ACP transcript.
`TestDaemonE2EAgentDelegatesThroughHostedMCP` owns this backend persistence assertion; the browser
lane still owns the card rendering and navigation assertions above.

QA walk 2026-10-09: no card renders for real Claude delegations on the stock build; on a QA-local patched build the card, group, hover, drill-in divider, and waiting banner with Stop worked. Verdict: fail. Report: `docs/qa/reports/2026-10-09-subagents.md`.

Re-walk 2026-10-09 (stock 980d51fbe): cards, group, hover (pointer and keyboard focus, Escape), drill-in divider, ⌘-click second window, waiting banner, plain-text previews, and the running count pass. The card still lags behind a delegate tool row in a composer-submitted live turn. Verdict: fail. Report: `docs/qa/reports/2026-10-09-subagents-r2.md`.
