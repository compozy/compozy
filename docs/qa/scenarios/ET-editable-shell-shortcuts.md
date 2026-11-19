---
id: ET-editable-shell-shortcuts
area: ET
title: Keep modifier shortcuts available inside inputs and the terminal
persona: Bruno
journey: J-operate-desktop-shell
expected: Every bound chord with Command, Control or Option runs from an input, the composer or a terminal, plain and Shift-only keys keep typing, text fields keep their editing chords, and unbound terminal chords reach the shell.
entry_points: web desktop keyboard; session composer; Settings inputs; Terminal app
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; .compozy/tasks/herdr-parity/evidence/visual/task_05
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps: ET-web-command-palette-shortcuts
---

Flagged 2026-08-16 for the Herdr parity QA tail after the shell-chord registry migration.

QA 2026-08-16 Herdr parity: The full Web E2E, daemon settings contract suites, and inspected visual bundles covered editable shortcuts, array/range persistence, blocked and shadowed diagnostics, Terminal preset preview/apply/revert, live cheatsheet freshness, and editable-context routing.

## Palette selection continuity

Open the palette from a focused composer while its command catalog is still loading. Open the
action panel with the palette chord again. When recent or pinned commands reorder the results,
the selected command and its action panel must remain available. Removing the selected command
may select its nearest remaining neighbor. The existing Herdr E2E-016 journey owns the real
keyboard flow; the palette hook suite covers asynchronous catalog and ranking arrival.

QA impact 2026-10-18: the same continuity must hold when opening the action panel and refreshing
ranking share one update. Select the row at the operator's action, before the new ordering can
move the automatic highlight; retain normal automatic selection when no action was taken.

qa-impact: 2026-11-18 keyboard-first navigation (#1826). One eligibility policy now decides for text
fields and the terminal. Walk on macOS (and Linux where available):

1. In the composer, a textarea and a Settings input, press Option+H/L, Command+W, Control+Tab and
   Option+; — each runs its command. Type `?`, Shift+A and plain letters — they type.
2. In the same fields, Command+Z undoes text (not the layout), Command+B bolds in the composer, and
   Option+← jumps a word on macOS; on Linux Control+← jumps a word while Option+H still focuses.
3. In the Terminal app on macOS, Option+H/L move focus and Command+K opens the palette; Control+K,
   Control+V and Control+0 reach the shell, and Command+V pastes. On Linux Control+W and Control+1
   reach the shell, while Control+K opens the palette.
4. Hold Option+H at the leftmost window: focus stays put and no ˙ characters appear.
5. With a non-Latin IME composing, Option chords do not fire.

6. With the macOS German input source, press Option+L in the composer and in the Terminal app: `@`
   is typed and focus does not move. Option+L in an empty shell area still focuses right.
