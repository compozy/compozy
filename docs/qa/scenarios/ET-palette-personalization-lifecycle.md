---
id: ET-palette-personalization-lifecycle
area: ET
title: The palette learns habits and stays correctable per workspace
persona: Bruno
journey: J-command-os-from-palette
expected: Repeated use raises a command in ranking and in the Recents group, a learned query resolves to what was picked for it before, pins hold the top of the rest state in pin order, and identical input with identical history always yields identical order. All of it is workspace-scoped daemon state — another workspace and a second tab see their own truth, never argument or password values — and it is correctable: reset from Settings or CLI returns the root to curated defaults, and the personalization master switch stops recording while keeping existing data until reset.
entry_points: Command-K; command palette action panel Pin/Unpin; Settings > Palette; compozy cmd-palette personalization show|reset; compozy cmd-palette pin|unpin; compozy config get|set cmd_palette.personalization; GET|DELETE /api/cmd-palette/personalization; PUT|DELETE /api/cmd-palette/pins/{id}; GET /api/cmd-palette/rank-signals; POST /api/cmd-palette/usage
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-palette-registry-driven-root; ET-palette-action-panel; ET-agent-palette-config-parity
---

Minted by command-palette task 11 (planning): tasks 03–05 shipped frecency, adaptive query
learning, pins, recents, reset, and the `cmd_palette.personalization` master switch, but no
scenario owned the learning-and-reset lifecycle — `ET-palette-registry-driven-root` walks the rest
state's presence, not its evolution. Task 12 owns the first walk.

Walk (task_11 plan):

1. Execute one command several times, another once — reopen with a short shared query: the heavy
   command ranks first; the Recents group lists both, most recent first, and never lists "open
   palette" itself.
2. Search a distinctive query and pick a mid-ranked result — retype the same prefix later: the
   picked command now carries the learned boost.
3. Pin two commands from the action panel — the Pinned group leads the rest state in pin order;
   `cmd-palette pin` / `unpin` round-trips the same state; double-pin stays idempotent.
4. Repeat the same query twice without new usage — ordering is byte-identical (determinism).
5. Switch workspace — none of the learned ranking, recents, or pins leak; `personalization show`
   reports per-workspace counts.
6. Run a command with a password-typed argument — `personalization show` and the rank-signals
   projection contain the pre-selection query only, never argument or password values.
7. Set `cmd_palette.personalization = false` — recording stops (a new execution changes nothing)
   while existing signals persist; re-enable resumes.
8. Reset from Settings > Palette (scope confirmation) — pins, recents, frecency, and query learning
   clear for this workspace only; the root returns to curated defaults; a second tab reflects the
   reset on next open; `DELETE /api/cmd-palette/personalization` behaves identically.

Expected evidence: before/after screenshots of the rest state (learning, pins, post-reset curated
defaults), `personalization show` transcripts for both workspaces and around the master-switch
toggle, and the rank-signals/usage excerpts proving no argument values are stored.
