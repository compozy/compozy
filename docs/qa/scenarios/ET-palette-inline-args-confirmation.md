---
id: ET-palette-inline-args-confirmation
area: ET
title: Supply command arguments and confirm destructive execution
persona: Bruno
journey: J-command-os-from-palette
expected: An argument-bearing command replaces palette search with its declared text, password, and dropdown fields. Tab follows field order, invalid or missing values block execution and focus the first failing field, password values stay masked and leave no history or personalization trace, and Escape discards every value and restores search. A declared confirmation names the effect with Cancel focused, ignores the triggering key repeat, refuses an invalidated target, and hands successful or failed asynchronous execution to truthful pending and toast feedback with Retry only when the command is safe to repeat.
entry_points: command palette command row; bound command shortcut; command palette action panel
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-palette-action-panel; ET-palette-registry-driven-root; ET-agent-command-invoke
---

Flagged by command-palette task 04. Task 12 owns the first real-user walk, E2E-014, visual-contract
comparison, and verdict.

Walk (task_11 plan):

1. Select an argument-bearing command — the input bar morphs into its declared fields with
   placeholders; ⇥ traverses in declared order; a dropdown opens with type-to-filter.
2. Press ⏎ with a required field empty — execution blocks and the first empty field focuses; paste
   a type-invalid value — the field shows its message and blocks until fixed.
3. Fill a password-typed field — it renders masked; after execution, confirm no trace of the value
   in recents, query learning, or `compozy cmd-palette personalization show`.
4. Press Esc mid-entry — search restores with every draft discarded; re-selecting starts clean.
5. Invoke the command via its bound chord — the palette opens directly in argument mode.
6. Run a destructive command — the declared confirmation renders naming the target with Cancel
   focused; a held ⏎ from the trigger cannot confirm; Esc returns without executing; invalidate the
   target first (close the window it addressed) — the confirmation refuses with an honest message.
7. Run a slow async command — in-palette pending, then a toast that names success or the failure
   reason; Retry appears only on idempotent-safe failures; a second invoke while one is in flight
   is rejected as already running.

Expected evidence: screenshots of the args bar (pristine, blocked, dropdown), masked password with
the matching personalization-show output, the confirmation step, and the pending/failure toasts;
note the chord used for the direct argument-mode entry.
