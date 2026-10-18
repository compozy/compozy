# Issue 705: nested child runs on the parent run page

## Finding

On `f0036344c`, a step parked on its child run (`awaiting_child`) drew the same
dashed ring as an unreached `pending` step, and nothing on the parent — not the
graph the reporter works in, not Progress, not the node panel — said which step
any child was on, how long it had been there or which inputs it ran with. The
roster already served `child_loop_run_id` on every `awaiting_child` row, so the
Web can read each child through its own routes without a new contract.

## Walks

Two targeted labs (surfaces web, cli, runtime), daemon built from the branch, no
agents or providers. Each lab served `main` and the branch Web against the same
daemon for the before/after.

1. `issue-705-nested-loops`: one parent fanning three `run-loop` (`mode: await`)
   children parked on a durable wait, one paused.
2. `issue-705-nested-depth`: three levels — `nightly-waves.wave` →
   `run-one-wave` (`wave: 2`) → three `fix-one-batch` grandchildren
   (`batch: api|billing|docs`), one paused. This mirrors the reporter's graph.

`main` reproduced the issue in both. The branch showed the delegated glyph, the
child line on the `wave` card, the child and its grandchildren open in the node
panel with their inputs, status, step, park reason, time on step and step count,
and the same rows behind Progress's disclosure.

## Defects found by the walks and fixed before delivery

- `GET /loop-runs/:id` serves a zeroed `progress`; step counts now come from the
  child's briefing, which owns them. The story fixture serves the detail zeroed
  the same way, so the component test proves the route choice.
- Branches of one fanned node all read "fix batch"; such branches now carry their
  item slot, and the inputs that differ between siblings tell them apart.
- The run clock freezes at the last progress, so a stuck child read like a
  healthy one; rows now show time on the current step, which keeps counting.
- The roster does not time a step parked in a durable wait; that time now comes
  from the step's wait cell.

## Verification

- Strict lab auditor: PASS with zero blockers in both labs; teardown clean.
- Targeted local checks: `oxfmt --check`, `oxlint --deny-warnings`, React Doctor
  (the CI version) at 100/100 on the changed web sources, `typecheck` for web and
  `@compozy/ui`, the full loops Vitest system plus the design-system showcase
  (859 tests), and the StateGlyph suite (16 tests).

The full `make gate` and E2E lanes are delegated to the PR's required CI checks at
the user's request. Scenario: `LP-web-run-nested-child-runs`.

## Impact

Web only. No schema, migration, CLI, HTTP/UDS, config, hook or native-tool
change. `@compozy/ui` gains the `delegated` StateGlyph state.
