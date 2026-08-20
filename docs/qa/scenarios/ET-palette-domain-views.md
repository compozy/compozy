---
id: ET-palette-domain-views
area: ET
title: Browse any list-bearing domain inside the palette
persona: Sol
journey: J-command-os-from-palette
expected: Every list-bearing domain opens as a palette view with domain-appropriate chips carrying truthful counts, single-select semantics, and one-keystroke clear on zero matches; state badges come from the shared status-tone dictionary and are never color-only. A selected row's detail pane previews metadata and sanitized text without stealing list focus and clears when the row disappears. Form views traverse typed fields in declared order, block invalid submits on the first failing field, and discard values on pop. Grid views navigate in two dimensions with placeholder tiles on failed media. Overflowing lists either scroll everything or state the exact "showing N of M"; a cold-cache open shows loading, never a false empty; vault rows render names and metadata only.
entry_points: Command-K Views group; command palette domain commands (Sessions, Tasks, Loops, Jobs, Agents, Extensions, Marketplace, Vault and peers); marketplace Grid view; vault view
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-palette-nested-views; ET-palette-sessions-view-switch; ET-palette-registry-driven-root
---

Minted by command-palette task 11 (planning): task 06 registered a curated view for every
list-bearing domain and added the Detail, Form, and Grid kinds, but existing scenarios own only the
stack semantics (`ET-palette-nested-views`) and the Sessions exemplar
(`ET-palette-sessions-view-switch`) — the generalized domain grammar and the three new kind bodies
had no owner. Persona Sol: the palette's ARIA combobox pattern, keyboard-only reachability, and
never-color-alone state are contract, not polish. Task 12 owns the first walk.

Walk (task_11 plan):

1. Open the Tasks view — chips show truthful counts with single-select; pick a chip with zero
   matches: the empty state names the filter and one keystroke clears it; state badges pair glyph
   and label (announceable, never color-only).
2. Open a domain never visited this session — a loading state renders, never a blank treated as
   empty; rows arrive without a flash.
3. Select rows with a detail pane — the preview follows selection while keyboard focus stays in
   the list; long content scrolls the pane independently; delete the selected row from another
   surface: the pane clears instead of showing stale content.
4. Open a Form view — ⇥ traverses fields in declared order; submit with a required field empty:
   the first invalid field focuses with its inline message; Esc/⌫-on-empty pops with values
   discarded; a re-push starts clean.
5. Open the Marketplace Grid — ←→↑↓ move across sections; ⏎ and the action panel behave exactly
   as on list rows; a broken image renders a placeholder tile with the title visible.
6. Open the Vault view — names and metadata only; no secret value appears in rows, previews, or
   match highlights.
7. Push one domain view past its mount cap — the view scrolls virtually or states the exact
   overflow.

Expected evidence: screenshots per kind (tasks chips + empty-with-filter, detail pane populated and
cleared, form blocked on first invalid field, grid with placeholder tile, vault names-only), a
screen-reader or keyboard-only pass note for the combobox/2D-navigation contract, and the overflow
note at scale.
