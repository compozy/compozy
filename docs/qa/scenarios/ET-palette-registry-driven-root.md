---
id: ET-palette-registry-driven-root
area: ET
title: Command palette renders every row from the daemon registry
persona: Bruno
journey: J-command-os-from-palette
expected: Command-K opens instantly against the last-known catalog and lists apps, shell, window, tab, layout and settings commands sourced from the daemon registry with their effective chords; a command unavailable in context stays visible and disabled carrying the runtime's own reason verbatim, a command irrelevant to this surface is absent rather than dead, a capped group states the exact overflow, and the same id, label and chord appear on the palette row, the menubar item, the cheatsheet line and the settings shortcut table.
entry_points: Command-K; menubar palette affordance; menubar Go/Window/Session/Help menus; Help > Keyboard shortcuts; Settings > Layouts > Shortcuts; compozy cmd-palette list
qa_status: untested
bug_ids: BUG-0017; BUG-20260813-desktop-shell-context-order; BUG-20260729-session-window-cross-tab-focus
fix_status: fixed
retest_status: pending
fix_commits: c3c50b6; 531b9f5; 538777e
evidence:
last_report:
overlaps: ET-window-tab-palette-search; ET-web-command-palette-shortcuts; ET-palette-nested-views; ET-palette-personalization-lifecycle
---

Covers the P1 web absorption: one registry projection behind every command surface, availability
resolved against this client's context, and honest degradation when the daemon is cold or
reconnecting. Walk the disabled-with-reason and cross-surface parity paths explicitly — they are the
invariants the projection exists to hold.

2026-08-19 qa-impact: Ranking, personalization, ghost completion, and asynchronous entity sections
changed this root journey. It remains `untested` for the task_12 tail QA walk.

2026-08-20 qa-impact: Palette density/alignment polish — 32px compact rows and a shared 20px left rail. Visual language of the root list changed; keep `untested` until the next QA walk confirms scanability and keyboard row-step.

Walk (task_11 plan):

1. Open ⌘K at rest — Pinned, Recents, and curated groups render instantly; first-run state is never
   an empty pane.
2. Type 2–4 characters — commands, entities, and settings resolve in fixed group order; the ghost
   completion tail preserves typed casing; async entity sections land without stealing selection.
3. Pick one command visible in the palette, the menubar, the cheatsheet, and the settings table —
   confirm identical id, label, and effective chord on all four surfaces.
4. Find one context-unavailable command — its row is disabled with the runtime's verbatim reason;
   the same reason appears on `compozy cmd-palette list --available=false`.
5. Stop the daemon with the palette open — action rows disable with "runtime unavailable",
   availability-exempt commands keep working; restart re-enables rows without reopening.
6. Seed a large group — the group either scrolls fully or states the exact "showing N of M".

Expected evidence: screenshots of rest state, cross-surface parity (palette + menubar + cheatsheet
+ settings), disabled-with-reason row beside the matching CLI listing, and the daemon-stopped
degradation; structured `cmd-palette list` output for the parity and reason checks.
