---
id: ET-extension-palette-contributions
area: ET
title: Use extension commands and views in the command palette
persona: Bruno
journey: J-extension-dev-lifecycle
expected: Enabling a valid extension adds its namespaced commands, declarative and programmable views, and free default shortcuts to the selected workspace; each attached client gets an isolated program session, parent frames keep state through navigation, canceled or stale work cannot replace the current frame, conflicts stay dormant with their owner named, unhealthy entries stay visible but unavailable with the runtime reason, disabling removes membership without deleting operator overrides, and a valid dev reload replaces the projection while a broken reload keeps the last good one.
entry_points: extension manifest `resources.cmd_palette`; `view.provider` capability; `palette-fixture-go` + `view-program-ts` fixtures; `compozy extension build|validate|enable|disable|dev|reload`; `compozy extension init --template view-provider-ts`; `compozy cmd-palette list --source ext.<name>`; `compozy cmd-palette inspect <id>`; Command-K; Settings > Extensions > Palette; GET /api/cmd-palette/views/{id} + /stream (HTTP + UDS); POST /api/cmd-palette/views/{id}/open (HTTP + UDS); /api/cmd-palette/view-sessions/{session}/{events,stream} (HTTP + UDS)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-palette-registry-driven-root; ET-palette-nested-views; ET-extension-dev-reload-loop; ET-web-command-palette-shortcuts
---

Task 07 introduced the `resources.cmd_palette` extension family, its workspace projection, the
extension-default shortcut tier, declarative view invocation, and the per-extension Settings panel.
Walk the published and dev-overlay paths with the Go `notes` fixture, including a shortcut collision,
an unhealthy runtime, disable and re-enable, one valid reload, and one invalid reload.

2026-08-19 qa-impact: Task 08 added TypeScript programmable views, per-client sessions, React frame
navigation, cancellation, pushed patches, and bounded degradation. Keep this scenario `untested` and
walk the `view-program-ts` fixture in the Task 12 QA pass.

2026-08-23 qa-impact (Profiles): effective visibility of a contribution is now enablement in this
profile AND a matching placement, evaluated before the projection and included in the catalog
revision — so the same installed extension can contribute in one profile and contribute nothing in
another, and toggling enablement must invalidate the revision rather than wait for a reload.
Already `untested`, so no reset was needed. Extend the walk: after step 1, disable the fixture in
one profile only and confirm its commands, views, aliases, and default chords disappear there while
staying live in the other, that the catalog revision changes, and that a user-authored override on
a contributed command survives as dormant and reactivates on re-enable in that same profile.
`ET-extension-profile-enablement` owns the enablement contract across surfaces; this row owns what
the palette does with it.

Walk (task_11 plan):

1. Enable the Go `notes` fixture — its namespaced commands, declarative view, and free default
   chord appear in ⌘K and in `cmd-palette list --source ext.notes`; a conflicting default stays
   dormant with its owner named in Settings.
2. Open the declarative (Tier-1) view — rows/chips/empty state render from the tool payload under
   the shared stack contract; `GET /api/cmd-palette/views/ext.notes.recent` returns the validated
   envelope.
3. Enable the TS view-program fixture and open its programmable view — typing echoes instantly,
   results refine live, a pushed detail keeps the parent's query and selection on pop.
4. Open the same program view from two attached clients — searching in one never moves the other
   (per-client sessions).
5. Drive the slow-mode fixture past the soft budget (previous rows + busy), the hard ack
   (degraded + retry), and three misses (circuit-broken). Esc, Backspace, the root palette, and a
   built-in view must keep working in both clients.
6. Type faster than a pending round-trip — the echo stays instant and a stale controlled-value
   echo never reverts typing. Force a patch-revision gap: the host requests a full payload instead
   of applying the gap, and the current generation wins.
7. Trigger one host effect, acknowledge it, then reconnect — the effect never repeats. A
   file-picker result returns only to its matching effect id.
8. Kill the fixture subprocess — contributions stay listed, unavailable with the crash reason;
   recovery restores availability; disable removes membership while a user-authored override on its
   command persists dormant and reactivates on re-enable.
9. `compozy extension dev` — edit a command title (projection updates live), then break the
   manifest (last-good stays, the error reaches dev diagnostics); a program edit drops open
   sessions with the "view reloaded" note.
10. Check Settings > Extensions > Palette — contributed commands/views with effective and dormant
    bindings; the unhealthy state grays contributions with the health reason.

Expected evidence: `cmd-palette list --source ext.<name>` transcripts for healthy enable/disable
and re-enable; `cmd-palette list --source ext.<name> --available=false` for the kill/unavailable
capture; screenshots of the dormant-conflict row, the program view in both clients, slow-mode
soft/hard/circuit states, a generation-gap resync, an at-most-once effect, the degraded/reloaded
frames, and the Settings palette panel; dev-diagnostics excerpt for the broken reload.
