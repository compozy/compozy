---
id: ET-web-subagent-card
area: ET
title: Watch, open, and stop subagents from the session view
persona: Bruno
journey: J-13-follow-a-live-run
expected: When a session's agent delegates, the parent transcript shows a Checked subagent capabilities row and, at the delegation point, a subagent card (provider mark without a status dot, title clipped at 72 characters and dimmed once settled, a trailing session StateGlyph with the status word Queued/Working/Waiting on subagents/Waiting for you/Done/Idle/Failed/Canceled/Interrupted, live progress or result line coalesced to once per second, elapsed after Working (ticking) and Done (frozen), chevron for inspectable subagents, including pending admission) that replaces the delegate tool row and is exempt from turn folding; adjacent same-turn cards collapse into one group ("3 subagents", the most urgent member glyph — attention > failed > working > done — before "2 working · 1 needs you · 1 done · 1 failed", up to 3 avatars plus +N); hover or keyboard focus opens a hover card with model (Not reported when absent), effort, status, elapsed, and a 280-character preview, and Escape closes it; click opens a live read-only drawer without changing the parent; Open as session navigates to the child under a "Subagent of <parent>" divider, and ⌘/Ctrl-click opens a new window; when the parent's turn ends with live delegated subagents the composer shows "Waiting on subagent <title>" or "Waiting on N subagents" with name buttons, "and N more", and Stop (Stopping…, toast "Could not stop subagents." on failure); the status line counts live subagents as "N agents running"; the composer toolbar shows a subagents control after the context ring and before Attach files — a quiet icon while none run, the icon with a live dot (pulsing while running, accent when one needs you) and the live count while any run — whose popover (title "Subagents · N running", total, current rows first, then "Previous subagents (N)", "Nothing delegated yet." when the session has none) uses the inspector roster rows and returns focus to the control on Escape; the card flips to Done and the wake turn renders before the parent's final reply; an isolated subagent (delegated with isolation "worktree") prepends its middle-truncated branch to line 2, links its PR as "#N" with a state glyph beside (never inside) the open control, lists Worktree, Branch, Base, Commits, PR and Observed in the hover only for facts the daemon observed, says "PR status unknown" when it could not check and "No pull request" when the forge answered none, and repeats the branch and PR link on its inspector roster row, while shared subagents stay unchanged; every surface matches its board VC in docs/design/opendesign/subagents/ and docs/design/opendesign/agent-collaboration/ (VC-06, VC-07).
entry_points: web session window transcript (SubagentCard, SubagentGroup, SubagentHoverContent); composer SubagentWaitingBanner, working status line, and SubagentComposerControl; child session transcript divider; docs/design/opendesign/subagents/subagents-transcript.html (VC-01…07) and subagents-composer.html (VC-01…03); docs/design/opendesign/agent-collaboration/agent-collaboration-subagents.html (VC-06, VC-07)
qa_status: untested
bug_ids: BUG-20261009-subagent-routes-unavailable; BUG-20261009-subagent-card-hosted-tool-name; BUG-20261009-subagent-card-live-missing; BUG-20261009-subagent-running-count-settled; BUG-20261009-subagent-preview-raw-markdown
fix_status: fixed
retest_status: pending
fix_commits: c5105724d; e7e9b276a; 1ca1f1a38; 7573fab3a; f4a2d1918; 4c5dff7e4
evidence: .compozy/tasks/subagents/orchestration/screens/pr/
last_report: docs/qa/reports/2026-10-09-subagents-r2.md
overlaps: RT-subagent-delegate; ET-web-native-subagent; ET-web-session-sidebar-threads; ET-web-session-transcript-calm-grammar
---

Spec: `.compozy/tasks/subagents/_uiux.md` S1–S7 and ADR-005; copy in `COPY.md` §6 "Subagent Terms".
Automated owners: E2E-001…E2E-005 (acpmock "claude" + "codex"), UT-W01…UT-W16, and the subagent
component stories. This scenario is the real-provider walk with visual parity against the boards.

1. **Card (E2E-001, transcript VC-01/VC-02).** In a Claude session, ask for a Codex second opinion.
   Expect `Checked subagent capabilities` (a single call shows no count), then a card with the Codex mark
   (no status dot), the title, and a trail with the spinning running glyph, `Working`, and a ticking
   elapsed; no "Used session spawn" or delegate tool row. While Codex works, line 2 shows its current
   step. On settle the trail reads the filled check glyph, `Done`, and the frozen elapsed in faint ink,
   the title dims, line 2 shows the result's first line, a wake turn renders, and the parent's final
   reply follows. A running card and a done card must never share a glyph or word. A failed delegate call
   (for example an unauthenticated provider) stays a failed tool row with no card.
2. **States.** Drive or seed queued (dashed ring, `Queued`), working with and without progress
   (spinning ring, `Working`), a subagent waiting on its own subagents (delegated ring, `Waiting on
   subagents`), waiting for you (accent dot, `Waiting for you`, child approval), done (check, `Done`),
   idle (a delivered result whose child went quiet: subtle dot, `Idle`), a settled child that resumed
   (back to `Working`), failed (danger ring ×, `Failed`, destructive line 2), canceled and interrupted
   (square, `Canceled` / `Interrupted`), unknown provider mark (bot glyph), and a 72-character title;
   done, idle, canceled and interrupted titles dim in place. Native and delegated rows use the same
   glyphs; compare each against `subagents-transcript.html#tr-card`. After a reconnect,
   a stale card does not tick.
3. **Group (E2E-002, VC-03).** Delegate three in one turn: one group card `3 subagents`, stacked marks,
   the spinning glyph before `3 working`; expanding lists three cards; collapse persists while the window stays open; as they
   settle the summary reads the failed glyph and `2 done · 1 failed` (destructive tone). With a waiting child it adds
   `1 needs you` and the glyph takes the accent attention dot. Four or more show 3 avatars and `+N`. Collapsed and all settled → dimmed.
4. **Hover (E2E-003, VC-04).** Hover a card (opens after ~200 ms) and Tab to it (opens on focus):
   title, provider mark + model (`Not reported` when absent) · effort, fast icon only for fast, status +
   elapsed, preview (progress or result, ≤ 280 chars + `…`). Escape closes it.
5. **Inspect and navigate (E2E-004).** Click a card: a live read-only drawer opens on the right,
   initially 480px wide, without changing the parent, its draft, scroll position, or sidebar selection.
   Resize it, close with Escape, and reopen: width is retained and focus returns to the trigger.
   Below 1440px the drawer overlays the parent. Open it from the sidebar, composer popover, and
   inspector too; each listing shows the recorded provider, agent, model and effort, status and elapsed.
   Selecting another subagent replaces the drawer content. Pending admission shows a waiting state,
   then the transcript when its child exists. Inspect a subagent of that child without nesting drawers.
   `Open as session` navigates to the child with `Subagent of <parent title>` and its composer;
   `Open parent` returns. ⌘/Ctrl-click opens a new window. With the parent deleted the divider reads
   `Subagent of a deleted session` with no action. A plain `session_spawn` child shows no divider.

6. **Waiting banner (E2E-005, composer VC-01/VC-02).** End the parent turn with two delegated subagents
   still running: banner `Waiting on 2 subagents` with both names as buttons (each opens its subagent);
   with more than three, `and N more` opens the inspector roster. `Stop` reads `Stopping…`, cancels
   both, and the banner disappears. Force a stop failure: toast `Could not stop subagents.` Only
   provider-native subagents live → no banner.
7. **Tool phrases and status line (VC-06, composer VC-03).** Status and cancel calls render as
   `Read subagent status` / `Canceled a subagent`, grouped as `… 2 times`; a capabilities call with no
   success reads `Tried to check subagent capabilities`. During a turn the status line reads
   `Working for … · N agents running`, counting live subagents of both origins once.
8. **Composer control (issue #766).** With no subagents, the toolbar shows a quiet subagents icon
   between the context ring and Attach files; its popover reads `Subagents` and
   `Nothing delegated yet.` While two subagents run, the control shows the icon with a pulsing live
   dot and `2`; with one waiting for you the dot takes the accent and the label adds `1 needs you`.
   Open it from the keyboard (Tab, Enter): `Subagents · N running`, the total, failed and live rows
   first, then `Previous subagents (N)` (open when nothing runs); each row shows the provider mark
   without a dot and ends with the state glyph, the word, and elapsed for Working/Done. A row opens its subagent like the
   inspector row; Escape closes the popover and returns focus to the control. Check light and dark
   themes and a narrow window, where the control stays in the toolbar.
9. **Accessibility.** Cards are buttons with `aria-label="Open <title>"` and the status as
   `aria-description`; group disclosure exposes `aria-expanded`; elapsed is text, not color only.
10. **Isolated subagents (agent-collaboration S5, VC-06/VC-07, E2E-004).** Ask the agent to delegate
   with `isolation: "worktree"`. While it runs, line 2 starts with the branch (mono, middle-truncated,
   full name on hover of the branch) and the hover shows only Worktree, Branch and Base. After it
   commits and runs `compozy worktree deliver`, the card shows `#N` with the PR state glyph; clicking
   the link opens the PR in the browser and does not open the child session, while the rest of the
   card still opens it. The hover adds `N ahead · clean` (or `N changed` in warning ink), `#N open`,
   and `Observed … ago`. Without a forge provider the card shows no link and the hover reads
   `PR status unknown`; with a forge and no PR it reads `No pull request`. The inspector roster row
   grows a branch line and carries the same PR link. A shared subagent shows no branch, PR or facts.
11. **Visual parity.** Capture each state with `eng-ui-screenshot` and compare with the cited board
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

Re-walk round 3 2026-10-09 (stock 1d7f690e5): with the prompt sent from the open window's composer, the group card appeared within 5 s of delegation mid-turn and followed both children to `2 done`; prose blocks around the card stay separate paragraphs. Verdict: pass. Report: `docs/qa/reports/2026-10-09-subagents-r2.md`.

QA impact 2026-10-09 (agent-collaboration task 08): isolated subagents add the branch, PR link,
hover worktree facts and roster branch line, and the card's open control became a stretched button
beside the PR link. Stale verdict reset to untested; historical evidence preserved; no QA session ran.

Automation follow-up 2026-10-16 (#751): E2E-004 waits for the card's accessible
`Open <title>` button before both drill-in clicks. A queued placeholder can be
visible before the UI roster confirms its child, even when the API already
returns the child session ID. All navigation assertions in step 5 remain;
verification runs in PR CI. This synchronization change does not change the
scenario's real-provider or visual-parity verdict.

QA impact 2026-10-16 (#766): the composer gains the subagents control and popover (step 8);
rendered evidence for none, previous only, active, mixed, light/dark, and a narrow composer is in
the PR. The inspector roster keeps its section. Real-provider walk pending; verdict unchanged.

## Profile-bound session regression (#783)

From All profiles, operate on a stopped session owned by a nondefault profile. Rename,
archive, and restore it without switching the selected profile; inspect its deep link
and subagent roster. Confirm the request uses the row owner for mutations, the roster
returns successfully, and a terminal 404/410 does not cause recurring control polls.
A failed cold roster page must stop automatic pagination until an explicit retry.
Record the session slice separately from this scenario's broader coverage in
`docs/qa/reports/2026-10-17-issue-783-profile-sessions.md`.

QA impact 2026-10-24 (#947): every roster/list row with a child session opens that child.
A provider-native row instead opens its parent and reveals the matching transcript card,
including nested activity when available. Exercise the sidebar branch, All projects modal,
composer control, and inspector, including repeat clicks while the parent is already open.
The waiting banner continues to list delegated work only. E2E-009 owns sidebar-to-child
navigation; E2E-006 owns native parent revelation.

QA impact 2026-10-27 (subagent status grammar): subagent rows stop reading as "all done". The card,
inspector/popover roster rows and group drop the avatar status dot and carry the session
`StateGlyph` plus a status word in the trail, derived from one display state: `Working` (was
`Running`), `Done` (was `Completed`), and the new `Waiting on subagents` and `Idle` (a delivered
result whose delegated child went quiet; a resumed child reads `Working` again). Settled titles dim;
the group leads its summary with the most urgent member glyph. Owning suites: `subagent-format`
(display-state table) and `subagent-components`. Steps 1–3 and 8 updated; live walk pending.

Drawer inspection steps were verified in the targeted [2026-10-28 run](../reports/2026-10-28-subagent-drawer.md). That run records its bounded coverage and does not re-certify unrelated lifecycle branches.

QA impact 2026-10-29 merge integration: in E2E-004, returning from the child to the
parent must finish hydrating the settled-turn disclosure before opening its cards.
A visible window route alone does not prove that its deferred transcript has loaded.
The journey still checks read-only inspection, draft retention, explicit navigation,
and modifier-click splitting with both sessions visible; no timeouts were increased.

Focused verification: E2E-004 completed the full drawer, parent return and
modifier-click journey on the production bundle (2026-10-29).
