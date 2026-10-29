---
id: ET-web-session-sidebar-threads
area: ET
title: In-window sessions sidebar with provenance threads and in-place switch
persona: Bruno
journey: J-14
expected: A session window's topbar shows a List-icon sessions toggle before the goal action; the sidebar starts open as a 264px left rail on the recessed `sunken` surface (a persisted close wins over the default); a `Resize sessions sidebar` separator on its right edge resizes it by drag or arrow keys between 200 and 480 px (never past half the window), double-click resets it to 264 px, dragging past the floor closes it, and the width is one operator preference shared by every session window hosting the shared sessions list (filter, Recent ⇄ All panes, agent groups). Sessions whose lineage.parent_session_id is loaded nest under their root (except `lineage.kind` `continue`/`fork`, which are top-level rows), each hanging off the root's rail by a tree elbow that ends at the last child; the parent row carries a count toggle on its meta line that folds the thread, and a collapsed thread with a failed/waiting/running child shows a danger/warning/accent signal dot. The current session row shares the selected-row tint and carries no accent left bar. Clicking another session switches this window to it in place (URL follows, one history entry); if that session already has its own window, that window is focused instead and no duplicate opens. The footer New session action opens the create flow. Open preference, rail width, and per-thread collapse persist across reloads (localStorage compozy:session:sidebar:v1). Subagent sessions (spawn_role subagent) never appear as rows or roots: the sidebar requests subagents=exclude unless a search is active; a parent row's meta line carries one discreet indicator per kind of descendant — child sessions (elbow icon and count, plus the most urgent child signal while the child list is not showing) and subagents (bot icon; most-urgent glyph attention > failed > running, or delegated when the parent is idle with live subagents; live/total text such as 3/10 while any is live, the total alone when none is live; aria "3 of 10 subagents running" / "6 subagents, 2 failed" / "4 subagents" once all settled cleanly) — and one chevron for the branch fold. With one kind the indicator and chevron are one toggle; with both, the indicators are a segmented control ("Child sessions and subagents") whose selection pill stretches toward the chosen segment while the list below slides left (to subagents) or right (back to child sessions); pressing the selected segment folds the branch like the chevron, and reduced motion switches instantly. Subagent lists start closed and read nothing until opened; an open list shows at most 8 rows per page, ordered by urgency within the loaded rows (waiting for you → failed → working → queued → settled, newest first within each), then "Show 8 more" pages the list route; each row leads with the session StateGlyph in its mark column and ends with the status word (`Working` with a ticking elapsed, `Done` with a frozen faint elapsed, otherwise the word alone: Queued, Waiting on subagents, Waiting for you, Idle, Failed, Canceled, Interrupted), dims the title of a done/idle/canceled/interrupted row, and opens that subagent's session; the subagent list choice persists like thread folds; viewing a subagent reveals only it under its parent row; search includes matching subagents nested under their parent; plain spawn, provenance, and recovery children keep their nesting. Every row has two lines: line 1 is the title plus one end slot (compact time such as `now`, `5m`, `2h`, `1mo`, swapped for the row actions on hover or keyboard focus); line 2 leads with the brand-ink provider mark of its effective runtime (`runtime.effective.provider`), then the agent and attention words, with the branch indicators and chevron right-aligned. A child running its root's agent on its root's provider omits the repeated who line (one line) unless it has an attention word or note, and still names its runtime and agent to assistive tech. When line 2 is full, notes give way first, then the agent name, and attention words last; the row's accessible name and the mark's tooltip read `Provider · Model` in the runtime catalog's display names (e.g. `Codex · GPT-6.1 Sol`, wire ids only where the catalog has no name, neutral fallback glyph for an unknown provider); the glyph is decorative and adds no tab stop, and the tooltip opens on the glyph's hover or the row's keyboard focus (Escape dismisses it); a session that never bound a runtime shows no mark, and the mark follows a runtime change (continue, fork, process replacement) on the next catalog re-read without a per-row request.
entry_points: web session window topbar (session-sidebar-toggle, List icon); SessionSidebar; sessions modal (shared threads); localStorage key compozy:session:sidebar:v1; session-list branch toggle (SessionListBranchToggle) and subagent branch; session inspector Subagents section; GET /api/workspaces/{workspace_id}/sessions?subagents=exclude; docs/design/opendesign/subagents/subagents-navigation.html (VC-01…03)
qa_status: untested
bug_ids: compozy/compozy#416; BUG-20261009-subagent-routes-unavailable
fix_status: fixed
retest_status: pass
fix_commits: ea021855; 49601716; c5105724d
evidence: .compozy/tasks/subagents/orchestration/screens/pr/
last_report: docs/qa/reports/2026-10-09-subagents-r2.md
overlaps: ET-web-sessions-catalog-modal; ET-web-session-thread-full-bleed; ET-web-session-inspector-toggle; ET-web-subagent-card; ET-web-native-subagent
---

Added by the session sidebar + parent provenance feature (2026-08-06), implementing
docs/design/opendesign/session/session-sidebar.html. Flag only — walk in the next QA cycle.

2026-08-06 walked live in lab compozy-session-sidebar-parent-20260806-212647: closed default, PanelLeft toggle, 264px rail, provenance thread (chip=5) nested behind connector, aria-current accent row, chip collapse/expand, in-place switch (URL changed, one window), dedup focus onto the session's existing window (no duplicate), persistence across reload, New session footer, modal shares the same threads. Evidence: journey-log.jsonl + session-sidebar-open-threads.png. Verdict: pass.

2026-08-06 post-walk adjustment: the sidebar toggle icon changed from PanelLeft to the dock's sessions icon (SquareTerminal) so the topbar doesn't show two near-identical panel glyphs. Toggle behavior, testid, and aria are unchanged (component suite re-run green); icon rendering re-verified in the live shell.

2026-08-06 PR 327 CodeRabbit impact: retargeting an already-open session now reconciles its requested route; cyclic lineage remains visible; filtered descendants retain their full ancestor path; collapsed thread and agent bodies are inert. Reset for a targeted live re-walk.

2026-08-06 PR 327 completion: a live root → child → grandchild thread retained its full ancestry under a grandchild-only filter. The thread toggle stayed keyboard-reachable while collapsed children left the accessibility tree, open/collapse preferences survived reload, and selecting an already-open root focused it with two windows still present. The exact differing-route branch and malformed multi-session cycle are public-state edge cases owned by the canonical regression suites. Verdict: pass.

2026-08-06 operator adjustment: the window-level sidebar toggle now uses Lucide List. The dock keeps its own Sessions icon; the topbar control now describes the list it reveals without implying a terminal. Re-verified live with the sidebar open: the List glyph rendered while `aria-pressed` and the accessible label remained truthful.

2026-08-16 issue #416 impact: Loop Goal system sessions now receive informational lineage from the session that started the Loop. Reset for an isolated re-walk that proves the Goal nests under a loaded origin, becomes a visible root when the origin is absent, and keeps neutral child-session copy without changing safe-spawn governance.

2026-08-17 isolated completion walk: the real Sessions list rendered a missing-origin Goal as a
root and its later spawned child beneath the thread connector. The count toggle reported one child
and remained expanded in the accessibility tree. Verdict: pass.

2026-09-11 bulk-actions impact: the shared workspace list now supports checkbox,
modifier/range, and keyboard selection, replacing its toolbar while selection is
active. Thread controls and in-place navigation at rest retain their behavior.
All-workspaces groups remain navigation-only. The new
`ET-web-session-list-bulk-actions` scenario owns selection, catalog membership,
sequential actions, confirmation, and retry; prior thread evidence above is retained.

2026-09-15 quiet-context pass: the current session row dropped its accent left bar; `bg-row-selected` alone marks it, the same tint as multi-selection. Expected updated; the owning unit suite re-verified, no live re-walk.

2026-09-28 session-continue-fork task_04 impact: continue children (lineage kind `continue`) nest under their source like any loaded child; their row subtitle is unchanged, and the session window status line of a continued child adds the neutral origin pill "Continued from {agent}" (fork pill lands with task_06). Reset to `untested` for a re-walk that proves a continued child nests under its source in the rail and the modal, and that selecting it in place shows the pill. Owner of the walk: session-continue-fork task_08.

QA walk 2026-09-28 (task_08 part B2): continued and forked children nest under their source in the window rail (count toggle "7" on the alpha source, "1" on the fork source) and in the catalog modal. Selecting the unopened continued child `sess-3540e32ceffb49f3` switched this window in place (URL followed, window count unchanged) and showed "Continued from alpha". Selecting a child that already has its own window focused it instead. Verdict: pass. Report: `docs/qa/reports/2026-09-28-session-continue-fork-exec-b2.md`.

2026-09-30 shell-rail polish impact (P5): the sessions rail now opens by default in every session window; an operator's persisted toggle (`compozy:session:sidebar:v1`) still wins across reloads. The rail sits on the `sunken` surface with 13.5px row titles and 12px meta, and the pressed topbar toggle reads as the `surface-2` plate (no ring). Pinned by `use-session-sidebar-state.test.ts`. Reset for a live re-walk: open a fresh session window (rail open), close it, reload (rail stays closed).

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

qa-impact: 2026-10-06 (BUG-20261006-derived-child-nested-under-source) — continued and forked sessions no longer nest: they render as
top-level rows while spawned, provenance, and recovery children keep nesting. Flag only.

qa-impact: 2026-10-08 (subagents, ADR-005) — subagent sessions leave the sidebar list; the parent row
gains a subagent chip and the session inspector gains a Subagents roster. Reset from `skipped` to
`untested` for a walk that proves (E2E-009, nav VC-01…03): a parent that delegated 10 subagents shows
one row with chip `10/10` and no subagent rows, and the page of 100 sessions is not consumed by
subagents; hovering the chip lists 5 subagents and `+5 more`; clicking it opens the inspector
Subagents section with 6 rows and `Show N more`, failed rows pinned on top and settled ones under
`Previous subagents (N)`, and `Stop subagent` on a live row (toast `Could not stop subagent` on
failure); after all settle without failure the chip hides; opening one subagent reveals only it under
the parent; a search that matches a subagent shows it nested under its parent; a plain spawned child of
another session still nests with its count toggle; archiving the parent archives its subagents, and a
subagent session offers no archive action. Automated owners: UT-053, UT-W17…UT-W19, IT-029, IT-030,
IT-032.

QA walk 2026-10-09 (subagent parts): on a QA-local patched build subagent sessions stay out of the rail, the chip shows live/total and the failed glyph, hover lists subagents, and click opens the inspector roster; on the stock build no chip renders because `subagent_summary` is missing. Verdict: fail. Report: `docs/qa/reports/2026-10-09-subagents.md`.

Re-walk 2026-10-09 (stock 980d51fbe, subagent parts): subagent sessions stay out of the rail; chip `3/3`, `1/1`, failed glyph with aria `2 subagents, 1 failed`; hover lists subagents; click opens the inspector roster with `Stop subagent` on a hovered live row. Verdict: pass. Report: `docs/qa/reports/2026-10-09-subagents-r2.md`.

qa-impact: 2026-10-16 (#754) — session rows show the provider mark of their effective runtime before
the agent name, in neutral ink, with `Provider · Model` display names as accessible name and tooltip; unbound
sessions show none, and the sub line now spans under the time column. Verified by the owning
`SessionList provider marks` suite and rendered Storybook evidence (`SessionSidebar/MixedRuntimes`,
`MixedRuntimesSelection`, light and dark) in PR for #754; the live walk was not run in that delivery
(heavy QA labs delegated by maintainer instruction). Reset to `untested` for a walk that proves:
a Claude session and a Codex session in one rail show the spark and knot marks; hovering a mark
or Tab-focusing its row shows `Provider · Model` in display names, and Tab moves past the glyph; a freshly created session shows no mark until its first bind; switching a
session's runtime to another provider repaints its row within one catalog re-read; selection mode
covers only the state mark; nested children carry their own provider's mark.

Issue #767 regression (2026-10-16): switch repeatedly between already-open session tabs,
including different worktrees. Initial connection and reconnects shorter than the existing
2-second session transport grace must not show the session-list warning. Hold a real session
stream failure past that grace: the warning appears, then clears immediately when the stream
recovers. Query cancellation on a scope switch must not become a catalog failure. The owning
sidebar hook suite covers the grace boundary; the catalog hook suite covers superseded reads.
Heavy packaged-desktop/E2E execution remains CI-owned for this round.

qa-impact: 2026-10-17 (session row anatomy v2, board `docs/design/opendesign/session-list-density/`) —
line 1 belongs to the title (compact time and on-hover actions share one end slot); owner glyph,
provider mark, agent, attention words, subagent chip, and thread toggle move to line 2, 3px below;
provider marks keep their brand ink; children hang off a tree elbow and collapse to one line when
they repeat their root's agent and provider. Verified by the owning `SessionList provider marks`
suite and rendered Storybook evidence (`SessionSidebar/AllProfiles`, `LongAgentNames`,
`MixedRuntimes`, light and dark) in the PR. Reset to `untested` for a live walk that proves: titles
keep line 1 with All profiles on; hover and Tab reveal the row actions in the time slot; a child on
the root's provider renders one line while a Codex child under a Claude root keeps its mark; a
long agent name truncates before "Needs attention" and the chip/toggle stay whole.

QA impact 2026-10-19 (#825): a just-created session now lands in an existing Session window on the landing desktop (focused, else empty-state, else most recently focused; pinned and other-desktop windows skipped) instead of opening another window. The window choice is owned by ET-web-new-session-reuses-session-window; replay this scenario's create step with a Session window already open.

QA impact 2026-10-19 (issue #843): the open preference and per-thread collapse are now stored per session window (`compozy:session:sidebar:v1`, schema 3, `byScope`) instead of once for every window; a new window starts from the last-used state. Re-walk the reload step with two windows; see ET-web-session-sidebar-per-window.

2026-10-19 issue #836 impact: the rail is now a resizable panel, sized per session window like its open state. The separator
carries the hover/active line, keyboard steps, and a double-click reset; the
width persists in the same `compozy:session:sidebar:v1` blob (a blob written
before widths existed upgrades without losing open or fold state). Collapse,
reopen, and narrow or tiled windows keep the pane at least half the window.
Reset the resize legs for a live re-walk; the focused browser E2E
`Issue 836` in `web/e2e/__tests__/os-shell.spec.ts` owns the drag, clamp,
reload, keyboard, reset, collapse, and narrow-window checks.

qa-impact: 2026-10-23 (#919) — the subagent chip and its hover preview are replaced by in-list
branches: per-kind indicators with one chevron, a segmented control with a travelling pill when a row
has both kinds, slide-left/right list switching, bounded subagent lists (8 rows, "Show 8 more"),
subagent rows that open their session, and a persisted subagent-list choice. Verified by the owning
`SessionList subagents` and `useSessionSidebarState` suites and rendered Storybook evidence
(`SessionSidebar/Branches`: `BranchIndicators`, `SubagentBranchOpen`, `ManySubagents`, `SegmentedSwitch`).
Flag only — a live daemon walk was not run on this machine per operator instruction (heavy local
runs moved to CI); walk it in the next QA cycle.

QA impact 2026-10-23 (merge train): a revealed subagent keeps its own delegated-work control
outside its parent's fold. Open that control and navigate its subagents. During segment travel,
change a live count and confirm the selection plate settles on the new bounds. Disabled branch
reads must not show an indefinite first-page loading state.

QA impact 2026-10-27 (subagent status grammar): open subagent lists no longer render a bare,
same-green dot for running and completed rows. Each row's mark column carries the session
`StateGlyph` (spinning ring for Working, check for Done, attention dot, failed ring, dashed ring for
Queued, subtle dot for Idle, square for Canceled/Interrupted) and its trail the status word; settled
titles dim; rows order by urgency within the loaded page. Owning suite: `SessionList subagents`
("distinct glyphs and words, most urgent first"); fixtures in
`SessionSidebar/Branches` carry a mixed working/waiting/done/failed/idle/queued set. Walk: open a
parent with running and finished subagents and confirm a running row and a done row never share a
glyph or word, and that a waiting or failed row sits above the working ones. Live walk pending.

QA impact 2026-10-28 (#1003): plain clicks on a window sidebar subagent row open its live
read-only drawer and retain the parent's selection and draft. Delegated rows retain explicit
`Open as session` and modifier-click split navigation. Native rows show their attributed activity
in the drawer without a fabricated child session. Rows in the sidebar, transcript, composer roster,
and inspector display recorded provider, agent, model/effort, status and elapsed. Walk selection
replacement, resize and reopen, inspector coexistence, narrow overlay, Escape and focus return.

Drawer inspection steps were verified in the targeted [2026-10-28 run](../reports/2026-10-28-subagent-drawer.md). That run records its bounded coverage and does not re-certify unrelated lifecycle branches.

2026-10-29 merge-train synchronization: E2E-009 waits for the branch's grid-fold
animation to settle before clicking `Show 2 more`. In CI run 40807072482,
Playwright scrolled the still-clipped fold to reach the button; its scrollTop
changed from 19 to 2 during the pointer action, so the click did not expand the
live ten-child roster. The test still asserts eight rows before the click and
ten afterwards, with unchanged timeouts. This synchronization observes only the
fold animation, not the children's continuously spinning Working glyphs.
Focused re-walk: all three repetitions of the unchanged full E2E-009 journey
passed against the isolated daemon (including the 8-to-10 row assertion, drawer
navigation, inspector roster, and live completion). No full local E2E lane ran.

QA impact 2026-10-29 (#1036): when a session's runtime moves to another provider, its row's provider
mark now follows the new binding in every scope, not only after a reload. Session-scoped rereads
(runtime and other session mutations, the open session's live tail, prompt completion) also cover
the All projects aggregate catalog, and an accepted prompt that binds another provider or model
rereads the catalogs when the daemon answers rather than at turn end. Changes from the CLI, a native
tool, or a daemon transition still arrive through the catalog stream. Walked in an isolated lab with
acpmock Claude and Codex providers. Composer switch with a held turn, catalog-stream wake suppressed
(window blur): before the fix the row kept the old mark for the whole turn in All projects and in a
single project; after it, the mark changed within 1s in both. `compozy session runtime set` plus
`session prompt` from the CLI, focused window: the mark changed within 1s in All projects and in a
single project. Owning suites: `session read invalidation` in `query-options.test.ts` and the
catalog-rows cases in `session-chat-runtime-provider.test.tsx`. Verdict for this leg: pass.
